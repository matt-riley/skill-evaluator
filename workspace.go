package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
func evalPath(workspace string, iteration int, evalID int) string {
	return filepath.Join(iterationPath(workspace, iteration), fmt.Sprintf("eval-%d", evalID))
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
