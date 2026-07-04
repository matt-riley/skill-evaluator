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
		return computeBenchmark(results, ws, iter, gradeActivations(ctx, cfg, ef, skillDir, ws, iter, *evalID))
	}
	return nil
}

// gradeActivations judges all activation evals and returns their results.
// Reads the skill's frontmatter (name/description) once via parseSkillMD.
// On parse error, prints a warning pointing to `skill-eval validate` and
// returns nil (no activation results — task evals still benchmark fine).
func gradeActivations(ctx context.Context, cfg *Config, ef *EvalFile, skillDir, ws string, iter int, evalID int) []ActivationResult {
	var activationEvals []Eval
	for _, eval := range ef.Evals {
		if !eval.isActivation() {
			continue
		}
		if evalID >= 0 && eval.ID != evalID {
			continue
		}
		activationEvals = append(activationEvals, eval)
	}
	if len(activationEvals) == 0 {
		return nil
	}

	// Read frontmatter once.
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillMDPath) // #nosec G304 -- skillDir from detectSkillDir(), SKILL.md is the conventional path
	if err != nil {
		fmt.Printf("  activation: cannot read SKILL.md: %v — run 'skill-eval validate' to check\n", err)
		return nil
	}
	_, sf, _, err := parseSkillMD(data)
	if err != nil {
		fmt.Printf("  activation: SKILL.md parse error: %v — run 'skill-eval validate'\n", err)
		return nil
	}
	if sf.Name == "" || sf.Description == "" {
		fmt.Printf("  activation: SKILL.md missing name or description — run 'skill-eval validate'\n")
		return nil
	}

	var activations []ActivationResult
	for _, eval := range activationEvals {
		fmt.Printf("  eval %d activation... ", eval.ID)
		ar, err := judgeActivation(ctx, cfg, sf.Name, sf.Description, eval, nil)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		verdictStr := "no"
		if ar.WouldActivate {
			verdictStr = "yes"
		}
		expectedStr := "yes"
		if !ar.Expected {
			expectedStr = "no"
		}
		fmt.Printf("would_activate=%s (expected %s)\n", verdictStr, expectedStr)

		// Write activation.json — model-independent, no config subdir.
		actPath := filepath.Join(evalPath(ws, iter, eval.ID, ""), "activation.json")
		if err := saveActivation(actPath, ar); err != nil {
			fmt.Printf("  warning: could not save activation.json: %v\n", err)
		}

		activations = append(activations, *ar)
	}
	return activations
}

// --- benchmark ---
