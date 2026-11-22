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
	exe := filepath.Join(root, "kapila-maker")
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
	root := filepath.Join(t.TempDir(), "bundle with spaces")
	resources, libraries, tools := root, filepath.Join(root, "lib"), filepath.Join(root, "tools", "bin")
	if app {
		root = filepath.Join(root, "kapila-maker.app", "Contents")
		resources, libraries, tools = filepath.Join(root, "Resources"), filepath.Join(root, "Frameworks"), filepath.Join(root, "Helpers")
	}
	modules := filepath.Join(libraries, "gdk-pixbuf-2.0", "2.10.0", "loaders")
	schemas := filepath.Join(resources, "share", "glib-2.0", "schemas")
	gtkPath := filepath.Join(libraries, "gtk-3.0")
	gioPath := filepath.Join(libraries, "gio", "modules")
	imPath := filepath.Join(gtkPath, "3.0.0", "immodules")
	if app {
		modules = filepath.Join(root, "PlugIns", "pixbuf")
		gtkPath = filepath.Join(root, "PlugIns", "gtk3")
		gioPath = filepath.Join(root, "PlugIns", "gio")
		imPath = filepath.Join(gtkPath, "immodules")
	}
	fonts := filepath.Join(resources, "etc", "fonts")
	for _, path := range []string{modules, schemas, imPath, gioPath, fonts, tools, libraries} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	var imModules []string
	for _, name := range []string{"fake-im-module.so", "fake-im-module.dylib", "ignored.dll", "ignored.cache"} {
		path := filepath.Join(imPath, name)
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib") {
			imModules = append(imModules, path)
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
	imScript := "#!/bin/sh\n[ \"$#\" -eq 0 ] || exit 1\nprintf '%s\\n' \"$GTK_PATH/fake-im-module.so\"\n"
	if app {
		imScript = "#!/bin/sh\n[ \"$#\" -eq 2 ] || exit 1\nprintf '%s\\n' \"$@\"\n"
	}
	if err := os.WriteFile(imQuery, []byte(imScript), 0700); err != nil {
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
	wantIM := filepath.Join(gtkPath, "fake-im-module.so") + "\n"
	if app {
		wantIM = strings.Join(imModules, "\n") + "\n"
	}
	if err != nil || string(imData) != wantIM {
		t.Fatalf("IM cache = %q, %v", imData, err)
	}
	if info, err := os.Stat(imCache); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("IM cache is not private: %v", err)
	}
	if app {
		entries, err := os.ReadDir(libraries)
		if err != nil || len(entries) != 0 {
			t.Fatalf("Frameworks must not require plugin/version directories: %v, %v", entries, err)
		}
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
	wantOutput := filepath.Join(cache, "kapila-maker", "output")
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

func TestConfigureHeadlessApp(t *testing.T) {
	root := filepath.Join(t.TempDir(), "headless.app", "Contents")
	for _, name := range []string{"MacOS", "Helpers", "Resources"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(root, "MacOS", "kapila-maker")
	if err := os.WriteFile(exe, nil, 0700); err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := bundleRoot(exe); got != wantRoot {
		t.Fatalf("headless bundleRoot = %q, want %q", got, wantRoot)
	}
	t.Setenv("PATH", "original")
	keys := []string{"XDG_DATA_DIRS", "GSETTINGS_SCHEMA_DIR", "GTK_DATA_PREFIX", "GTK_EXE_PREFIX", "GTK_PATH", "GIO_MODULE_DIR", "FONTCONFIG_PATH", "FONTCONFIG_FILE", "GDK_PIXBUF_MODULEDIR", "GDK_PIXBUF_MODULE_FILE", "GTK_IM_MODULE_FILE"}
	for _, key := range keys {
		t.Setenv(key, "host-setting")
	}
	// Old module directories must not become a fallback inside a proper app.
	for _, name := range []string{filepath.Join("gdk-pixbuf-2.0", "2.10.0", "loaders"), "gtk-3.0", filepath.Join("gio", "modules")} {
		if err := os.MkdirAll(filepath.Join(root, "Frameworks", name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, gtk := range []bool{false, true} {
		t.Setenv("PATH", "original")
		cleanup, err := configure(root, gtk)
		if err != nil {
			t.Fatal(err)
		}
		cleanup()
		if want := strings.Join([]string{root, filepath.Join(root, "Helpers"), "original"}, string(os.PathListSeparator)); os.Getenv("PATH") != want {
			t.Fatalf("helper PATH = %q, want %q", os.Getenv("PATH"), want)
		}
		for _, key := range keys {
			if gtk && key == "GTK_EXE_PREFIX" {
				if os.Getenv(key) != root {
					t.Fatalf("GTK_EXE_PREFIX = %q, want %q", os.Getenv(key), root)
				}
				continue
			}
			if os.Getenv(key) != "host-setting" {
				t.Fatalf("headless bootstrap touched %s", key)
			}
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
