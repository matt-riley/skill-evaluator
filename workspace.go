package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// workspacePath returns the workspace directory for a skill.
func workspacePath(skillDir string) string {
	return filepath.Join(skillDir, "..", filepath.Base(skillDir)+"-workspace")
}

// iterationPath returns the path for a specific iteration.
func iterationPath(workspace string, n int) string {
	return filepath.Join(workspace, "iteration-"+strconv.Itoa(n))
}

// evalPath returns the path for an eval within an iteration.
// If modelKey is non-empty, it nests under the model directory.
func evalPath(workspace string, iteration int, evalID int, modelKey string) string {
	base := filepath.Join(iterationPath(workspace, iteration), fmt.Sprintf("eval-%d", evalID))
	if modelKey != "" {
		return filepath.Join(base, modelKey)
	}
	return base
}

// ensureDir creates a directory and all parents.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// nextIteration scans the workspace and returns the next iteration number.
func nextIteration(workspace string) int {
	max := 0
	entries, _ := os.ReadDir(workspace)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "iteration-")); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// snapshotSkill copies a skill directory into the workspace as a snapshot.
func snapshotSkill(skillDir, workspace string, iteration int) (string, error) {
	dst := filepath.Join(iterationPath(workspace, iteration), "skill-snapshot")
	if err := os.CopyFS(dst, os.DirFS(skillDir)); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	return dst, nil
}

// lockPath returns the path to an iteration's lockfile.
func lockPath(workspace string, iter int) string {
	return filepath.Join(iterationPath(workspace, iter), ".lock.json")
}

// readLock reads and parses an iteration lockfile.
func readLock(workspace string, iter int) (*IterationLock, error) {
	data, err := os.ReadFile(lockPath(workspace, iter))
	if err != nil {
		return nil, err
	}
	var lock IterationLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	lock.Iteration = iter
	return &lock, nil
}

// writeLock atomically writes an iteration lockfile.
func writeLock(workspace string, lock *IterationLock) error {
	path := lockPath(workspace, lock.Iteration)
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".lock-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// acquireLock obtains an exclusive advisory lock on the iteration directory.
// Returns the file descriptor that must be closed to release the lock.
// Fails immediately if another process holds the lock (LOCK_NB).
func acquireLock(dir string) (*os.File, error) {
	f, err := os.OpenFile(dir, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("opening dir for lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
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

// isCompleted reports whether a run triple is already recorded as completed.
func isCompleted(lock *IterationLock, evalID int, model, config string) bool {
	for _, c := range lock.Completed {
		if c.EvalID == evalID && c.Model == model && c.Config == config {
			return true
		}
	}
	return false
}

// findRunningIteration returns the latest iteration whose lockfile status is "running".
func findRunningIteration(workspace string) (int, *IterationLock, error) {
	entries, _ := os.ReadDir(workspace)
	maxIter := 0
	var latest *IterationLock
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "iteration-"))
		if err != nil {
			continue
		}
		if n <= maxIter {
			continue
		}
		lock, err := readLock(workspace, n)
		if err != nil {
			continue
		}
		if lock.Status == "running" {
			maxIter = n
			latest = lock
		}
	}
	if latest == nil {
		return 0, nil, fmt.Errorf("no running iteration found in %s", workspace)
	}
	return maxIter, latest, nil
}
