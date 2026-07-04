package main

import "time"

// EvalFile is the top-level structure of evals/evals.json.
type EvalFile struct {
	SkillName string `json:"skill_name"`
	Evals     []Eval `json:"evals"`
}

// Eval is a single test case.
type Eval struct {
	ID             int         `json:"id"`
	Prompt         string      `json:"prompt"`
	ExpectedOutput string      `json:"expected_output"`
	Files          []string    `json:"files,omitempty"`
	Assertions     []string    `json:"assertions,omitempty"`
	Source         *EvalSource `json:"source,omitempty"`

	// Type is "task" (default, empty) or "activation".
	// Activation evals test skill discovery (does the description trigger
	// for this prompt?) rather than execution quality.
	Type string `json:"type,omitempty"`

	// ShouldActivate is the expected discovery verdict for activation
	// evals. Nil defaults to true (a positive case).
	ShouldActivate *bool `json:"should_activate,omitempty"`
}

// isActivation reports whether an eval tests discovery rather than execution.
func (e Eval) isActivation() bool { return e.Type == "activation" }

// expectedActivation returns the expected verdict, defaulting to true.
func (e Eval) expectedActivation() bool {
	return e.ShouldActivate == nil || *e.ShouldActivate
}

// EvalSource records where an eval came from so failing evals can be
// traced back to the session that produced them.
type EvalSource struct {
	AgitOrigin     string `json:"agit_origin,omitempty"`
	AgitSessionID  string `json:"agit_session_id,omitempty"`
	AgitStepHash   string `json:"agit_step_hash,omitempty"`
	Timestamp      int64  `json:"timestamp,omitempty"`
	EvalHash       string `json:"eval_hash,omitempty"`
	QualityScore   int    `json:"quality_score,omitempty"`
	Classification string `json:"classification,omitempty"`
}

// MatcherType identifies how an assertion should be evaluated.
type MatcherType string

const (
	// MatcherLLM sends the assertion to the configured judge agent.
	MatcherLLM MatcherType = "llm"
	// MatcherFileExists checks for a file in the output directory.
	MatcherFileExists MatcherType = "file_exists"
	// MatcherContainsText checks that a file contains a literal substring.
	MatcherContainsText MatcherType = "contains_text"
	// MatcherMatchesText checks that a file matches a regular expression.
	MatcherMatchesText MatcherType = "matches_text"
)

// ParsedAssertion is the structured form of an assertion string.
type ParsedAssertion struct {
	Original string
	Type     MatcherType
	File     string
	Arg      string
}

// TimingData captured from an agent run.
type TimingData struct {
	TotalTokens int `json:"total_tokens"`
	DurationMs  int `json:"duration_ms"`
}

// AssertionResult is the grading verdict for one assertion.
type AssertionResult struct {
	Text     string `json:"text"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence"`
}

// IterationLock tracks progress for a single iteration.
type IterationLock struct {
	Iteration int           `json:"iteration"`
	Status    string        `json:"status"` // "running" | "complete"
	Completed []RunIdentity `json:"completed"`
	StartedAt time.Time     `json:"started_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// RunIdentity identifies a completed eval/model/config triple.
type RunIdentity struct {
	EvalID int    `json:"eval_id"`
	Model  string `json:"model"`
	Config string `json:"config"`
}

// GradingFile is written to grading.json.
type GradingFile struct {
	AssertionResults []AssertionResult `json:"assertion_results"`
	Summary          GradingSummary    `json:"summary"`
}

// GradingSummary holds the pass/fail counts.
type GradingSummary struct {
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Total    int     `json:"total"`
	PassRate float64 `json:"pass_rate"`
}

// RunResult captures the outcome of a single run (with-skill or baseline).
type RunResult struct {
	EvalID  int
	Model   string // model key (e.g. "pi-claude-sonnet"), empty for single-model compat
	Config  string // "with_skill" or "baseline"
	Status  string // "ok" or "failed"
	Timing  *TimingData
	Grading *GradingFile
}

// RunSummary holds aggregated stats for one configuration across all evals.
type RunSummary struct {
	PassRate    Stats `json:"pass_rate"`
	TimeSeconds Stats `json:"time_seconds"`
	Tokens      Stats `json:"tokens"`
}

// Stats holds mean and stddev.
type Stats struct {
	Mean   float64 `json:"mean"`
	Stddev float64 `json:"stddev"`
}

// FixAttempt records one refinement attempt.
type FixAttempt struct {
	Attempt  int          `json:"attempt"`
	Grading  *GradingFile `json:"grading"`
	Critique string       `json:"critique"`
}

// FixResult records the full fix trajectory for one eval.
type FixResult struct {
	EvalID    int          `json:"eval_id"`
	Attempts  []FixAttempt `json:"attempts"`
	BestFix   int          `json:"best_fix"`
	Converged bool         `json:"converged"`
}

// Delta holds with_skill - baseline differences.
type Delta struct {
	PassRate    float64 `json:"pass_rate"`
	TimeSeconds float64 `json:"time_seconds"`
	Tokens      float64 `json:"tokens"`
}

// ModelBenchmark holds per-model aggregated stats.
type ModelBenchmark struct {
	WithSkill RunSummary `json:"with_skill"`
	Baseline  RunSummary `json:"baseline"`
	Delta     Delta      `json:"delta"`
}

// BenchmarkFile is written to benchmark.json.
type BenchmarkFile struct {
	// RunSummary is the legacy single-model format. Kept only to read
	// benchmark.json files written by older binaries; current writes
	// always populate Models instead.
	RunSummary struct {
		WithSkill RunSummary `json:"with_skill"`
		Baseline  RunSummary `json:"baseline"`
		Delta     Delta      `json:"delta"`
	} `json:"run_summary"`
	Models      map[string]ModelBenchmark `json:"models,omitempty"`
	BestModel   string                    `json:"best_model,omitempty"`
	WorstModel  string                    `json:"worst_model,omitempty"`
	GeneratedAt time.Time                 `json:"generated_at"`

	// New fields
	PreviousIteration int    `json:"previous_iteration,omitempty"`
	IterationDelta    *Delta `json:"iteration_delta,omitempty"`

	// Activation holds the skill-discovery metrics (precision/recall)
	// when activation evals were graded in this iteration.
	Activation *ActivationSummary `json:"activation,omitempty"`
}

// ActivationResult is the judged discovery verdict for one activation eval.
type ActivationResult struct {
	EvalID        int    `json:"eval_id"`
	Expected      bool   `json:"expected"`
	WouldActivate bool   `json:"would_activate"`
	Reason        string `json:"reason"`
}

// ActivationSummary aggregates activation verdicts for a benchmark.
type ActivationSummary struct {
	Total     int     `json:"total"`
	TP        int     `json:"tp"` // expected=true,  verdict=yes
	FP        int     `json:"fp"` // expected=false, verdict=yes
	FN        int     `json:"fn"` // expected=true,  verdict=no
	TN        int     `json:"tn"` // expected=false, verdict=no
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	Accuracy  float64 `json:"accuracy"`
}
