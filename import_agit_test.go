package main

import (
	"strings"
	"testing"
)

// TestImportAgitConvertSession exercises the pure transform from agit step
// data into Evals. No agit binary required.
func TestImportAgitConvertSession(t *testing.T) {
	rows := []agitLogRow{
		{Hash: "aaa", GitCommit: "c0ffee", GitDirty: false, Timestamp: 1719000000},
		{Hash: "bbb", GitCommit: "c0ffee", GitDirty: true, Timestamp: 1719000001},
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

	got := convertSession(steps, diffs, rows, "pi", "test-session-1")

	if len(got) != 2 {
		t.Fatalf("expected 2 evals (chatter skipped), got %d", len(got))
	}

	// IDs are sequential across the task turns only.
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("IDs = %d,%d, want 1,2", got[0].ID, got[1].ID)
	}

	// Eval 1: one added artifact → file_exists, modified → contains_text, plus the LLM assertion.
	e1 := got[0].Eval
	if !containsString(e1.Assertions, "file_exists: results.csv") {
		t.Errorf("eval 1 missing file_exists: results.csv; got %v", e1.Assertions)
	}
	if containsString(e1.Assertions, "file_exists: x.json") {
		t.Errorf("eval 1 should filter dotfile noise; got %v", e1.Assertions)
	}
	// modified README.md should have a contains_text assertion
	if !anyContains(e1.Assertions, "contains_text: README.md:") {
		t.Errorf("eval 1 missing contains_text for modified README.md; got %v", e1.Assertions)
	}
	if !anyContains(e1.Assertions, "matches the recorded reference") {
		t.Errorf("eval 1 missing LLM assertion; got %v", e1.Assertions)
	}
	if !strings.Contains(e1.ExpectedOutput, "committed") {
		t.Errorf("eval 1 expected_output should mention committed; got %q", e1.ExpectedOutput)
	}

	// Source metadata
	if e1.Source == nil {
		t.Fatal("eval 1 missing source metadata")
	}
	if e1.Source.AgitOrigin != "pi" || e1.Source.AgitSessionID != "test-session-1" {
		t.Errorf("eval 1 source = %+v, want origin=pi session=test-session-1", e1.Source)
	}
	if e1.Source.AgitStepHash != "aaa" {
		t.Errorf("eval 1 source step hash = %q, want aaa", e1.Source.AgitStepHash)
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

func TestImportAgitModifiedFilesGetContainsText(t *testing.T) {
	rows := []agitLogRow{{Hash: "z", Timestamp: 1719000000}}
	steps := map[string]agitStep{
		"z": {
			Messages: []agitMessage{
				{Role: "user", Content: "Refactor the auth module to use the new tokenvalidator interface"},
				{Role: "assistant", Content: "Refactored auth.go to use the new TokenValidator interface. Tests pass."},
			},
		},
	}
	diffs := map[string]*agitDiff{
		"z": {Changes: []agitChange{
			{Kind: "modified", Path: "auth.go"}, // modified, not added
		}, Counts: agitCounts{Modified: 1}},
	}

	got := convertSession(steps, diffs, rows, "claude", "s2")
	if len(got) != 1 {
		t.Fatalf("expected 1 eval, got %d", len(got))
	}

	// Should have contains_text for the modified file + LLM fallback
	e := got[0]
	foundContains := false
	for _, a := range e.Assertions {
		if strings.Contains(a, "contains_text: auth.go:") {
			foundContains = true
			break
		}
	}
	if !foundContains {
		t.Errorf("modified file should get contains_text assertion; got %v", e.Assertions)
	}
	// Should still have the LLM assertion
	if !anyContains(e.Assertions, "matches the recorded reference") {
		t.Errorf("eval missing LLM assertion; got %v", e.Assertions)
	}
	// Source metadata
	if e.Source == nil || e.Source.AgitOrigin != "claude" {
		t.Errorf("source = %+v", e.Source)
	}
}

func TestImportAgitShortPromptSkipped(t *testing.T) {
	rows := []agitLogRow{{Hash: "y"}}
	steps := map[string]agitStep{
		"y": {Messages: []agitMessage{{Role: "user", Content: "what the?"}}},
	}
	got := convertSession(steps, nil, rows, "pi", "s3")
	if len(got) != 0 {
		t.Fatalf("chatter turn should be skipped, got %d", len(got))
	}
}

func TestImportAgitNoOpSkipped(t *testing.T) {
	// Turn with a real prompt but zero file changes — should be filtered.
	rows := []agitLogRow{{Hash: "n", Timestamp: 1719000000}}
	steps := map[string]agitStep{
		"n": {
			Messages: []agitMessage{
				{Role: "user", Content: "Look up the current exchange rate for USD to EUR and tell me what it is"},
				{Role: "assistant", Content: "The current rate is 1 USD = 0.92 EUR."},
			},
		},
	}
	diffs := map[string]*agitDiff{
		"n": {Changes: []agitChange{}, Counts: agitCounts{}},
	}
	got := convertSession(steps, diffs, rows, "pi", "s4")
	if len(got) != 0 {
		t.Fatalf("no-op turn (zero file changes) should be skipped, got %d", len(got))
	}
}

func TestImportAgitAcknowledgementSkipped(t *testing.T) {
	rows := []agitLogRow{{Hash: "a", Timestamp: 1719000000}}
	steps := map[string]agitStep{
		"a": {
			Messages: []agitMessage{
				{Role: "user", Content: "thanks, that looks great"},
				{Role: "assistant", Content: "You're welcome!"},
			},
		},
	}
	diffs := map[string]*agitDiff{
		"a": {Changes: []agitChange{
			{Kind: "modified", Path: "README.md"},
		}, Counts: agitCounts{Modified: 1}},
	}
	// Even though there are file changes, the prompt is an acknowledgement
	got := convertSession(steps, diffs, rows, "pi", "s5")
	if len(got) != 0 {
		t.Fatalf("acknowledgement turn should be skipped, got %d evals: %+v", len(got), got)
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

func TestKeyTermFromSummary(t *testing.T) {
	tests := []struct {
		summary string
		want    string
	}{
		{"Added a new foo module for user auth", "a new foo module for user auth"},
		{"Implemented results.csv with monthly totals.", "results.csv with monthly totals."},
		{"Fixed the race condition in runner.go", "the race condition in runner.go"},
		{"Updated README with installation instructions", "README with installation instructions"},
		{"Refactored auth.go to use the new TokenValidator interface.", "auth.go to use the new TokenValidator interface."},
		{"Nothing useful here", ""},
		{"ok", ""},
	}
	for _, tt := range tests {
		got := keyTermFromSummary(tt.summary)
		if got != tt.want {
			t.Errorf("keyTermFromSummary(%q) = %q, want %q", tt.summary, got, tt.want)
		}
	}
}

func TestIsAcknowledgement(t *testing.T) {
	tests := []struct {
		prompt string
		ack    bool
	}{
		{"thanks", true},
		{"thanks, that looks great", true},
		{"ok", true},
		{"ok will do", true},
		{"cool, thanks", true},
		{"great work!", true},
		{"LGTM", true},
		{"sounds good", true},
		{"Add a results.csv summarising the monthly revenue totals please", false},
		{"Refactor the auth module to use the new tokenvalidator interface", false},
		{"Fix the bug in the payment processing pipeline where duplicate charges occur", false},
	}
	for _, tt := range tests {
		got := isAcknowledgement(tt.prompt)
		if got != tt.ack {
			t.Errorf("isAcknowledgement(%q) = %v, want %v", tt.prompt, got, tt.ack)
		}
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
