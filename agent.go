package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// AgentRunner knows how to build a command for a specific agent runtime.
// Each implementation encapsulates the CLI flags and conventions for one agent.
type AgentRunner interface {
	// Build returns an exec.Cmd configured for this agent.
	Build(model, task, skillPath string) *exec.Cmd
	// BuildContext returns an exec.Cmd bound to the given context for cancellation/timeout.
	BuildContext(ctx context.Context, model, task, skillPath string) *exec.Cmd
}

// validModelRe limits model names to known-safe characters.
// Hyphens are PERMITTED because model names like "gpt-4o-mini" and "claude-sonnet-4-5"
// are common. Defense against flag injection: validateModel also checks that the
// model string does not start with a hyphen (prevents CLI argument confusion).
var validModelRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:\-]*$`)

// validateModel returns an error if the model string looks suspicious.
func validateModel(model string) error {
	if model == "" {
		return nil // empty is fine — means use default
	}
	if !validModelRe.MatchString(model) {
		return fmt.Errorf("invalid model name: %q", model)
	}
	// Defense-in-depth: reject model strings that start with hyphens to
	// prevent downstream CLI argument parsers from interpreting them as flags.
	if strings.HasPrefix(model, "-") {
		return fmt.Errorf("invalid model name (starts with hyphen): %q", model)
	}
	return nil
}

// validAgents is the allowlist of supported agent runtimes.
var validAgents = map[string]bool{
	"pi":     true,
	"claude": true,
	"codex":  true,
}

// newAgentRunner returns the appropriate runner for the given agent name.
// Only agents in the explicit allowlist are permitted; unknown agents
// produce an error instead of falling through to arbitrary binary execution.
func newAgentRunner(agent string) (AgentRunner, error) {
	// Reject path-like agent names as a defense-in-depth measure
	if strings.ContainsAny(agent, "/\\") {
		return nil, fmt.Errorf("invalid agent name (contains path separator): %q", agent)
	}
	if !validAgents[agent] {
		return nil, fmt.Errorf("unknown agent: %q (valid: pi, claude, codex)", agent)
	}
	switch agent {
	case "pi":
		return &piRunner{}, nil
	case "claude":
		return &claudeRunner{}, nil
	case "codex":
		return &codexRunner{}, nil
	default:
		return nil, fmt.Errorf("unknown agent: %q (valid: pi, claude, codex)", agent)
	}
}

// ValidateAgent checks that an agent name is in the allowlist.
func ValidateAgent(agent string) error {
	if strings.ContainsAny(agent, "/\\") {
		return fmt.Errorf("invalid agent name (contains path separator): %q", agent)
	}
	if !validAgents[agent] {
		return fmt.Errorf("unknown agent: %q (valid: pi, claude, codex)", agent)
	}
	return nil
}

// piRunner builds commands for the pi CLI.
type piRunner struct{}

func (r *piRunner) Build(model, task, skillPath string) *exec.Cmd {
	return r.BuildContext(context.Background(), model, task, skillPath)
}

func (r *piRunner) BuildContext(ctx context.Context, model, task, skillPath string) *exec.Cmd {
	// pi docs: --mode json emits an event stream with usage.totalTokens;
	// -p/--no-session keep it ephemeral, --no-context-files keeps context clean,
	// --skill <path> loads the skill into the system prompt properly.
	args := []string{"-p", "--no-session", "--no-context-files", "--mode", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if skillPath != "" {
		args = append(args, "--skill", skillPath)
	}
	args = append(args, task)
	return exec.CommandContext(ctx, "pi", args...)
}

// claudeRunner builds commands for the Claude Code CLI.
type claudeRunner struct{}

func (r *claudeRunner) Build(model, task, _ string) *exec.Cmd {
	return r.BuildContext(context.Background(), model, task, "")
}

func (r *claudeRunner) BuildContext(ctx context.Context, model, task, _ string) *exec.Cmd {
	// claude --help: -p (print), --no-session-persistence, --model
	// No --skill flag — skill path is embedded in the prompt text
	args := []string{"-p", "--no-session-persistence"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, task)
	return exec.CommandContext(ctx, "claude", args...)
}

// codexRunner builds commands for the OpenAI Codex CLI.
type codexRunner struct{}

func (r *codexRunner) Build(model, task, _ string) *exec.Cmd {
	return r.BuildContext(context.Background(), model, task, "")
}

func (r *codexRunner) BuildContext(ctx context.Context, model, task, _ string) *exec.Cmd {
	// codex exec --help: exec subcommand, -m (model), --ephemeral
	// No --skill flag — skill path is embedded in the prompt text
	args := []string{"exec", "--ephemeral"}
	if model != "" {
		args = append(args, "-m", model)
	}
	args = append(args, task)
	return exec.CommandContext(ctx, "codex", args...)
}

// --- token extraction helpers below ---
