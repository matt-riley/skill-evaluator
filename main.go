package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
  --fix                       (loop) Auto-refine failing evals up to --max-fix-attempts
  --max-fix-attempts <n>      Max fix attempts per eval (default: 3, with --fix)

Config:
  ~/.config/skill-eval/config.yaml   Global defaults
  <skill-dir>/.skill-eval.yaml       Per-skill overrides
`)
}

// --- helpers ---

// --- init ---

func cmdInit(args []string) error {
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

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	baselineFlag := fs.String("baseline", "none", "Baseline for runs")
	evalID := fs.Int("eval", -1, "Run a single eval by ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	baselinePath := *baselineFlag

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
		if *evalID >= 0 && eval.ID != *evalID {
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
	fs := flag.NewFlagSet("grade", flag.ContinueOnError)
	doBenchmark := fs.Bool("benchmark", false, "Compute benchmark after grading")
	evalID := fs.Int("eval", -1, "Grade a single eval by ID")
	if err := fs.Parse(args); err != nil {
		return err
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
		if *evalID >= 0 && eval.ID != *evalID {
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

	if *doBenchmark {
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
		evalID, _ := strconv.Atoi(strings.TrimPrefix(e.Name(), "eval-"))

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
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	baselinePath := fs.String("baseline", "none", "Baseline for runs")
	fixFlag := fs.Bool("fix", false, "Auto-refine failing evals")
	maxFixAttempts := fs.Int("max-fix-attempts", 3, "Max fix attempts per eval")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("=== skill-eval loop ===")

	runArgs := []string{"--baseline", *baselinePath}

	fmt.Println("\n[1/3] Running evals...")
	if err := cmdRun(runArgs); err != nil {
		return fmt.Errorf("run phase: %w", err)
	}

	fmt.Println("\n[2/3] Grading...")
	if err := cmdGrade(nil); err != nil {
		return fmt.Errorf("grade phase: %w", err)
	}

	// Fix phase (only when --fix is set)
	if *fixFlag {
		fmt.Println("\n[3/4] Auto-fixing failed evals...")
		if err := runFixPhase(*maxFixAttempts); err != nil {
			return fmt.Errorf("fix phase: %w", err)
		}
	} else {
		fmt.Println("\n[3/3] Benchmarking...")
		if err := cmdBenchmark(nil); err != nil {
			return fmt.Errorf("benchmark phase: %w", err)
		}
		fmt.Println("\nLoop complete.")
		return nil
	}

	fmt.Println("\n[4/4] Benchmarking...")
	if err := cmdBenchmark(nil); err != nil {
		return fmt.Errorf("benchmark phase: %w", err)
	}

	fmt.Println("\nLoop complete.")
	return nil
}

// runFixPhase loads the current iteration's results and auto-refines each
// failing with-skill eval up to maxAttempts.
func runFixPhase(maxAttempts int) error {
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
		return fmt.Errorf("no iterations found")
	}

	ctx := context.Background()

	for _, eval := range ef.Evals {
		gradingPath := filepath.Join(evalPath(ws, iter, eval.ID), "with_skill", "grading.json")
		data, err := os.ReadFile(gradingPath)
		if err != nil {
			fmt.Printf("  eval %d: no grading, skipping\n", eval.ID)
			continue
		}
		var gf GradingFile
		if err := json.Unmarshal(data, &gf); err != nil {
			fmt.Printf("  eval %d: invalid grading, skipping\n", eval.ID)
			continue
		}

		if gf.Summary.Failed == 0 {
			fmt.Printf("  eval %d: already passing, skipping\n", eval.ID)
			continue
		}

		fmt.Printf("  eval %d: %d/%d failed — fixing...\n", eval.ID, gf.Summary.Failed, gf.Summary.Total)

		fr, err := fixEval(ctx, cfg, skillDir, eval, ws, iter, "", maxAttempts)
		if err != nil {
			fmt.Printf("    fix error: %v\n", err)
			continue
		}

		for _, a := range fr.Attempts {
			status := ""
			if a.Grading.Summary.Failed == 0 {
				status = " ✓"
			}
			fmt.Printf("    attempt %d: %d/%d passed%s\n",
				a.Attempt, a.Grading.Summary.Passed, a.Grading.Summary.Total, status)
		}

		if fr.Converged {
			fmt.Printf("    (plateaued at attempt %d)\n", len(fr.Attempts))
		}
		fmt.Printf("    best: attempt %d (%.0f%% pass)\n", fr.BestFix+1,
			fr.Attempts[fr.BestFix].Grading.Summary.PassRate*100)
	}

	return nil
}
