package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- TestEvalActivationDefaults ---

func TestEvalActivationDefaults(t *testing.T) {
	t.Run("nil defaults to true", func(t *testing.T) {
		e := Eval{Type: "activation"}
		if !e.expectedActivation() {
			t.Fatal("expected default true when ShouldActivate is nil")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		b := true
		e := Eval{Type: "activation", ShouldActivate: &b}
		if !e.expectedActivation() {
			t.Fatal("expected true when ShouldActivate is explicitly true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		b := false
		e := Eval{Type: "activation", ShouldActivate: &b}
		if e.expectedActivation() {
			t.Fatal("expected false when ShouldActivate is explicitly false")
		}
	})

	t.Run("task eval is not activation", func(t *testing.T) {
		e := Eval{Type: ""}
		if e.isActivation() {
			t.Fatal("empty type should not be activation")
		}
	})

	t.Run("activation eval is activation", func(t *testing.T) {
		e := Eval{Type: "activation"}
		if !e.isActivation() {
			t.Fatal("type=activation should be activation")
		}
	})
}

// --- TestBuildActivationPrompt ---

func TestBuildActivationPrompt(t *testing.T) {
	prompt := buildActivationPrompt("git-commit", "Helps write commit messages", "Write a commit for staged changes")

	checks := []struct {
		label string
		want  string
	}{
		{"contains name", "git-commit"},
		{"contains description", "Helps write commit messages"},
		{"contains task", "Write a commit for staged changes"},
		{"contains task marker", "<task>"},
		{"mentions progressive disclosure", "Progressive disclosure"},
		{"requests JSON", `"would_activate": true`},
		{"says no for adjacent", "Say NO"},
	}

	for _, c := range checks {
		if !strings.Contains(prompt, c.want) {
			t.Errorf("%s: prompt missing %q", c.label, c.want)
		}
	}
}

// --- TestJudgeActivationParsesVerdict ---

func TestJudgeActivationParsesVerdict(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantAct bool
		wantErr bool
	}{
		{
			name:    "would activate",
			output:  `{"would_activate": true, "reason": "the skill matches the task"}`,
			wantAct: true,
			wantErr: false,
		},
		{
			name:    "would not activate",
			output:  `{"would_activate": false, "reason": "task is unrelated to this skill"}`,
			wantAct: false,
			wantErr: false,
		},
		{
			name:    "verbose judge wraps JSON in prose",
			output:  `Here is my verdict: {"would_activate": true, "reason": "yes"} Done!`,
			wantAct: true,
			wantErr: false,
		},
		{
			name:    "malformed JSON",
			output:  `{"would_activate": true, "reason": `,
			wantErr: true,
		},
		{
			name:    "no JSON at all",
			output:  `The skill should not be activated for this task.`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := parseActivationOutput(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (verdict=%+v)", v)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.WouldActivate != tt.wantAct {
				t.Errorf("WouldActivate: got %v, want %v", v.WouldActivate, tt.wantAct)
			}
		})
	}
}

func TestJudgeActivationWithStubCmd(t *testing.T) {
	mockCmd := CmdBuilder(func(ctx context.Context, agent, model, task, skillPath string) *exec.Cmd {
		return exec.Command("echo", `{"would_activate": true, "reason": "matches"}`)
	})

	cfg := &Config{
		Defaults: DefaultsConfig{Agent: "pi"},
		Judge:    JudgeConfig{},
	}

	t.Run("positive case", func(t *testing.T) {
		eval := Eval{ID: 1, Prompt: "do the thing", Type: "activation"}
		ar, err := judgeActivation(context.Background(), cfg, "my-skill", "does the thing", eval, mockCmd)
		if err != nil {
			t.Fatalf("judgeActivation: %v", err)
		}
		if !ar.WouldActivate {
			t.Error("expected WouldActivate=true")
		}
		if !ar.Expected {
			t.Error("expected Expected=true (nil ShouldActivate defaults to true)")
		}
		if ar.EvalID != 1 {
			t.Errorf("EvalID: got %d, want 1", ar.EvalID)
		}
	})

	t.Run("negative case (should_activate=false)", func(t *testing.T) {
		b := false
		eval := Eval{ID: 2, Prompt: "unrelated task", Type: "activation", ShouldActivate: &b}
		ar, err := judgeActivation(context.Background(), cfg, "my-skill", "does the thing", eval, mockCmd)
		if err != nil {
			t.Fatalf("judgeActivation: %v", err)
		}
		if ar.Expected {
			t.Error("expected Expected=false")
		}
	})

	t.Run("judge error", func(t *testing.T) {
		errCmd := CmdBuilder(func(ctx context.Context, agent, model, task, skillPath string) *exec.Cmd {
			return exec.Command("false") // exits non-zero
		})
		eval := Eval{ID: 3, Prompt: "do thing", Type: "activation"}
		_, err := judgeActivation(context.Background(), cfg, "s", "d", eval, errCmd)
		if err == nil {
			t.Fatal("expected error from failed judge command")
		}
	})
}

// --- TestRunSkipsActivationEvals ---

func TestRunSkipsActivationEvals(t *testing.T) {
	// Verify the eval-count logic: activation evals increment
	// activationCount, not evalCount. This mirrors the loop in cmd_run.go
	// that gates whether an eval enters the agent-invocation path.
	ef := EvalFile{
		Evals: []Eval{
			{ID: 1, Prompt: "task eval prompt that is long enough", ExpectedOutput: "result"},
			{ID: 2, Type: "activation", Prompt: "activation eval prompt"},
		},
	}

	activationCount := 0
	evalCount := 0
	for _, eval := range ef.Evals {
		if eval.isActivation() {
			activationCount++
			continue
		}
		evalCount++
	}
	if activationCount != 1 {
		t.Errorf("activationCount: got %d, want 1", activationCount)
	}
	if evalCount != 1 {
		t.Errorf("evalCount: got %d, want 1", evalCount)
	}
}

// --- TestSummarizeActivation ---

func TestSummarizeActivation(t *testing.T) {
	t.Run("standard case", func(t *testing.T) {
		// 2 TP, 1 FP, 1 FN, 1 TN → precision 0.667, recall 0.667, accuracy 0.6
		results := []ActivationResult{
			{EvalID: 1, Expected: true, WouldActivate: true},   // TP
			{EvalID: 2, Expected: true, WouldActivate: true},   // TP
			{EvalID: 3, Expected: false, WouldActivate: true},  // FP
			{EvalID: 4, Expected: true, WouldActivate: false},  // FN
			{EvalID: 5, Expected: false, WouldActivate: false}, // TN
		}
		s := summarizeActivation(results)
		if s.Total != 5 {
			t.Errorf("Total: got %d, want 5", s.Total)
		}
		if s.TP != 2 {
			t.Errorf("TP: got %d, want 2", s.TP)
		}
		if s.FP != 1 {
			t.Errorf("FP: got %d, want 1", s.FP)
		}
		if s.FN != 1 {
			t.Errorf("FN: got %d, want 1", s.FN)
		}
		if s.TN != 1 {
			t.Errorf("TN: got %d, want 1", s.TN)
		}
		if !approxEqual(s.Precision, 2.0/3.0) {
			t.Errorf("Precision: got %.4f, want %.4f", s.Precision, 2.0/3.0)
		}
		if !approxEqual(s.Recall, 2.0/3.0) {
			t.Errorf("Recall: got %.4f, want %.4f", s.Recall, 2.0/3.0)
		}
		if !approxEqual(s.Accuracy, 0.6) {
			t.Errorf("Accuracy: got %.4f, want 0.6", s.Accuracy)
		}
	})

	t.Run("all positive corpus (zero FP division guard)", func(t *testing.T) {
		// All expected=true, all activate → 3 TP, 0 FP, 0 FN, 0 TN
		results := []ActivationResult{
			{Expected: true, WouldActivate: true},
			{Expected: true, WouldActivate: true},
			{Expected: true, WouldActivate: true},
		}
		s := summarizeActivation(results)
		if s.Precision != 1.0 {
			t.Errorf("Precision: got %.4f, want 1.0", s.Precision)
		}
		if s.Recall != 1.0 {
			t.Errorf("Recall: got %.4f, want 1.0", s.Recall)
		}
	})

	t.Run("all negatives rejected (zero TP division guard)", func(t *testing.T) {
		// All expected=false, none activate → 0 TP, 0 FP, 0 FN, 3 TN
		results := []ActivationResult{
			{Expected: false, WouldActivate: false},
			{Expected: false, WouldActivate: false},
			{Expected: false, WouldActivate: false},
		}
		s := summarizeActivation(results)
		if s.Precision != 0 {
			t.Errorf("Precision: got %.4f, want 0", s.Precision)
		}
		if s.Recall != 0 {
			t.Errorf("Recall: got %.4f, want 0", s.Recall)
		}
		if s.Accuracy != 1.0 {
			t.Errorf("Accuracy: got %.4f, want 1.0", s.Accuracy)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		s := summarizeActivation(nil)
		if s.Total != 0 {
			t.Errorf("Total: got %d, want 0", s.Total)
		}
		if s.Precision != 0 || s.Recall != 0 || s.Accuracy != 0 {
			t.Errorf("expected all zeros, got P=%.2f R=%.2f A=%.2f", s.Precision, s.Recall, s.Accuracy)
		}
	})
}

// --- TestGradeActivation ---

func TestGradeActivation(t *testing.T) {
	tmp := t.TempDir()
	ws := tmp
	iter := 1

	// Stub judge: always returns would_activate=true.
	mockCmd := CmdBuilder(func(ctx context.Context, agent, model, task, skillPath string) *exec.Cmd {
		return exec.Command("echo", `{"would_activate": true, "reason": "looks good"}`)
	})

	cfg := &Config{
		Defaults: DefaultsConfig{Agent: "pi"},
		Judge:    JudgeConfig{},
	}

	ef := &EvalFile{
		Evals: []Eval{
			{ID: 1, Type: "activation", Prompt: "do the thing"},
			{ID: 2, Type: "activation", Prompt: "unrelated thing"},
		},
	}

	// Test the grade+save path directly: judge each activation eval and
	// write activation.json, then verify it can be read back.
	for _, eval := range ef.Evals {
		if !eval.isActivation() {
			continue
		}
		ar, err := judgeActivation(context.Background(), cfg, "test-skill", "A test skill", eval, mockCmd)
		if err != nil {
			t.Fatalf("judgeActivation eval %d: %v", eval.ID, err)
		}

		actPath := filepath.Join(evalPath(ws, iter, eval.ID, ""), "activation.json")
		_ = os.MkdirAll(filepath.Dir(actPath), 0o750)
		if err := saveActivation(actPath, ar); err != nil {
			t.Fatalf("saveActivation eval %d: %v", eval.ID, err)
		}

		// Verify the file exists and can be read back.
		data, err := os.ReadFile(actPath) // #nosec G304 -- test-controlled path from t.TempDir()
		if err != nil {
			t.Fatalf("reading activation.json for eval %d: %v", eval.ID, err)
		}
		if !strings.Contains(string(data), `"would_activate": true`) {
			t.Errorf("eval %d: activation.json missing would_activate\n%s", eval.ID, string(data))
		}

		// Verify it round-trips through JSON.
		var back ActivationResult
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("unmarshaling activation.json for eval %d: %v", eval.ID, err)
		}
		if back.EvalID != eval.ID {
			t.Errorf("eval %d: round-trip EvalID got %d", eval.ID, back.EvalID)
		}
		if !back.WouldActivate {
			t.Errorf("eval %d: round-trip WouldActivate got false", eval.ID)
		}
	}
}

// --- TestReportRendersActivation ---

func TestReportRendersActivation(t *testing.T) {
	data := &ReportData{
		SkillName: "test-skill",
		Iteration: 1,
		Models:    map[string]ModelBenchmark{},
		Activation: &ActivationSummary{
			Total:     5,
			TP:        2,
			FP:        1,
			FN:        1,
			TN:        1,
			Precision: 0.667,
			Recall:    0.667,
			Accuracy:  0.6,
		},
	}

	htmlBytes, err := renderReport(data)
	if err != nil {
		t.Fatalf("renderReport: %v", err)
	}
	html := string(htmlBytes)

	checks := []struct {
		label string
		want  string
	}{
		{"has activation section heading", "Activation Metrics"},
		{"has precision", "Precision"},
		{"has recall", "Recall"},
		{"has accuracy", "Accuracy"},
		{"has TP count", "True Positives"},
		{"has FP count", "False Positives"},
		{"has precision value", "66.7%"},
		{"has total count", "5"},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.want) {
			t.Errorf("%s: HTML missing %q", c.label, c.want)
		}
	}
}

// --- helpers ---

func approxEqual(a, b float64) bool {
	const epsilon = 1e-3
	d := a - b
	return d < epsilon && d > -epsilon
}
