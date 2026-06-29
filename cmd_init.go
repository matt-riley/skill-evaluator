package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func cmdInit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	global := fs.Bool("global", false, "Create global config")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *global {
		return initGlobalConfig()
	}

	skillDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Verify we're in a skill directory
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		return fmt.Errorf("SKILL.md not found — run from a skill directory")
	}

	// Create evals/ dir and skeleton evals.json
	evalsDir := filepath.Join(skillDir, "evals")
	if err := ensureDir(evalsDir); err != nil {
		return err
	}

	evalsPath := filepath.Join(evalsDir, "evals.json")
	if _, err := os.Stat(evalsPath); os.IsNotExist(err) {
		skillName := filepath.Base(skillDir)
		skeleton := EvalFile{
			SkillName: skillName,
			Evals: []Eval{
				{
					ID:             1,
					Prompt:         "Describe the task you want the agent to perform here.",
					ExpectedOutput: "Describe what success looks like.",
					Files:          []string{},
					Assertions:     []string{"Example: The output includes a summary of results."},
				},
			},
		}
		data, err := json.MarshalIndent(skeleton, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling skeleton evals.json: %w", err)
		}
		if err := os.WriteFile(evalsPath, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", evalsPath)
	}

	// Create workspace
	ws := workspacePath(skillDir)
	if err := ensureDir(ws); err != nil {
		return err
	}
	fmt.Printf("Workspace: %s\n", ws)

	return nil
}

func initGlobalConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfgDir := filepath.Join(home, ".config", "skill-eval")
	if err := ensureDir(cfgDir); err != nil {
		return err
	}

	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("%s already exists, skipping.\n", cfgPath)
		return nil
	}

	cfg := `# Skill Evaluator global configuration
# https://github.com/matt-riley/skill-evaluator

defaults:
  agent: pi
  # model: claude-sonnet-4-5   # optional, uses agent's default if unset

judge:
  agent: pi
  # model: gpt-4o-mini         # optional, cheaper model recommended for grading
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", cfgPath)
	return nil
}

// --- run ---
