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
var validModelRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:\-]*$`)

// validateModel returns an error if the model string looks suspicious.
func validateModel(model string) error {
	if model == "" {
		return nil // empty is fine — means use default
	}
	if !validModelRe.MatchString(model) {
		return fmt.Errorf("invalid model name: %q", model)
	}
	return nil
}

// newAgentRunner returns the appropriate runner for the given agent name.
// Rejects agent names containing path separators to prevent arbitrary binary execution.
func newAgentRunner(agent string) AgentRunner {
	// Reject path-like agent names as a defense-in-depth measure
	if strings.ContainsAny(agent, "/\\") {
		panic(fmt.Sprintf("invalid agent name (contains path separator): %q", agent))
	}
	switch agent {
	case "pi":
		return &piRunner{}
	case "claude":
		return &claudeRunner{}
	case "codex":
		return &codexRunner{}
	default:
		return &genericRunner{name: agent}
	}
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

// genericRunner is the fallback for unknown agent runtimes.
type genericRunner struct {
	name string
}

func (r *genericRunner) Build(model, task, skillPath string) *exec.Cmd {
	return r.BuildContext(context.Background(), model, task, skillPath)
}

func (r *genericRunner) BuildContext(ctx context.Context, model, task, skillPath string) *exec.Cmd {
	args := []string{}
	if model != "" {
		args = append(args, "--model", model)
	}
	if skillPath != "" {
		args = append(args, "--skill", skillPath)
	}
	args = append(args, task)
	return exec.CommandContext(ctx, r.name, args...)
}
