package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func cmdLoop(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	baselinePath := fs.String("baseline", "none", "Baseline for runs")
	baselineOnly := fs.Bool("baseline-only", false, "Run only the baseline config")
	modelsRaw := fs.String("models", "", "Comma-separated agent:model pairs")
	fixFlag := fs.Bool("fix", false, "Auto-refine failing evals")
	maxFixAttempts := fs.Int("max-fix-attempts", 3, "Max fix attempts per eval")
	resume := fs.Bool("resume", false, "Resume the latest running iteration")
	timeoutFlag := fs.Duration("timeout", 0, "Max duration per agent invocation (e.g. 5m)")
	parallelFlag := fs.Int("parallel", 2, "Concurrent agent invocations")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("=== skill-eval loop ===")

	runArgs := []string{"--baseline", *baselinePath}
	if *baselineOnly {
		runArgs = append(runArgs, "--baseline-only")
	}
	if *modelsRaw != "" {
		runArgs = append(runArgs, "--models", *modelsRaw)
	}
	if *resume {
		runArgs = append(runArgs, "--resume")
	}
	if *timeoutFlag > 0 {
		runArgs = append(runArgs, "--timeout", timeoutFlag.String())
	}
	if *parallelFlag != 2 {
		runArgs = append(runArgs, "--parallel", fmt.Sprintf("%d", *parallelFlag))
	}

	fmt.Println("\n[1/3] Running evals...")
	if err := cmdRun(ctx, runArgs); err != nil {
		return fmt.Errorf("run phase: %w", err)
	}

	gradeArgs := []string{}
	if *modelsRaw != "" {
		gradeArgs = append(gradeArgs, "--models", *modelsRaw)
	}

	fmt.Println("\n[2/3] Grading...")
	if err := cmdGrade(ctx, gradeArgs); err != nil {
		return fmt.Errorf("grade phase: %w", err)
	}

	benchArgs := []string{}
	if *modelsRaw != "" {
		benchArgs = append(benchArgs, "--models", *modelsRaw)
	}

	if *fixFlag {
		fmt.Println("\n[3/4] Auto-fixing failed evals...")
		if err := runFixPhase(ctx, *modelsRaw, *maxFixAttempts); err != nil {
			return fmt.Errorf("fix phase: %w", err)
		}
	} else {
		fmt.Println("\n[3/3] Benchmarking...")
		if err := cmdBenchmark(ctx, benchArgs); err != nil {
			return fmt.Errorf("benchmark phase: %w", err)
		}
		fmt.Println("\nLoop complete.")
		return nil
	}

	fmt.Println("\n[4/4] Benchmarking...")
	if err := cmdBenchmark(ctx, benchArgs); err != nil {
		return fmt.Errorf("benchmark phase: %w", err)
	}

	fmt.Println("\nLoop complete.")
	return nil
}

func runFixPhase(ctx context.Context, modelsRaw string, maxAttempts int) error {
	cliModels, err := parseModels(modelsRaw)
	if err != nil {
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

	models := resolveModels(cfg, cliModels)

	ws := workspacePath(skillDir)
	iter := nextIteration(ws) - 1
	if iter < 1 {
		return fmt.Errorf("no iterations found")
	}

	for _, eval := range ef.Evals {
		for _, m := range models {
			mk := m.Key()
			gradingPath := filepath.Join(evalPath(ws, iter, eval.ID, mk), "with_skill", "grading.json")
			data, err := os.ReadFile(gradingPath) // #nosec G304 -- gradingPath built via evalPath(), internal workspace convention
			if err != nil {
				fmt.Printf("  eval %d/%s: no grading, skipping\n", eval.ID, mk)
				continue
			}
			var gf GradingFile
			if err := json.Unmarshal(data, &gf); err != nil {
				fmt.Printf("  eval %d/%s: invalid grading, skipping\n", eval.ID, mk)
				continue
			}

			if gf.Summary.Failed == 0 {
				fmt.Printf("  eval %d/%s: already passing, skipping\n", eval.ID, mk)
				continue
			}

			fmt.Printf("  eval %d/%s: %d/%d failed — fixing...\n", eval.ID, mk, gf.Summary.Failed, gf.Summary.Total)

			fr, err := fixEval(ctx, cfg, skillDir, eval, ws, iter, mk, "", maxAttempts, nil)
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
	}

	return nil
}
