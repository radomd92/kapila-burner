//go:build linux

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxBundledToolPayload(t *testing.T) {
	for _, name := range []string{"dvda-author", "mkisofs"} {
		t.Run(name, func(t *testing.T) {
			app, pathDir := t.TempDir(), t.TempDir()
			for _, env := range []string{"FFMPEG", "FFPROBE", "VCDIMAGER", "CDRDAO"} {
				t.Setenv(env, filepath.Join(app, "missing"))
			}
			t.Setenv("DVDA_AUTHOR", "")
			t.Setenv("MKISOFS", "")
			t.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			local := filepath.Join(app, "tools", "bin", name)
			if err := os.MkdirAll(filepath.Dir(local), 0700); err != nil {
				t.Fatal(err)
			}
			wrapper, err := os.ReadFile(filepath.Join("..", "..", "tools", "bin", name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(local, wrapper, 0700); err != nil {
				t.Fatal(err)
			}
			installed := filepath.Join(pathDir, name)
			fake := []byte("#!/bin/sh\nprintf '%s\\n' '-dvd-audio'\n")
			if err := os.WriteFile(installed, fake, 0700); err != nil {
				t.Fatal(err)
			}
			check := func(want string, ready bool) {
				t.Helper()
				for _, tool := range discoverTools(app) {
					if tool.Name == name {
						if tool.Path != want || tool.Ready != ready {
							t.Fatalf("tool = %+v, want path %q, ready %t", tool, want, ready)
						}
						return
					}
				}
				t.Fatalf("tool %s not discovered", name)
			}
			// A fresh clone has the tracked launcher, but no ignored payload.
			check(installed, true)
			payload := filepath.Join(app, "tools", "opt", "dvda-author", "dvda")
			if name == "mkisofs" {
				payload = filepath.Join(app, "tools", "opt", "dvda-author", "bin", "mkisofs")
			}
			if err := os.MkdirAll(payload, 0700); err != nil {
				t.Fatal(err)
			}
			// An incomplete install with a directory at the payload path is not usable.
			check(installed, true)
			if err := os.Remove(payload); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(payload, fake, 0700); err != nil {
				t.Fatal(err)
			}
			check(local, true)
			env := "DVDA_AUTHOR"
			if name == "mkisofs" {
				env = "MKISOFS"
			}
			missing := filepath.Join(app, "explicit-missing-tool")
			t.Setenv(env, missing)
			check(missing, false)
			if err := os.Remove(payload); err != nil {
				t.Fatal(err)
			}
			// Explicit custom local tools do not require a bundled payload.
			if err := os.WriteFile(local, fake, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv(env, local)
			check(local, true)
		})
	}
}
