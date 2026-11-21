package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// A burn source is registered only after this engine has completed authoring.
// Hashes also prevent a replaced CUE from directing cdrdao to arbitrary files.
type vcdImage struct {
	Cue      string `json:"cue"`
	Bin      string `json:"bin"`
	CueHash  string `json:"cueHash"`
	BinHash  string `json:"binHash"`
	Capacity int64  `json:"capacityBytes"`
}

func (s *Engine) estimateVCD(ctx context.Context, p Project) (Estimate, error) {
	e := Estimate{}
	if p.VideoStandard != "" && p.VideoStandard != "pal" && p.VideoStandard != "ntsc" {
		return e, errorf(ctx, "VCD video standard must be pal or ntsc")
	}
	switch p.Capacity {
	case "cd74":
		e.CapacityBytes = 333000 * 2352
	case "cd80":
		e.CapacityBytes = 360000 * 2352
	default:
		return e, errorf(ctx, "VCD capacity must be cd74 or cd80")
	}
	if len(p.Groups) != 1 || len(p.Groups[0].Tracks) < 1 || len(p.Groups[0].Tracks) > 98 {
		return e, errorf(ctx, "VCD requires one group with 1-98 sequence tracks")
	}
	// ISO/VCD control track allowance plus 90 seconds of lead-out reserve.
	sectors := float64(4500 + 6750)
	for _, path := range p.Groups[0].Tracks {
		f := s.probe(ctx, path)
		if f.Error != "" {
			if f.Error == text(ctx, "ffprobe is unavailable") {
				return e, classified(ctx, ErrDependencies, "%s is unavailable or incompatible; check tool status", "ffprobe")
			}
			return e, errorf(ctx, "%s: %s", path, f.Error)
		}
		if f.Duration <= 0 || math.IsNaN(f.Duration) || math.IsInf(f.Duration, 0) || f.Duration > 24*3600 {
			return e, errorf(ctx, "Unknown or invalid audio duration or format")
		}
		e.Duration += f.Duration
		// VCD mux is 75 sectors/sec. Round each occurrence separately and
		// allow padding, encoder delay, a 4-second minimum, and track gaps.
		sectors += math.Ceil(math.Max(4, f.Duration+1)*75) + 300
	}
	e.EstimatedBytes = int64(sectors) * 2352
	e.WorkspaceBytes = e.EstimatedBytes + int64(math.Ceil((e.Duration+float64(len(p.Groups[0].Tracks)))*1411200/8)) + 256*1024*1024
	e.Fits = e.EstimatedBytes <= e.CapacityBytes
	return e, nil
}

func fileHash(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Engine) runVCD(ctx context.Context, p Project, e Estimate, dir string) error {
	s.appendLog(text(ctx, "Creating VCD image only; audio-only tracks use a static black background. No disc will be burned automatically."))
	standard, size, rate := "pal-vcd", "352x288", "25"
	if p.VideoStandard == "ntsc" {
		standard, size, rate = "ntsc-vcd", "352x240", "30000/1001"
	}
	args := []string{"-t", "vcd2", "-c", filepath.Join(dir, "disc.cue"), "-b", filepath.Join(dir, "disc.bin")}
	for i, path := range p.Groups[0].Tracks {
		full, err := s.source(ctx, path)
		if err != nil {
			return err
		}
		f := s.probe(ctx, path)
		if f.Error != "" {
			return errorf(ctx, "%s: %s", path, f.Error)
		}
		mpeg := filepath.Join(dir, "track-"+strconv.Itoa(i+1)+".mpg")
		ff := []string{"-nostdin", "-hide_banner", "-loglevel", "warning", "-y", "-i", full}
		video, audio := "0:V:0", "0:a:0"
		if !f.HasVideo {
			ff = append(ff, "-f", "lavfi", "-i", "color=c=black:s="+size+":r="+rate)
			video = "1:v:0"
		} else if f.Channels == 0 {
			ff = append(ff, "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo")
			audio = "1:a:0"
		}
		width, height, sar := "352", "288", "12/11"
		if p.VideoStandard == "ntsc" {
			height = "240"
			sar = "10/11"
		}
		// Fit the display aspect ratio, including anamorphic input, then pad.
		filter := "scale=w='max(2,trunc(min(" + width + "," + height + "*dar/(" + sar + "))/2)*2)':h='max(2,trunc(min(" + height + "," + width + "*(" + sar + ")/dar)/2)*2)',setsar=" + sar + ",pad=" + width + ":" + height + ":(ow-iw)/2:(oh-ih)/2,setdar=4/3,format=yuv420p"
		duration := strconv.FormatFloat(f.Duration, 'f', 6, 64)
		if f.HasVideo {
			// Keep short video alive for longer audio; -t bounds the finite
			// last-frame padding, just as it bounds audio silence from apad.
			filter += ",tpad=stop_mode=clone:stop_duration=" + duration
		}
		ff = append(ff, "-map", video, "-map", audio, "-sn", "-dn", "-map_metadata", "-1", "-vf", filter, "-af", "apad", "-target", standard, "-c:v", "mpeg1video", "-b:v", "1150k", "-c:a", "mp2", "-b:a", "224k", "-ar", "44100", "-ac", "2", "-t", duration, "-shortest", mpeg)
		s.appendLog(text(ctx, "Group %d / track %d: %s", 1, i+1, path))
		if err := s.command(ctx, dir, s.toolPath("ffmpeg"), ff...); err != nil {
			return err
		}
		args = append(args, mpeg)
	}
	if err := s.command(ctx, dir, s.toolPath("vcdimager"), args...); err != nil {
		return err
	}
	image := vcdImage{Cue: filepath.Join(dir, "disc.cue"), Bin: filepath.Join(dir, "disc.bin"), Capacity: e.CapacityBytes}
	if err := validateImage(ctx, image); err != nil {
		return err
	}
	var err error
	if image.CueHash, err = fileHash(ctx, image.Cue); err != nil {
		return err
	}
	if image.BinHash, err = fileHash(ctx, image.Bin); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "image.json"), image); err != nil {
		return err
	}
	s.mu.Lock()
	if s.images == nil {
		s.images = make(map[string]vcdImage)
	}
	s.images[filepath.Base(dir)] = image
	s.job.Output = image.Cue
	s.mu.Unlock()
	s.appendLog(text(ctx, "Completed: ") + image.Cue)
	return nil
}

func validateImage(ctx context.Context, image vcdImage) error {
	for _, path := range []string{image.Cue, image.Bin} {
		canonical, err := filepath.EvalSymlinks(path)
		info, statErr := os.Lstat(path)
		if err != nil || statErr != nil || canonical != path || !info.Mode().IsRegular() || info.Size() == 0 {
			return errorf(ctx, "VCD image is missing, invalid or modified")
		}
		if path == image.Bin && (info.Size()%2352 != 0 || info.Size()+6750*2352 > image.Capacity) {
			return errorf(ctx, "VCD exceeds selected CD capacity")
		}
	}
	return nil
}
