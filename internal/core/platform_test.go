package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCommandProcessHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] != "core-process-helper" {
		return
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestPortableCommandCancellation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := testStudio(t)
	s.job = &Job{State: "running"}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = s.command(ctx, s.output, executable, "-test.run=^TestCommandProcessHelper$", "--", "core-process-helper")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("cancellation took %v", elapsed)
	}
}

func TestWorkspaceAvailable(t *testing.T) {
	dir := t.TempDir()
	if available, err := workspaceAvailable(dir); err != nil || available == 0 {
		t.Fatalf("available workspace: %d, %v", available, err)
	}
	if _, err := workspaceAvailable(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing workspace silently accepted")
	}
}

func TestLockReleaseAndClassification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, err := openLock(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if second, err := openLock(path); !errors.Is(err, ErrBusy) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("contended lock must report ErrBusy: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := openLock(path)
	if err != nil {
		t.Fatalf("closed lock not released: %v", err)
	}
	_ = second.Close()
}

func TestLockRejectsSharedPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses ACLs, not POSIX owner permission bits")
	}
	path := filepath.Join(t.TempDir(), "lock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if lock, err := openLock(path); !errors.Is(err, os.ErrPermission) {
		if lock != nil {
			_ = lock.Close()
		}
		t.Fatalf("unsafe permissions accepted: %v", err)
	}
}
