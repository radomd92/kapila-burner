// Package runtimeenv configures relocatable native distributions without cgo.
package runtimeenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BundleRoot is executable-relative, never working-directory-relative.
// Linux continues to use its existing launcher contract.
func BundleRoot() string {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return bundleRoot(exe)
}

func bundleRoot(exe string) string {
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	root := filepath.Dir(exe)
	if info, err := os.Stat(filepath.Join(root, "tools", "bin")); err == nil && info.IsDir() {
		return root
	}
	return ""
}

// ApplicationDir preserves the local-development cwd default outside bundles.
func ApplicationDir() (string, error) {
	if root := BundleRoot(); root != "" {
		return root, nil
	}
	return os.Getwd()
}

// DefaultPaths only calculates paths; help and flag parsing must not create files.
func DefaultPaths(cwd string) (root, output string, err error) {
	return defaultPaths(cwd, BundleRoot() != "")
}

func defaultPaths(cwd string, bundled bool) (root, output string, err error) {
	if !bundled {
		return filepath.Dir(cwd), filepath.Join(cwd, "output"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("default music directory: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", "", fmt.Errorf("default output directory: %w", err)
	}
	return filepath.Join(home, "Music"), filepath.Join(cache, "Kapila Burner", "output"), nil
}

// PrepareDefaultLibrary runs after parsing flags, never for a custom library.
// An absent Music directory is created rather than falling back to scanning home.
func PrepareDefaultLibrary(root, defaultRoot string) error {
	return prepareDefaultLibrary(root, defaultRoot, BundleRoot() != "")
}

func prepareDefaultLibrary(root, defaultRoot string, bundled bool) error {
	if !bundled || root != defaultRoot {
		return nil
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return fmt.Errorf("create default music directory: %w", err)
	}
	return nil
}

// CommandEnv makes DLLs and bundled helper executables available to utilities.
func CommandEnv(root string) []string {
	if root == "" {
		return os.Environ()
	}
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "PATH") {
			env = append(env, entry)
		}
	}
	path := root + string(os.PathListSeparator) + filepath.Join(root, "tools", "bin")
	if old := os.Getenv("PATH"); old != "" {
		path += string(os.PathListSeparator) + old
	}
	return append(env, "PATH="+path)
}

// Configure must run before GTK initialization (gtk=true) or engine creation.
// The returned cleanup removes any private, dynamically generated loader cache.
func Configure(gtk bool) (func(), error) {
	return configure(BundleRoot(), gtk)
}

func configure(root string, gtk bool) (func(), error) {
	var caches []string
	cleanup := func() {
		for _, path := range caches {
			_ = os.Remove(path)
		}
	}
	if root == "" {
		return cleanup, nil
	}
	for _, entry := range CommandEnv(root) {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PATH") {
			if err := os.Setenv("PATH", value); err != nil {
				return cleanup, err
			}
		}
	}
	if !gtk {
		return cleanup, nil
	}
	for _, item := range []struct{ key, relative, value string }{
		{"XDG_DATA_DIRS", "share", ""},
		{"GSETTINGS_SCHEMA_DIR", "share/glib-2.0/schemas", ""},
		{"GTK_DATA_PREFIX", "share", root},
		{"GTK_EXE_PREFIX", "lib", root},
		{"GTK_PATH", "lib/gtk-3.0", ""},
		{"GIO_MODULE_DIR", "lib/gio/modules", ""},
		{"FONTCONFIG_PATH", "etc/fonts", ""},
		{"FONTCONFIG_FILE", "etc/fonts/fonts.conf", ""},
	} {
		path := filepath.Join(root, filepath.FromSlash(item.relative))
		if info, err := os.Stat(path); err == nil && (info.IsDir() && item.key != "FONTCONFIG_FILE" || info.Mode().IsRegular() && item.key == "FONTCONFIG_FILE") {
			value := item.value
			if value == "" {
				value = path
			}
			if item.key == "XDG_DATA_DIRS" && os.Getenv(item.key) != "" {
				value += string(os.PathListSeparator) + os.Getenv(item.key)
			}
			if err := os.Setenv(item.key, value); err != nil {
				return cleanup, err
			}
		}
	}
	// The versioned gdk-pixbuf directory is part of GTK's on-disk ABI.
	modules, err := filepath.Glob(filepath.Join(root, "lib", "gdk-pixbuf-2.0", "*", "loaders"))
	if err != nil {
		return cleanup, err
	}
	for _, item := range []struct{ helper, key string }{
		{"gdk-pixbuf-query-loaders", "GDK_PIXBUF_MODULE_FILE"},
		{"gtk-query-immodules-3.0", "GTK_IM_MODULE_FILE"},
	} {
		if item.key == "GDK_PIXBUF_MODULE_FILE" {
			if len(modules) == 0 {
				continue
			}
			if err := os.Setenv("GDK_PIXBUF_MODULEDIR", modules[0]); err != nil {
				return cleanup, err
			}
		}
		helper := item.helper
		if runtime.GOOS == "windows" {
			helper += ".exe"
		}
		query := filepath.Join(root, helper)
		if _, err := os.Stat(query); os.IsNotExist(err) {
			query = filepath.Join(root, "tools", "bin", helper)
		}
		if _, err := os.Stat(query); os.IsNotExist(err) && item.key == "GTK_IM_MODULE_FILE" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, query)
		cmd.WaitDelay = time.Second
		data, err := cmd.Output()
		cancel()
		if err != nil {
			cleanup()
			return cleanup, fmt.Errorf("query bundled %s: %w", item.helper, err)
		}
		cache, err := os.CreateTemp("", "kapila-gtk-*.cache")
		if err != nil {
			cleanup()
			return cleanup, err
		}
		caches = append(caches, cache.Name())
		_, writeErr := cache.Write(data)
		closeErr := cache.Close()
		if writeErr != nil {
			cleanup()
			return cleanup, writeErr
		}
		if closeErr != nil {
			cleanup()
			return cleanup, closeErr
		}
		if err := os.Setenv(item.key, cache.Name()); err != nil {
			cleanup()
			return cleanup, err
		}
	}
	return cleanup, nil
}
