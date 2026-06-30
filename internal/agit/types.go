package agit

// types.go — agit JSON wire shapes (only the fields we consume).

// --- agit JSON shapes (only the fields we consume) ---

type Log struct {
	Origin    string   `json:"origin"`
	SessionID string   `json:"session_id"`
	Steps     []LogRow `json:"steps"`
}

type LogRow struct {
	Hash      string `json:"hash"`
	TurnID    string `json:"turn_id"`
	Timestamp int64  `json:"timestamp"`
	GitCommit string `json:"git_commit"`
	GitDirty  bool   `json:"git_dirty"`
}

type Show struct {
	Hash string `json:"hash"`
	Step Step   `json:"step"`
}

type Step struct {
	Messages  []Message `json:"messages"`
	ToolCalls []Tool    `json:"tool_calls"`
	GitCommit string    `json:"git_commit"`
	GitDirty  bool      `json:"git_dirty"`
	Outcome   string    `json:"outcome"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	ToolName string `json:"tool_name"`
	Args     string `json:"args"`
	Result   string `json:"result"`
}

type Diff struct {
	Changes []Change `json:"changes"`
	Counts  Counts   `json:"counts"`
}

type Change struct {
	Kind string `json:"kind"` // "added" | "modified" | "deleted"
	Path string `json:"path"`
}

type Counts struct {
	Added     int `json:"added"`
	Modified  int `json:"modified"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// --- agit steps types (agit v1.26+ batch command) ---

type Steps struct {
	Origin    string    `json:"origin"`
	SessionID string    `json:"session_id"`
	Steps     []StepRow `json:"steps"`
}

type StepRow struct {
	Hash      string `json:"hash"`
	TurnID    string `json:"turn_id"`
	Timestamp int64  `json:"timestamp"`
	Model     string `json:"model,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch,omitempty"`
	GitDirty  bool   `json:"git_dirty"`
	Step      *Step  `json:"step"`
	Diff      *Diff  `json:"diff"`
}

// --- agit sessions types ---

type Sessions struct {
	Sessions []SessionRow `json:"sessions"`
}

type SessionRow struct {
	Origin    string `json:"origin"`
	SessionID string `json:"session_id"`
	HeadHash  string `json:"head_hash,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

// --- agit eval types ---

// EvalReport is agit's quality assessment of a session, as returned by
// `agit eval --json`. Not to be confused with ConvertedEval, which is a
// skill-evaluator eval candidate derived from a session.
type EvalReport struct {
	Scope              EvalScope  `json:"scope"`
	EvalHash           string     `json:"eval_hash"`
	InScopeAssessment  Assessment `json:"in_scope_assessment"`
	FollowUpAssessment FollowUp   `json:"follow_up_assessment"`
	CurrentAssessment  Current    `json:"current_assessment"`
}

type EvalScope struct {
	Kind      string `json:"kind"`
	Origin    string `json:"origin,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	StepCount int64  `json:"step_count,omitempty"`
}

type Assessment struct {
	Classification string      `json:"classification"`
	Confidence     string      `json:"confidence"`
	Dimensions     *Dimensions `json:"dimensions"`
}

type Dimensions struct {
	GoalClarity      DimensionReport `json:"goal_clarity"`
	ExecutionFocus   DimensionReport `json:"execution_focus"`
	FailureRecovery  DimensionReport `json:"failure_recovery"`
	Verification     DimensionReport `json:"verification"`
	CompletionSignal DimensionReport `json:"completion_signal"`
	ChurnRisk        DimensionReport `json:"churn_risk"`
}

type DimensionReport struct {
	Rating     string   `json:"rating"`
	Score      int      `json:"score"`
	Confidence string   `json:"confidence"`
	Reasons    []string `json:"reasons"`
	Signals    Signals  `json:"signals"`
}

type Signals struct {
	ToolCalls            int `json:"tool_calls"`
	ErrorResults         int `json:"error_results"`
	RecoveredErrors      int `json:"recovered_errors"`
	RepeatedCommands     int `json:"repeated_commands"`
	VerificationCommands int `json:"verification_commands"`
	FileChanges          int `json:"file_changes"`
}

type FollowUp struct {
	ClassificationDelta string `json:"classification_delta"`
}

type Current struct {
	Classification string `json:"classification"`
	Confidence     string `json:"confidence"`
}
