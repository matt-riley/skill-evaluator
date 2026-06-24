package main

import "time"

// EvalFile is the top-level structure of evals/evals.json.
type EvalFile struct {
	SkillName string `json:"skill_name"`
	Evals     []Eval `json:"evals"`
}

// Eval is a single test case.
type Eval struct {
	ID             int      `json:"id"`
	Prompt         string   `json:"prompt"`
	ExpectedOutput string   `json:"expected_output"`
	Files          []string `json:"files,omitempty"`
	Assertions     []string `json:"assertions,omitempty"`
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
	Config  string // "with_skill" or "baseline"
	Status  string // "ok" or "failed"
	ErrMsg  string
	Timing  *TimingData
	Outputs []string // relative paths within the output dir
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

// BenchmarkFile is written to benchmark.json.
type BenchmarkFile struct {
	RunSummary struct {
		WithSkill RunSummary `json:"with_skill"`
		Baseline  RunSummary `json:"baseline"`
		Delta     struct {
			PassRate    float64 `json:"pass_rate"`
			TimeSeconds float64 `json:"time_seconds"`
			Tokens      float64 `json:"tokens"`
		} `json:"delta"`
	} `json:"run_summary"`
	GeneratedAt time.Time `json:"generated_at"`
}
