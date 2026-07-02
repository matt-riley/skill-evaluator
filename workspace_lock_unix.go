//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

// acquireLock obtains an exclusive advisory lock on the iteration directory.
// Returns the file descriptor that must be closed to release the lock.
// Fails immediately if another process holds the lock (LOCK_NB).
func acquireLock(dir string) (*os.File, error) {
	f, err := os.OpenFile(dir, os.O_RDONLY, 0) // #nosec G304 -- dir is iterationPath(), internal convention, used to flock the iteration directory
	if err != nil {
		return nil, fmt.Errorf("opening dir for lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another process is running this iteration: %w", err)
	}
	return f, nil
}

// releaseLock releases an advisory lock and closes the file descriptor.
func releaseLock(f *os.File) error {
	if f == nil {
		return nil
	}
	return f.Close()
}
