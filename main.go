package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
)

var (
	// skipsPrompts is set by --yes/-y to bypass interactive confirmation prompts.
	skipsPrompts bool
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "skill-eval: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	subcmd, args, verbose, yes := parseGlobalArgs(os.Args[1:])
	initLogger(verbose)
	skipsPrompts = yes

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if subcmd == "" {
		printUsage()
		return fmt.Errorf("no subcommand specified")
	}

	switch subcmd {
	case "init":
		return cmdInit(ctx, args)
	case "run":
		return cmdRun(ctx, args)
	case "grade":
		return cmdGrade(ctx, args)
	case "benchmark":
		return cmdBenchmark(ctx, args)
	case "loop":
		return cmdLoop(ctx, args)
	case "import-agit":
		return cmdImportAgit(ctx, args)
	case "report":
		return cmdReport(ctx, args)
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
  --yes, -y                   Skip confirmation prompts
  --baseline <path|previous>  Baseline for runs (default: none)
  --baseline-only             Run only the baseline config
  --eval <id>                 Run/Grade a single eval by ID
  --global                    For init: create global config
  --fix                       (loop) Auto-refine failing evals up to --max-fix-attempts
  --max-fix-attempts <n>      Max fix attempts per eval (default: 3, with --fix)
  --models <a:m,a:m,...>      Run against multiple agent:model pairs (e.g. pi:claude-sonnet,claude)
  --timeout <duration>        Max duration per agent invocation (e.g. 5m)
  --parallel <n>              Concurrent agent invocations (default: 2)

Config:
  ~/.config/skill-eval/config.yaml   Global defaults
  <skill-dir>/.skill-eval.yaml       Per-skill overrides
`)
}

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
		agent = strings.TrimSpace(agent)
		model = strings.TrimSpace(model)
		if err := ValidateAgent(agent); err != nil {
			return nil, fmt.Errorf("invalid agent in --models: %w", err)
		}
		models = append(models, ModelConfig{Agent: agent, Model: model})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("invalid --models value: %q", raw)
	}
	return models, nil
}

// parseGlobalArgs extracts the global --verbose/-v and --yes/-y flags and returns the
// subcommand and its remaining arguments.
func parseGlobalArgs(raw []string) (subcmd string, args []string, verbose bool, yes bool) {
	for _, a := range raw {
		if a == "-v" || a == "--verbose" {
			verbose = true
			continue
		}
		if a == "-y" || a == "--yes" {
			yes = true
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
