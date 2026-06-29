package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func cmdBenchmark(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
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
		return fmt.Errorf("no iterations found")
	}

	var results []*RunResult
	for _, eval := range ef.Evals {
		for _, m := range models {
			mk := m.Key()
			for _, config := range []string{"with_skill", "baseline"} {
				gradingPath := filepath.Join(evalPath(ws, iter, eval.ID, mk), config, "grading.json")

				if _, err := os.Stat(gradingPath); os.IsNotExist(err) {
					if mk == m.Agent && len(models) == 1 {
						gradingPath = filepath.Join(evalPath(ws, iter, eval.ID, ""), config, "grading.json")
					} else {
						continue
					}
				}

				data, err := os.ReadFile(gradingPath)
				if err != nil {
					continue
				}
				var gf GradingFile
				if err := json.Unmarshal(data, &gf); err != nil {
					continue
				}

				rr := &RunResult{
					EvalID:  eval.ID,
					Model:   mk,
					Config:  config,
					Grading: &gf,
				}

				timingPath := filepath.Join(filepath.Dir(gradingPath), "timing.json")
				if td, err := os.ReadFile(timingPath); err == nil {
					var t TimingData
					if json.Unmarshal(td, &t) == nil {
						rr.Timing = &t
					}
				}

				results = append(results, rr)
			}
		}
	}

	return computeBenchmark(results, ws, iter)
}
