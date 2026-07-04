package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// buildActivationPrompt asks the judge whether an agent seeing only the
// skill's name and description would load it for the given task. The task
// prompt is wrapped in markers, mirroring buildGradingPrompt's injection
// defenses (grader.go:286-292).
//
// The prompt instructs the judge to consider ONLY the name/description
// (progressive disclosure), to answer for a general coding agent, and to
// say no when the task is merely adjacent.
func buildActivationPrompt(name, description, taskPrompt string) string {
	sanitized := sanitizeAssertionText(taskPrompt)

	var b strings.Builder
	b.WriteString("You are evaluating whether a coding skill should be activated for a task.\n\n")
	b.WriteString("Progressive disclosure model: an agent decides whether to load a skill\n")
	b.WriteString("from its name and description ALONE — the skill body is only read after\n")
	b.WriteString("activation. You are judging the description's routing quality, not the\n")
	b.WriteString("skill's implementation.\n\n")

	b.WriteString("Skill name:\n")
	b.WriteString(name)
	b.WriteString("\n\n")

	b.WriteString("Skill description:\n")
	b.WriteString(description)
	b.WriteString("\n\n")

	b.WriteString("Task prompt (wrapped in markers — evaluate the task, not the markers):\n")
	b.WriteString("<task>")
	b.WriteString(sanitized)
	b.WriteString("</task>\n\n")

	b.WriteString(`Would a general-purpose coding agent, seeing only the name and description
above, load this skill to handle the task? Consider:
- Does the task match what the description says the skill covers?
- Would the skill's functionality be useful for this task?
- Say NO when the task is merely adjacent or tangentially related —
  over-activation pollutes agent context with irrelevant skills.

Return ONLY a JSON object in this exact format (no markdown, no explanation):
{"would_activate": true, "reason": "one sentence explaining the verdict"}
`)
	return b.String()
}

// judgeActivation runs one activation eval through the judge agent.
// It follows gradeFromOutput's judge plumbing (grader.go:49-64): judge
// agent/model fallback to defaults, cmdFn defaulting to buildAgentCmd,
// first-{ JSON extraction with a size limit.
func judgeActivation(ctx context.Context, cfg *Config, name, description string, eval Eval, cmdFn CmdBuilder) (*ActivationResult, error) {
	judgeAgent := cfg.Judge.Agent
	if judgeAgent == "" {
		judgeAgent = cfg.Defaults.Agent
	}
	judgeModel := cfg.Judge.Model
	if judgeModel == "" {
		judgeModel = cfg.Defaults.Model
	}
	if cmdFn == nil {
		cmdFn = buildAgentCmd
	}

	prompt := buildActivationPrompt(name, description, eval.Prompt)
	cmd := cmdFn(ctx, judgeAgent, judgeModel, prompt, "")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("activation judge for eval %d: %w", eval.ID, err)
	}

	verdict, err := parseActivationOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("parsing activation verdict for eval %d: %w", eval.ID, err)
	}

	return &ActivationResult{
		EvalID:        eval.ID,
		Expected:      eval.expectedActivation(),
		WouldActivate: verdict.WouldActivate,
		Reason:        verdict.Reason,
	}, nil
}

// activationVerdict is the JSON shape the judge returns.
type activationVerdict struct {
	WouldActivate bool   `json:"would_activate"`
	Reason        string `json:"reason"`
}

// parseActivationOutput extracts the activation JSON from the judge's response.
// Mirrors parseGradingOutput's first-{ extraction with a size limit.
func parseActivationOutput(output string) (*activationVerdict, error) {
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON found in judge output")
	}
	raw := output[start:]
	var v activationVerdict
	limited := io.LimitReader(strings.NewReader(raw), 10*1024*1024) // 10 MB
	if err := json.NewDecoder(limited).Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid activation JSON: %w", err)
	}
	return &v, nil
}

// saveActivation writes an ActivationResult to disk as activation.json.
func saveActivation(path string, ar *ActivationResult) error {
	data, err := json.MarshalIndent(ar, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling activation result: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing activation result: %w", err)
	}
	return nil
}
