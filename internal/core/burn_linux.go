//go:build linux

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"dvd-audio-studio/internal/i18n"
)

// Recorders reads kernel metadata only. It never opens a drive or queries media.
func (s *Engine) Recorders(ctx context.Context) ([]Recorder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.recorderList != nil {
		return s.recorderList(ctx)
	}
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil, err
	}
	result := []Recorder{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		base := filepath.Join("/sys/class/block", entry.Name())
		kind, err := os.ReadFile(filepath.Join(base, "device/type"))
		if err != nil || strings.TrimSpace(string(kind)) != "5" {
			continue
		}
		device := filepath.Join("/dev", entry.Name())
		canonical, err := filepath.EvalSymlinks(device)
		if err != nil || canonical != device {
			continue
		}
		info, err := os.Stat(device)
		if err != nil || info.Mode()&os.ModeDevice == 0 || info.Mode()&os.ModeCharDevice != 0 {
			continue
		}
		// Match the actual block node to sysfs, not just its filename.
		dev, err := os.ReadFile(filepath.Join(base, "dev"))
		if err != nil {
			continue
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		major := (st.Rdev>>8)&0xfff | (st.Rdev>>32)&0xfffff000
		minor := st.Rdev&0xff | (st.Rdev>>12)&0xffffff00
		if strings.TrimSpace(string(dev)) != fmt.Sprintf("%d:%d", major, minor) {
			continue
		}
		vendor, _ := os.ReadFile(filepath.Join(base, "device/vendor"))
		model, _ := os.ReadFile(filepath.Join(base, "device/model"))
		name := strings.TrimSpace(strings.TrimSpace(string(vendor)) + " " + strings.TrimSpace(string(model)))
		if name == "" {
			name = entry.Name()
		}
		result = append(result, Recorder{Device: device, Name: name})
	}
	return result, nil
}

func writerLock(device string) (*os.File, error) {
	// A per-user private directory is shared across engine output directories.
	dir := filepath.Join(os.TempDir(), "dvd-audio-studio-writers-"+strconv.Itoa(os.Getuid()))
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 || st.Uid != uint32(os.Getuid()) {
		return nil, os.ErrPermission
	}
	return openLock(filepath.Join(dir, filepath.Base(device)+".lock"))
}

func (s *Engine) Burn(ctx context.Context, request BurnRequest) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, classified(ctx, ErrClosed, "Engine is closed")
	}
	if s.cancel != nil {
		return nil, classified(ctx, ErrBusy, "Another authoring job is running")
	}
	if !request.Confirmed {
		return nil, errorf(ctx, "Explicit confirmation is required even for a simulated burn; cancellation may ruin the disc")
	}
	if request.Speed < 1 || request.Speed > 52 {
		return nil, errorf(ctx, "Burn speed must be between 1 and 52")
	}
	image, ok := s.images[request.JobID]
	if !ok {
		return nil, errorf(ctx, "Burn source must be a completed VCD authored by this engine")
	}
	// A registered image is not sufficient if finalization was cancelled or failed.
	var source Job
	data, err := os.ReadFile(filepath.Join(filepath.Dir(image.Cue), "job.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &source); err != nil || source.State != "completed" || source.Format != "vcd" || source.Kind != "author" || source.ID != request.JobID || source.Output != image.Cue {
		return nil, errorf(ctx, "Burn source must be a completed VCD authored by this engine")
	}
	recorders, err := s.Recorders(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, r := range recorders {
		if r.Device == request.Device {
			found = true
			break
		}
	}
	if !found {
		return nil, errorf(ctx, "Select a real Linux optical recorder; files and device aliases are not allowed")
	}
	if s.toolPath("cdrdao") == "" {
		return nil, classified(ctx, ErrDependencies, "%s is unavailable or incompatible; check tool status", "cdrdao")
	}
	lock, err := openLock(filepath.Join(s.output, ".authoring.lock"))
	if err != nil {
		return nil, classified(ctx, ErrBusy, "Another authoring job is running")
	}
	started := false
	defer func() {
		if !started {
			_ = lock.Close()
		}
	}()
	writer, err := writerLock(request.Device)
	if err != nil {
		return nil, errorf(ctx, "Optical recorder is busy or its lock is unsafe")
	}
	defer func() {
		if !started {
			_ = writer.Close()
		}
	}()
	if err := validateImage(ctx, image); err != nil {
		return nil, err
	}
	for path, want := range map[string]string{image.Cue: image.CueHash, image.Bin: image.BinHash} {
		hash, err := fileHash(ctx, path)
		if err != nil {
			return nil, err
		}
		if hash != want {
			return nil, errorf(ctx, "VCD image is missing, invalid or modified")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(s.output, "burn-")
	if err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(dir, "burn.json"), request); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(dir, "source.json"), image); err != nil {
		return nil, err
	}
	jobCtx, cancel := context.WithCancel(WithLanguage(context.Background(), i18n.Language(ctx)))
	s.cancel, s.lock, s.done = cancel, lock, make(chan struct{})
	s.job = &Job{ID: filepath.Base(dir), State: "running", Format: "vcd", Kind: "burn", Output: image.Cue, Log: []string{text(ctx, "Burn cancellation may ruin the disc. Image files will be retained.")}}
	started = true
	go func() {
		args := burnArgs(request, image.Cue)
		err := s.command(jobCtx, filepath.Dir(image.Cue), s.toolPath("cdrdao"), args...)
		_ = writer.Close()
		s.finishJob(jobCtx, err, dir)
	}()
	return cloneJob(s.job), nil
}
