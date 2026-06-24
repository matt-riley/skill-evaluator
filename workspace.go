package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// workspacePath returns the workspace directory for a skill.
func workspacePath(skillDir string) string {
	return filepath.Join(skillDir, "..", filepath.Base(skillDir)+"-workspace")
}

// iterationPath returns the path for a specific iteration.
func iterationPath(workspace string, n int) string {
	return filepath.Join(workspace, "iteration-"+strconv.Itoa(n))
}

// evalDirName creates a safe directory name from an eval ID.
// ponytail: IDs are ints, so strconv is fine.
func evalDirName(evalID int) string {
	return fmt.Sprintf("eval-%d", evalID)
}

// evalPath returns the path for an eval within an iteration.
func evalPath(workspace string, iteration int, evalID int) string {
	return filepath.Join(iterationPath(workspace, iteration), evalDirName(evalID))
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
		var n int
		if _, err := fmt.Sscanf(e.Name(), "iteration-%d", &n); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// snapshotSkill copies a skill directory into the workspace as a snapshot.
func snapshotSkill(skillDir, workspace string, iteration int) (string, error) {
	dst := filepath.Join(iterationPath(workspace, iteration), "skill-snapshot")
	if err := copyDir(skillDir, dst); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	return dst, nil
}

// copyDir copies a directory recursively. ponytail: stdlib, no deps.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
