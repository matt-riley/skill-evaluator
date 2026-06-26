package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraderParseGradingOutput(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		assertions []string
		wantPass   int
		wantErr    bool
	}{
		{
			name:       "plain json output",
			output:     `{"assertion_results": [{"text": "a", "passed": true, "evidence": "ok"}]}`,
			assertions: []string{"a"},
			wantPass:   1,
		},
		{
			name:     "markdown fenced json",
			output:   "Some explanation\n```json\n{\"assertion_results\": [{\"text\": \"a\", \"passed\": false, \"evidence\": \"missing\"}]}\n```\n",
			wantPass: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gf, err := parseGradingOutput(tt.output, tt.assertions)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			passed := 0
			for _, ar := range gf.AssertionResults {
				if ar.Passed {
					passed++
				}
			}
			if passed != tt.wantPass {
				t.Errorf("passed = %d, want %d", passed, tt.wantPass)
			}
		})
	}

	t.Run("no json found", func(t *testing.T) {
		_, err := parseGradingOutput("just text", []string{"a"})
		if err == nil || !strings.Contains(err.Error(), "no JSON") {
			t.Errorf("expected no JSON error, got %v", err)
		}
	})
}

func TestGraderExtractFailedReasoning(t *testing.T) {
	t.Run("concatenates failed evidence", func(t *testing.T) {
		gf := &GradingFile{
			AssertionResults: []AssertionResult{
				{Passed: true, Evidence: "ok"},
				{Passed: false, Evidence: "missing X"},
				{Passed: false, Evidence: "missing Y"},
			},
		}
		got := extractFailedReasoning(gf)
		want := "missing X\n- missing Y"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty when all pass", func(t *testing.T) {
		gf := &GradingFile{
			AssertionResults: []AssertionResult{
				{Passed: true, Evidence: "ok"},
			},
		}
		if got := extractFailedReasoning(gf); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestParseAssertion(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want ParsedAssertion
	}{
		{
			name: "file_exists",
			s:    "file_exists: results.csv",
			want: ParsedAssertion{Original: "file_exists: results.csv", Type: MatcherFileExists, File: "results.csv"},
		},
		{
			name: "contains_text",
			s:    "contains_text: summary.txt:Total revenue",
			want: ParsedAssertion{Original: "contains_text: summary.txt:Total revenue", Type: MatcherContainsText, File: "summary.txt", Arg: "Total revenue"},
		},
		{
			name: "matches_text",
			s:    "matches_text: output.md:^## Summary$",
			want: ParsedAssertion{Original: "matches_text: output.md:^## Summary$", Type: MatcherMatchesText, File: "output.md", Arg: "^## Summary$"},
		},
		{
			name: "llm fallback",
			s:    "The chart uses a sensible color palette",
			want: ParsedAssertion{Original: "The chart uses a sensible color palette", Type: MatcherLLM},
		},
		{
			name: "malformed contains_text falls back to llm",
			s:    "contains_text: summary.txt",
			want: ParsedAssertion{Original: "contains_text: summary.txt", Type: MatcherLLM},
		},
		{
			name: "trims whitespace",
			s:    "  file_exists:  results.csv  ",
			want: ParsedAssertion{Original: "file_exists:  results.csv", Type: MatcherFileExists, File: "results.csv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAssertion(tt.s)
			if got != tt.want {
				t.Errorf("parseAssertion(%q) = %+v, want %+v", tt.s, got, tt.want)
			}
		})
	}
}

func TestMatcherFileExists(t *testing.T) {
	tmp := t.TempDir()
	contents := map[string]string{}

	_ = os.WriteFile(filepath.Join(tmp, "results.csv"), []byte("ok"), 0o644)

	got := evaluateMatcher(parseAssertion("file_exists: results.csv"), tmp, contents)
	if !got.Passed || got.Evidence != "file results.csv exists" {
		t.Errorf("present file: got %+v", got)
	}

	got = evaluateMatcher(parseAssertion("file_exists: missing.txt"), tmp, contents)
	if got.Passed || got.Evidence != "file missing.txt does not exist" {
		t.Errorf("missing file: got %+v", got)
	}
}

func TestMatcherContainsText(t *testing.T) {
	tmp := t.TempDir()
	contents := map[string]string{"summary.txt": "Total revenue: 100\n"}

	got := evaluateMatcher(parseAssertion("contains_text: summary.txt:Total revenue"), tmp, contents)
	if !got.Passed || !strings.Contains(got.Evidence, "contains") {
		t.Errorf("contains match: got %+v", got)
	}

	got = evaluateMatcher(parseAssertion("contains_text: summary.txt:Net profit"), tmp, contents)
	if got.Passed || !strings.Contains(got.Evidence, "does not contain") {
		t.Errorf("contains miss: got %+v", got)
	}

	got = evaluateMatcher(parseAssertion("contains_text: missing.txt:Total revenue"), tmp, contents)
	if got.Passed || !strings.Contains(got.Evidence, "not found") {
		t.Errorf("missing file: got %+v", got)
	}
}

func TestMatcherMatchesText(t *testing.T) {
	tmp := t.TempDir()
	contents := map[string]string{"output.md": "## Summary\n"}

	got := evaluateMatcher(parseAssertion("matches_text: output.md:^## Summary"), tmp, contents)
	if !got.Passed || !strings.Contains(got.Evidence, "matches") {
		t.Errorf("regex match: got %+v", got)
	}

	got = evaluateMatcher(parseAssertion("matches_text: output.md:^[0-9]+$"), tmp, contents)
	if got.Passed || !strings.Contains(got.Evidence, "does not match") {
		t.Errorf("regex miss: got %+v", got)
	}

	got = evaluateMatcher(parseAssertion("matches_text: output.md:(?P<broken"), tmp, contents)
	if got.Passed || !strings.Contains(got.Evidence, "invalid regex") {
		t.Errorf("invalid regex: got %+v", got)
	}
}

func TestGradeMixedMatchers(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "outputs")
	gradingPath := filepath.Join(tmp, "grading.json")
	_ = os.MkdirAll(outDir, 0o755)
	_ = os.WriteFile(filepath.Join(outDir, "results.csv"), []byte("ok"), 0o644)

	orig := buildAgentCmd
	buildAgentCmd = func(agent, model, task, skillPath string) *exec.Cmd {
		return exec.Command("echo", `{"assertion_results": [{"text": "The output is useful", "passed": true, "evidence": "looks good"}]}`)
	}
	defer func() { buildAgentCmd = orig }()

	cfg := &Config{
		Defaults: DefaultsConfig{Agent: "pi"},
		Judge:    JudgeConfig{},
	}
	eval := Eval{
		ID: 1,
		Assertions: []string{
			"file_exists: results.csv",
			"The output is useful",
		},
	}

	gf, err := gradeFromOutput(context.Background(), cfg, eval, outDir, gradingPath, "eval 1 (with_skill)")
	if err != nil {
		t.Fatalf("gradeFromOutput error: %v", err)
	}

	if len(gf.AssertionResults) != 2 {
		t.Fatalf("expected 2 results, got %d", len(gf.AssertionResults))
	}
	if !gf.AssertionResults[0].Passed {
		t.Errorf("deterministic assertion should pass, got %+v", gf.AssertionResults[0])
	}
	if gf.AssertionResults[0].Evidence != "file results.csv exists" {
		t.Errorf("deterministic evidence = %q", gf.AssertionResults[0].Evidence)
	}
	if !gf.AssertionResults[1].Passed {
		t.Errorf("llm assertion should pass, got %+v", gf.AssertionResults[1])
	}
	if gf.Summary.Total != 2 || gf.Summary.Passed != 2 || gf.Summary.Failed != 0 {
		t.Errorf("summary = %+v", gf.Summary)
	}
}
