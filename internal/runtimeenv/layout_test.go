package runtimeenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacAppLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Kapila Burner.app", "Contents")
	exe := filepath.Join(root, "MacOS", "kapila-burner")
	for _, name := range []string{"MacOS", "Helpers", "Resources", "Frameworks"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(exe, nil, 0700); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := bundleRoot(exe); got != want {
		t.Fatalf("bundleRoot = %q, want %q", got, want)
	}
	for _, item := range []struct {
		got, want string
	}{
		{ToolDir(root), filepath.Join(root, "Helpers")},
		{ResourceDir(root), filepath.Join(root, "Resources")},
		{LibraryDir(root), filepath.Join(root, "Frameworks")},
	} {
		if item.got != item.want {
			t.Fatalf("layout path = %q, want %q", item.got, item.want)
		}
	}
	if err := os.Remove(filepath.Join(root, "Resources")); err != nil {
		t.Fatal(err)
	}
	if got := bundleRoot(exe); got != "" {
		t.Fatalf("incomplete app recognized: %q", got)
	}
	flat := t.TempDir()
	if ToolDir(flat) != filepath.Join(flat, "tools", "bin") || ResourceDir(flat) != flat || LibraryDir(flat) != filepath.Join(flat, "lib") {
		t.Fatal("flat layout changed")
	}
}
