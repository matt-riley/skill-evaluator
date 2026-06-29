package main

import (
	"reflect"
	"strings"
	"testing"
)

// TestImportAgitConvertSession exercises the pure transform from agit step
// data into Evals. No agit binary required.
func TestImportAgitConvertSession(t *testing.T) {
	rows := []agitLogRow{
		{Hash: "aaa", GitCommit: "c0ffee", GitDirty: false},
		{Hash: "bbb", GitCommit: "c0ffee", GitDirty: true},
		{Hash: "ccc"}, // short prompt → chatter, skipped
	}

	steps := map[string]agitStep{
		"aaa": {
			Messages: []agitMessage{
				{Role: "user", Content: "Add a results.csv summarising the monthly revenue totals please"},
				{Role: "assistant", Content: "Implemented results.csv with monthly totals. Verified with zig build test."},
			},
			GitCommit: "c0ffee",
			GitDirty:  false,
			Outcome:   "success",
		},
		"bbb": {
			Messages: []agitMessage{
				{Role: "user", Content: "Now also generate a chart.png bar chart from that data for the dashboard"},
				{Role: "assistant", Content: "Added chart.png. The bars use a sensible palette."},
			},
			GitCommit: "c0ffee",
			GitDirty:  true,
		},
		"ccc": {
			Messages: []agitMessage{
				{Role: "user", Content: "ok thanks"},
				{Role: "assistant", Content: "Done"},
			},
		},
	}

	diffs := map[string]*agitDiff{
		"aaa": {Changes: []agitChange{
			{Kind: "added", Path: "results.csv"},
			{Kind: "modified", Path: "README.md"},
			{Kind: "added", Path: ".agit/tmp/x.json"}, // filtered: dotfile
		}, Counts: agitCounts{Added: 2, Modified: 1}},
		"bbb": {Changes: []agitChange{
			{Kind: "added", Path: "chart.png"},
		}, Counts: agitCounts{Added: 1}},
	}

	got := convertSession(steps, diffs, rows)

	if len(got) != 2 {
		t.Fatalf("expected 2 evals (chatter skipped), got %d", len(got))
	}

	// IDs are sequential across the task turns only.
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("IDs = %d,%d, want 1,2", got[0].ID, got[1].ID)
	}

	// Eval 1: one added artifact → file_exists, plus the LLM assertion.
	e1 := got[0].Eval
	if !containsString(e1.Assertions, "file_exists: results.csv") {
		t.Errorf("eval 1 missing file_exists: results.csv; got %v", e1.Assertions)
	}
	if containsString(e1.Assertions, "file_exists: x.json") {
		t.Errorf("eval 1 should filter dotfile noise; got %v", e1.Assertions)
	}
	if !anyContains(e1.Assertions, "correctly implements") {
		t.Errorf("eval 1 missing LLM assertion; got %v", e1.Assertions)
	}
	if !strings.Contains(e1.ExpectedOutput, "committed") {
		t.Errorf("eval 1 expected_output should mention committed; got %q", e1.ExpectedOutput)
	}

	// Eval 2: dirty commit.
	e2 := got[1].Eval
	if !containsString(e2.Assertions, "file_exists: chart.png") {
		t.Errorf("eval 2 missing file_exists: chart.png; got %v", e2.Assertions)
	}
	if !strings.Contains(e2.ExpectedOutput, "committed then left dirty") {
		t.Errorf("eval 2 expected_output should mention dirty; got %q", e2.ExpectedOutput)
	}
}

func TestImportAgitNoAddedFilesFallsBackToLLM(t *testing.T) {
	rows := []agitLogRow{{Hash: "z"}}
	steps := map[string]agitStep{
		"z": {
			Messages: []agitMessage{
				{Role: "user", Content: "Refactor the auth module to use the new tokenvalidator interface"},
				{Role: "assistant", Content: "Refactored auth.go. Tests pass."},
			},
		},
	}
	diffs := map[string]*agitDiff{
		"z": {Changes: []agitChange{
			{Kind: "modified", Path: "auth.go"}, // modified, not added → no file_exists
		}},
	}

	got := convertSession(steps, diffs, rows)
	if len(got) != 1 {
		t.Fatalf("expected 1 eval, got %d", len(got))
	}
	if len(got[0].Assertions) == 0 {
		t.Fatalf("expected at least a fallback LLM assertion")
	}
	for _, a := range got[0].Assertions {
		if strings.Contains(a, "file_exists") {
			t.Errorf("no added files should yield no file_exists assertion; got %q", a)
		}
	}
	if !reflect.DeepEqual(got[0].Assertions, []string{llmAssertion("")}) {
		t.Errorf("expected single LLM fallback, got %v", got[0].Assertions)
	}
}

func TestImportAgitShortPromptSkipped(t *testing.T) {
	rows := []agitLogRow{{Hash: "y"}}
	steps := map[string]agitStep{
		"y": {Messages: []agitMessage{{Role: "user", Content: "what the?"}}},
	}
	got := convertSession(steps, nil, rows)
	if len(got) != 0 {
		t.Fatalf("chatter turn should be skipped, got %d", len(got))
	}
}

func TestDecodeEnvelope(t *testing.T) {
	raw := []byte(`{"schema_version":"cli-json-v1","command":"log","data":{"origin":"pi","session_id":"s1","steps":[{"hash":"x"}]}}`)
	got, err := decodeEnvelope[agitLog](raw)
	if err != nil {
		t.Fatalf("decodeEnvelope error: %v", err)
	}
	if got.Origin != "pi" || got.SessionID != "s1" || len(got.Steps) != 1 || got.Steps[0].Hash != "x" {
		t.Errorf("decoded wrong: %+v", got)
	}
}

func containsString(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}

func anyContains(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
