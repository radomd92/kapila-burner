//go:build windows

package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/windows"
)

func workspaceAvailable(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available uint64
	err = windows.GetDiskFreeSpaceEx(p, &available, nil, nil)
	return available, err
}

func configureProcess(cmd *exec.Cmd) {
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		// Resolve the OS utility outside PATH and pass arguments without a shell.
		dir, err := windows.GetSystemDirectory()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			kill := exec.CommandContext(ctx, filepath.Join(dir, "taskkill.exe"), "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
			kill.WaitDelay = time.Second
			if err := kill.Run(); err == nil {
				return nil
			}
		}
		return cmd.Process.Kill()
	}
}

func openLock(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.NumberOfLinks != 1 {
		_ = f.Close()
		return nil, os.ErrPermission
	}
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped); err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrBusy
		}
		return nil, err
	}
	// Closing this handle releases the byte-range lock, including on process exit.
	return f, nil
}
