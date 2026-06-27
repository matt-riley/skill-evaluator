package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCmdReportGeneratesHTML(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	evalsDir := filepath.Join(skillDir, "evals")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatalf("mkdir evals: %v", err)
	}
	evalsJSON := `{"skill_name":"test-skill","evals":[{"id":1,"prompt":"p","expected_output":"e","assertions":["file_exists:x"]}]}`
	if err := os.WriteFile(filepath.Join(evalsDir, "evals.json"), []byte(evalsJSON), 0o644); err != nil {
		t.Fatalf("write evals.json: %v", err)
	}

	ws := workspacePath(skillDir)
	iter := 1
	if err := os.MkdirAll(iterationPath(ws, iter), 0o755); err != nil {
		t.Fatalf("mkdir iteration: %v", err)
	}

	bf := &BenchmarkFile{
		Models: map[string]ModelBenchmark{
			"pi-deepseek/deepseek-v4-flash": {
				WithSkill: RunSummary{PassRate: Stats{Mean: 0.9}, TimeSeconds: Stats{Mean: 10}, Tokens: Stats{Mean: 50}},
				Baseline:  RunSummary{PassRate: Stats{Mean: 0.7}, TimeSeconds: Stats{Mean: 8}, Tokens: Stats{Mean: 40}},
				Delta:     Delta{PassRate: 0.2, TimeSeconds: 2, Tokens: 10},
			},
			"pi-neuralwatt/glm-5.2-flex": {
				WithSkill: RunSummary{PassRate: Stats{Mean: 0.4}, TimeSeconds: Stats{Mean: 12}, Tokens: Stats{Mean: 0}},
				Baseline:  RunSummary{PassRate: Stats{Mean: 0.5}, TimeSeconds: Stats{Mean: 10}, Tokens: Stats{Mean: 0}},
				Delta:     Delta{PassRate: -0.1, TimeSeconds: 2, Tokens: 0},
			},
		},
		BestModel:         "pi-deepseek/deepseek-v4-flash",
		WorstModel:        "pi-neuralwatt/glm-5.2-flex",
		GeneratedAt:       time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		PreviousIteration: 0,
		IterationDelta:    nil,
	}
	data, _ := json.MarshalIndent(bf, "", "  ")
	if err := os.WriteFile(filepath.Join(iterationPath(ws, iter), "benchmark.json"), data, 0o644); err != nil {
		t.Fatalf("write benchmark: %v", err)
	}

	t.Chdir(skillDir)

	if err := cmdReport(nil); err != nil {
		t.Fatalf("cmdReport failed: %v", err)
	}

	reportPath := filepath.Join(iterationPath(ws, iter), "report.html")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report.html: %v", err)
	}
	body := string(report)

	for _, want := range []string{
		"test-skill",
		"Iteration 1",
		"pi-deepseek/deepseek-v4-flash",
		"pi-neuralwatt/glm-5.2-flex",
		"90.0%",
		"20.0%",
		"Best performer was",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("report missing expected content %q\n%s", want, body)
		}
	}
}

func TestBuildSuggestionsHandlesEmptyData(t *testing.T) {
	suggestions := buildSuggestions(&ReportData{Models: nil})
	if len(suggestions) != 1 || !strings.Contains(suggestions[0], "No benchmark data") {
		t.Fatalf("unexpected suggestions for empty data: %v", suggestions)
	}
}

func TestBuildSuggestionsHighBaselineWarning(t *testing.T) {
	data := &ReportData{
		Models: map[string]ModelBenchmark{
			"m1": {
				WithSkill: RunSummary{PassRate: Stats{Mean: 0.98}},
				Baseline:  RunSummary{PassRate: Stats{Mean: 0.97}},
				Delta:     Delta{PassRate: 0.01},
			},
		},
	}
	suggestions := buildSuggestions(data)
	got := strings.Join(suggestions, " ")
	if !strings.Contains(got, "Baseline already passes nearly all evals") {
		t.Fatalf("expected high-baseline warning, got: %v", suggestions)
	}
}

func TestBuildSuggestionsRegressionWarning(t *testing.T) {
	data := &ReportData{
		Models: map[string]ModelBenchmark{
			"m1": {
				WithSkill: RunSummary{PassRate: Stats{Mean: 0.4}},
				Baseline:  RunSummary{PassRate: Stats{Mean: 0.6}},
				Delta:     Delta{PassRate: -0.2},
			},
		},
		PreviousIteration: 1,
		IterationDelta:    &Delta{PassRate: -0.1, TimeSeconds: 1, Tokens: 0},
	}
	suggestions := buildSuggestions(data)
	got := strings.Join(suggestions, " ")
	for _, want := range []string{"regressed", "10.0%", "hurts m1", "20.0% drop"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing suggestion %q in %v", want, suggestions)
		}
	}
}
