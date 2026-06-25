package main

import (
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
