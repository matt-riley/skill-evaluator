package main

import (
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

// maxFileAssertions caps per-eval file_exists assertions so a turn that adds
// 1.8k files (e.g. an init) doesn't bury the signal. ponytail: known ceiling.
const maxFileAssertions = 5

// convertedEval carries an Eval plus its agit provenance for auditing.
type convertedEval struct {
	Eval
	AgitStepHash string `json:"-"` // not serialized into evals.json; for logs only
}

// convertSession turns a fetched session (steps + per-step diffs) into Evals.
// Pure: no I/O, no agit binary. Unit-testable with fixtures.
func convertSession(stepsByHash map[string]agitStep, diffsByHash map[string]*agitDiff, logRows []agitLogRow) []convertedEval {
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
		id++
		ce := convertedEval{
			Eval: Eval{
				ID:     id,
				Prompt: prompt,
			},
			AgitStepHash: row.Hash,
		}

		assistant := lastAssistantMessage(step.Messages)
		diff := diffsByHash[row.Hash]

		ce.ExpectedOutput = buildExpectedOutput(assistant, row, step, diff)

		ce.Assertions = buildAssertions(diff, step.Outcome)
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

// buildAssertions emits deterministic file_exists assertions for genuinely new
// artifacts, plus one LLM-judged assertion that checks the output implements
// the requested change without embedding brittle raw text from the recorded
// assistant response.
func buildAssertions(diff *agitDiff, outcome string) []string {
	if diff == nil {
		diff = &agitDiff{}
	}
	var out []string
	for _, c := range diff.Changes {
		if c.Kind != "added" {
			continue
		}
		base := filepath.Base(c.Path)
		// ponytail: skip agit/config-internal noise (dot-prefixed paths)
		if strings.HasPrefix(c.Path, ".") || strings.Contains(base, ".lock") {
			continue
		}
		out = append(out, "file_exists: "+base)
		if len(out) >= maxFileAssertions {
			break
		}
	}
	out = append(out, llmAssertion(outcome))
	return out
}

// llmAssertion returns a stable LLM-judged assertion. If outcome is non-empty
// it is included as context; otherwise a generic assertion is used. Raw
// assistant text is deliberately NOT embedded — it makes evals brittle.
func llmAssertion(outcome string) string {
	if outcome != "" {
		return fmt.Sprintf("The produced output correctly implements the user's request, achieving the outcome: %s.", outcome)
	}
	return "The produced output correctly implements the user's request."
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

func cmdImportAgit(args []string) error {
	fs := flag.NewFlagSet("import-agit", flag.ContinueOnError)
	session := fs.String("session", "", "Specific agit session (origin/id); default: most recent")
	skillDir := fs.String("skill", "", "Skill directory to write evals.json into (default: detect upward)")
	outPath := fs.String("out", "", "Output path (default: <skill>/evals/evals.json)")
	force := fs.Bool("force", false, "Overwrite an existing evals.json")
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

	converted := convertSession(stepsByHash, diffsByHash, log.Steps)
	if len(converted) == 0 {
		return fmt.Errorf("no task-like turns found (all user prompts shorter than %d chars)", minPromptLen)
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
	if _, err := os.Stat(*outPath); err == nil && !*force {
		return fmt.Errorf("%s already exists — pass --force to overwrite, or --out <path>", *outPath)
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
