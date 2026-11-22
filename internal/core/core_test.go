package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func testStudio(t *testing.T) *Engine {
	t.Helper()
	root := t.TempDir()
	s := &Engine{root: root, output: filepath.Join(root, "output"), app: filepath.Join(root, "app"), cache: make(map[string]cachedAudio)}
	for _, dir := range []string{s.output, filepath.Join(s.app, "tools")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(s.Close)
	return s
}

func waitJob(t *testing.T, s *Engine) *Job {
	t.Helper()
	deadline := time.After(2 * time.Minute)
	for {
		job := s.Snapshot()
		if job != nil && job.State != "running" {
			return job
		}
		select {
		case <-deadline:
			s.Close()
			t.Fatal("job did not finish")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func fakeAuthoring(t *testing.T, s *Engine) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell launcher")
	}
	// The job remains active until cancellation, without requiring installed tools.
	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 120\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ffmpeg", "ffprobe", "dvda-author", "mkisofs"} {
		s.tools = append(s.tools, Tool{Name: name, Path: path, Ready: true, Detail: "Ready"})
	}
}

func TestJobContract(t *testing.T) {
	s := testStudio(t)
	if s.Snapshot() != nil {
		t.Fatal("initial job must be nil")
	}
	if !errors.Is(s.Cancel(), ErrNoJob) {
		t.Fatal("expected ErrNoJob")
	}
	cacheTrack(t, s, "a.wav", 1)
	fakeAuthoring(t, s)
	ctx, cancel := context.WithCancel(WithLanguage(context.Background(), "plt"))
	p := testProject()
	job, err := s.Start(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	// Mutations to input and returned snapshots must not affect the running job.
	p.Groups[0].Tracks[0] = "changed.wav"
	job.State = "corrupted"
	deadline := time.Now().Add(5 * time.Second)
	for len(s.Snapshot().Log) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	snapshot := s.Snapshot()
	if snapshot.State != "running" {
		t.Fatalf("request cancellation stopped job: %+v", snapshot)
	}
	if len(snapshot.Log) == 0 || !strings.Contains(snapshot.Log[0], "Manomana") {
		t.Fatalf("job language: %+v", snapshot)
	}
	snapshot.Log[0] = "corrupted"
	if s.Snapshot().Log[0] == "corrupted" {
		t.Fatal("snapshot shares log")
	}
	if _, err := s.Start(context.Background(), testProject()); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy: %v", err)
	}
	// A second engine can scan the same output, but cannot author there.
	other := testStudio(t)
	other.output = s.output
	cacheTrack(t, other, "a.wav", 1)
	fakeAuthoring(t, other)
	if _, err := other.Library(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Start(ctx, testProject()); !errors.Is(err, ErrBusy) || !strings.Contains(err.Error(), "Misy asa") {
		t.Fatalf("output lock: %v", err)
	}
	s.Close()
	final := s.Snapshot()
	if final.State != "cancelled" || !strings.Contains(final.Error, "Nofoanana") {
		t.Fatalf("close: %+v", final)
	}
	var saved Project
	data, err := os.ReadFile(filepath.Join(s.Output(), final.ID, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &saved); err != nil || saved.Groups[0].Tracks[0] != "a.wav" {
		t.Fatalf("project copy: %+v, %v", saved, err)
	}
	var persisted Job
	data, err = os.ReadFile(filepath.Join(s.Output(), final.ID, "job.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &persisted); err != nil || !reflect.DeepEqual(persisted, *final) {
		t.Fatalf("persisted status: %+v, %v", persisted, err)
	}
	if _, err := s.Start(context.Background(), testProject()); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}
	// Close has released the output lock, not just signalled cancellation.
	if _, err := other.Start(context.Background(), testProject()); err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	other.Close()
}

func TestLanguageIsolation(t *testing.T) {
	s := testStudio(t)
	s.tools = []Tool{{Name: "ffprobe", Detail: "Ready", Ready: true}}
	for _, lang := range []string{"en", "plt", "mg"} {
		ctx := WithLanguage(context.Background(), lang)
		_, err := s.Estimate(ctx, Project{})
		want := "Project name"
		ready := "Ready"
		if lang != "en" {
			want, ready = "anaran'ny tetikasa", "Azo ampiasaina"
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: %v", lang, err)
		}
		tools := s.Tools(ctx)
		if tools[0].Detail != ready {
			t.Fatalf("%s: %+v", lang, tools)
		}
		tools[0].Detail = "corrupted"
		if s.Tools(ctx)[0].Detail != ready {
			t.Fatal("Tools returned shared slice")
		}
	}
	s.tools = nil
	for _, lang := range []string{"en", "plt", "en"} {
		f := s.probe(WithLanguage(context.Background(), lang), "/outside.wav")
		want := "Source must"
		if lang == "plt" {
			want = "Tsy maintsy"
		}
		if !strings.Contains(f.Error, want) {
			t.Fatalf("probe %s: %+v", lang, f)
		}
	}
}

func TestConcurrentCallers(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 1)
	fakeAuthoring(t, s)
	var callers sync.WaitGroup
	for i := 0; i < 12; i++ {
		callers.Add(1)
		go func(i int) {
			defer callers.Done()
			lang, want := "en", "Project name"
			if i%2 == 0 {
				lang, want = "plt", "anaran'ny tetikasa"
			}
			ctx := WithLanguage(context.Background(), lang)
			for n := 0; n < 10; n++ {
				_, err := s.Estimate(ctx, Project{})
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s: %v", lang, err)
				}
				s.Tools(ctx)[0].Detail = "caller-owned"
				if _, err := s.Library(ctx); err != nil {
					t.Error(err)
				}
			}
		}(i)
	}
	callers.Wait()
	if _, err := s.Start(context.Background(), testProject()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		callers.Add(1)
		go func() { defer callers.Done(); s.Close() }()
	}
	callers.Wait()
	if s.Snapshot().State != "cancelled" {
		t.Fatal("concurrent Close did not await cancellation")
	}
}

func TestCancelExitedLauncher(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 1)
	fakeAuthoring(t, s)
	// The parent exits, but a descendant retains stdout. Cancellation must still
	// kill the process group and unblock the log reader before Close returns.
	if err := os.WriteFile(s.tools[0].Path, []byte("#!/bin/sh\nsleep 120 &\necho launcher-exited\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(context.Background(), testProject()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(strings.Join(s.Snapshot().Log, "\n"), "launcher-exited") {
		if time.Now().After(deadline) {
			t.Fatal("launcher did not start")
		}
		time.Sleep(time.Millisecond)
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked on an exited launcher's child")
	}
}

func TestStartGates(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 1)
	if _, err := s.Start(context.Background(), testProject()); !errors.Is(err, ErrDependencies) {
		t.Fatalf("dependencies: %v", err)
	}
	cacheTrack(t, s, "a.wav", 15000)
	p := testProject()
	p.Groups[0].Tracks = []string{"a.wav", "a.wav"}
	if _, err := s.Start(context.Background(), p); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("capacity: %v", err)
	}
	if s.Snapshot() != nil {
		t.Fatal("rejected job published a snapshot")
	}
}

func TestNewCanonicalPaths(t *testing.T) {
	root, app := t.TempDir(), t.TempDir()
	alias := filepath.Join(t.TempDir(), "library")
	if err := os.Symlink(root, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink privilege unavailable: %v", err)
		}
		t.Fatal(err)
	}
	s, err := New(Config{Root: alias, Output: filepath.Join(root, "out"), App: app})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Root() != root || s.Output() != filepath.Join(root, "out") {
		t.Fatalf("noncanonical paths: %s, %s", s.Root(), s.Output())
	}
	for _, config := range []Config{
		{Root: root, Output: root, App: app},
		{Root: root, Output: filepath.Dir(root), App: app},
		{Root: "", Output: filepath.Join(root, "out"), App: app},
		{Root: root, Output: "", App: app},
		{Root: root, Output: filepath.Join(root, "out"), App: filepath.Join(app, "missing")},
	} {
		if engine, err := New(config); err == nil {
			engine.Close()
			t.Fatalf("invalid config accepted: %+v", config)
		}
	}
}

func cacheTrack(t *testing.T, s *Engine, path string, duration float64) {
	t.Helper()
	full := filepath.Join(s.root, path)
	if err := os.WriteFile(full, []byte("dummy audio"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	s.cache[path] = cachedAudio{size: info.Size(), modified: info.ModTime(), file: AudioFile{Path: path, Duration: duration, Channels: 2, SampleRate: 44100}}
}

func testProject() Project {
	return Project{Name: "Test disc", SampleRate: 48000, Bits: 16, Channels: 2, Capacity: "dvd5", Groups: []Group{{Name: "First", Tracks: []string{"a.wav"}}}}
}

func TestEstimateFormulas(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 1.000001)
	cacheTrack(t, s, "b.wav", 2.5)
	for _, iso := range []bool{false, true} {
		for _, capacity := range []string{"dvd5", "dvd9"} {
			t.Run(fmt.Sprintf("iso=%t/%s", iso, capacity), func(t *testing.T) {
				p := testProject()
				p.ISO, p.Capacity = iso, capacity
				p.Groups = []Group{{Tracks: []string{"a.wav", "b.wav"}}, {Tracks: []string{"a.wav"}}}
				e, err := s.Estimate(context.Background(), p)
				if err != nil {
					t.Fatal(err)
				}
				// Each occurrence is rounded independently, including duplicate tracks.
				pcm := int64(192001*2 + 480000)
				disc := int64(float64(pcm)*1.08) + 3*1024*1024 + 16*1024*1024
				workspace := pcm + disc + 256*1024*1024
				if iso {
					workspace += disc
				}
				capBytes := int64(4700000000)
				if capacity == "dvd9" {
					capBytes = 8547991552
				}
				want := Estimate{Duration: 4.500002, PCMBytes: pcm, EstimatedBytes: disc, WorkspaceBytes: workspace, CapacityBytes: capBytes, Fits: true}
				if math.Abs(e.Duration-want.Duration) > 1e-9 {
					t.Fatalf("duration = %v, want %v", e.Duration, want.Duration)
				}
				e.Duration = want.Duration
				if e != want {
					t.Fatalf("Estimate = %+v, want %+v", e, want)
				}
			})
		}
	}
}

func TestEstimateValidation(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 1)
	for _, tc := range []struct {
		name, message string
		change        func(*Project)
	}{
		{"blank name", "Project name", func(p *Project) { p.Name = " \t" }},
		{"long name", "Project name", func(p *Project) { p.Name = strings.Repeat("x", 201) }},
		{"bits", "16-bit or 24-bit", func(p *Project) { p.Bits = 32 }},
		{"channels", "mono or stereo", func(p *Project) { p.Channels = 6 }},
		{"rate", "Unsupported", func(p *Project) { p.SampleRate = 22050 }},
		{"capacity", "dvd5 or dvd9", func(p *Project) { p.Capacity = "bluray" }},
		{"no groups", "1-9 groups", func(p *Project) { p.Groups = nil }},
		{"ten groups", "1-9 groups", func(p *Project) { p.Groups = make([]Group, 10) }},
		{"empty groups", "at least one", func(p *Project) { p.Groups = []Group{{}, {}} }},
		{"hundred tracks", "at most 99", func(p *Project) { p.Groups[0].Tracks = make([]string, 100) }},
		{"missing source", "missing.wav", func(p *Project) { p.Groups[0].Tracks = []string{"missing.wav"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testProject()
			tc.change(&p)
			if _, err := s.Estimate(context.Background(), p); err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error = %v, want %q", err, tc.message)
			}
		})
	}
	for _, rate := range []int{44100, 48000, 88200, 96000, 176400, 192000} {
		for _, bits := range []int{16, 24} {
			for _, channels := range []int{1, 2} {
				p := testProject()
				p.SampleRate, p.Bits, p.Channels = rate, bits, channels
				p.Groups = make([]Group, 9)
				for i := 0; i < 99; i++ {
					p.Groups[8].Tracks = append(p.Groups[8].Tracks, "a.wav")
				}
				e, err := s.Estimate(context.Background(), p)
				if err != nil || e.Duration != 99 || e.PCMBytes != int64(99*rate*bits/8*channels) {
					t.Fatalf("profile %d/%d/%d: %+v, %v", rate, bits, channels, e, err)
				}
			}
		}
	}
}

func TestEstimateWAVLimitAndCapacity(t *testing.T) {
	s := testStudio(t)
	p := testProject()
	const bytesPerSecond = 48000 * 2 * 2
	seconds := float64((0xffffffff - 4096) / bytesPerSecond)
	cacheTrack(t, s, "a.wav", seconds)
	if _, err := s.Estimate(context.Background(), p); err != nil {
		t.Fatalf("below WAV limit: %v", err)
	}
	cacheTrack(t, s, "a.wav", seconds+1)
	if _, err := s.Estimate(context.Background(), p); err == nil || !strings.Contains(err.Error(), "4 GiB WAV limit") {
		t.Fatalf("above WAV limit: %v", err)
	}
	cacheTrack(t, s, "a.wav", 15000)
	p.Groups[0].Tracks = []string{"a.wav", "a.wav"}
	e, err := s.Estimate(context.Background(), p)
	if err != nil || e.Fits || e.Duration != 30000 {
		t.Fatalf("over DVD5 capacity: %+v, %v", e, err)
	}
	p.Capacity = "dvd9"
	if e, err = s.Estimate(context.Background(), p); err != nil || !e.Fits {
		t.Fatalf("within DVD9 capacity: %+v, %v", e, err)
	}
}

func TestScanFolderScopesMediaToSelectedDirectory(t *testing.T) {
	s := testStudio(t)
	for _, name := range []string{"outside.wav", "selected/nested/track.wav", "selected/.hidden/skip.wav"} {
		path := filepath.Join(s.root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		cacheTrack(t, s, name, 1)
	}
	ctx := context.Background()
	for _, folder := range []string{filepath.Join("selected", "nested"), filepath.Join(s.root, "selected")} {
		files, err := s.ScanFolder(ctx, folder)
		if err != nil || len(files) != 1 || files[0].Path != "selected/nested/track.wav" || files[0].Error != "" {
			t.Fatalf("ScanFolder(%q) = %+v, %v", folder, files, err)
		}
	}
	for _, folder := range []string{"..", "outside.wav", "missing", filepath.Join("selected", ".hidden"), s.output, s.app} {
		if _, err := s.ScanFolder(ctx, folder); err == nil {
			t.Errorf("ScanFolder(%q) unexpectedly succeeded", folder)
		}
	}
	link := filepath.Join(s.root, "linked")
	if err := os.Symlink(filepath.Dir(s.root), link); err == nil {
		if _, err := s.ScanFolder(ctx, filepath.Join(link, filepath.Base(s.root))); err == nil {
			t.Error("ScanFolder followed a symlinked ancestor")
		}
	} else if runtime.GOOS != "windows" {
		t.Fatal(err)
	}
}

func TestSourceRestrictions(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 1)
	outside := filepath.Join(t.TempDir(), "outside.wav")
	for _, path := range []string{outside, filepath.Join(s.output, "out.wav"), filepath.Join(s.app, "tools", "tool.wav")} {
		if err := os.WriteFile(path, []byte("dummy"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{"inside.wav": filepath.Join(s.root, "a.wav"), "outside.wav": outside, "output.wav": s.output, "tools.wav": filepath.Join(s.app, "tools")} {
		if err := os.Symlink(target, filepath.Join(s.root, name)); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlink privilege unavailable: %v", err)
			}
			t.Fatal(err)
		}
	}
	for _, path := range []string{"a.wav", "inside.wav"} {
		if full, err := s.source(context.Background(), path); err != nil || full != filepath.Join(s.root, "a.wav") {
			t.Errorf("source(%q) = %q, %v", path, full, err)
		}
	}
	relOutside, err := filepath.Rel(s.root, outside)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", outside, relOutside, "missing.wav", ".", "outside.wav", "output/out.wav", "app/tools/tool.wav", "output.wav/out.wav", "tools.wav/tool.wav"} {
		if full, err := s.source(context.Background(), path); err == nil {
			t.Errorf("source(%q) unexpectedly allowed %q", path, full)
		}
	}
	// A cache entry must not bypass source validation or stale metadata checks.
	s.cache["outside.wav"] = s.cache["a.wav"]
	if f := s.probe(context.Background(), "outside.wav"); f.Error == "" {
		t.Error("cached symlink escape was accepted")
	}
	if err := os.WriteFile(filepath.Join(s.root, "a.wav"), []byte("changed size"), 0600); err != nil {
		t.Fatal(err)
	}
	if f := s.probe(context.Background(), "a.wav"); f.Error != "ffprobe is unavailable" {
		t.Fatalf("stale cache was used: %+v", f)
	}
}

func TestIntegration(t *testing.T) {
	if os.Getenv("DVDA_INTEGRATION") != "1" {
		t.Skip("set DVDA_INTEGRATION=1 to run installed authoring tools")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd = filepath.Clean(filepath.Join(cwd, "../.."))
	tools := discoverTools(cwd)
	for _, tool := range tools {
		switch tool.Name {
		case "ffmpeg", "ffprobe", "dvda-author", "mkisofs":
			if !tool.Ready {
				t.Fatalf("required tool %s unavailable: %s (%s)", tool.Name, tool.Detail, tool.Path)
			}
		}
	}
	for _, profile := range []struct {
		name                 string
		rate, bits, channels int
	}{
		{"stereo44100_16", 44100, 16, 2},
		{"mono48000_16", 48000, 16, 1},
		{"stereo96000_24", 96000, 24, 2},
	} {
		t.Run(profile.name, func(t *testing.T) {
			root := t.TempDir()
			s, err := New(Config{Root: root, Output: filepath.Join(root, "output"), App: cwd})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(s.Close)
			ctx, timeout := context.WithTimeout(context.Background(), 2*time.Minute)
			defer timeout()
			for i, name := range []string{"a.wav", "b.wav"} {
				cmd := exec.CommandContext(ctx, s.toolPath("ffmpeg"), "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:duration=1", 440+i*220), "-c:a", "pcm_s16le", filepath.Join(s.root, name))
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("synthetic WAV: %v\n%s", err, output)
				}
			}
			p := testProject()
			p.SampleRate, p.Bits, p.Channels, p.ISO = profile.rate, profile.bits, profile.channels, true
			p.Groups = []Group{{Name: "Reverse", Tracks: []string{"b.wav", "a.wav"}}, {Name: "Repeat", Tracks: []string{"b.wav"}}}
			e, err := s.Estimate(ctx, p)
			if err != nil || !e.Fits || math.Abs(e.Duration-3) > 0.01 {
				t.Fatalf("Estimate: %+v, %v", e, err)
			}
			started, err := s.Start(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(s.Close)
			final := waitJob(t, s)
			if final.State != "completed" {
				t.Fatalf("job: %+v\n%s", final, strings.Join(final.Log, "\n"))
			}
			dir := filepath.Join(s.Output(), started.ID)
			for _, name := range []string{"AUDIO_TS.IFO", "AUDIO_TS.BUP", "ATS_01_0.IFO", "ATS_02_0.IFO", "ATS_01_1.AOB", "ATS_02_1.AOB"} {
				info, err := os.Stat(filepath.Join(dir, "disc", "AUDIO_TS", name))
				if err != nil || info.Size() == 0 {
					t.Fatalf("missing/empty %s: %v", name, err)
				}
			}
			var saved Project
			data, err := os.ReadFile(filepath.Join(dir, "project.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &saved); err != nil || !reflect.DeepEqual(saved, p) {
				t.Fatalf("saved Project/order: %+v, %v", saved, err)
			}
			log := strings.Join(final.Log, "\n")
			last := -1
			for _, line := range []string{"Group 1 / track 1: b.wav", "Group 1 / track 2: a.wav", "Group 2 / track 1: b.wav"} {
				index := strings.Index(log, line)
				if index <= last {
					t.Fatalf("track order missing/incorrect: %s\n%s", line, log)
				}
				last = index
			}
			if final.Output != filepath.Join(dir, "dvd-audio.iso") {
				t.Fatalf("unexpected ISO output: %s", final.Output)
			}
			iso, err := os.Open(final.Output)
			if err != nil {
				t.Fatal(err)
			}
			defer iso.Close()
			// UDF volume recognition descriptors follow the ISO9660 descriptors.
			header := make([]byte, 64*2048)
			if _, err := iso.ReadAt(header, 0); err != nil {
				t.Fatal(err)
			}
			for _, magic := range []string{"CD001", "BEA01", "NSR02", "TEA01"} {
				if !bytes.Contains(header[16*2048:], []byte(magic)) {
					t.Errorf("ISO missing volume descriptor %s", magic)
				}
			}
			var savedJob Job
			data, err = os.ReadFile(filepath.Join(dir, "job.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &savedJob); err != nil || savedJob.State != "completed" || savedJob.Output != final.Output {
				t.Fatalf("saved Job: %+v, %v", savedJob, err)
			}
		})
	}
}
