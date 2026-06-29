package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

// decodeEnvelope unwraps agit's cli-json-v1 {"schema_version":...,"data":...} envelope.
func decodeEnvelope[T any](raw []byte) (*T, error) {
	var env struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
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

// convertSession turns a fetched session (steps + per-step diffs) into Evals.
// Pure: no I/O, no agit binary. Unit-testable with fixtures.
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
	if err := fs.Parse(args); err != nil {
		return err
	}

	dir := *skillDir
	if dir == "" {
		d, err := detectSkillDir()
		if err != nil {
			return fmt.Errorf("no SKILL.md found — pass --skill <dir> or run from a skill directory: %w", err)
		}
		dir = d
	}

	log, err := agitLogJSON(*session)
	if err != nil {
		return err
	}
	if len(log.Steps) == 0 {
		return fmt.Errorf("no steps recorded for session %s/%s", log.Origin, log.SessionID)
	}
	fmt.Printf("Importing %s/%s — %d steps\n", log.Origin, log.SessionID, len(log.Steps))

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

	converted := convertSession(stepsByHash, diffsByHash, log.Steps, log.Origin, log.SessionID)
	if len(converted) == 0 {
		return fmt.Errorf("no task-like turns found (all user prompts shorter than %d chars or filtered)", minPromptLen)
	}

	evalFile := EvalFile{
		SkillName: filepath.Base(dir),
		Evals:     make([]Eval, 0, len(converted)),
	}
	for _, ce := range converted {
		evalFile.Evals = append(evalFile.Evals, ce.Eval)
	}

	if *outPath == "" {
		*outPath = filepath.Join(dir, "evals", "evals.json")
	}

	if *merge {
		existing, err := readEvalsFile(*outPath)
		if err == nil {
			// Append new evals after existing ones, renumbering
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
