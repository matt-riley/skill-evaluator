package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockReadWrite(t *testing.T) {
	ws := t.TempDir()
	lock := &IterationLock{
		Iteration: 1,
		Status:    "running",
		Completed: []RunIdentity{{EvalID: 7, Model: "pi:claude", Config: "with_skill"}},
		StartedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 25, 12, 1, 0, 0, time.UTC),
	}

	if err := writeLock(ws, lock); err != nil {
		t.Fatalf("writeLock: %v", err)
	}

	got, err := readLock(ws, 1)
	if err != nil {
		t.Fatalf("readLock: %v", err)
	}
	if got.Iteration != 1 {
		t.Errorf("iteration = %d, want 1", got.Iteration)
	}
	if got.Status != "running" {
		t.Errorf("status = %q, want running", got.Status)
	}
	if len(got.Completed) != 1 || got.Completed[0].EvalID != 7 {
		t.Errorf("completed = %v, want [{7}]", got.Completed)
	}
	if !got.StartedAt.Equal(lock.StartedAt) {
		t.Errorf("started_at = %v, want %v", got.StartedAt, lock.StartedAt)
	}
	if !got.UpdatedAt.Equal(lock.UpdatedAt) {
		t.Errorf("updated_at = %v, want %v", got.UpdatedAt, lock.UpdatedAt)
	}

	if _, err := readLock(ws, 2); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("readLock missing = %v, want ErrNotExist", err)
	}
}

func TestIsCompleted(t *testing.T) {
	lock := &IterationLock{
		Completed: []RunIdentity{
			{EvalID: 1, Model: "pi:claude", Config: "with_skill"},
			{EvalID: 1, Model: "pi:claude", Config: "baseline"},
		},
	}

	if !isCompleted(lock, 1, "pi:claude", "with_skill") {
		t.Error("expected completed (1, pi:claude, with_skill)")
	}
	if isCompleted(lock, 2, "pi:claude", "with_skill") {
		t.Error("expected not completed (2, ...)")
	}
	if isCompleted(lock, 1, "pi:gpt", "with_skill") {
		t.Error("expected not completed (... pi:gpt ...)")
	}
	if isCompleted(lock, 1, "pi:claude", "grade") {
		t.Error("expected not completed (... grade)")
	}
}

func TestFindRunningIteration(t *testing.T) {
	ws := t.TempDir()

	mustWrite := func(iter int, status string) {
		lock := &IterationLock{Iteration: iter, Status: status, StartedAt: time.Now()}
		if err := writeLock(ws, lock); err != nil {
			t.Fatalf("writeLock %d: %v", iter, err)
		}
	}

	mustWrite(1, "complete")
	mustWrite(3, "running")
	mustWrite(2, "running")

	iter, lock, err := findRunningIteration(ws)
	if err != nil {
		t.Fatalf("findRunningIteration: %v", err)
	}
	if iter != 3 {
		t.Errorf("iter = %d, want 3", iter)
	}
	if lock.Status != "running" {
		t.Errorf("status = %q, want running", lock.Status)
	}
}

func TestGradeBlocksIncompleteLock(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	evalsPath := filepath.Join(skillDir, "evals", "evals.json")
	if err := os.MkdirAll(filepath.Dir(evalsPath), 0o755); err != nil {
		t.Fatalf("mkdir evals: %v", err)
	}
	ef := EvalFile{
		SkillName: "skill",
		Evals: []Eval{
			{ID: 1, Prompt: "test", ExpectedOutput: "ok"},
		},
	}
	data, _ := json.MarshalIndent(ef, "", "  ")
	if err := os.WriteFile(evalsPath, data, 0o644); err != nil {
		t.Fatalf("write evals.json: %v", err)
	}

	lock := &IterationLock{Iteration: 1, Status: "running", StartedAt: time.Now(), UpdatedAt: time.Now()}
	if err := writeLock(workspacePath(skillDir), lock); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(skillDir); err != nil {
		t.Fatalf("chdir skill: %v", err)
	}

	err = cmdGrade(nil)
	if err == nil {
		t.Fatal("cmdGrade expected error for running lock")
	}
	if !strings.Contains(err.Error(), "iteration 1 is still running") {
		t.Errorf("error = %v, want mention of running iteration 1", err)
	}
}
