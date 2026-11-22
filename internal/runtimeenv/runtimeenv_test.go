package runtimeenv

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestBundleRoot(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "kapila-burner")
	if got := bundleRoot(exe); got != "" {
		t.Fatalf("nonbundle root = %q", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "tools", "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, nil, 0700); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := bundleRoot(exe); got != want {
		t.Fatalf("root = %q, want %q", got, want)
	}
	alias := filepath.Join(t.TempDir(), "launcher")
	if err := os.Symlink(exe, alias); err == nil {
		if got := bundleRoot(alias); got != want {
			t.Fatalf("symlink root = %q, want %q", got, want)
		}
	}
}

func TestConfigure(t *testing.T) {
	for _, layout := range []string{"flat", "app"} {
		t.Run(layout, func(t *testing.T) { testConfigure(t, layout == "app") })
	}
}

func testConfigure(t *testing.T, app bool) {
	if runtime.GOOS == "windows" {
		t.Skip("fake query uses a POSIX script")
	}
	for _, key := range []string{"PATH", "XDG_DATA_DIRS", "GSETTINGS_SCHEMA_DIR", "GDK_PIXBUF_MODULEDIR", "GDK_PIXBUF_MODULE_FILE", "GTK_DATA_PREFIX", "GTK_EXE_PREFIX", "GTK_PATH", "GIO_MODULE_DIR", "FONTCONFIG_PATH", "FONTCONFIG_FILE", "GTK_IM_MODULE_FILE"} {
		t.Setenv(key, "original")
	}
	cleanup, err := configure("", true)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if os.Getenv("PATH") != "original" || os.Getenv("GDK_PIXBUF_MODULE_FILE") != "original" {
		t.Fatal("nonbundle bootstrap altered environment")
	}
	root := t.TempDir()
	resources, libraries, tools := root, filepath.Join(root, "lib"), filepath.Join(root, "tools", "bin")
	if app {
		root = filepath.Join(root, "Kapila Burner.app", "Contents")
		resources, libraries, tools = filepath.Join(root, "Resources"), filepath.Join(root, "Frameworks"), filepath.Join(root, "Helpers")
	}
	modules := filepath.Join(libraries, "gdk-pixbuf-2.0", "2.10.0", "loaders")
	schemas := filepath.Join(resources, "share", "glib-2.0", "schemas")
	gtkPath := filepath.Join(libraries, "gtk-3.0")
	gioPath := filepath.Join(libraries, "gio", "modules")
	fonts := filepath.Join(resources, "etc", "fonts")
	for _, path := range []string{modules, schemas, gtkPath, gioPath, fonts, tools} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fonts, "fonts.conf"), []byte("<fontconfig/>"), 0600); err != nil {
		t.Fatal(err)
	}
	query := filepath.Join(root, "gdk-pixbuf-query-loaders")
	if app {
		query = filepath.Join(tools, "gdk-pixbuf-query-loaders")
	}
	if err := os.WriteFile(query, []byte("#!/bin/sh\nprintf '%s\\n' \"$GDK_PIXBUF_MODULEDIR/fake-loader.so\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	imQuery := filepath.Join(tools, "gtk-query-immodules-3.0")
	if err := os.WriteFile(imQuery, []byte("#!/bin/sh\nprintf '%s\\n' \"$GTK_PATH/fake-im-module.so\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cleanup, err = configure(root, false)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	for _, key := range []string{"GDK_PIXBUF_MODULE_FILE", "GTK_DATA_PREFIX", "GTK_EXE_PREFIX", "GTK_PATH", "GIO_MODULE_DIR", "FONTCONFIG_PATH", "FONTCONFIG_FILE", "GTK_IM_MODULE_FILE"} {
		if os.Getenv(key) != "original" {
			t.Fatalf("web bootstrap touched %s", key)
		}
	}
	// Reset PATH to check exact preservation of the inherited suffix.
	t.Setenv("PATH", "original")
	cleanup, err = configure(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if os.Getenv("PATH") != strings.Join([]string{root, tools, "original"}, string(os.PathListSeparator)) || os.Getenv("GSETTINGS_SCHEMA_DIR") != schemas || os.Getenv("GDK_PIXBUF_MODULEDIR") != modules || os.Getenv("XDG_DATA_DIRS") != filepath.Join(resources, "share")+string(os.PathListSeparator)+"original" {
		t.Fatal("incorrect bundle environment")
	}
	for key, want := range map[string]string{"GTK_DATA_PREFIX": resources, "GTK_EXE_PREFIX": root, "GTK_PATH": gtkPath, "GIO_MODULE_DIR": gioPath, "FONTCONFIG_PATH": fonts, "FONTCONFIG_FILE": filepath.Join(fonts, "fonts.conf")} {
		if got := os.Getenv(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	cache := os.Getenv("GDK_PIXBUF_MODULE_FILE")
	data, err := os.ReadFile(cache)
	if err != nil || string(data) != filepath.Join(modules, "fake-loader.so")+"\n" {
		t.Fatalf("cache = %q, %v", data, err)
	}
	info, err := os.Stat(cache)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("cache is not private: %v", err)
	}
	imCache := os.Getenv("GTK_IM_MODULE_FILE")
	imData, err := os.ReadFile(imCache)
	if err != nil || string(imData) != filepath.Join(gtkPath, "fake-im-module.so")+"\n" {
		t.Fatalf("IM cache = %q, %v", imData, err)
	}
	cleanup()
	for _, path := range []string{cache, imCache} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("cache not removed: %v", err)
		}
	}
}

func TestDefaultPaths(t *testing.T) {
	home := t.TempDir()
	for _, key := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(key, home)
	}
	for _, key := range []string{"XDG_CACHE_HOME", "LOCALAPPDATA"} {
		t.Setenv(key, filepath.Join(home, "cache"))
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(home, "Music")
	wantOutput := filepath.Join(cache, "Kapila Burner", "output")
	for _, cwd := range []string{filepath.Join(home, "Applications"), filepath.Join(home, "unrelated", "working-directory")} {
		root, output, err := defaultPaths(cwd, true)
		if err != nil || root != wantRoot || output != wantOutput {
			t.Fatalf("bundle defaults from %q: %q, %q, %v", cwd, root, output, err)
		}
		flags := flag.NewFlagSet("installed", flag.ContinueOnError)
		var help bytes.Buffer
		flags.SetOutput(&help)
		flags.String("root", root, "Audio library root directory")
		flags.String("output", output, "Authoring output directory")
		if err := flags.Parse([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("help error: %v", err)
		}
		if !strings.Contains(help.String(), "default "+strconv.Quote(root)) || !strings.Contains(help.String(), "default "+strconv.Quote(output)) {
			t.Fatalf("help is missing installed defaults: %s", help.String())
		}
		root, output, err = defaultPaths(cwd, false)
		if err != nil || root != filepath.Dir(cwd) || output != filepath.Join(cwd, "output") {
			t.Fatalf("local defaults: %q, %q, %v", root, output, err)
		}
	}
	for _, path := range []string{wantRoot, wantOutput} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("calculating defaults/help created %s: %v", path, err)
		}
	}
	for _, params := range []struct {
		root    string
		bundled bool
	}{
		{wantRoot, false},
		{filepath.Join(home, "custom-library"), true},
		{wantRoot, true},
	} {
		if err := prepareDefaultLibrary(params.root, wantRoot, params.bundled); err != nil {
			t.Fatal(err)
		}
		_, err := os.Stat(params.root)
		if params.bundled && params.root == wantRoot {
			if err != nil {
				t.Fatalf("default Music was not created: %v", err)
			}
			info, err := os.Stat(params.root)
			if err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
				t.Fatalf("default Music is not private: %v", err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("unexpected directory creation: %s (%v)", params.root, err)
		}
	}
}

func TestQueryFailureCleansCaches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake queries use POSIX scripts")
	}
	for _, key := range []string{"PATH", "GDK_PIXBUF_MODULEDIR", "GDK_PIXBUF_MODULE_FILE", "GTK_EXE_PREFIX", "GTK_IM_MODULE_FILE"} {
		t.Setenv(key, os.Getenv(key))
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders"), 0700); err != nil {
		t.Fatal(err)
	}
	for helper, script := range map[string]string{
		"gdk-pixbuf-query-loaders": "#!/bin/sh\nprintf 'loader-cache\\n'\n",
		"gtk-query-immodules-3.0":  "#!/bin/sh\nexit 1\n",
	} {
		if err := os.WriteFile(filepath.Join(root, helper), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	cleanup, err := configure(root, true)
	if err == nil || !strings.Contains(err.Error(), "gtk-query-immodules-3.0") {
		t.Fatalf("query failure not reported: %v", err)
	}
	cache := os.Getenv("GDK_PIXBUF_MODULE_FILE")
	if cache == "" {
		t.Fatal("first query did not generate a cache")
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("first cache leaked after second query failed: %v", err)
	}
	cleanup()
}

func TestConfigureMissingResources(t *testing.T) {
	keys := []string{"XDG_DATA_DIRS", "GSETTINGS_SCHEMA_DIR", "GTK_DATA_PREFIX", "GTK_EXE_PREFIX", "GTK_PATH", "GIO_MODULE_DIR", "FONTCONFIG_PATH", "FONTCONFIG_FILE", "GDK_PIXBUF_MODULEDIR", "GDK_PIXBUF_MODULE_FILE", "GTK_IM_MODULE_FILE"}
	t.Setenv("PATH", os.Getenv("PATH"))
	for _, key := range keys {
		t.Setenv(key, "host-setting")
	}
	root := t.TempDir()
	cleanup, err := configure(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, key := range keys {
		if got := os.Getenv(key); got != "host-setting" {
			t.Fatalf("missing resource overwrote %s: %q", key, got)
		}
	}
	fonts := filepath.Join(root, "etc", "fonts")
	if err := os.MkdirAll(fonts, 0700); err != nil {
		t.Fatal(err)
	}
	cleanup, err = configure(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if os.Getenv("FONTCONFIG_PATH") != fonts || os.Getenv("FONTCONFIG_FILE") != "host-setting" {
		t.Fatal("font directory must not override an absent fonts.conf")
	}
}

func TestDefaultPathsMissingHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	root, output, err := defaultPaths(t.TempDir(), true)
	if err == nil || root != "" || output != "" {
		t.Fatalf("missing home must fail rather than scan a fallback: %q, %q, %v", root, output, err)
	}
}
