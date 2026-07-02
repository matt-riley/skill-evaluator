package agit

import (
	"strings"
	"testing"
)

// TestConvertSession exercises the pure transform from agit step
// data into ConvertedEvals. No agit binary required.
func TestConvertSession(t *testing.T) {
	rows := []LogRow{
		{Hash: "aaa", GitCommit: "c0ffee", GitDirty: false, Timestamp: 1719000000},
		{Hash: "bbb", GitCommit: "c0ffee", GitDirty: true, Timestamp: 1719000001},
		{Hash: "ccc"}, // short prompt → chatter, skipped
	}

	steps := map[string]Step{
		"aaa": {
			Messages: []Message{
				{Role: "user", Content: "Add a results.csv summarising the monthly revenue totals please"},
				{Role: "assistant", Content: "Implemented results.csv with monthly totals. Verified with zig build test."},
			},
			GitCommit: "c0ffee",
			GitDirty:  false,
			Outcome:   "success",
		},
		"bbb": {
			Messages: []Message{
				{Role: "user", Content: "Now also generate a chart.png bar chart from that data for the dashboard"},
				{Role: "assistant", Content: "Added chart.png. The bars use a sensible palette."},
			},
			GitCommit: "c0ffee",
			GitDirty:  true,
		},
		"ccc": {
			Messages: []Message{
				{Role: "user", Content: "ok thanks"},
				{Role: "assistant", Content: "Done"},
			},
		},
	}

	diffs := map[string]*Diff{
		"aaa": {Changes: []Change{
			{Kind: "added", Path: "results.csv"},
			{Kind: "modified", Path: "README.md"},
			{Kind: "added", Path: ".agit/tmp/x.json"}, // filtered: dotfile
		}, Counts: Counts{Added: 2, Modified: 1}},
		"bbb": {Changes: []Change{
			{Kind: "added", Path: "chart.png"},
		}, Counts: Counts{Added: 1}},
	}

	got := ConvertSession(steps, diffs, rows, "pi", "test-session-1")

	if len(got) != 2 {
		t.Fatalf("expected 2 evals (chatter skipped), got %d", len(got))
	}

	// IDs are sequential across the task turns only.
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("IDs = %d,%d, want 1,2", got[0].ID, got[1].ID)
	}

	// Eval 1: one added artifact → file_exists, modified → contains_text, plus the LLM assertion.
	e1 := got[0]
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
	if e1.Source.Origin != "pi" || e1.Source.SessionID != "test-session-1" {
		t.Errorf("eval 1 source = %+v, want origin=pi session=test-session-1", e1.Source)
	}
	if e1.Source.StepHash != "aaa" {
		t.Errorf("eval 1 source step hash = %q, want aaa", e1.Source.StepHash)
	}

	// Eval 2: dirty commit.
	e2 := got[1]
	if !containsString(e2.Assertions, "file_exists: chart.png") {
		t.Errorf("eval 2 missing file_exists: chart.png; got %v", e2.Assertions)
	}
	if !strings.Contains(e2.ExpectedOutput, "committed then left dirty") {
		t.Errorf("eval 2 expected_output should mention dirty; got %q", e2.ExpectedOutput)
	}
}

func TestModifiedFilesGetContainsText(t *testing.T) {
	rows := []LogRow{{Hash: "z", Timestamp: 1719000000}}
	steps := map[string]Step{
		"z": {
			Messages: []Message{
				{Role: "user", Content: "Refactor the auth module to use the new tokenvalidator interface"},
				{Role: "assistant", Content: "Refactored auth.go to use the new TokenValidator interface. Tests pass."},
			},
		},
	}
	diffs := map[string]*Diff{
		"z": {Changes: []Change{
			{Kind: "modified", Path: "auth.go"}, // modified, not added
		}, Counts: Counts{Modified: 1}},
	}

	got := ConvertSession(steps, diffs, rows, "claude", "s2")
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
	if e.Source.Origin != "claude" {
		t.Errorf("source = %+v", e.Source)
	}
}

func TestShortPromptSkipped(t *testing.T) {
	rows := []LogRow{{Hash: "y"}}
	steps := map[string]Step{
		"y": {Messages: []Message{{Role: "user", Content: "what the?"}}},
	}
	got := ConvertSession(steps, nil, rows, "pi", "s3")
	if len(got) != 0 {
		t.Fatalf("chatter turn should be skipped, got %d", len(got))
	}
}

func TestNoOpSkipped(t *testing.T) {
	// Turn with a real prompt but zero file changes — should be filtered.
	rows := []LogRow{{Hash: "n", Timestamp: 1719000000}}
	steps := map[string]Step{
		"n": {
			Messages: []Message{
				{Role: "user", Content: "Look up the current exchange rate for USD to EUR and tell me what it is"},
				{Role: "assistant", Content: "The current rate is 1 USD = 0.92 EUR."},
			},
		},
	}
	diffs := map[string]*Diff{
		"n": {Changes: []Change{}, Counts: Counts{}},
	}
	got := ConvertSession(steps, diffs, rows, "pi", "s4")
	if len(got) != 0 {
		t.Fatalf("no-op turn (zero file changes) should be skipped, got %d", len(got))
	}
}

func TestAcknowledgementSkipped(t *testing.T) {
	rows := []LogRow{{Hash: "a", Timestamp: 1719000000}}
	steps := map[string]Step{
		"a": {
			Messages: []Message{
				{Role: "user", Content: "thanks, that looks great"},
				{Role: "assistant", Content: "You're welcome!"},
			},
		},
	}
	diffs := map[string]*Diff{
		"a": {Changes: []Change{
			{Kind: "modified", Path: "README.md"},
		}, Counts: Counts{Modified: 1}},
	}
	// Even though there are file changes, the prompt is an acknowledgement
	got := ConvertSession(steps, diffs, rows, "pi", "s5")
	if len(got) != 0 {
		t.Fatalf("acknowledgement turn should be skipped, got %d evals: %+v", len(got), got)
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

// --- Tests for agit steps integration (Phase 1) ---

func TestConvertStepsFromStepsJSON(t *testing.T) {
	// Simulate agit steps --json output.
	steps := &Steps{
		Origin:    "codex",
		SessionID: "test-session-1",
		Steps: []StepRow{
			{
				Hash:      "aaa",
				TurnID:    "turn-1",
				Timestamp: 1719000000,
				Model:     "claude-sonnet-4",
				Outcome:   "success",
				GitCommit: "c0ffee01d",
				GitBranch: "feature/x",
				GitDirty:  false,
				Step: &Step{
					Messages: []Message{
						{Role: "user", Content: "Add a results.csv summarising the monthly revenue totals please"},
						{Role: "assistant", Content: "Implemented results.csv with monthly totals. Verified with zig build test."},
					},
					GitCommit: "c0ffee01d",
					GitDirty:  false,
					Outcome:   "success",
				},
				Diff: &Diff{
					Changes: []Change{
						{Kind: "added", Path: "results.csv"},
						{Kind: "modified", Path: "README.md"},
						{Kind: "added", Path: ".agit/tmp/x.json"},
					},
					Counts: Counts{Added: 2, Modified: 1},
				},
			},
			{
				Hash:      "bbb",
				TurnID:    "turn-2",
				Timestamp: 1719000001,
				Model:     "claude-sonnet-4",
				GitCommit: "c0ffee01d",
				GitDirty:  true,
				Step: &Step{
					Messages: []Message{
						{Role: "user", Content: "Now also generate a chart.png bar chart from that data for the dashboard"},
						{Role: "assistant", Content: "Added chart.png. The bars use a sensible palette."},
					},
				},
				Diff: &Diff{
					Changes: []Change{{Kind: "added", Path: "chart.png"}},
					Counts:  Counts{Added: 1},
				},
			},
			{
				Hash: "ccc",
				Step: &Step{
					Messages: []Message{
						{Role: "user", Content: "ok thanks"},
						{Role: "assistant", Content: "Done"},
					},
				},
			},
		},
	}

	got := ConvertSteps(steps, nil, nil)

	if len(got) != 2 {
		t.Fatalf("expected 2 evals (chatter skipped), got %d", len(got))
	}

	// Check IDs
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("IDs = %d,%d, want 1,2", got[0].ID, got[1].ID)
	}

	// Check source metadata enriched from steps --json
	e1 := got[0]
	if e1.Source.Origin != "codex" || e1.Source.SessionID != "test-session-1" {
		t.Errorf("eval 1 source origin/session = %s/%s, want codex/test-session-1", e1.Source.Origin, e1.Source.SessionID)
	}

	// Check expanded expected output includes model and branch
	if !strings.Contains(e1.ExpectedOutput, "claude-sonnet-4") {
		t.Errorf("eval 1 expected_output should mention model; got %q", e1.ExpectedOutput)
	}
	if !strings.Contains(e1.ExpectedOutput, "feature/x") {
		t.Errorf("eval 1 expected_output should mention branch; got %q", e1.ExpectedOutput)
	}
	if !strings.Contains(e1.ExpectedOutput, "committed") {
		t.Errorf("eval 1 expected_output should mention committed; got %q", e1.ExpectedOutput)
	}

	// Assertions: file_exists for added + contains_text for modified + LLM
	if !containsString(e1.Assertions, "file_exists: results.csv") {
		t.Errorf("eval 1 missing file_exists: results.csv; got %v", e1.Assertions)
	}
	if containsString(e1.Assertions, "file_exists: x.json") {
		t.Errorf("eval 1 should filter dotfile noise; got %v", e1.Assertions)
	}

	// Eval 2: dirty commit
	e2 := got[1]
	if !strings.Contains(e2.ExpectedOutput, "committed then left dirty") {
		t.Errorf("eval 2 expected_output should mention dirty; got %q", e2.ExpectedOutput)
	}
}

func TestConvertStepsWithEvalFilter(t *testing.T) {
	steps := &Steps{
		Origin:    "codex",
		SessionID: "bad-session",
		Steps: []StepRow{
			{
				Hash: "zzz",
				Step: &Step{
					Messages: []Message{
						{Role: "user", Content: "Write a function that adds two numbers together and returns the result"},
						{Role: "assistant", Content: "Done. Wrote add function."},
					},
				},
				Diff: &Diff{
					Changes: []Change{{Kind: "added", Path: "math.go"}},
					Counts:  Counts{Added: 1},
				},
			},
		},
	}

	// Eval classified as "bad" — should be filtered when filter=good,mixed
	ae := &EvalReport{
		EvalHash: "abc123",
		InScopeAssessment: Assessment{
			Classification: "bad",
			Confidence:     "high",
			Dimensions: &Dimensions{
				ChurnRisk:        DimensionReport{Rating: "bad", Score: 80, Signals: Signals{RepeatedCommands: 5}},
				GoalClarity:      DimensionReport{Score: 30},
				ExecutionFocus:   DimensionReport{Score: 20},
				FailureRecovery:  DimensionReport{Score: 10},
				Verification:     DimensionReport{Score: 15},
				CompletionSignal: DimensionReport{Score: 10},
			},
		},
	}

	filter := map[string]bool{"good": true, "mixed": true}
	got := ConvertSteps(steps, ae, filter)
	if len(got) != 0 {
		t.Fatalf("bad session should be filtered when filter=good,mixed, got %d", len(got))
	}

	// Without filter, should pass through
	got = ConvertSteps(steps, nil, nil)
	if len(got) != 1 {
		t.Fatalf("without filter, bad session should pass through, got %d", len(got))
	}
}

func TestEvalQualityScore(t *testing.T) {
	dims := &Dimensions{
		GoalClarity:      DimensionReport{Score: 80},
		ExecutionFocus:   DimensionReport{Score: 70},
		FailureRecovery:  DimensionReport{Score: 60},
		Verification:     DimensionReport{Score: 90},
		CompletionSignal: DimensionReport{Score: 75},
		ChurnRisk:        DimensionReport{Score: 30}, // inverted: 100-30=70
	}
	got := EvalQualityScore(dims)
	// (80+70+60+90+75+70)/6 = 445/6 = 74 (integer division)
	want := (80 + 70 + 60 + 90 + 75 + 70) / 6
	if got != want {
		t.Errorf("EvalQualityScore = %d, want %d", got, want)
	}

	if EvalQualityScore(nil) != 0 {
		t.Error("EvalQualityScore on nil should return 0")
	}
}

func TestParseEvalFilter(t *testing.T) {
	tests := []struct {
		raw  string
		size int
	}{
		{"", 0},
		{"good", 1},
		{"good,mixed", 2},
		{"good,mixed,bad,unknown", 4},
		{"good, ,mixed", 2}, // whitespace trimmed
	}
	for _, tt := range tests {
		f := ParseEvalFilter(tt.raw)
		if tt.size == 0 {
			if f != nil {
				t.Errorf("ParseEvalFilter(%q) should be nil, got %v", tt.raw, f)
			}
		} else if f == nil || len(f) != tt.size {
			t.Errorf("ParseEvalFilter(%q) = %v, want %d entries", tt.raw, f, tt.size)
		}
	}

	f := ParseEvalFilter("good,mixed")
	if !f["good"] || !f["mixed"] {
		t.Errorf("ParseEvalFilter(\"good,mixed\") should contain good and mixed, got %v", f)
	}
}

func TestBuildExpectedOutputSteps(t *testing.T) {
	row := StepRow{
		Model:     "gpt-5",
		GitCommit: "abcd1234",
		GitBranch: "feature/foo",
		GitDirty:  false,
		Outcome:   "success",
	}
	step := Step{
		Messages: []Message{
			{Role: "assistant", Content: "Implemented the feature."},
		},
	}
	diff := &Diff{
		Changes: []Change{{Kind: "added", Path: "foo.go"}},
		Counts:  Counts{Added: 1},
	}

	out := buildExpectedOutputSteps("Implemented the feature.", row, step, diff)

	if !strings.Contains(out, "Model: gpt-5") {
		t.Errorf("expected model in output, got %q", out)
	}
	if !strings.Contains(out, "Branch: feature/foo") {
		t.Errorf("expected branch in output, got %q", out)
	}
	if !strings.Contains(out, "committed (abcd1234)") {
		t.Errorf("expected commit hash in output, got %q", out)
	}
}

func TestBuildAssertionsWithSignals(t *testing.T) {
	diff := &Diff{
		Changes: []Change{
			{Kind: "added", Path: "test.go"},
		},
		Counts: Counts{Added: 1},
	}
	assistant := "Added test.go with unit tests."

	// Without eval, should produce file_exists + LLM assertion.
	got := buildAssertionsWithSignals(diff, assistant, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 assertions (file_exists + LLM), got %d: %v", len(got), got)
	}

	// With high churn risk eval, assertions might be reduced.
	ae := &EvalReport{
		InScopeAssessment: Assessment{
			Dimensions: &Dimensions{
				ChurnRisk: DimensionReport{
					Rating:  "bad",
					Score:   85,
					Signals: Signals{RepeatedCommands: 10},
				},
				GoalClarity:      DimensionReport{Score: 50},
				ExecutionFocus:   DimensionReport{Score: 50},
				FailureRecovery:  DimensionReport{Score: 50},
				Verification:     DimensionReport{Score: 50},
				CompletionSignal: DimensionReport{Score: 50},
			},
		},
	}

	got = buildAssertionsWithSignals(diff, assistant, ae)
	// High churn reduces to max 2 assertions, file_exists + the base still produces them
	// The reduction caps at 2, and base already has 2, so same count
	if len(got) == 0 {
		t.Fatal("expected at least 1 assertion even with high churn")
	}
}

func TestConvertStepsSourceHasEvalMetadata(t *testing.T) {
	steps := &Steps{
		Origin:    "pi",
		SessionID: "s1",
		Steps: []StepRow{
			{
				Hash:      "eee",
				TurnID:    "turn-1",
				Timestamp: 1719000000,
				Outcome:   "success",
				Step: &Step{
					Messages: []Message{
						{Role: "user", Content: "Write a helper function to validate email addresses"},
						{Role: "assistant", Content: "Added email validation helper."},
					},
					Outcome: "success",
				},
				Diff: &Diff{
					Changes: []Change{{Kind: "added", Path: "validate.go"}},
					Counts:  Counts{Added: 1},
				},
			},
		},
	}

	ae := &EvalReport{
		EvalHash: "def456",
		InScopeAssessment: Assessment{
			Classification: "good",
			Confidence:     "high",
			Dimensions: &Dimensions{
				GoalClarity:      DimensionReport{Score: 85},
				ExecutionFocus:   DimensionReport{Score: 90},
				FailureRecovery:  DimensionReport{Score: 75},
				Verification:     DimensionReport{Score: 80},
				CompletionSignal: DimensionReport{Score: 90},
				ChurnRisk:        DimensionReport{Score: 10}, // low churn = good
			},
		},
	}

	got := ConvertSteps(steps, ae, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 eval, got %d", len(got))
	}

	e := got[0]
	if e.Source.EvalHash != "def456" {
		t.Errorf("EvalHash = %q, want def456", e.Source.EvalHash)
	}
	if e.Source.Classification != "good" {
		t.Errorf("Classification = %q, want good", e.Source.Classification)
	}
	if e.Source.QualityScore == 0 {
		t.Errorf("QualityScore should be > 0, got %d", e.Source.QualityScore)
	}
}

func TestParseEvalFilterWithOrigin(t *testing.T) {
	// Verify that ParseEvalFilter doesn't interact with origin filtering.
	// Origin filtering happens at the session level before eval filtering.
	filter := ParseEvalFilter("good")
	if filter == nil || !filter["good"] {
		t.Error("ParseEvalFilter should work independently of origin filtering")
	}
}
