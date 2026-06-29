package main

import "os/exec"

// AgentRunner knows how to build a command for a specific agent runtime.
// Each implementation encapsulates the CLI flags and conventions for one agent.
type AgentRunner interface {
	// Build returns an exec.Cmd configured for this agent.
	Build(model, task, skillPath string) *exec.Cmd
}

// newAgentRunner returns the appropriate runner for the given agent name.
// Unknown agents fall back to a generic runner that passes the task as a positional argument.
func newAgentRunner(agent string) AgentRunner {
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
	return exec.Command("pi", args...)
}

// claudeRunner builds commands for the Claude Code CLI.
type claudeRunner struct{}

func (r *claudeRunner) Build(model, task, _ string) *exec.Cmd {
	// claude --help: -p (print), --no-session-persistence, --model
	// No --skill flag — skill path is embedded in the prompt text
	args := []string{"-p", "--no-session-persistence"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, task)
	return exec.Command("claude", args...)
}

// codexRunner builds commands for the OpenAI Codex CLI.
type codexRunner struct{}

func (r *codexRunner) Build(model, task, _ string) *exec.Cmd {
	// codex exec --help: exec subcommand, -m (model), --ephemeral
	// No --skill flag — skill path is embedded in the prompt text
	args := []string{"exec", "--ephemeral"}
	if model != "" {
		args = append(args, "-m", model)
	}
	args = append(args, task)
	return exec.Command("codex", args...)
}

// genericRunner is the fallback for unknown agent runtimes.
type genericRunner struct {
	name string
}

func (r *genericRunner) Build(model, task, skillPath string) *exec.Cmd {
	args := []string{}
	if model != "" {
		args = append(args, "--model", model)
	}
	if skillPath != "" {
		args = append(args, "--skill", skillPath)
	}
	args = append(args, task)
	return exec.Command(r.name, args...)
}
