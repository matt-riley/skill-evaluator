package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdGrade(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("grade", flag.ContinueOnError)
	doBenchmark := fs.Bool("benchmark", false, "Compute benchmark after grading")
	evalID := fs.Int("eval", -1, "Grade a single eval by ID")
	modelsRaw := fs.String("models", "", "Comma-separated agent:model pairs")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cliModels, err := parseModels(*modelsRaw)
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
		return fmt.Errorf("no iterations found — run 'skill-eval run' first")
	}

	lock, err := readLock(ws, iter)
	if err == nil && lock.Status == "running" {
		var missing []string
		for _, eval := range ef.Evals {
			if *evalID >= 0 && eval.ID != *evalID {
				continue
			}
			for _, m := range models {
				for _, config := range []string{"with_skill", "baseline"} {
					outputDir := filepath.Join(evalPath(ws, iter, eval.ID, m.Key()), config, "outputs")
					if _, err := os.Stat(outputDir); os.IsNotExist(err) {
						missing = append(missing, fmt.Sprintf("eval-%d %s/%s", eval.ID, m.Key(), config))
					}
				}
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("iteration %d is still running (missing: %s); finish with 'skill-eval run --resume'", iter, strings.Join(missing, ", "))
		}
		return fmt.Errorf("iteration %d is still running; finish with 'skill-eval run --resume'", iter)
	}

	fmt.Printf("Grading iteration %d\n", iter)

	var results []*RunResult

	for _, eval := range ef.Evals {
		if *evalID >= 0 && eval.ID != *evalID {
			continue
		}

		for _, m := range models {
			mk := m.Key()
			for _, config := range []string{"with_skill", "baseline"} {
				// Default to the model-keyed layout; fall back to the legacy
				// single-model (no model key) layout for backward compat.
				gradeKey := mk
				modelOutput := filepath.Join(evalPath(ws, iter, eval.ID, mk), config, "outputs")
				if _, err := os.Stat(modelOutput); os.IsNotExist(err) {
					if mk == m.Agent && len(models) == 1 {
						legacyOutput := filepath.Join(evalPath(ws, iter, eval.ID, ""), config, "outputs")
						if _, err := os.Stat(legacyOutput); err == nil {
							gradeKey = ""
						} else {
							logger.Debug("no outputs, skipping", "eval", eval.ID, "model", mk, "config", config)
							continue
						}
					} else {
						logger.Debug("no outputs, skipping", "eval", eval.ID, "model", mk, "config", config)
						continue
					}
				}

				fmt.Printf("  eval %d %s/%s... ", eval.ID, mk, config)
				gf, err := gradeEval(ctx, cfg, eval, ws, iter, gradeKey, config, nil)
				if err != nil {
					fmt.Printf("error: %v\n", err)
					continue
				}
				fmt.Printf("%d/%d passed\n", gf.Summary.Passed, gf.Summary.Total)
				results = append(results, &RunResult{
					EvalID:  eval.ID,
					Model:   gradeKey,
					Config:  config,
					Grading: gf,
				})
			}
		}
	}

	if *doBenchmark {
		return computeBenchmark(results, ws, iter)
	}
	return nil
}

// --- benchmark ---
