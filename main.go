package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "skill-eval: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	subcmd, args, verbose := parseGlobalArgs(os.Args[1:])
	initLogger(verbose)

	if subcmd == "" {
		printUsage()
		return fmt.Errorf("no subcommand specified")
	}

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
	case "import-agit":
		return cmdImportAgit(args)
	case "report":
		return cmdReport(args)
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
  skill-eval benchmark                Aggregate results into benchmark.json
  skill-eval report [--iteration N]
                      [--llm-suggestions]
                                      Generate an HTML report from benchmark.json
  skill-eval loop                     Full cycle: run → grade → benchmark
  skill-eval import-agit              Convert recorded agit sessions into evals/evals.json

Flags:
  --verbose, -v               Enable structured debug logging to stderr
  --baseline <path|previous>  Baseline for runs (default: none)
  --baseline-only             Run only the baseline config
  --eval <id>                 Run/Grade a single eval by ID
  --global                    For init: create global config
  --fix                       (loop) Auto-refine failing evals up to --max-fix-attempts
  --max-fix-attempts <n>      Max fix attempts per eval (default: 3, with --fix)
  --models <a:m,a:m,...>      Run against multiple agent:model pairs (e.g. pi:claude-sonnet,claude)

Config:
  ~/.config/skill-eval/config.yaml   Global defaults
  <skill-dir>/.skill-eval.yaml       Per-skill overrides
`)
}

// --- helpers ---

// parseModels parses a comma-separated list of agent:model pairs.
func parseModels(raw string) ([]ModelConfig, error) {
	if raw == "" {
		return nil, nil
	}
	var models []ModelConfig
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		agent, model, _ := strings.Cut(part, ":")
		models = append(models, ModelConfig{Agent: strings.TrimSpace(agent), Model: strings.TrimSpace(model)})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("invalid --models value: %q", raw)
	}
	return models, nil
}

// parseGlobalArgs extracts the global --verbose/-v flag and returns the
// subcommand and its remaining arguments.
func parseGlobalArgs(raw []string) (subcmd string, args []string, verbose bool) {
	for _, a := range raw {
		if a == "-v" || a == "--verbose" {
			verbose = true
			continue
		}
		if subcmd == "" {
			subcmd = a
		} else {
			args = append(args, a)
		}
	}
	return
}

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
	baselineOnly := fs.Bool("baseline-only", false, "Run only the baseline config")
	evalID := fs.Int("eval", -1, "Run a single eval by ID")
	modelsRaw := fs.String("models", "", "Comma-separated agent:model pairs (e.g. pi:claude-sonnet,claude)")
	resume := fs.Bool("resume", false, "Resume the latest running iteration")
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
	if totalRuns > 10 && multiModel {
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

	ctx := context.Background()

	// Batch size: 2 concurrent runs
	sem := make(chan struct{}, 2)
	type runJob struct {
		eval     Eval
		modelKey string
		agent    string
		model    string
		config   string
	}

	for _, eval := range ef.Evals {
		if *evalID >= 0 && eval.ID != *evalID {
			continue
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

				// Build model-specific config
				runCfg := &Config{
					Defaults: DefaultsConfig{Agent: j.agent, Model: j.model},
					Judge:    cfg.Judge,
				}
				if runCfg.Defaults.Model == "" {
					runCfg.Defaults.Model = cfg.Defaults.Model
				}

				r, err := runEval(ctx, runCfg, skillDir, j.eval, ws, iter, j.modelKey, j.config, baselinePath)
				results[idx] = r
				errs[idx] = err
			}(i, job)
		}
		wg.Wait()

		for i, job := range jobs {
			if errs[i] != nil {
				return fmt.Errorf("eval %d %s/%s: %w", job.eval.ID, job.modelKey, job.config, errs[i])
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
	return nil
}

// --- grade ---

func cmdGrade(args []string) error {
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
	ctx := context.Background()

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
				gf, err := gradeEval(ctx, cfg, eval, ws, iter, gradeKey, config)
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

func cmdBenchmark(args []string) error {
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

// --- loop ---

func cmdLoop(args []string) error {
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	baselinePath := fs.String("baseline", "none", "Baseline for runs")
	baselineOnly := fs.Bool("baseline-only", false, "Run only the baseline config")
	modelsRaw := fs.String("models", "", "Comma-separated agent:model pairs")
	fixFlag := fs.Bool("fix", false, "Auto-refine failing evals")
	maxFixAttempts := fs.Int("max-fix-attempts", 3, "Max fix attempts per eval")
	resume := fs.Bool("resume", false, "Resume the latest running iteration")
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

	fmt.Println("\n[1/3] Running evals...")
	if err := cmdRun(runArgs); err != nil {
		return fmt.Errorf("run phase: %w", err)
	}

	gradeArgs := []string{}
	if *modelsRaw != "" {
		gradeArgs = append(gradeArgs, "--models", *modelsRaw)
	}

	fmt.Println("\n[2/3] Grading...")
	if err := cmdGrade(gradeArgs); err != nil {
		return fmt.Errorf("grade phase: %w", err)
	}

	benchArgs := []string{}
	if *modelsRaw != "" {
		benchArgs = append(benchArgs, "--models", *modelsRaw)
	}

	if *fixFlag {
		fmt.Println("\n[3/4] Auto-fixing failed evals...")
		if err := runFixPhase(*modelsRaw, *maxFixAttempts); err != nil {
			return fmt.Errorf("fix phase: %w", err)
		}
	} else {
		fmt.Println("\n[3/3] Benchmarking...")
		if err := cmdBenchmark(benchArgs); err != nil {
			return fmt.Errorf("benchmark phase: %w", err)
		}
		fmt.Println("\nLoop complete.")
		return nil
	}

	fmt.Println("\n[4/4] Benchmarking...")
	if err := cmdBenchmark(benchArgs); err != nil {
		return fmt.Errorf("benchmark phase: %w", err)
	}

	fmt.Println("\nLoop complete.")
	return nil
}

func runFixPhase(modelsRaw string, maxAttempts int) error {
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

	ctx := context.Background()

	for _, eval := range ef.Evals {
		for _, m := range models {
			mk := m.Key()
			gradingPath := filepath.Join(evalPath(ws, iter, eval.ID, mk), "with_skill", "grading.json")
			data, err := os.ReadFile(gradingPath)
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

			fr, err := fixEval(ctx, cfg, skillDir, eval, ws, iter, mk, "", maxAttempts)
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
