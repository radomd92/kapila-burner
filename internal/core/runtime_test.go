package core

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dvd-audio-studio/internal/runtimeenv"
)

func TestBundledAuthorIntegration(t *testing.T) {
	if os.Getenv("DVDA_INTEGRATION") != "1" {
		t.Skip("set DVDA_INTEGRATION=1 and DVDA_AUTHOR_RAW to a source-built executable")
	}
	source := os.Getenv("DVDA_AUTHOR_RAW")
	if source == "" {
		t.Skip("set DVDA_AUTHOR_RAW to a source-built executable")
	}
	if runtime.GOOS != "linux" {
		t.Skip("this raw ELF fixture uses system libraries; native package smoke tests cover the other platforms")
	}
	root := filepath.Join(t.TempDir(), "kapila relocated with spaces", strings.Repeat("long-bundle-path-", 6), "kapila-maker-linux-amd64")
	stage := filepath.Dir(filepath.Dir(filepath.Dir(source)))
	ffmpeg := filepath.Join(filepath.Dir(source), "ffmpeg")
	if _, err := os.Stat(ffmpeg); err != nil {
		ffmpeg, err = exec.LookPath("ffmpeg")
		if err != nil {
			t.Fatal(err)
		}
	}
	dataDir := filepath.Join("tools", "share", "dvda-author")
	if err := os.MkdirAll(filepath.Join(root, dataDir), 0700); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ source, target string }{
		{source, filepath.Join("tools", "bin", "dvda-author")},
		{ffmpeg, filepath.Join("tools", "bin", "ffmpeg")},
		{filepath.Join(stage, dataDir), dataDir},
	} {
		if item.target == dataDir {
			if _, err := os.Stat(item.source); os.IsNotExist(err) {
				continue
			}
		}
		err := filepath.WalkDir(item.source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name, err := filepath.Rel(item.source, path)
			if err != nil {
				return err
			}
			target := filepath.Join(root, item.target, name)
			if entry.IsDir() {
				return os.MkdirAll(target, 0700)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0700)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	job := filepath.Join(t.TempDir(), "unrelated job with spaces")
	if err := os.MkdirAll(job, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runtimeenv.ToolDir(root), "dvda-author")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, name := range []string{"audio", "lplex", "lplex-output"} {
		if err := os.Mkdir(filepath.Join(job, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	fullHelp := []string{"-W", "--bindir", filepath.Dir(path), "--datadir", filepath.Join(runtimeenv.ResourceDir(root), "tools", "share", "dvda-author"), "--workdir", job, "-D", filepath.Join(job, "audio"), "--lplex-tempdir", filepath.Join(job, "lplex"), "--lplex-output", filepath.Join(job, "lplex-output"), "-h"}
	for _, dir := range []string{job, os.TempDir()} {
		for _, args := range [][]string{{"--help"}, {"-h"}, {"-W", "--help"}, {"-W", "-h"}, fullHelp} {
			probeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			cmd := exec.CommandContext(probeCtx, path, args...)
			cmd.Dir, cmd.Env = dir, runtimeenv.CommandEnv(root)
			configureProcess(cmd)
			output, err := cmd.CombinedOutput()
			stop()
			if err != nil {
				t.Logf("probe %q in %q: %v\n%s", args, dir, err, output)
			} else {
				t.Logf("probe %q in %q: success (%d output bytes)", args, dir, len(output))
			}
			if (strings.Join(args, " ") == "-W -h" || len(args) == len(fullHelp)) && err != nil {
				t.Errorf("config-independent help failed in %q: %v", dir, err)
			}
		}
	}
	t.Setenv("DVDA_AUTHOR", "")
	var author Tool
	for _, tool := range discoverToolsFor(t.TempDir(), root, runtime.GOOS) {
		if tool.Name == "dvda-author" {
			author = tool
		}
	}
	if !author.Ready || author.bundleRoot != root {
		t.Errorf("relocated author readiness: %+v", author)
	}
	pcm := filepath.Join(job, "input.wav")
	cmd := exec.CommandContext(ctx, filepath.Join(runtimeenv.ToolDir(root), "ffmpeg"), "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-ar", "48000", "-ac", "2", "-c:a", "pcm_s16le", pcm)
	cmd.Dir, cmd.Env = job, runtimeenv.CommandEnv(root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("synthetic WAV: %v\n%s", err, output)
	}
	disc := filepath.Join(job, "disc")
	if err := os.Mkdir(disc, 0700); err != nil {
		t.Fatal(err)
	}
	s := &Engine{job: &Job{}, tools: []Tool{author}}
	if err := s.command(ctx, job, path, "-W", "-P0", "-o", disc, "-g", pcm); err != nil {
		t.Fatalf("relocated authoring: %v\n%s", err, strings.Join(s.job.Log, "\n"))
	}
	for _, name := range []string{"AUDIO_TS.IFO", "ATS_01_0.IFO", "ATS_01_1.AOB"} {
		if info, err := os.Stat(filepath.Join(disc, "AUDIO_TS", name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing/empty %s: %v", name, err)
		}
	}
	t.Log("relocated two-second DVD-Audio authoring produced nonempty IFO/AOB files")
}

func writeRuntimeTool(t *testing.T, path, script string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestBundledAuthorReadiness(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools use POSIX scripts")
	}
	for _, bundled := range []bool{true, false} {
		for _, fail := range []bool{false, true} {
			root := filepath.Join(t.TempDir(), "bundle with spaces")
			path := filepath.Join(runtimeenv.ToolDir(root), "dvda-author")
			script := "[ \"$#\" -eq 2 ] && [ \"$1\" = -W ] && [ \"$2\" = -h ] || exit 9\n"
			if bundled {
				t.Setenv("DVDA_AUTHOR", "")
			} else {
				path = filepath.Join(t.TempDir(), "external-author")
				t.Setenv("DVDA_AUTHOR", path)
				script = "[ \"$#\" -eq 1 ] && [ \"$1\" = --help ] || exit 9\n"
			}
			if fail {
				script += "printf 'dependency failure\\n' >&2\nexit 1\n"
			} else {
				script += "exit 0\n"
			}
			writeRuntimeTool(t, path, script)
			for _, tool := range discoverToolsFor(t.TempDir(), root, runtime.GOOS) {
				if tool.Name == "dvda-author" {
					if tool.Ready == fail || tool.Path != path || (fail && !strings.Contains(tool.Detail, "exit status 1")) {
						t.Fatalf("bundled=%v fail=%v: %+v", bundled, fail, tool)
					}
				}
			}
		}
	}
}

func TestBundledDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools use POSIX scripts")
	}
	for _, layout := range []string{"linux", "darwin", "windows", "darwin-app"} {
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
			for _, key := range []string{"FFMPEG", "FFPROBE", "DVDA_AUTHOR", "MKISOFS", "VCDIMAGER", "CDRDAO", "DVDAUTHOR", "DVD_VIDEO_MKISOFS"} {
				t.Setenv(key, "")
			}
			suffix := ""
			if goos == "windows" {
				suffix = ".exe"
			}
			for _, name := range []string{"ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager"} {
				for _, root := range []string{bundle, app} {
					writeRuntimeTool(t, filepath.Join(runtimeenv.ToolDir(root), name+suffix), "case \"$PATH\" in \"$EXPECTED_BUNDLE\"*) ;; *) exit 3;; esac\nprintf '%s\\n' '-dvd-audio' '-dvd-video'\n")
				}
			}
			check := func(wantRoot string) {
				t.Helper()
				t.Setenv("EXPECTED_BUNDLE", wantRoot)
				tools := discoverToolsFor(app, bundle, goos)
				for _, tool := range tools {
					if tool.Name == "cdrdao" {
						if goos != "linux" || tool.bundleRoot != "" {
							t.Fatalf("unexpected bundled recorder tool: %+v", tool)
						}
						continue
					}
					if tool.Name == "dvdauthor" {
						continue // DVD-Video authoring is an external opt-in dependency.
					}
					name := tool.Name
					if name == "dvd-video-mkisofs" {
						name = "mkisofs"
					}
					if !tool.Ready || tool.Path != filepath.Join(runtimeenv.ToolDir(wantRoot), name+suffix) || tool.bundleRoot != wantRoot {
						t.Fatalf("tool = %+v, bundleRoot = %q", tool, tool.bundleRoot)
					}
				}
			}
			check(bundle)
			if err := os.RemoveAll(runtimeenv.ToolDir(bundle)); err != nil {
				t.Fatal(err)
			}
			if goos != "linux" {
				check(app)
			}
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
