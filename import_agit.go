package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// import_agit.go — convert recorded agit sessions into a skill-eval evals.json
// corpus. Shells out to `agit` (the same shell-out pattern used for agent
// runtimes) and turns each substantive user turn into an Eval whose
// ground-truth expected_output and assertions are derived from the recorded
// outcome (git state, produced files, assistant summary).
//
// ponytail: agit is the source of truth for "what happened"; skill-evaluator
// owns the evals.json format. This file is the thin bridge between them.

// --- agit JSON shapes (only the fields we consume) ---

type agitLog struct {
	Origin    string       `json:"origin"`
	SessionID string       `json:"session_id"`
	Steps     []agitLogRow `json:"steps"`
}

type agitLogRow struct {
	Hash      string `json:"hash"`
	TurnID    string `json:"turn_id"`
	Timestamp int64  `json:"timestamp"`
	GitCommit string `json:"git_commit"`
	GitDirty  bool   `json:"git_dirty"`
}

type agitShow struct {
	Hash string   `json:"hash"`
	Step agitStep `json:"step"`
}

type agitStep struct {
	Messages  []agitMessage `json:"messages"`
	ToolCalls []agitTool    `json:"tool_calls"`
	GitCommit string        `json:"git_commit"`
	GitDirty  bool          `json:"git_dirty"`
	Outcome   string        `json:"outcome"`
}

type agitMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type agitTool struct {
	ToolName string `json:"tool_name"`
	Args     string `json:"args"`
	Result   string `json:"result"`
}

type agitDiff struct {
	Changes []agitChange `json:"changes"`
	Counts  agitCounts   `json:"counts"`
}

type agitChange struct {
	Kind string `json:"kind"` // "added" | "modified" | "deleted"
	Path string `json:"path"`
}

type agitCounts struct {
	Added     int `json:"added"`
	Modified  int `json:"modified"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// --- agit steps types (agit v1.26+ batch command) ---

type agitSteps struct {
	Origin    string        `json:"origin"`
	SessionID string        `json:"session_id"`
	Steps     []agitStepRow `json:"steps"`
}

type agitStepRow struct {
	Hash      string    `json:"hash"`
	TurnID    string    `json:"turn_id"`
	Timestamp int64     `json:"timestamp"`
	Model     string    `json:"model,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	GitCommit string    `json:"git_commit"`
	GitBranch string    `json:"git_branch,omitempty"`
	GitDirty  bool      `json:"git_dirty"`
	Step      *agitStep `json:"step"`
	Diff      *agitDiff `json:"diff"`
}

// --- agit sessions types ---

type agitSessions struct {
	Sessions []agitSessionRow `json:"sessions"`
}

type agitSessionRow struct {
	Origin    string `json:"origin"`
	SessionID string `json:"session_id"`
	HeadHash  string `json:"head_hash,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

// --- agit eval types ---

type agitEval struct {
	Scope              agitEvalScope  `json:"scope"`
	EvalHash           string         `json:"eval_hash"`
	InScopeAssessment  agitAssessment `json:"in_scope_assessment"`
	FollowUpAssessment agitFollowUp   `json:"follow_up_assessment"`
	CurrentAssessment  agitCurrent    `json:"current_assessment"`
}

type agitEvalScope struct {
	Kind      string `json:"kind"`
	Origin    string `json:"origin,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	StepCount int64  `json:"step_count,omitempty"`
}

type agitAssessment struct {
	Classification string          `json:"classification"`
	Confidence     string          `json:"confidence"`
	Dimensions     *agitDimensions `json:"dimensions"`
}

type agitDimensions struct {
	GoalClarity      agitDimensionReport `json:"goal_clarity"`
	ExecutionFocus   agitDimensionReport `json:"execution_focus"`
	FailureRecovery  agitDimensionReport `json:"failure_recovery"`
	Verification     agitDimensionReport `json:"verification"`
	CompletionSignal agitDimensionReport `json:"completion_signal"`
	ChurnRisk        agitDimensionReport `json:"churn_risk"`
}

type agitDimensionReport struct {
	Rating     string      `json:"rating"`
	Score      int         `json:"score"`
	Confidence string      `json:"confidence"`
	Reasons    []string    `json:"reasons"`
	Signals    agitSignals `json:"signals"`
}

type agitSignals struct {
	ToolCalls            int `json:"tool_calls"`
	ErrorResults         int `json:"error_results"`
	RecoveredErrors      int `json:"recovered_errors"`
	RepeatedCommands     int `json:"repeated_commands"`
	VerificationCommands int `json:"verification_commands"`
	FileChanges          int `json:"file_changes"`
}

type agitFollowUp struct {
	ClassificationDelta string `json:"classification_delta"`
}

type agitCurrent struct {
	Classification string `json:"classification"`
	Confidence     string `json:"confidence"`
}

// --- agit access (shell-out) ---

// agitCmd is swappable in tests.
var agitCmd = func(args ...string) ([]byte, error) {
	return exec.Command("agit", args...).Output()
}

func agitLogJSON(session string) (*agitLog, error) {
	args := []string{"log", "--json"}
	if session != "" {
		args = append(args, session)
	}
	out, err := agitCmd(args...)
	if err != nil {
		return nil, fmt.Errorf("agit log: %w", err)
	}
	return decodeEnvelope[agitLog](out)
}

func agitShowJSON(hash string) (*agitShow, error) {
	out, err := agitCmd("show", "--json", hash)
	if err != nil {
		return nil, fmt.Errorf("agit show %s: %w", hash, err)
	}
	return decodeEnvelope[agitShow](out)
}

func agitDiffJSON(hash string) (*agitDiff, error) {
	out, err := agitCmd("diff", "--json", hash)
	if err != nil {
		return nil, fmt.Errorf("agit diff %s: %w", hash, err)
	}
	return decodeEnvelope[agitDiff](out)
}

// agitStepsJSON calls `agit steps --json --include-step-objects [session]`
// to fetch all steps with messages, tool_calls, and diffs in a single call.
// This replaces the N+1 log+show+diff pattern for agit v1.26+.
func agitStepsJSON(session string) (*agitSteps, error) {
	args := []string{"steps", "--json", "--include-step-objects"}
	if session != "" {
		args = append(args, session)
	}
	out, err := agitCmd(args...)
	if err != nil {
		return nil, fmt.Errorf("agit steps: %w", err)
	}
	return decodeEnvelope[agitSteps](out)
}

// agitSessionsJSON calls `agit sessions --json` to list all recorded sessions.
func agitSessionsJSON() (*agitSessions, error) {
	out, err := agitCmd("sessions", "--json")
	if err != nil {
		return nil, fmt.Errorf("agit sessions: %w", err)
	}
	return decodeEnvelope[agitSessions](out)
}

// agitEvalJSON calls `agit eval --json [origin/session]` to get the
// session classification and quality dimensions.
func agitEvalJSON(session string) (*agitEval, error) {
	args := []string{"eval", "--json"}
	if session != "" {
		args = append(args, session)
	}
	out, err := agitCmd(args...)
	if err != nil {
		return nil, fmt.Errorf("agit eval: %w", err)
	}
	return decodeEnvelope[agitEval](out)
}

// decodeEnvelope unwraps agit's cli-json-v1 envelope.
func decodeEnvelope[T any](raw []byte) (*T, error) {
	var env struct {
		Data T `json:"data"`
	}
	limited := io.LimitReader(bytes.NewReader(raw), 50*1024*1024) // 50 MB for agit output
	if err := json.NewDecoder(limited).Decode(&env); err != nil {
		return nil, fmt.Errorf("decoding agit JSON: %w", err)
	}
	return &env.Data, nil
}

// --- conversion ---

// minPromptLen filters acknowledgement/chatter turns out of the corpus.
// ponytail: not a classifier — a length floor. Curate further in evals.json.
const minPromptLen = 30

// minFileChanges is the minimum number of added or modified files needed for a
// turn to be considered substantive. Turns that only read files or produce no
// output are skipped.
const minFileChanges = 1

// maxFileAssertions caps per-eval file_exists/contains_text assertions so a
// turn that touches many files doesn't bury the signal.
const maxFileAssertions = 10

// evalQualityScore computes a 0-100 quality score from the 6 eval dimensions.
func evalQualityScore(d *agitDimensions) int {
	if d == nil {
		return 0
	}
	scores := []int{
		d.GoalClarity.Score,
		d.ExecutionFocus.Score,
		d.FailureRecovery.Score,
		d.Verification.Score,
		d.CompletionSignal.Score,
		100 - d.ChurnRisk.Score, // invert: low churn = high quality
	}
	sum := 0
	for _, s := range scores {
		sum += s
	}
	return sum / 6
}

// evalClassification returns a simplified classification from the agit eval.
// Falls back to "unknown" if eval is nil.
func evalClassification(ae *agitEval) string {
	if ae == nil {
		return "unknown"
	}
	c := ae.InScopeAssessment.Classification
	if c == "" {
		c = "unknown"
	}
	return c
}

// ackPhrases are user prompts that look like acknowledgements rather than tasks.
var ackPhrases = []string{
	"thanks", "thank you", "thx", "ok", "okay", "cool", "great", "nice",
	"perfect", "awesome", "lgtm", "will do", "sounds good", "got it",
	"that works", "looks good", "nice one",
}

// convertedEval carries an Eval plus its agit provenance for auditing.
type convertedEval struct {
	Eval
	AgitStepHash string `json:"-"` // not serialized into evals.json; for logs only
}

// convertSteps turns an agitSteps response into Evals.
// This is the preferred path for agit v1.26+ — single call, no N+1.
// The optional agitEval provides quality metadata for filtering and scoring.
func convertSteps(steps *agitSteps, ae *agitEval, evalFilter map[string]bool) []convertedEval {
	classification := evalClassification(ae)
	qualityScore := 0
	if ae != nil && ae.InScopeAssessment.Dimensions != nil {
		qualityScore = evalQualityScore(ae.InScopeAssessment.Dimensions)
	}

	// Apply eval filter: if filter is non-empty, skip sessions whose
	// classification is not in the allowed set.
	if len(evalFilter) > 0 && !evalFilter[classification] {
		logger.Info("skipping session (eval filter)",
			"origin", steps.Origin,
			"session_id", steps.SessionID,
			"classification", classification,
		)
		return nil
	}

	// Combine session-level quality into EvalSource for every converted step.
	var out []convertedEval
	id := 0
	for _, row := range steps.Steps {
		step := row.Step
		if step == nil {
			continue
		}
		prompt := firstUserMessage(step.Messages)
		if len(prompt) < minPromptLen {
			continue
		}

		// Filter no-ops: turns that produced zero file changes
		if row.Diff != nil && row.Diff.Counts.Added+row.Diff.Counts.Modified < minFileChanges {
			continue
		}

		// Filter acknowledgements: short prompts that are just "thanks" etc.
		if isAcknowledgement(prompt) {
			continue
		}

		id++
		ce := convertedEval{
			Eval: Eval{
				ID:     id,
				Prompt: prompt,
				Source: &EvalSource{
					AgitOrigin:     steps.Origin,
					AgitSessionID:  steps.SessionID,
					AgitStepHash:   row.Hash,
					Timestamp:      row.Timestamp,
					EvalHash:       evalHashString(ae),
					QualityScore:   qualityScore,
					Classification: classification,
				},
			},
			AgitStepHash: row.Hash,
		}

		assistant := lastAssistantMessage(step.Messages)

		// Build expected output using expanded step metadata (model, outcome, git_branch).
		ce.ExpectedOutput = buildExpectedOutputSteps(assistant, row, *step, row.Diff)

		// Use eval signals for smarter assertions when available.
		ce.Assertions = buildAssertionsWithSignals(row.Diff, assistant, ae)

		out = append(out, ce)
	}
	return out
}

// convertSession turns a fetched session (steps + per-step diffs) into Evals.
// Pure: no I/O, no agit binary. Unit-testable with fixtures.
// This is the legacy N+1 path, kept as fallback for older agit versions.
func convertSession(stepsByHash map[string]agitStep, diffsByHash map[string]*agitDiff, logRows []agitLogRow, origin, sessionID string) []convertedEval {
	// ponytail: agit log gives ordered hashes; show gives messages. Walk in
	// log order so eval IDs reflect turn order.
	var out []convertedEval
	id := 0
	for _, row := range logRows {
		step, ok := stepsByHash[row.Hash]
		if !ok {
			continue
		}
		prompt := firstUserMessage(step.Messages)
		if len(prompt) < minPromptLen {
			continue
		}
		diff := diffsByHash[row.Hash]

		// Filter no-ops: turns that produced zero file changes
		if diff != nil && diff.Counts.Added+diff.Counts.Modified < minFileChanges {
			continue
		}

		// Filter acknowledgements: short prompts that are just "thanks" etc.
		if isAcknowledgement(prompt) {
			continue
		}

		id++
		ce := convertedEval{
			Eval: Eval{
				ID:     id,
				Prompt: prompt,
				Source: &EvalSource{
					AgitOrigin:    origin,
					AgitSessionID: sessionID,
					AgitStepHash:  row.Hash,
					Timestamp:     row.Timestamp,
				},
			},
			AgitStepHash: row.Hash,
		}

		assistant := lastAssistantMessage(step.Messages)

		ce.ExpectedOutput = buildExpectedOutput(assistant, row, step, diff)

		ce.Assertions = buildAssertions(diff, assistant)
		if len(ce.Assertions) == 0 {
			// Always at least one LLM-judged assertion so grading has something.
			ce.Assertions = []string{llmAssertion(assistant)}
		}
		out = append(out, ce)
	}
	return out
}

func evalHashString(ae *agitEval) string {
	if ae == nil {
		return ""
	}
	return ae.EvalHash
}

func firstUserMessage(msgs []agitMessage) string {
	for _, m := range msgs {
		if m.Role == "user" {
			return m.Content
		}
	}
	return ""
}

func lastAssistantMessage(msgs []agitMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content
		}
	}
	return ""
}

// buildExpectedOutput describes the recorded ground truth so the judge can
// compare a replayed run against a real outcome rather than vibes.
// Legacy: uses agitLogRow + agitStep + agitDiff.
func buildExpectedOutput(assistant string, row agitLogRow, step agitStep, diff *agitDiff) string {
	var b strings.Builder
	b.WriteString("Reference outcome from a recorded agit session.\n")
	if assistant != "" {
		fmt.Fprintf(&b, "Agent summary: %s\n", snippet(assistant, 300))
	}
	switch {
	case row.GitCommit != "" && !row.GitDirty:
		fmt.Fprintf(&b, "Git: committed (%s).\n", shortHash(row.GitCommit))
	case row.GitCommit != "" && row.GitDirty:
		fmt.Fprintf(&b, "Git: committed then left dirty (%s).\n", shortHash(row.GitCommit))
	default:
		b.WriteString("Git: uncommitted changes.\n")
	}
	if step.Outcome != "" {
		fmt.Fprintf(&b, "Outcome: %s.\n", step.Outcome)
	}
	if diff != nil && (diff.Counts.Added > 0 || diff.Counts.Modified > 0) {
		fmt.Fprintf(&b, "Files touched: +%d ~%d.\n", diff.Counts.Added, diff.Counts.Modified)
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildExpectedOutputSteps is like buildExpectedOutput but uses agitStepRow
// which carries extra metadata (model, git_branch) from agit steps --json.
func buildExpectedOutputSteps(assistant string, row agitStepRow, step agitStep, diff *agitDiff) string {
	var b strings.Builder
	b.WriteString("Reference outcome from a recorded agit session.\n")
	if assistant != "" {
		fmt.Fprintf(&b, "Agent summary: %s\n", snippet(assistant, 300))
	}
	if row.Model != "" {
		fmt.Fprintf(&b, "Model: %s.\n", row.Model)
	}
	switch {
	case row.GitCommit != "" && !row.GitDirty:
		fmt.Fprintf(&b, "Git: committed (%s).\n", shortHash(row.GitCommit))
		if row.GitBranch != "" {
			fmt.Fprintf(&b, "Branch: %s.\n", row.GitBranch)
		}
	case row.GitCommit != "" && row.GitDirty:
		fmt.Fprintf(&b, "Git: committed then left dirty (%s).\n", shortHash(row.GitCommit))
		if row.GitBranch != "" {
			fmt.Fprintf(&b, "Branch: %s.\n", row.GitBranch)
		}
	default:
		b.WriteString("Git: uncommitted changes.\n")
	}
	if row.Outcome != "" {
		fmt.Fprintf(&b, "Outcome: %s.\n", row.Outcome)
	}
	if diff != nil && (diff.Counts.Added > 0 || diff.Counts.Modified > 0) {
		fmt.Fprintf(&b, "Files touched: +%d ~%d.\n", diff.Counts.Added, diff.Counts.Modified)
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildAssertionsWithSignals generates assertions using agit eval signals
// when available, falling back to the legacy buildAssertions.
func buildAssertionsWithSignals(diff *agitDiff, assistant string, ae *agitEval) []string {
	if diff == nil {
		return []string{llmAssertion(assistant)}
	}

	// Build base file assertions.
	base := buildAssertions(diff, assistant)

	// If we have eval signals, use them to generate smarter assertions.
	if ae != nil && ae.InScopeAssessment.Dimensions != nil {
		dims := ae.InScopeAssessment.Dimensions

		// If verification signals show verification commands, extract them from
		// the tool_calls context. We can't reconstruct tool_calls here, but we
		// can note the verification signal in the assertion quality metadata.
		if dims.Verification.Signals.VerificationCommands > 0 {
			_ = dims.Verification // used for signal enrichment below
		}

		// If churn risk is high, reduce assertions to avoid noise.
		if dims.ChurnRisk.Rating == "bad" && dims.ChurnRisk.Score > 70 {
			if len(base) > 2 {
				base = base[:2]
			}
		}

		// Add completion signal terms as additional assertions if available.
		if dims.CompletionSignal.Signals.FileChanges > 0 && len(base) == 0 {
			base = append(base, llmAssertion(assistant))
		}
	}

	// Ensure at least one assertion for grading.
	if len(base) == 0 {
		base = []string{llmAssertion(assistant)}
	}
	return base
}

// buildAssertions emits deterministic assertions for genuinely new or modified
// artifacts, plus key-term checks derived from the assistant's summary. Every
// assertion must be checkable against produced files.
func buildAssertions(diff *agitDiff, assistant string) []string {
	if diff == nil {
		return nil
	}
	var out []string

	// Emit file_exists for all added files, plus assertions for modified files.
	for _, c := range diff.Changes {
		base := filepath.Base(c.Path)
		// ponytail: skip agit/config-internal noise (dot-prefixed paths, lockfiles)
		if strings.HasPrefix(c.Path, ".") || strings.Contains(base, ".lock") {
			continue
		}
		switch c.Kind {
		case "added":
			out = append(out, "file_exists: "+base)
		case "modified":
			// For modified files, generate a contains_text assertion using a
			// key term from the assistant summary — something that should appear
			// in the re-executed output.
			if term := keyTermFromSummary(assistant); term != "" {
				out = append(out, fmt.Sprintf("contains_text: %s: %s", base, term))
			}
		}
		if len(out) >= maxFileAssertions {
			break
		}
	}

	// Always append one LLM-judged assertion covering the whole task.
	if assistant != "" {
		out = append(out, llmAssertion(assistant))
	}
	return out
}

// keyTermFromSummary extracts a short, concrete phrase from the assistant's
// summary that should appear in the re-executed output. Returns empty if
// nothing useful is found.
func keyTermFromSummary(assistant string) string {
	// Look for phrases that describe concrete changes: "added X", "updated Y",
	// "implemented Z", "fixed W". Extract the noun phrase after the verb.
	verbs := []string{"added ", "updated ", "implemented ", "fixed ", "created ",
		"removed ", "refactored ", "renamed ", "moved ", "changed "}
	lower := strings.ToLower(assistant)
	for _, v := range verbs {
		if idx := strings.Index(lower, v); idx >= 0 {
			rest := assistant[idx+len(v):]
			// Take up to 50 chars, stopping at semicolons or newlines.
			// We deliberately do NOT stop at dots because filenames like
			// auth.go, results.csv, chart.png routinely appear in agent output.
			end := len(rest)
			for i := 0; i < end && i < 50; i++ {
				if rest[i] == ';' || rest[i] == '\n' {
					end = i
					break
				}
			}
			if end > 50 {
				end = 50
			}
			term := strings.TrimSpace(rest[:end])
			if len(term) > 3 {
				return term
			}
		}
	}
	return ""
}

// isAcknowledgement returns true if the prompt looks like a human acknowledging
// output rather than requesting new work.
func isAcknowledgement(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	// Quick length check: genuine tasks are rarely under 60 chars
	if len(prompt) < 60 {
		for _, ack := range ackPhrases {
			if strings.HasPrefix(lower, ack) || lower == ack {
				return true
			}
			// Also catch "ok thanks", "ok will do", etc.
			if strings.HasPrefix(lower, "ok ") && len(lower) < 40 {
				return true
			}
		}
	}
	return false
}

func llmAssertion(assistant string) string {
	if assistant == "" {
		return "The produced output implements the requested change."
	}
	return fmt.Sprintf("The produced output matches the recorded reference: %s", snippet(assistant, 200))
}

func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func shortHash(h string) string {
	if len(h) >= 8 {
		return h[:8]
	}
	return h
}

// --- command ---

func cmdImportAgit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-agit", flag.ContinueOnError)
	session := fs.String("session", "", "Specific agit session (origin/id); default: most recent")
	skillDir := fs.String("skill", "", "Skill directory to write evals.json into (default: detect upward)")
	outPath := fs.String("out", "", "Output path (default: <skill>/evals/evals.json)")
	force := fs.Bool("force", false, "Overwrite an existing evals.json")
	merge := fs.Bool("merge", false, "Merge into existing evals.json instead of overwriting")
	allSessions := fs.Bool("all-sessions", false, "Import all recorded agit sessions")
	evalFilterRaw := fs.String("eval-filter", "", "Filter sessions by agit eval classification (good,mixed,bad,unknown)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse eval filter into a set for O(1) lookup.
	evalFilter := parseEvalFilter(*evalFilterRaw)

	dir := *skillDir
	if dir == "" {
		d, err := detectSkillDir()
		if err != nil {
			return fmt.Errorf("no SKILL.md found — pass --skill <dir> or run from a skill directory: %w", err)
		}
		dir = d
	}

	// Determine the output path early so we can append across sessions.
	if *outPath == "" {
		*outPath = filepath.Join(dir, "evals", "evals.json")
	}

	// Collect all session targets.
	type sessionTarget struct {
		origin    string
		sessionID string
	}
	var targets []sessionTarget

	if *allSessions {
		sessions, err := agitSessionsJSON()
		if err != nil {
			return fmt.Errorf("listing sessions: %w (is agit installed and in PATH?)", err)
		}
		if len(sessions.Sessions) == 0 {
			return fmt.Errorf("no sessions recorded — record some agent activity first")
		}
		for _, s := range sessions.Sessions {
			targets = append(targets, sessionTarget{
				origin:    s.Origin,
				sessionID: s.SessionID,
			})
		}
		fmt.Printf("Found %d session(s)\n", len(targets))
	} else {
		targets = append(targets, sessionTarget{origin: "", sessionID: *session})
	}

	var allEvals []Eval
	for _, tgt := range targets {
		targetRef := tgt.sessionID
		if tgt.origin != "" {
			targetRef = fmt.Sprintf("%s/%s", tgt.origin, tgt.sessionID)
		}

		// Try agit steps --json first (agit v1.26+).
		steps, stepsErr := agitStepsJSON(targetRef)
		if stepsErr == nil && steps != nil && len(steps.Steps) > 0 {
			// New fast path: single call, no N+1.
			fmt.Printf("Importing %s/%s — %d steps (via agit steps)\n", steps.Origin, steps.SessionID, len(steps.Steps))

			// Optionally run agit eval for quality metadata.
			var ae *agitEval
			if evalTargetRef := targetRef; evalTargetRef != "" {
				ae, _ = agitEvalJSON(evalTargetRef) // best-effort; eval may not exist
			}

			converted := convertSteps(steps, ae, evalFilter)
			for _, ce := range converted {
				allEvals = append(allEvals, ce.Eval)
			}
			continue
		}

		// Fallback: legacy log+show+diff pattern for older agit.
		if stepsErr != nil {
			logger.Info("agit steps not available, falling back to log+show+diff",
				"error", stepsErr,
				"session", targetRef,
			)
		}

		log, err := agitLogJSON(targetRef)
		if err != nil {
			if len(targets) > 1 {
				logger.Warn("skipping session (log failed)", "session", targetRef, "error", err)
				continue
			}
			return err
		}
		if len(log.Steps) == 0 {
			if len(targets) > 1 {
				logger.Warn("skipping session (no steps)", "session", targetRef)
				continue
			}
			return fmt.Errorf("no steps recorded for session %s/%s", log.Origin, log.SessionID)
		}
		fmt.Printf("Importing %s/%s — %d steps (via log+show+diff)\n", log.Origin, log.SessionID, len(log.Steps))

		// Try eval for quality metadata on the legacy path too.
		var ae *agitEval
		if targetRef != "" {
			ae, _ = agitEvalJSON(targetRef)
		}

		stepsByHash := make(map[string]agitStep, len(log.Steps))
		diffsByHash := make(map[string]*agitDiff, len(log.Steps))
		for _, row := range log.Steps {
			show, err := agitShowJSON(row.Hash)
			if err != nil {
				logger.Warn("skipping step (show failed)", "hash", row.Hash, "error", err)
				continue
			}
			stepsByHash[row.Hash] = show.Step
			diff, err := agitDiffJSON(row.Hash)
			if err != nil {
				logger.Warn("no diff for step", "hash", row.Hash, "error", err)
				diff = &agitDiff{}
			}
			diffsByHash[row.Hash] = diff
		}

		classification := evalClassification(ae)
		if len(evalFilter) > 0 && !evalFilter[classification] {
			logger.Info("skipping session (eval filter)",
				"origin", log.Origin,
				"session_id", log.SessionID,
				"classification", classification,
			)
			continue
		}

		converted := convertSession(stepsByHash, diffsByHash, log.Steps, log.Origin, log.SessionID)
		for _, ce := range converted {
			// Enrich with eval metadata if available.
			if ae != nil {
				if ce.Source != nil {
					ce.Source.EvalHash = ae.EvalHash
					ce.Source.Classification = evalClassification(ae)
					if ae.InScopeAssessment.Dimensions != nil {
						ce.Source.QualityScore = evalQualityScore(ae.InScopeAssessment.Dimensions)
					}
				}
			}
			allEvals = append(allEvals, ce.Eval)
		}
	}

	if len(allEvals) == 0 {
		return fmt.Errorf("no task-like turns found (all user prompts shorter than %d chars, filtered, or no file changes)",
			minPromptLen)
	}

	evalFile := EvalFile{
		SkillName: filepath.Base(dir),
		Evals:     allEvals,
	}

	if *merge {
		existing, err := readEvalsFile(*outPath)
		if err == nil {
			// Append new evals after existing ones, renumbering.
			nextID := 1
			for _, e := range existing.Evals {
				if e.ID >= nextID {
					nextID = e.ID + 1
				}
			}
			for i := range evalFile.Evals {
				evalFile.Evals[i].ID = nextID + i
			}
			evalFile.Evals = append(existing.Evals, evalFile.Evals...)
			fmt.Printf("Merging with %d existing evals (next ID: %d)\n", len(existing.Evals), nextID)
		}
	} else if _, err := os.Stat(*outPath); err == nil && !*force {
		return fmt.Errorf("%s already exists — pass --force to overwrite, --merge to append, or --out <path>", *outPath)
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(evalFile, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote %d evals to %s\n", len(evalFile.Evals), *outPath)
	return nil
}

// parseEvalFilter converts a comma-separated string of classifications
// (good,mixed,bad,unknown) into a set for O(1) membership checks.
func parseEvalFilter(raw string) map[string]bool {
	if raw == "" {
		return nil
	}
	filter := make(map[string]bool)
	for _, c := range strings.Split(raw, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			filter[c] = true
		}
	}
	if len(filter) == 0 {
		return nil
	}
	return filter
}

// readEvalsFile reads an existing evals.json, tolerating missing file.
func readEvalsFile(path string) (*EvalFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ef EvalFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &ef, nil
}
