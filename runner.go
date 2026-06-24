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
	os.WriteFile(timingPath, timingJSON, 0o644)

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
		b.WriteString(fmt.Sprintf("- Skill path: %s\n", skillPath))
	}
	b.WriteString(fmt.Sprintf("- Task: %s\n", eval.Prompt))
	if len(eval.Files) > 0 {
		b.WriteString(fmt.Sprintf("- Input files: %s\n", strings.Join(eval.Files, ", ")))
	}
	b.WriteString(fmt.Sprintf("- Save outputs to: %s\n", outDir))

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

	case "copilot":
		// gh copilot --help: invoked via `gh copilot`
		// ponytail: placeholder — exact flags TBD once tested
		args := []string{"copilot"}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, task)
		return exec.Command("gh", args...)

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
