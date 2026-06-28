# Plan 004: Add structured `--verbose` logging for agent debugging

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat c6dac63..HEAD -- main.go runner.go grader.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S-M
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx
- **Planned at**: commit `c6dac63`, 2026-06-25

## Why this matters

Right now the CLI prints progress with `fmt.Printf` everywhere. When an agent shell-out fails, the user sees the exit error but not the command, args, or output that led to the failure. A leveled logger gated by `--verbose` makes the tool debuggable in CI and saves time reproducing agent failures.

## Current state

- Go 1.26.4, so `log/slog` is available in the standard library.
- `main.go:48-71` — `printUsage` lists flags; no `--verbose` flag exists.
- `runner.go:60-73` — `runEval` calls `cmd.CombinedOutput()` but never logs the command.
- `grader.go:48` — judge command is built but not logged.

Relevant excerpt from `main.go:48-71`:

```go
func printUsage() {
	fmt.Print(`skill-eval — automated skill evaluation

Usage:
  skill-eval init             Scaffold evals/evals.json + workspace
  skill-eval run              Run all evals (with-skill and baseline)
  ...
Flags:
  --baseline <path|previous>  Baseline for runs (default: none)
  --eval <id>                 Run/Grade a single eval by ID
  --global                    For init: create global config
  --fix                       (loop) Auto-refine failing evals up to --max-fix-attempts
  --max-fix-attempts <n>      Max fix attempts per eval (default: 3, with --fix)
  --models <a:m,a:m,...>      Run against multiple agent:model pairs
`)
}
```

Relevant excerpt from `runner.go:60-73`:

```go
	start := time.Now()
	cmd := buildAgentCmd(agent, model, task, skillPath)
	cmd.Dir = skillDir
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		result.Status = "failed"
		result.ErrMsg = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ErrMsg = string(exitErr.Stderr)
		}
	}
```

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w *.go` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `main.go`
- `runner.go`
- `grader.go`
- New `log.go` optional if it reduces duplication

**Out of scope**:
- Do NOT change the default non-verbose output format that users already rely on.
- Do NOT log secrets (agent tokens, file contents of user inputs).
- Do NOT log full judge output at info level; keep that debug-only.

## Git workflow

- Branch: `advisor/004-structured-logging`
- Commit message style: `feat: add --verbose flag and structured logging`.
- Do NOT push unless instructed.

## Steps

### Step 1: Add a package-level logger

Create `log.go` in the root package (or add to `main.go` if you prefer one less file) with:

```go
var logger *slog.Logger

func initLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
```

Import `log/slog` and `os`.

**Verify**: `go build ./...` passes.

### Step 2: Add `--verbose` flag parsing

Add a `-v` / `--verbose` boolean flag to `run()`, recognized before the subcommand. A simple approach: scan `os.Args` for `--verbose` or `-v` and set a package-level bool; remove it from args before dispatch, or add it to every subcommand's flag set.

Recommended: parse in `run()`:

```go
verbose := false
args := []string{}
for _, a := range os.Args[2:] {
	if a == "-v" || a == "--verbose" {
		verbose = true
		continue
	}
	args = append(args, a)
}
initLogger(verbose)
```

Then pass `args` instead of `os.Args[2:]` to the subcommands. Update `printUsage` to list `--verbose` in the global flags.

**Verify**: `skill-eval --verbose loop --help` shows `--verbose` and doesn't crash.

### Step 3: Convert key prints to log calls

In `runner.go`:
- Before `cmd.CombinedOutput()`, `logger.Debug("running agent", "agent", agent, "model", model, "dir", cmd.Dir)`.
- On failure, `logger.Debug("agent output", "output", string(output))`.
- After run, `logger.Info("eval completed", "eval", eval.ID, "config", configLabel, "status", result.Status, "duration_ms", result.Timing.DurationMs)`.

In `grader.go`:
- Before judge call, `logger.Debug("grading", "eval", eval.ID, "assertions", len(llmAssertions))`.
- On judge error, `logger.Warn("judge failed", "error", err)`.

In `main.go`:
- Keep user-facing progress banners as `fmt.Printf` (they are user output, not logs).
- Convert internal notes like `Snapshotted skill as baseline` or `no outputs, skipping` to `logger.Info`/`logger.Debug`.

**Verify**: `go test ./...` passes.

### Step 4: Add tests for logger setup

In `main_test.go` (from Plan 001), add:

- `TestVerboseFlagParsed`: call a helper that parses args and confirms `--verbose`/`-v` is recognized.

If that requires exporting helpers, keep the test surface minimal.

**Verify**: `go test ./...` passes.

### Step 5: Final checks

```bash
gofmt -w *.go
go test ./...
go vet ./...
golangci-lint run
```

All pass.

## Test plan

- `TestVerboseFlagParsed`: `-v` and `--verbose` both recognized.
- Existing tests still pass (logging does not change pure-function outputs).
- Manual smoke test: `go run . --verbose loop --help` prints help and exits 0.

## Done criteria

- [ ] `--verbose` / `-v` flag recognized globally (before subcommand).
- [ ] Default output (without flag) is unchanged for normal users.
- [ ] With `--verbose`, agent commands, exit errors, and durations are logged to stderr.
- [ ] No secrets or large file contents are logged at any level.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Parsing `--verbose` before subcommands breaks subcommand flag parsing.
- Changing prints to logs alters CLI output caught by tests or scripts.
- Any log call could emit user file contents or environment secrets.

## Maintenance notes

- Future features should prefer `logger.Debug` for operational details and `fmt.Printf` for intentional user-facing progress.
- Reviewers should check that new code doesn't reintroduce `fmt.Printf` for debug info that should be leveled logging.
- Follow-up: add a `--json` flag to switch `slog` to JSON output for CI pipelines.
