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
	"strconv"
	"strings"
	"testing"
	"time"
)

func vcdProject() Project {
	return Project{Name: "Video CD", Format: "vcd", Capacity: "cd74", Groups: []Group{{Tracks: []string{"a.wav"}}}}
}

func TestVCDEstimate(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "a.wav", 10)
	p := vcdProject()
	e, err := s.Estimate(context.Background(), p)
	if err != nil || !e.Fits || e.PCMBytes != 0 || e.CapacityBytes != 333000*2352 || e.EstimatedBytes != (11250+825+300)*2352 || e.WorkspaceBytes <= e.EstimatedBytes {
		t.Fatalf("%+v %v", e, err)
	}
	p.Groups[0].Tracks = []string{"a.wav", "a.wav"}
	e, err = s.Estimate(context.Background(), p)
	if err != nil || e.Duration != 20 || e.EstimatedBytes != (11250+2*(825+300))*2352 {
		t.Fatalf("duplicates: %+v %v", e, err)
	}
	for _, mutate := range []func(*Project){
		func(p *Project) { p.Capacity = "dvd5" },
		func(p *Project) { p.VideoStandard = "secam" },
		func(p *Project) { p.Format = "svcd" },
		func(p *Project) { p.Groups = nil },
		func(p *Project) { p.Groups = append(p.Groups, Group{}) },
		func(p *Project) { p.Groups[0].Tracks = nil },
		func(p *Project) { p.Groups[0].Tracks = make([]string, 99) },
	} {
		p := vcdProject()
		mutate(&p)
		if _, err := s.Estimate(context.Background(), p); err == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
	cacheTrack(t, s, "a.wav", 73*60)
	p = vcdProject()
	if e, err := s.Estimate(context.Background(), p); err != nil || e.Fits {
		t.Fatalf("reserve: %+v %v", e, err)
	}
	if _, err := s.Start(context.Background(), p); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatal(err)
	}
	p.Capacity = "cd80"
	if e, err := s.Estimate(context.Background(), p); err != nil || !e.Fits || e.CapacityBytes != 360000*2352 {
		t.Fatalf("80min: %+v %v", e, err)
	}
	// DVD must continue rejecting silent video, despite the broader library.
	cacheTrack(t, s, "a.wav", 1)
	c := s.cache["a.wav"]
	c.file.HasVideo = true
	c.file.Channels = 0
	s.cache["a.wav"] = c
	if _, err := s.Estimate(context.Background(), testProject()); err == nil {
		t.Fatal("DVD accepted silent video")
	}
	if _, err := s.Estimate(context.Background(), vcdProject()); err != nil {
		t.Fatal(err)
	}
}

func registerTestImage(t *testing.T, s *Engine) vcdImage {
	t.Helper()
	dir := filepath.Join(s.output, "disc-test")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	image := vcdImage{Cue: filepath.Join(dir, "disc.cue"), Bin: filepath.Join(dir, "disc.bin"), Capacity: 333000 * 2352}
	if err := os.WriteFile(image.Cue, []byte("FILE \"disc.bin\" BINARY\n TRACK 01 MODE2/2352\n INDEX 01 00:00:00\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image.Bin, make([]byte, 2352), 0600); err != nil {
		t.Fatal(err)
	}
	image.CueHash, _ = fileHash(context.Background(), image.Cue)
	image.BinHash, _ = fileHash(context.Background(), image.Bin)
	s.images = map[string]vcdImage{"disc-test": image}
	if err := writeJSON(filepath.Join(dir, "job.json"), Job{ID: "disc-test", State: "completed", Kind: "author", Format: "vcd", Output: image.Cue}); err != nil {
		t.Fatal(err)
	}
	return image
}

func TestBurnSafety(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("physical burning is Linux-only")
	}
	s := testStudio(t)
	image := registerTestImage(t, s)
	ctx := context.Background()
	for _, req := range []BurnRequest{
		{JobID: "disc-test", Device: "/dev/sr0", Speed: 4},
		{JobID: "missing", Device: "/dev/sr0", Speed: 4, Confirmed: true},
		{JobID: "disc-test", Device: image.Bin, Speed: 4, Confirmed: true},
		{JobID: "disc-test", Device: "/dev/null", Speed: 4, Confirmed: true},
		{JobID: "disc-test", Device: "/dev/sr0", Speed: 0, Confirmed: true},
		{JobID: "disc-test", Device: "/dev/sr0", Speed: 53, Confirmed: true},
		{JobID: "disc-test", Device: "/dev/sr0", Speed: 4, Dummy: true},
	} {
		if _, err := s.Burn(ctx, req); err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
	if s.Snapshot() != nil {
		t.Fatal("refused burn published job")
	}
	// From here on both the device list AND writer executable are fake.
	s.recorderList = func(context.Context) ([]Recorder, error) {
		return []Recorder{{Device: "/dev/sr-test", Name: "Mock"}}, nil
	}
	fake := filepath.Join(t.TempDir(), "cdrdao")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	s.tools = []Tool{{Name: "cdrdao", Path: fake, Ready: true}}
	req := BurnRequest{JobID: "disc-test", Device: "/dev/sr-test", Speed: 4, Dummy: true, Confirmed: true}
	if _, err := s.Burn(ctx, req); err != nil {
		t.Fatal(err)
	}
	job := waitJob(t, s)
	if job.State != "completed" || job.Kind != "burn" || job.Output != image.Cue {
		t.Fatalf("%+v", job)
	}
	log := strings.Join(job.Log, "\n")
	if !strings.Contains(log, filepath.Dir(image.Cue)) || !strings.Contains(log, "write\n--device\n/dev/sr-test\n--speed\n4\n--simulate\n"+image.Cue) {
		t.Fatal(log)
	}
	for _, name := range []string{"job.json", "burn.json", "source.json"} {
		if _, err := os.Stat(filepath.Join(s.output, job.ID, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(image.Bin); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image.Cue, []byte("FILE arbitrary BINARY"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Burn(ctx, req); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatal(err)
	}
	want := []string{"write", "--device", "/dev/sr-test", "--speed", "4", image.Cue}
	req.Dummy = false
	if !reflect.DeepEqual(burnArgs(req, image.Cue), want) {
		t.Fatal(burnArgs(req, image.Cue))
	}
}

func TestSafeLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, err := openLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := openLock(path); err == nil {
		second.Close()
		t.Fatal("lock was not exclusive")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(path, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink privilege unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if lock, err := openLock(alias); err == nil {
		lock.Close()
		t.Fatal("symlink allowed")
	}
}

func TestBurnCancellationAndWriterExclusion(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("physical burning is Linux-only")
	}
	s := testStudio(t)
	image := registerTestImage(t, s)
	s.recorderList = func(context.Context) ([]Recorder, error) {
		return []Recorder{{Device: "/dev/mock-cancel", Name: "Mock"}}, nil
	}
	fake := filepath.Join(t.TempDir(), "cdrdao")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'mock-writer-started\\n'\nsleep 120\n"), 0700); err != nil {
		t.Fatal(err)
	}
	s.tools = []Tool{{Name: "cdrdao", Path: fake, Ready: true}}
	req := BurnRequest{JobID: "disc-test", Device: "/dev/mock-cancel", Speed: 4, Confirmed: true}
	if _, err := s.Burn(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(strings.Join(s.Snapshot().Log, "\n"), "mock-writer-started") {
		if time.Now().After(deadline) {
			t.Fatal("mock writer did not start")
		}
		time.Sleep(time.Millisecond)
	}
	other := testStudio(t)
	registerTestImage(t, other)
	other.recorderList, other.tools = s.recorderList, s.tools
	if _, err := other.Burn(context.Background(), req); err == nil {
		t.Fatal("same device accepted across output directories")
	}
	if _, err := s.Start(context.Background(), vcdProject()); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	s.Close()
	job := s.Snapshot()
	if job.State != "cancelled" || !strings.Contains(strings.Join(job.Log, "\n"), "ruin") {
		t.Fatalf("%+v", job)
	}
	var saved Job
	data, err := os.ReadFile(filepath.Join(s.output, job.ID, "job.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &saved); err != nil || !reflect.DeepEqual(saved, *job) {
		t.Fatalf("%+v %v", saved, err)
	}
	if _, err := os.Stat(image.Cue); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Burn(context.Background(), req); err != nil {
		t.Fatal("Close retained writer lock:", err)
	}
	other.Close()
}

func TestVCDIntegration(t *testing.T) {
	if os.Getenv("DVDA_INTEGRATION") != "1" && os.Getenv("VCD_INTEGRATION") != "1" {
		t.Skip("set DVDA_INTEGRATION=1 or VCD_INTEGRATION=1")
	}
	for _, standard := range []string{"pal", "ntsc"} {
		t.Run(standard, func(t *testing.T) {
			s := testStudio(t)
			s.tools = discoverTools(filepath.Clean(filepath.Join(s.root, "..")))
			ctx := context.Background()
			for _, input := range []struct {
				name string
				args []string
			}{
				{"a.wav", []string{"-f", "lavfi", "-i", "sine=duration=2"}},
				{"video.mkv", []string{"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25:duration=2", "-c:v", "ffv1", "-an"}},
			} {
				args := append([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, input.args...)
				args = append(args, filepath.Join(s.root, input.name))
				if out, err := exec.CommandContext(ctx, s.toolPath("ffmpeg"), args...).CombinedOutput(); err != nil {
					t.Fatalf("%v %s", err, out)
				}
			}
			p := vcdProject()
			p.VideoStandard = standard
			p.Groups[0].Tracks = []string{"video.mkv", "a.wav", "video.mkv"}
			if _, err := s.Start(ctx, p); err != nil {
				t.Fatal(err)
			}
			job := waitJob(t, s)
			if job.State != "completed" {
				t.Fatalf("%+v\n%s", job, strings.Join(job.Log, "\n"))
			}
			cue, err := os.ReadFile(job.Output)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Count(cue, []byte("TRACK ")) != 4 || !bytes.Contains(cue, []byte("MODE2/2352")) {
				t.Fatalf("%s", cue)
			}
			bin, err := os.ReadFile(filepath.Join(filepath.Dir(job.Output), "disc.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if len(bin)%2352 != 0 || !bytes.Contains(bin, []byte("CD001")) || !bytes.Contains(bin, []byte("VIDEO_CD")) {
				t.Fatal("invalid VCD BIN")
			}
			for _, name := range []string{"track-1.mpg", "track-2.mpg", "track-3.mpg"} {
				out, err := exec.CommandContext(ctx, s.toolPath("ffprobe"), "-v", "error", "-show_streams", "-of", "json", filepath.Join(filepath.Dir(job.Output), name)).Output()
				if err != nil {
					t.Fatal(err)
				}
				var data struct {
					Streams []struct {
						Codec         string `json:"codec_name"`
						Width, Height int
						Rate          string `json:"avg_frame_rate"`
						Sample        string `json:"sample_rate"`
						Channels      int
						BitRate       string `json:"bit_rate"`
					}
				}
				if err := json.Unmarshal(out, &data); err != nil {
					t.Fatal(err)
				}
				if len(data.Streams) != 2 {
					t.Fatalf("%s", out)
				}
				v, a := data.Streams[0], data.Streams[1]
				height, rate := 288, "25/1"
				if standard == "ntsc" {
					height, rate = 240, "30000/1001"
				}
				if v.Codec != "mpeg1video" || v.BitRate != "1150000" || v.Width != 352 || v.Height != height || v.Rate != rate || a.Codec != "mp2" || a.Sample != "44100" || a.Channels != 2 || a.BitRate != "224000" {
					t.Fatalf("%s", out)
				}
			}
			if _, err := s.Burn(ctx, BurnRequest{JobID: job.ID, Device: "/dev/null", Speed: 4, Confirmed: true}); err == nil || errors.Is(err, ErrBusy) {
				t.Fatalf("nonoptical burn: %v", err)
			}
		})
	}
}

func TestVCDMixedDurationIntegration(t *testing.T) {
	if os.Getenv("DVDA_INTEGRATION") != "1" && os.Getenv("VCD_INTEGRATION") != "1" {
		t.Skip("set DVDA_INTEGRATION=1 or VCD_INTEGRATION=1")
	}
	for _, standard := range []string{"pal", "ntsc"} {
		t.Run(standard, func(t *testing.T) {
			s := testStudio(t)
			s.tools = discoverTools(filepath.Clean(filepath.Join(s.root, "..")))
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			p := vcdProject()
			p.VideoStandard = standard
			p.Groups[0].Tracks = nil
			for i, lengths := range [][2]int{{1, 3}, {3, 1}} {
				name := fmt.Sprintf("video%d-audio%d.mkv", lengths[0], lengths[1])
				// No -shortest: the MKV must retain the full longer stream.
				args := []string{"-nostdin", "-hide_banner", "-loglevel", "error",
					"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=320x240:rate=25:duration=%d", lengths[0]),
					"-f", "lavfi", "-i", fmt.Sprintf("sine=duration=%d", lengths[1]),
					"-map", "0:v:0", "-map", "1:a:0", "-c:v", "ffv1", "-c:a", "pcm_s16le", filepath.Join(s.root, name)}
				if out, err := exec.CommandContext(ctx, s.toolPath("ffmpeg"), args...).CombinedOutput(); err != nil {
					t.Fatalf("input %d: %v\n%s", i+1, err, out)
				}
				if f := s.probe(ctx, name); f.Error != "" || math.Abs(f.Duration-3) > 0.01 {
					t.Fatalf("source duration: %+v", f)
				}
				p.Groups[0].Tracks = append(p.Groups[0].Tracks, name)
			}
			if _, err := s.Start(ctx, p); err != nil {
				t.Fatal(err)
			}
			job := waitJob(t, s)
			if job.State != "completed" {
				t.Fatalf("%+v\n%s", job, strings.Join(job.Log, "\n"))
			}
			fps := 25.0
			if standard == "ntsc" {
				fps = 30000.0 / 1001
			}
			for i, name := range p.Groups[0].Tracks {
				t.Run(name, func(t *testing.T) {
					mpeg := filepath.Join(filepath.Dir(job.Output), fmt.Sprintf("track-%d.mpg", i+1))
					out, err := exec.CommandContext(ctx, s.toolPath("ffprobe"), "-v", "error", "-count_frames", "-show_entries", "stream=codec_type,duration,nb_read_frames", "-of", "json", mpeg).Output()
					if err != nil {
						t.Fatal(err)
					}
					var data struct {
						Streams []struct {
							Type     string `json:"codec_type"`
							Duration string `json:"duration"`
							Frames   string `json:"nb_read_frames"`
						}
					}
					if err := json.Unmarshal(out, &data); err != nil {
						t.Fatal(err)
					}
					if len(data.Streams) != 2 {
						t.Fatalf("expected video and audio: %s", out)
					}
					for _, stream := range data.Streams {
						if stream.Type == "video" {
							// MPEG-PS stream duration is inferred from sparse timestamps;
							// decoded frame count measures the actual video duration.
							frames, err := strconv.Atoi(stream.Frames)
							if err != nil || math.Abs(float64(frames)-3*fps) > 1 {
								t.Fatalf("video must contain about 3s of frames: %s", out)
							}
						} else {
							duration, err := strconv.ParseFloat(stream.Duration, 64)
							if err != nil || math.Abs(duration-3) > 2/fps {
								t.Fatalf("%s duration must be about 3s: %s", stream.Type, out)
							}
						}
					}
				})
			}
		})
	}
}
