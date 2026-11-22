package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dvd-audio-studio/internal/runtimeenv"
)

func writeRuntimeTool(t *testing.T, path, script string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestBundledDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools use POSIX scripts")
	}
	for _, layout := range []string{"darwin", "windows", "darwin-app"} {
		t.Run(layout, func(t *testing.T) {
			goos := strings.TrimSuffix(layout, "-app")
			bundle, app := t.TempDir(), t.TempDir()
			if layout == "darwin-app" {
				bundle, app = filepath.Join(bundle, "Bundled.app", "Contents"), filepath.Join(app, "App.app", "Contents")
				for _, root := range []string{bundle, app} {
					for _, name := range []string{"Helpers", "Resources"} {
						if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			t.Setenv("PATH", "")
			for _, key := range []string{"FFMPEG", "FFPROBE", "DVDA_AUTHOR", "MKISOFS", "VCDIMAGER", "CDRDAO"} {
				t.Setenv(key, "")
			}
			suffix := ""
			if goos == "windows" {
				suffix = ".exe"
			}
			for _, name := range []string{"ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager"} {
				for _, root := range []string{bundle, app} {
					writeRuntimeTool(t, filepath.Join(runtimeenv.ToolDir(root), name+suffix), "case \"$PATH\" in \"$EXPECTED_BUNDLE\"*) ;; *) exit 3;; esac\nprintf '%s\\n' '-dvd-audio'\n")
				}
			}
			check := func(wantRoot string) {
				t.Helper()
				t.Setenv("EXPECTED_BUNDLE", wantRoot)
				for _, tool := range discoverToolsFor(app, bundle, goos) {
					if tool.Name == "cdrdao" {
						continue
					}
					if !tool.Ready || tool.Path != filepath.Join(runtimeenv.ToolDir(wantRoot), tool.Name+suffix) || tool.bundleRoot != wantRoot {
						t.Fatalf("tool = %+v, bundleRoot = %q", tool, tool.bundleRoot)
					}
				}
			}
			check(bundle)
			if err := os.RemoveAll(runtimeenv.ToolDir(bundle)); err != nil {
				t.Fatal(err)
			}
			check(app)
			custom := filepath.Join(t.TempDir(), "custom")
			writeRuntimeTool(t, custom, "exit 0\n")
			t.Setenv("FFMPEG", custom)
			tool := discoverToolsFor(app, bundle, goos)[0]
			if !tool.Ready || tool.Path != custom || tool.bundleRoot != "" {
				t.Fatalf("explicit override = %+v", tool)
			}
			t.Setenv("FFMPEG", custom+"-missing")
			if discoverToolsFor(app, bundle, goos)[0].Ready {
				t.Fatal("invalid override must not silently fall back")
			}
			t.Setenv("FFMPEG", "")
			if err := os.Remove(filepath.Join(runtimeenv.ToolDir(app), "ffmpeg"+suffix)); err != nil {
				t.Fatal(err)
			}
			pathDir := t.TempDir()
			fallback := filepath.Join(pathDir, "ffmpeg")
			writeRuntimeTool(t, fallback, "exit 0\n")
			t.Setenv("PATH", pathDir)
			tool = discoverToolsFor(app, bundle, goos)[0]
			if !tool.Ready || tool.Path != fallback || tool.bundleRoot != "" {
				t.Fatalf("PATH fallback = %+v", tool)
			}
		})
	}
}

func TestBundledAuthorCommand(t *testing.T) {
	for _, layout := range []string{"flat", "app"} {
		t.Run(layout, func(t *testing.T) { testBundledAuthorCommand(t, layout == "app") })
	}
}

func testBundledAuthorCommand(t *testing.T, app bool) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tool uses a POSIX script")
	}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			root, job := t.TempDir(), t.TempDir()
			if app {
				root = filepath.Join(root, "Kapila Burner.app", "Contents")
				for _, name := range []string{"Helpers", "Resources"} {
					if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			path := filepath.Join(runtimeenv.ToolDir(root), "dvda-author")
			data := filepath.Join(runtimeenv.ResourceDir(root), "tools", "share", "dvda-author")
			if err := os.MkdirAll(data, 0700); err != nil {
				t.Fatal(err)
			}
			disc, pcm := filepath.Join(job, "disc"), filepath.Join(job, "input.wav")
			if err := os.Mkdir(disc, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pcm, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			code := "0"
			if fail {
				code = "1"
			}
			writeRuntimeTool(t, path, `printf '%s\n' "$@" > "$TEST_JOB/args"
case "$PATH" in "$TEST_BUNDLE"*) ;; *) exit 10;; esac
while [ "$#" -gt 0 ]; do
  case "$1" in
    --bindir|--datadir|--workdir|-D|--lplex-tempdir|--lplex-output|-o)
      shift
      [ -d "$1" ] || exit 11
      ;;
  esac
  shift
done
exit `+code+"\n")
			t.Setenv("TEST_JOB", job)
			t.Setenv("TEST_BUNDLE", root)
			s := &Engine{job: &Job{}, tools: []Tool{{Name: "dvda-author", Path: path, Ready: true, bundleRoot: root}}}
			err := s.command(context.Background(), job, path, "-W", "-P0", "-o", disc, "-g", pcm)
			if (err != nil) != fail {
				t.Fatalf("command error: %v", err)
			}
			args, err := os.ReadFile(filepath.Join(job, "args"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(args)), "\n")
			if len(lines) != 19 || lines[0] != "-W" || lines[1] != "--bindir" || lines[2] != filepath.Dir(path) || lines[3] != "--datadir" || lines[4] != data || lines[5] != "--workdir" || lines[6] != job || lines[7] != "-D" || lines[9] != "--lplex-tempdir" || lines[11] != "--lplex-output" || strings.Join(lines[13:], "|") != strings.Join([]string{"-W", "-P0", "-o", disc, "-g", pcm}, "|") {
				t.Fatalf("unexpected arguments: %q", lines)
			}
			for _, index := range []int{8, 10, 12} {
				if _, err := os.Stat(lines[index]); !os.IsNotExist(err) {
					t.Fatalf("scratch directory not removed: %s (%v)", lines[index], err)
				}
			}
			contents, err := os.ReadFile(pcm)
			if err != nil || string(contents) != "unchanged" {
				t.Fatalf("input modified: %q, %v", contents, err)
			}
		})
	}
}

func TestRuntimeDirectoriesNotSources(t *testing.T) {
	for _, layout := range []string{"flat", "app"} {
		t.Run(layout, func(t *testing.T) { testRuntimeDirectoriesNotSources(t, layout == "app") })
	}
}

func testRuntimeDirectoriesNotSources(t *testing.T, app bool) {
	root := t.TempDir()
	names := []string{"tools", "lib", "share", "etc"}
	if app {
		root = filepath.Join(root, "Kapila Burner.app", "Contents")
		names = []string{"Helpers", "Frameworks", "Resources", "MacOS"}
		for _, name := range names {
			if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := &Engine{root: root, app: root, output: filepath.Join(root, "output")}
	for _, name := range names {
		path := filepath.Join(root, name, "secret.wav")
		writeRuntimeTool(t, path, "")
		if _, err := s.source(context.Background(), filepath.Join(name, "secret.wav")); err == nil {
			t.Fatalf("runtime file accepted: %s", path)
		}
	}
	files, err := s.Library(context.Background())
	if err != nil || len(files) != 0 {
		t.Fatalf("runtime files scanned: %+v, %v", files, err)
	}
}
