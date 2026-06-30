package agit

import (
	"fmt"
	"path/filepath"
	"strings"
)

// MinPromptLen filters acknowledgement/chatter turns out of the corpus.
// ponytail: not a classifier — a length floor. Curate further in evals.json.
const MinPromptLen = 30

// minFileChanges is the minimum number of added or modified files needed for a
// turn to be considered substantive. Turns that only read files or produce no
// output are skipped.
const minFileChanges = 1

// maxFileAssertions caps per-eval file_exists/contains_text assertions so a
// turn that touches many files doesn't bury the signal.
const maxFileAssertions = 10

// EvalQualityScore computes a 0-100 quality score from the 6 eval dimensions.
func EvalQualityScore(d *Dimensions) int {
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

// EvalClassification returns a simplified classification from the agit eval.
// Falls back to "unknown" if eval is nil.
func EvalClassification(ae *EvalReport) string {
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

// ConvertedEval is an eval candidate derived from an agit session, decoupled
// from skill-evaluator's own Eval/EvalSource schema so this package has no
// dependency on package main.
type ConvertedEval struct {
	ID             int
	Prompt         string
	ExpectedOutput string
	Assertions     []string
	Source         EvalSource
}

// EvalSource records agit provenance for a converted eval.
type EvalSource struct {
	Origin         string
	SessionID      string
	StepHash       string
	Timestamp      int64
	EvalHash       string
	QualityScore   int
	Classification string
}

// ConvertSteps turns a Steps response into ConvertedEvals.
// This is the preferred path for agit v1.26+ — single call, no N+1.
// The optional EvalReport provides quality metadata for filtering and scoring.
func ConvertSteps(steps *Steps, ae *EvalReport, evalFilter map[string]bool) []ConvertedEval {
	classification := EvalClassification(ae)
	qualityScore := 0
	if ae != nil && ae.InScopeAssessment.Dimensions != nil {
		qualityScore = EvalQualityScore(ae.InScopeAssessment.Dimensions)
	}

	// Apply eval filter: if filter is non-empty, skip sessions whose
	// classification is not in the allowed set.
	if len(evalFilter) > 0 && !evalFilter[classification] {
		Logger.Info("skipping session (eval filter)",
			"origin", steps.Origin,
			"session_id", steps.SessionID,
			"classification", classification,
		)
		return nil
	}

	// Combine session-level quality into EvalSource for every converted step.
	var out []ConvertedEval
	id := 0
	for _, row := range steps.Steps {
		step := row.Step
		if step == nil {
			continue
		}
		prompt := firstUserMessage(step.Messages)
		if len(prompt) < MinPromptLen {
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
		ce := ConvertedEval{
			ID:     id,
			Prompt: prompt,
			Source: EvalSource{
				Origin:         steps.Origin,
				SessionID:      steps.SessionID,
				StepHash:       row.Hash,
				Timestamp:      row.Timestamp,
				EvalHash:       evalHashString(ae),
				QualityScore:   qualityScore,
				Classification: classification,
			},
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

// ConvertSession turns a fetched session (steps + per-step diffs) into ConvertedEvals.
// Pure: no I/O, no agit binary. Unit-testable with fixtures.
// This is the legacy N+1 path, kept as fallback for older agit versions.
func ConvertSession(stepsByHash map[string]Step, diffsByHash map[string]*Diff, logRows []LogRow, origin, sessionID string) []ConvertedEval {
	// ponytail: agit log gives ordered hashes; show gives messages. Walk in
	// log order so eval IDs reflect turn order.
	var out []ConvertedEval
	id := 0
	for _, row := range logRows {
		step, ok := stepsByHash[row.Hash]
		if !ok {
			continue
		}
		prompt := firstUserMessage(step.Messages)
		if len(prompt) < MinPromptLen {
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
		ce := ConvertedEval{
			ID:     id,
			Prompt: prompt,
			Source: EvalSource{
				Origin:    origin,
				SessionID: sessionID,
				StepHash:  row.Hash,
				Timestamp: row.Timestamp,
			},
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

func evalHashString(ae *EvalReport) string {
	if ae == nil {
		return ""
	}
	return ae.EvalHash
}

// maxMessageContentLen caps per-message content length from agit JSON to
// prevent bombastically large fields from inflating evals.json or judge prompts.
const maxMessageContentLen = 50 * 1024 // 50 KB

func firstUserMessage(msgs []Message) string {
	for _, m := range msgs {
		if m.Role == "user" {
			return truncateContent(m.Content)
		}
	}
	return ""
}

func lastAssistantMessage(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return truncateContent(msgs[i].Content)
		}
	}
	return ""
}

// truncateContent caps message content to a safe maximum length.
func truncateContent(content string) string {
	if len(content) > maxMessageContentLen {
		return content[:maxMessageContentLen]
	}
	return content
}

// buildExpectedOutput describes the recorded ground truth so the judge can
// compare a replayed run against a real outcome rather than vibes.
// Legacy: uses LogRow + Step + Diff.
func buildExpectedOutput(assistant string, row LogRow, step Step, diff *Diff) string {
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

// buildExpectedOutputSteps is like buildExpectedOutput but uses StepRow
// which carries extra metadata (model, git_branch) from agit steps --json.
func buildExpectedOutputSteps(assistant string, row StepRow, step Step, diff *Diff) string {
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
func buildAssertionsWithSignals(diff *Diff, assistant string, ae *EvalReport) []string {
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
// assertion must be checkable against produced files. Paths are sanitized to
// strip traversal sequences from untrusted agit diff output.
func buildAssertions(diff *Diff, assistant string) []string {
	if diff == nil {
		return nil
	}
	var out []string

	// Emit file_exists for all added files, plus assertions for modified files.
	for _, c := range diff.Changes {
		// Sanitize the path from agit before using it.
		cleanPath, safe := sanitizeAssertionPath(c.Path)
		if !safe || strings.TrimSpace(cleanPath) == "" {
			continue
		}
		base := filepath.Base(cleanPath)
		// ponytail: skip agit/config-internal noise (dot-prefixed paths, lockfiles)
		if strings.HasPrefix(cleanPath, ".") || strings.Contains(base, ".lock") {
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

// ParseEvalFilter converts a comma-separated string of classifications
// (good,mixed,bad,unknown) into a set for O(1) membership checks.
func ParseEvalFilter(raw string) map[string]bool {
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
