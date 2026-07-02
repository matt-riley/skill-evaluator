//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// acquireLock obtains an exclusive lock on the iteration directory
// by creating a lock file. On Windows, flock is not available so we
// use a file-based lock instead.
func acquireLock(dir string) (*os.File, error) {
	lockFile := filepath.Join(dir, ".windows-lock")
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another process is running this iteration")
		}
		return nil, fmt.Errorf("opening lock file: %w", err)
	}
	return f, nil
}

// releaseLock releases the lock by closing and removing the lock file.
func releaseLock(f *os.File) error {
	if f == nil {
		return nil
	}
	lockPath := f.Name()
	_ = f.Close()
	return os.Remove(lockPath)
}
