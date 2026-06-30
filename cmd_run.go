package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"
)

func cmdRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	baselineFlag := fs.String("baseline", "none", "Baseline for runs")
	baselineOnly := fs.Bool("baseline-only", false, "Run only the baseline config")
	evalID := fs.Int("eval", -1, "Run a single eval by ID")
	modelsRaw := fs.String("models", "", "Comma-separated agent:model pairs (e.g. pi:claude-sonnet,claude)")
	resume := fs.Bool("resume", false, "Resume the latest running iteration")
	timeoutFlag := fs.Duration("timeout", 0, "Max duration per agent invocation (e.g. 5m)")
	parallelFlag := fs.Int("parallel", 2, "Concurrent agent invocations")
	if err := fs.Parse(args); err != nil {
		return err
	}
	baselinePath := *baselineFlag

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
	multiModel := len(cliModels) > 0 || len(cfg.Models) > 0

	ws := workspacePath(skillDir)

	var iter int
	var lock *IterationLock
	if *resume {
		var err error
		iter, lock, err = findRunningIteration(ws)
		if err != nil {
			return fmt.Errorf("cannot resume: %w", err)
		}
		fmt.Printf("Resuming iteration %d\n", iter)
	} else {
		iter = nextIteration(ws)
		lock = &IterationLock{
			Iteration: iter,
			Status:    "running",
			Completed: []RunIdentity{},
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := ensureDir(iterationPath(ws, iter)); err != nil {
			return err
		}
	}

	// Acquire an exclusive advisory lock on the iteration directory
	// to prevent concurrent runs from corrupting state.
	iterDir := iterationPath(ws, iter)
	lockFd, err := acquireLock(iterDir)
	if err != nil {
		return fmt.Errorf("cannot acquire lock on iteration %d: %w", iter, err)
	}
	defer func() {
		if err := releaseLock(lockFd); err != nil {
			logger.Warn("failed to release lock", "error", err)
		}
	}()

	lock.UpdatedAt = time.Now()
	if err := writeLock(ws, lock); err != nil {
		return fmt.Errorf("writing lock: %w", err)
	}

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

	// Count total runs for cost warning
	evalCount := 0
	for _, eval := range ef.Evals {
		if *evalID >= 0 && eval.ID != *evalID {
			continue
		}
		evalCount++
	}
	configsToRun := []string{"with_skill", "baseline"}
	if *baselineOnly {
		configsToRun = []string{"baseline"}
	}
	totalRuns := evalCount * len(models) * len(configsToRun)
	if totalRuns > 10 && multiModel && !skipsPrompts {
		fmt.Printf("⚠️  This will run %d agent invocations (%d evals × %d models × %d configs).\n", totalRuns, evalCount, len(models), len(configsToRun))
		fmt.Print("Continue? [y/N]: ")
		var answer string
		_, _ = fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return fmt.Errorf("aborted")
		}
	}

	modelNames := make([]string, len(models))
	for i, m := range models {
		modelNames[i] = m.Key()
	}

	fmt.Printf("Iteration %d — %d evals × %d model(s)\n", iter, evalCount, len(models))

	// Batch size: configurable concurrent runs
	sem := make(chan struct{}, *parallelFlag)

	type runJob struct {
		eval     Eval
		modelKey string
		agent    string
		model    string
		config   string
	}

	var hadFailure bool
	for _, eval := range ef.Evals {
		if *evalID >= 0 && eval.ID != *evalID {
			continue
		}
		// Bail early if the context has been cancelled (e.g. Ctrl+C)
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		fmt.Printf("\nEval %d: %s\n", eval.ID, truncate(eval.Prompt, 80))

		var jobs []runJob
		for _, m := range models {
			for _, config := range configsToRun {
				if *resume && isCompleted(lock, eval.ID, m.Key(), config) {
					continue
				}
				jobs = append(jobs, runJob{eval, m.Key(), m.Agent, m.Model, config})
			}
		}

		results := make([]*RunResult, len(jobs))
		errs := make([]error, len(jobs))
		var wg sync.WaitGroup

		for i, job := range jobs {
			wg.Add(1)
			go func(idx int, j runJob) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// Per-invocation timeout: each agent call gets its own deadline
				runCtx := ctx
				var cancel context.CancelFunc
				if *timeoutFlag > 0 {
					runCtx, cancel = context.WithTimeout(ctx, *timeoutFlag)
					defer cancel()
				}

				// Build model-specific config
				runCfg := &Config{
					Defaults: DefaultsConfig{Agent: j.agent, Model: j.model},
					Judge:    cfg.Judge,
				}
				if runCfg.Defaults.Model == "" {
					runCfg.Defaults.Model = cfg.Defaults.Model
				}

				r, err := runEval(runCtx, runCfg, skillDir, j.eval, ws, iter, j.modelKey, j.config, baselinePath, nil)
				results[idx] = r
				errs[idx] = err
			}(i, job)
		}
		wg.Wait()

		for i, job := range jobs {
			if errs[i] != nil {
				fmt.Printf("  %s/%-14s FAILED: %v\n", job.modelKey, job.config, errs[i])
				if !isCompleted(lock, job.eval.ID, job.modelKey, job.config) {
					lock.Completed = append(lock.Completed, RunIdentity{EvalID: job.eval.ID, Model: job.modelKey, Config: job.config})
				}
				hadFailure = true
				continue
			}
			r := results[i]
			if !isCompleted(lock, job.eval.ID, job.modelKey, job.config) {
				lock.Completed = append(lock.Completed, RunIdentity{EvalID: job.eval.ID, Model: job.modelKey, Config: job.config})
			}
			fmt.Printf("  %s/%-14s %s (%dms)\n", job.modelKey, job.config, r.Status, r.Timing.DurationMs)
		}

		lock.UpdatedAt = time.Now()
		lock.Status = "running"
		if err := writeLock(ws, lock); err != nil {
			return fmt.Errorf("writing lock: %w", err)
		}
	}

	lock.UpdatedAt = time.Now()
	lock.Status = "complete"
	if err := writeLock(ws, lock); err != nil {
		return fmt.Errorf("writing final lock: %w", err)
	}

	fmt.Printf("\nDone. Results in %s\n", iterationPath(ws, iter))
	if hadFailure {
		return fmt.Errorf("one or more evals failed; check output above for details")
	}
	return nil
}
