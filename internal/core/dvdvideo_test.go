package core

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDVDVideoBudget(t *testing.T) {
	const dvd5, dvd9 = 4_700_000_000, 8_540_000_000
	short := planDVDVideo(20*60, 1, dvd5)
	dvd5Long := planDVDVideo(4*3600, 2, dvd5)
	dvd9Long := planDVDVideo(4*3600, 2, dvd9)
	for _, e := range []Estimate{short, dvd5Long, dvd9Long} {
		if !e.Fits || e.EstimatedBytes > e.CapacityBytes || e.WorkspaceBytes < 3*e.EstimatedBytes {
			t.Fatalf("invalid budget: %+v", e)
		}
	}
	if short.VideoWidth != 720 || short.VideoBitrate != dvdVideoMaxRate || dvd5Long.VideoWidth != 352 || dvd9Long.VideoWidth != 720 || dvd9Long.VideoBitrate <= dvd5Long.VideoBitrate {
		t.Fatalf("unexpected plans: short=%+v, DVD-5=%+v, DVD-9=%+v", short, dvd5Long, dvd9Long)
	}
	if e := planDVDVideo(20*3600, 1, dvd5); e.Fits || e.VideoBitrate != dvdVideoMinRate || e.EstimatedBytes <= dvd5 {
		t.Fatalf("quality floor not enforced: %+v", e)
	}
}

func TestDVDVideoRejectsAudioOnly(t *testing.T) {
	s := testStudio(t)
	cacheTrack(t, s, "audio.wav", 1)
	p := Project{Name: "Audio only", Format: "dvd-video", Capacity: "dvd5", Groups: []Group{{Tracks: []string{"audio.wav"}}}}
	if _, err := s.Estimate(context.Background(), p); err == nil || !strings.Contains(err.Error(), "requires a video stream") {
		t.Fatalf("audio-only track accepted: %v", err)
	}
}

func TestDVDVideoIntegration(t *testing.T) {
	if os.Getenv("DVD_VIDEO_INTEGRATION") != "1" {
		t.Skip("set DVD_VIDEO_INTEGRATION=1 to run real DVD-Video authoring")
	}
	root := t.TempDir()
	s, err := New(Config{Root: root, Output: filepath.Join(root, "output"), App: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for _, tool := range []string{"ffmpeg", "ffprobe", "dvdauthor", "dvd-video-mkisofs"} {
		if s.toolPath(tool) == "" {
			t.Fatalf("missing %s: %+v", tool, s.tools)
		}
	}
	for _, name := range []string{"silent clip.mpg", "with audio.mpg"} {
		args := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=320x240:r=25:d=2"}
		if name == "with audio.mpg" {
			args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-c:a", "mp2")
		}
		args = append(args, "-c:v", "mpeg1video", "-f", "mpeg", filepath.Join(root, name))
		if out, err := exec.Command(s.toolPath("ffmpeg"), args...).CombinedOutput(); err != nil {
			t.Fatalf("generate clip: %v: %s", err, out)
		}
	}
	for _, profile := range []struct{ standard, capacity string }{{"pal", "dvd5"}, {"ntsc", "dvd9"}} {
		t.Run(profile.standard, func(t *testing.T) {
			width := "720"
			if profile.standard == "pal" && runtime.GOOS == "linux" {
				// Inject a sparse oversize ISO once; the retry must use the real
				// builder at the reduced resolution and publish only that ISO.
				width = "352"
				t.Setenv("REAL_DVD_ISO_BUILDER", s.toolPath("dvd-video-mkisofs"))
				wrapper := filepath.Join(t.TempDir(), "mkisofs")
				writeRuntimeTool(t, wrapper, `if [ ! -e first-attempt ]; then
    : > first-attempt
    while [ "$1" != -o ] && [ "$#" -gt 0 ]; do shift; done
    truncate -s 18000000000 "$2"
else
    exec "$REAL_DVD_ISO_BUILDER" "$@"
fi
`)
				for i := range s.tools {
					if s.tools[i].Name == "dvd-video-mkisofs" {
						original := s.tools[i].Path
						s.tools[i].Path = wrapper
						t.Cleanup(func() { s.tools[i].Path = original })
						break
					}
				}
			}
			p := Project{Format: "dvd-video", VideoStandard: profile.standard, Capacity: profile.capacity, Name: "Video smoke", Groups: []Group{{Tracks: []string{"with audio.mpg", "silent clip.mpg"}}}}
			if _, err := s.Start(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			job := waitJob(t, s)
			if job.State != "completed" || filepath.Base(job.Output) != "dvd-video.iso" {
				t.Fatalf("DVD-Video job: %s: %s\n%s", job.State, job.Error, strings.Join(job.Log, "\n"))
			}
			iso, err := os.Open(job.Output)
			if err != nil {
				t.Fatal(err)
			}
			defer iso.Close()
			header := make([]byte, 64*2048)
			if _, err := io.ReadFull(iso, header); err != nil {
				t.Fatal(err)
			}
			for _, marker := range []string{"CD001", "NSR02"} {
				if !bytes.Contains(header, []byte(marker)) {
					t.Errorf("ISO missing %s", marker)
				}
			}
			vob := filepath.Join(filepath.Dir(job.Output), "disc", "VIDEO_TS", "VTS_01_1.VOB")
			out, err := exec.Command(s.toolPath("ffprobe"), "-v", "error", "-show_entries", "stream=codec_name,width,height", "-of", "csv=p=0", vob).CombinedOutput()
			height := "576"
			if profile.standard == "ntsc" {
				height = "480"
			}
			if err != nil || !bytes.Contains(out, []byte("mpeg2video,"+width+","+height)) || !bytes.Contains(out, []byte("ac3")) {
				t.Fatalf("unexpected DVD-Video streams: %v: %s", err, out)
			}
		})
	}
}
