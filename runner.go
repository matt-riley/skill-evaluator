package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// runEval executes one eval (with-skill or baseline) and returns the result.
func runEval(ctx context.Context, cfg *Config, skillDir string, eval Eval, workspace string, iteration int, modelKey string, configLabel string, baselinePath string, cmdFn CmdBuilder) (*RunResult, error) {
	evalDir := evalPath(workspace, iteration, eval.ID, modelKey)
	outDir := filepath.Join(evalDir, configLabel, "outputs")
	if err := ensureDir(outDir); err != nil {
		return nil, err
	}

	result := &RunResult{
		EvalID: eval.ID,
		Model:  modelKey,
		Config: configLabel,
		Status: "ok",
	}

	skillPath := resolveSkillPath(skillDir, configLabel, baselinePath)
	agent := cfg.Defaults.Agent
	model := cfg.Defaults.Model

	// Build the task prompt following the agentskills.io convention
	task := buildPrompt(skillPath, eval, outDir, "")

	if cmdFn == nil {
		cmdFn = buildAgentCmd
	}

	start := time.Now()
	cmd := cmdFn(ctx, agent, model, task, skillPath)
	cmd.Dir = skillDir
	logger.Debug("running agent", "agent", agent, "model", model, "dir", cmd.Dir)
	// ponytail: capture combined stdout+stderr — token counts may be on stderr
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		result.Status = "failed"
		if ctx.Err() == context.DeadlineExceeded {
			logger.Debug("agent run timed out", "timeout", ctx.Err())
		} else {
			// Security: do NOT log full agent output at any level — it may
			// contain API keys, PII, or other secrets from the agent's response.
			// Log only the error message and output length for diagnostics.
			outputLen := len(output)
			truncatedErr := truncate(err.Error(), 200)
			logger.Debug("agent run failed", "error", truncatedErr, "output_bytes", outputLen)
		}
	}

	result.Timing = &TimingData{
		DurationMs: int(elapsed.Milliseconds()),
	}
	logger.Info("eval completed", "eval", eval.ID, "config", configLabel, "status", result.Status, "duration_ms", result.Timing.DurationMs)
	result.Timing.TotalTokens = tokensFromOutput(agent, string(output))

	timingPath := filepath.Join(evalDir, configLabel, "timing.json")
	if timingJSON, err := json.MarshalIndent(result.Timing, "", "  "); err != nil {
		logger.Warn("failed to marshal timing", "error", err)
	} else if err := os.WriteFile(timingPath, timingJSON, 0o600); err != nil {
		logger.Warn("failed to write timing", "path", timingPath, "error", err)
	}

	return result, nil
}

func resolveSkillPath(skillDir, configLabel, baselinePath string) string {
	if configLabel == "with_skill" {
		return skillDir
	}
	if baselinePath != "" && baselinePath != "none" {
		return baselinePath
	}
	return ""
}

// CmdBuilder constructs an exec.Cmd for a given agent runtime.
// The context enables cancellation and timeout propagation to the subprocess.
type CmdBuilder = func(ctx context.Context, agent, model, task, skillPath string) *exec.Cmd

// buildAgentCmd constructs the exec.Cmd for a given agent runtime.
// Prefer passing CmdBuilder as a parameter to functions instead of using
// this directly — it avoids data races when tests swap it under t.Parallel().
// Returns an error if the agent name is not in the allowlist or is path-like.
var buildAgentCmd CmdBuilder = func(ctx context.Context, agent, model, task, skillPath string) *exec.Cmd {
	if err := validateModel(model); err != nil {
		logger.Warn("invalid model name", "error", err)
	}
	runner, err := newAgentRunner(agent)
	if err != nil {
		logger.Warn("invalid agent", "agent", agent, "error", err)
		// Return a command that will fail with a clear error message
		return exec.CommandContext(ctx, "false")
	}
	return runner.BuildContext(ctx, model, task, skillPath)
}

// tokensFromOutput picks the right token extractor for the agent runtime.
// pi emits a JSON event stream (parsed precisely); others fall back to the
// regex heuristic.
func tokensFromOutput(agent, output string) int {
	if agent == "pi" {
		return extractPiTokens(output)
	}
	return extractTokens(output)
}

// extractPiTokens sums usage.totalTokens across assistant messages in a pi
// --mode json event stream. Each assistant message_end fires once and carries
// the final usage for that turn, so summing them gives the run total.
func extractPiTokens(output string) int {
	var total int
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Role  string `json:"role"`
				Usage *struct {
					TotalTokens int `json:"totalTokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.Type == "message_end" && ev.Message.Role == "assistant" && ev.Message.Usage != nil {
			total += ev.Message.Usage.TotalTokens
		}
	}
	return total
}

// extractTokens tries to find token counts in agent output.
// ponytail: regex heuristic across runtimes. Harden once real output is known.
var tokenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`"total_tokens"\s*:\s*(\d+)`),
	regexp.MustCompile(`total.tokens.?:?\s*(\d+)`),
	regexp.MustCompile(`tokens:\s*(\d+)`),
	regexp.MustCompile(`input_tokens.*?(\d+)`),
}

func extractTokens(output string) int {
	for _, re := range tokenPatterns {
		if m := re.FindStringSubmatch(output); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				return n
			}
		}
	}
	return 0
}

// detectSkillDir finds a skill directory (containing SKILL.md) from the current dir.
func detectSkillDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		skillPath := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no SKILL.md found in current directory or any parent")
		}
		dir = parent
	}
}

// readEvals loads evals.json from a skill directory.
// Enforces size limits to prevent DoS via oversized inputs.
func readEvals(skillDir string) (*EvalFile, error) {
	const maxEvalsFileSize = 1 * 1024 * 1024 // 1 MB
	const maxEvalCount = 100
	const maxEvalPromptLen = 10 * 1024 // 10 KB
	const maxEvalOutputLen = 10 * 1024 // 10 KB

	path := filepath.Join(skillDir, "evals", "evals.json")

	// Stat the file first to check size before reading
	if fi, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("reading evals.json: %w", err)
	} else if fi.Size() > maxEvalsFileSize {
		return nil, fmt.Errorf("evals.json is too large: %d bytes (max %d)", fi.Size(), maxEvalsFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading evals.json: %w", err)
	}
	var ef EvalFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parsing evals.json: %w", err)
	}

	if len(ef.Evals) > maxEvalCount {
		return nil, fmt.Errorf("too many evals: %d (max %d)", len(ef.Evals), maxEvalCount)
	}

	// Validate individual eval field lengths
	for i, e := range ef.Evals {
		if len(e.Prompt) > maxEvalPromptLen {
			return nil, fmt.Errorf("eval %d: prompt too long: %d bytes (max %d)", e.ID, len(e.Prompt), maxEvalPromptLen)
		}
		if len(e.ExpectedOutput) > maxEvalOutputLen {
			return nil, fmt.Errorf("eval %d: expected_output too long: %d bytes (max %d)", e.ID, len(e.ExpectedOutput), maxEvalOutputLen)
		}
		_ = i
	}

	return &ef, nil
}

func buildPrompt(skillPath string, eval Eval, outDir, critique string) string {
	var b strings.Builder
	b.WriteString("Execute this task in non-interactive mode (do not ask questions, do not wait for confirmation).\n")

	if critique != "" {
		b.WriteString("\nYour previous output had these issues. Fix them and regenerate:\n")
		b.WriteString(critique)
		b.WriteString("\n\n")
	}

	if skillPath != "" {
		fmt.Fprintf(&b, "- Skill path: %s\n", skillPath)
	}
	fmt.Fprintf(&b, "- Task: %s\n", eval.Prompt)
	if len(eval.Files) > 0 {
		fmt.Fprintf(&b, "- Input files: %s\n", strings.Join(eval.Files, ", "))
	}
	fmt.Fprintf(&b, "- Save outputs to: %s\n", outDir)

	return b.String()
}

// fixEval runs an iterative refinement loop on a failing with-skill eval.
// It re-runs the agent with critique from failed assertions until all pass,
// the score plateaus, or maxAttempts is exhausted.
func fixEval(ctx context.Context, cfg *Config, skillDir string, eval Eval,
	workspace string, iteration int, modelKey string, baselinePath string, maxAttempts int, cmdFn CmdBuilder) (*FixResult, error) {

	evalDir := evalPath(workspace, iteration, eval.ID, modelKey)
	gradingPath := filepath.Join(evalDir, "with_skill", "grading.json")

	// Load initial grading
	data, err := os.ReadFile(gradingPath)
	if err != nil {
		return nil, fmt.Errorf("reading initial grading for eval %d: %w", eval.ID, err)
	}
	var initialGf GradingFile
	if err := json.Unmarshal(data, &initialGf); err != nil {
		return nil, fmt.Errorf("parsing initial grading for eval %d: %w", eval.ID, err)
	}

	fr := &FixResult{EvalID: eval.ID}

	// Record initial attempt (already done by normal grade step)
	fr.Attempts = append(fr.Attempts, FixAttempt{Attempt: 1, Grading: &initialGf})

	// If already passing, nothing to fix
	if initialGf.Summary.Failed == 0 {
		return fr, nil
	}

	lastFailed := extractFailedReasoning(&initialGf)
	skillPath := resolveSkillPath(skillDir, "with_skill", baselinePath)
	agent := cfg.Defaults.Agent
	model := cfg.Defaults.Model

	if cmdFn == nil {
		cmdFn = buildAgentCmd
	}

	for attempt := 2; attempt <= maxAttempts+1; attempt++ {
		// Bail early if context cancelled
		if err := ctx.Err(); err != nil {
			break
		}
		critique := lastFailed
		fixDir := filepath.Join(evalDir, "with_skill", fmt.Sprintf("fix-%d", attempt))
		outDir := filepath.Join(fixDir, "outputs")
		if err := ensureDir(outDir); err != nil {
			return fr, fmt.Errorf("creating fix-%d dir: %w", attempt, err)
		}

		// Run agent with critique
		task := buildPrompt(skillPath, eval, outDir, critique)
		start := time.Now()
		cmd := cmdFn(ctx, agent, model, task, skillPath)
		cmd.Dir = skillDir
		output, _ := cmd.CombinedOutput()
		elapsed := time.Since(start)

		// Save timing
		td := &TimingData{DurationMs: int(elapsed.Milliseconds())}
		td.TotalTokens = tokensFromOutput(agent, string(output))
		if tdJSON, err := json.MarshalIndent(td, "", "  "); err != nil {
			logger.Warn("failed to marshal fix timing", "error", err)
		} else if err := os.WriteFile(filepath.Join(fixDir, "timing.json"), tdJSON, 0o600); err != nil {
			logger.Warn("failed to write fix timing", "path", filepath.Join(fixDir, "timing.json"), "error", err)
		}

		// Grade this fix attempt
		gf, err := gradeFixAttempt(ctx, cfg, eval, workspace, iteration, modelKey, attempt, cmdFn)
		if err != nil {
			// Can't grade — stop fixing, keep previous best
			break
		}

		fa := FixAttempt{Attempt: attempt, Grading: gf, Critique: critique}
		fr.Attempts = append(fr.Attempts, fa)

		// All pass — stop
		if gf.Summary.Failed == 0 {
			break
		}

		// Convergence check: same failures as last time
		failed := extractFailedReasoning(gf)
		if failed == lastFailed {
			fr.Converged = true
			break
		}
		lastFailed = failed
	}

	// Find best attempt by pass rate
	fr.BestFix = 0
	bestRate := 0.0
	for i, a := range fr.Attempts {
		if a.Grading.Summary.PassRate > bestRate {
			bestRate = a.Grading.Summary.PassRate
			fr.BestFix = i
		}
	}

	// Overwrite grading.json with the best attempt's result
	best := fr.Attempts[fr.BestFix]
	if err := saveGrading(gradingPath, best.Grading); err != nil {
		logger.Warn("failed to save best-fix grading", "path", gradingPath, "error", err)
	}

	// Save fix trajectory
	fixPath := filepath.Join(evalDir, "with_skill", "fix-results.json")
	fixJSON, err := json.MarshalIndent(fr, "", "  ")
	if err != nil {
		logger.Warn("failed to marshal fix results", "error", err)
	} else if err := os.WriteFile(fixPath, fixJSON, 0o600); err != nil {
		logger.Warn("failed to write fix results", "path", fixPath, "error", err)
	}

	return fr, nil
}
