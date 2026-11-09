package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// openDataLock holds an exclusive lock until the database and workers are closed.
func openDataLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("打开数据目录锁: %w", err)
	}
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped); err != nil {
		return nil, errors.Join(fmt.Errorf("数据目录正在使用或无法锁定: %w", err), file.Close())
	}
	return file, nil
}
