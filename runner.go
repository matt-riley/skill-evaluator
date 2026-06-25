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
func runEval(ctx context.Context, cfg *Config, skillDir string, eval Eval, workspace string, iteration int, configLabel string, baselinePath string) (*RunResult, error) {
	evalDir := evalPath(workspace, iteration, eval.ID)
	outDir := filepath.Join(evalDir, configLabel, "outputs")
	if err := ensureDir(outDir); err != nil {
		return nil, err
	}

	result := &RunResult{
		EvalID: eval.ID,
		Config: configLabel,
		Status: "ok",
	}

	skillPath := resolveSkillPath(skillDir, configLabel, baselinePath)
	agent := cfg.Defaults.Agent
	model := cfg.Defaults.Model

	// Build the task prompt following the agentskills.io convention
	task := buildRunPrompt(skillPath, eval, outDir)

	start := time.Now()
	cmd := buildAgentCmd(agent, model, task, skillPath)
	cmd.Dir = skillDir
	// ponytail: capture combined stdout+stderr — token counts may be on stderr
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		result.Status = "failed"
		result.ErrMsg = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ErrMsg = string(exitErr.Stderr)
		}
	}

	result.Timing = &TimingData{
		DurationMs: int(elapsed.Milliseconds()),
	}
	result.Timing.TotalTokens = extractTokens(string(output))

	timingPath := filepath.Join(evalDir, configLabel, "timing.json")
	timingJSON, _ := json.MarshalIndent(result.Timing, "", "  ")
	_ = os.WriteFile(timingPath, timingJSON, 0o644)

	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if !e.IsDir() {
			result.Outputs = append(result.Outputs, e.Name())
		}
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

// buildRunPrompt follows the agentskills.io eval run convention.
func buildRunPrompt(skillPath string, eval Eval, outDir string) string {
	var b strings.Builder
	b.WriteString("Execute this task in non-interactive mode (do not ask questions, do not wait for confirmation).\n")

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

// buildAgentCmd constructs the exec.Cmd for a given agent runtime.
func buildAgentCmd(agent, model, task, skillPath string) *exec.Cmd {
	switch agent {
	case "pi":
		// pi docs: -p (print), --no-session (ephemeral), --no-context-files (clean context)
		// --skill <path> loads the skill into the system prompt properly
		args := []string{"-p", "--no-session", "--no-context-files"}
		if model != "" {
			args = append(args, "--model", model)
		}
		if skillPath != "" {
			args = append(args, "--skill", skillPath)
		}
		args = append(args, task)
		return exec.Command("pi", args...)

	case "claude":
		// claude --help: -p (print), --no-session-persistence, --model
		// No --skill flag — skill path is embedded in the prompt text
		args := []string{"-p", "--no-session-persistence"}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, task)
		return exec.Command("claude", args...)

	case "codex":
		// codex exec --help: exec subcommand, -m (model), --ephemeral
		// No --skill flag — skill path is embedded in the prompt text
		args := []string{"exec", "--ephemeral"}
		if model != "" {
			args = append(args, "-m", model)
		}
		args = append(args, task)
		return exec.Command("codex", args...)

	default:
		args := []string{task}
		return exec.Command(agent, args...)
	}
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
func readEvals(skillDir string) (*EvalFile, error) {
	path := filepath.Join(skillDir, "evals", "evals.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading evals.json: %w", err)
	}
	var ef EvalFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parsing evals.json: %w", err)
	}
	return &ef, nil
}

// buildFixPrompt creates a task prompt with critique from a previous failed attempt.
func buildFixPrompt(skillPath string, eval Eval, outDir string, critique string) string {
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
	workspace string, iteration int, baselinePath string, maxAttempts int) (*FixResult, error) {

	evalDir := evalPath(workspace, iteration, eval.ID)
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

	for attempt := 2; attempt <= maxAttempts+1; attempt++ {
		critique := lastFailed
		fixDir := filepath.Join(evalDir, "with_skill", fmt.Sprintf("fix-%d", attempt))
		outDir := filepath.Join(fixDir, "outputs")
		if err := ensureDir(outDir); err != nil {
			return fr, fmt.Errorf("creating fix-%d dir: %w", attempt, err)
		}

		// Run agent with critique
		task := buildFixPrompt(skillPath, eval, outDir, critique)
		start := time.Now()
		cmd := buildAgentCmd(agent, model, task, skillPath)
		cmd.Dir = skillDir
		output, err := cmd.CombinedOutput()
		elapsed := time.Since(start)

		// Save timing
		td := &TimingData{DurationMs: int(elapsed.Milliseconds())}
		td.TotalTokens = extractTokens(string(output))
		tdJSON, _ := json.MarshalIndent(td, "", "  ")
		_ = os.WriteFile(filepath.Join(fixDir, "timing.json"), tdJSON, 0o644)

		// Grade this fix attempt
		gf, err := gradeFixAttempt(ctx, cfg, eval, workspace, iteration, attempt)
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
	saveGrading(gradingPath, best.Grading)

	// Save fix trajectory
	fixPath := filepath.Join(evalDir, "with_skill", "fix-results.json")
	fixJSON, _ := json.MarshalIndent(fr, "", "  ")
	_ = os.WriteFile(fixPath, fixJSON, 0o644)

	return fr, nil
}
