package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspacePath(t *testing.T) {
	got := workspacePath("/tmp/skill")
	want := filepath.Join("/tmp", "skill-workspace")
	if got != want {
		t.Errorf("workspacePath = %q, want %q", got, want)
	}
}

func TestWorkspaceIterationPath(t *testing.T) {
	ws := workspacePath("/tmp/skill")
	got := iterationPath(ws, 3)
	want := filepath.Join("/tmp", "skill-workspace", "iteration-3")
	if got != want {
		t.Errorf("iterationPath = %q, want %q", got, want)
	}
}

func TestWorkspaceEvalPath(t *testing.T) {
	ws := workspacePath("/tmp/skill")
	tests := []struct {
		name     string
		modelKey string
		want     string
	}{
		{
			name:     "without model key",
			modelKey: "",
			want:     filepath.Join("/tmp", "skill-workspace", "iteration-1", "eval-5"),
		},
		{
			name:     "with model key",
			modelKey: "pi-claude",
			want:     filepath.Join("/tmp", "skill-workspace", "iteration-1", "eval-5", "pi-claude"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalPath(ws, 1, 5, tt.modelKey)
			if got != tt.want {
				t.Errorf("evalPath = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkspaceNextIteration(t *testing.T) {
	tmp := t.TempDir()

	// Seed some iteration directories and a distractor.
	for _, name := range []string{"iteration-1", "iteration-3", "iteration-10", "logs", "iteration-bad"} {
		if err := os.MkdirAll(filepath.Join(tmp, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "iteration-99"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := nextIteration(tmp); got != 11 {
		t.Errorf("nextIteration = %d, want 11", got)
	}
}
