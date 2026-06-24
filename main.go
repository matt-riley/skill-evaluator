package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "skill-eval: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("no subcommand specified")
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	switch subcmd {
	case "init":
		return cmdInit(args)
	case "run":
		return cmdRun(args)
	case "grade":
		return cmdGrade(args)
	case "benchmark":
		return cmdBenchmark(args)
	case "loop":
		return cmdLoop(args)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand: %s", subcmd)
	}
}

func printUsage() {
	fmt.Print(`skill-eval — automated skill evaluation

Usage:
  skill-eval init             Scaffold evals/evals.json + workspace
  skill-eval run              Run all evals (with-skill and baseline)
  skill-eval grade            Grade all runs in the current iteration
  skill-eval benchmark        Aggregate results into benchmark.json
  skill-eval loop             Full cycle: run → grade → benchmark

Flags:
  --baseline <path|previous>  Baseline for runs (default: none)
  --eval <id>                 Run/Grade a single eval by ID
  --global                    For init: create global config

Config:
  ~/.config/skill-eval/config.yaml   Global defaults
  <skill-dir>/.skill-eval.yaml       Per-skill overrides
`)
}

// --- helpers ---

// parseEvalFlag scans args for --eval <id> and returns the ID, or -1 if not found.
func parseEvalFlag(args []string) int {
	for i := 0; i < len(args); i++ {
		if args[i] == "--eval" && i+1 < len(args) {
			var id int
			_, _ = fmt.Sscanf(args[i+1], "%d", &id)
			return id
		}
	}
	return -1
}

// --- init ---

func cmdInit(args []string) error {
	for _, a := range args {
		if a == "--global" {
			return initGlobalConfig()
		}
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
		data, _ := json.MarshalIndent(skeleton, "", "  ")
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
# https://github.com/biztocorp/skill-evaluator

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

func cmdRun(args []string) error {
	baselinePath := "none"
	singleEvalID := parseEvalFlag(args)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--baseline":
			if i+1 < len(args) {
				baselinePath = args[i+1]
				i++
			}
		}
	}

	skillDir, err := detectSkillDir()
	if err != nil {
		return err
	}

	cfg, err := LoadConfig(skillDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ef, err := readEvals(skillDir)
	if err != nil {
		return err
	}

	ws := workspacePath(skillDir)
	iter := nextIteration(ws)
	if err := ensureDir(iterationPath(ws, iter)); err != nil {
		return err
	}

	fmt.Printf("Iteration %d — %d evals\n", iter, len(ef.Evals))

	// Resolve baseline
	if baselinePath == "previous" {
		prevIter := iter - 1
		if prevIter < 1 {
			return fmt.Errorf("no previous iteration to use as baseline")
		}
		snapshotPath, err := snapshotSkill(skillDir, ws, iter)
		if err != nil {
			return err
		}
		baselinePath = snapshotPath
		fmt.Printf("Snapshotted skill as baseline: %s\n", snapshotPath)
	}

	ctx := context.Background()

	for _, eval := range ef.Evals {
		if singleEvalID >= 0 && eval.ID != singleEvalID {
			continue
		}

		fmt.Printf("\nEval %d: %s\n", eval.ID, truncate(eval.Prompt, 80))

		// With-skill run
		fmt.Print("  with_skill... ")
		r, err := runEval(ctx, cfg, skillDir, eval, ws, iter, "with_skill", baselinePath)
		if err != nil {
			return fmt.Errorf("eval %d with_skill: %w", eval.ID, err)
		}
		fmt.Printf("%s (%dms)\n", r.Status, r.Timing.DurationMs)

		// Baseline run
		fmt.Print("  baseline... ")
		r, err = runEval(ctx, cfg, skillDir, eval, ws, iter, "baseline", baselinePath)
		if err != nil {
			return fmt.Errorf("eval %d baseline: %w", eval.ID, err)
		}
		fmt.Printf("%s (%dms)\n", r.Status, r.Timing.DurationMs)
	}

	fmt.Printf("\nDone. Results in %s\n", iterationPath(ws, iter))
	return nil
}

// --- grade ---

func cmdGrade(args []string) error {
	doBenchmark := false
	singleEvalID := parseEvalFlag(args)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--benchmark":
			doBenchmark = true
		}
	}

	skillDir, err := detectSkillDir()
	if err != nil {
		return err
	}

	cfg, err := LoadConfig(skillDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ef, err := readEvals(skillDir)
	if err != nil {
		return err
	}

	ws := workspacePath(skillDir)
	iter := nextIteration(ws) - 1
	if iter < 1 {
		return fmt.Errorf("no iterations found — run 'skill-eval run' first")
	}

	fmt.Printf("Grading iteration %d\n", iter)

	var results []*RunResult
	ctx := context.Background()

	for _, eval := range ef.Evals {
		if singleEvalID >= 0 && eval.ID != singleEvalID {
			continue
		}

		for _, config := range []string{"with_skill", "baseline"} {
			evalDir := evalPath(ws, iter, eval.ID)
			outputDir := filepath.Join(evalDir, config, "outputs")

			if _, err := os.Stat(outputDir); os.IsNotExist(err) {
				fmt.Printf("  eval %d %s: no outputs, skipping\n", eval.ID, config)
				continue
			}

			fmt.Printf("  eval %d %s... ", eval.ID, config)
			gf, err := gradeEval(ctx, cfg, eval, ws, iter, config)
			if err != nil {
				fmt.Printf("error: %v\n", err)
				continue
			}
			fmt.Printf("%d/%d passed\n", gf.Summary.Passed, gf.Summary.Total)
			results = append(results, &RunResult{
				EvalID:  eval.ID,
				Config:  config,
				Grading: gf,
			})
		}
	}

	if doBenchmark {
		return computeBenchmark(results, ws, iter)
	}
	return nil
}

// --- benchmark ---

func cmdBenchmark(args []string) error {
	skillDir, err := detectSkillDir()
	if err != nil {
		return err
	}

	ws := workspacePath(skillDir)
	iter := nextIteration(ws) - 1
	if iter < 1 {
		return fmt.Errorf("no iterations found")
	}

	// Collect grading results from all evals
	var results []*RunResult
	entries, err := os.ReadDir(iterationPath(ws, iter))
	if err != nil {
		return fmt.Errorf("reading iteration: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "eval-") {
			continue
		}
		var evalID int
		_, _ = fmt.Sscanf(e.Name(), "eval-%d", &evalID)

		for _, config := range []string{"with_skill", "baseline"} {
			gradingPath := filepath.Join(iterationPath(ws, iter), e.Name(), config, "grading.json")
			data, err := os.ReadFile(gradingPath)
			if err != nil {
				continue
			}
			var gf GradingFile
			if err := json.Unmarshal(data, &gf); err != nil {
				continue
			}
			results = append(results, &RunResult{
				EvalID:  evalID,
				Config:  config,
				Grading: &gf,
			})

			// Also load timing
			timingPath := filepath.Join(iterationPath(ws, iter), e.Name(), config, "timing.json")
			if td, err := os.ReadFile(timingPath); err == nil {
				var t TimingData
				if json.Unmarshal(td, &t) == nil {
					results[len(results)-1].Timing = &t
				}
			}
		}
	}

	return computeBenchmark(results, ws, iter)
}

// --- loop ---

func cmdLoop(args []string) error {
	baselinePath := "none"
	for i := 0; i < len(args); i++ {
		if args[i] == "--baseline" && i+1 < len(args) {
			baselinePath = args[i+1]
			i++
		}
	}

	fmt.Println("=== skill-eval loop ===")

	// Build run args
	runArgs := []string{}
	if baselinePath != "" {
		runArgs = append(runArgs, "--baseline", baselinePath)
	}

	fmt.Println("\n[1/3] Running evals...")
	if err := cmdRun(runArgs); err != nil {
		return fmt.Errorf("run phase: %w", err)
	}

	fmt.Println("\n[2/3] Grading...")
	if err := cmdGrade([]string{"--benchmark"}); err != nil {
		return fmt.Errorf("grade phase: %w", err)
	}

	fmt.Println("\n[3/3] Loop complete.")
	return nil
}
