# Plan 031: Fail fast on missing or invalid agent runtimes — kill the `exec.CommandContext(ctx, "false")` trap

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- agent.go runner.go cmd_run.go cmd_grade.go cmd_loop.go report.go config.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW (adds preflight checks; no behavior change for valid setups)
- **Depends on**: — (Plan 029 documents the current trap; whichever lands second updates that troubleshooting entry — both plans say so)
- **Category**: error design / first-run experience
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

The single worst error-handling path in the codebase: when the configured
agent is invalid, `buildAgentCmd` logs a Warn (visible only with
`--verbose`) and silently substitutes `exec.CommandContext(ctx, "false")`
(`runner.go:98-108`). The user experiences every run instantly "failing"
with no message that mentions the agent at all — the most confusing
possible symptom for the most common first-run mistake. The adjacent
cases are nearly as bad:

- An agent that is *valid but not installed* (allowlisted name, binary not
  on PATH) fails inside `cmd.CombinedOutput()` with the error truncated
  into a Debug-level log (`runner.go:56-61`) — the user sees `FAILED`
  with no cause, once per eval × config × model, burning a whole
  iteration to discover a missing binary.
- `validateModel` failures are also warn-only (`runner.go:99-101`) and
  proceed to invoke the runtime with a rejected model string.
- The judge path has the same hole: `gradeFromOutput` builds the judge
  command the same way (`grader.go:59-64`), so a bad judge config degrades
  into "judge error" evidence on every assertion instead of one clear
  startup error.

`newAgentRunner` and `ValidateAgent` already produce excellent errors
(`agent.go:52-81`: allowlist, path-separator defense, "valid: pi, claude,
codex" hint) — they are simply swallowed at the one call site that
matters. The fix is preflight validation at command start, making the
swallowing path unreachable, without changing the `CmdBuilder` signature
(which would ripple through every test stub).

## Current state

- `runner.go:98-108` — the trap:

```go
var buildAgentCmd CmdBuilder = func(ctx context.Context, agent, model, task, skillPath string) *exec.Cmd {
	if err := validateModel(model); err != nil {
		logger.Warn("invalid model name", "error", err)
	}
	runner, err := newAgentRunner(agent)
	if err != nil {
		logger.Warn("invalid agent", "agent", agent, "error", err)
		// Return a command that will fail with a clear error message
		return exec.CommandContext(ctx, "false")
	}
	return runner.BuildContext(ctx, model, task, skillPath)
}
```

  (The comment "clear error message" is aspirational — `false` produces
  no message.)
- `agent.go:43-47` — `validAgents` allowlist (`pi`, `claude`, `codex`).
  Note: `configuration.md` and `main_test`-era docs mention `copilot`;
  the allowlist does not include it — docs plans 026 already corrects
  the page, keep this plan code-only.
- `agent.go:86-138` — each runner hardcodes its binary name (`"pi"`,
  `"claude"`, `"codex"` in `exec.CommandContext`); a preflight needs that
  name — today it is not exposed.
- Entry points that resolve an agent and should preflight:
  `cmdRun` (`cmd_run.go:37-47` — defaults + `--models`), `cmdGrade`
  (judge, `cmd_grade.go:31-41`), `cmdLoop` (delegates to both),
  `runFixPhase` (`cmd_loop.go:99-109`), `cmdReport --llm-suggestions`
  (`report.go:204` judge path), and Plan 016/027's authoring/promotion
  judge calls if landed.
- `internal/agit/client.go:30-37` shows the house pattern for
  binary-presence checks (`exec.LookPath` with a "is X installed and in
  PATH?" error).

## Design decisions (read before coding)

1. **Preflight, not signature change.** `CmdBuilder`'s
   `func(...) *exec.Cmd` shape stays (changing it ripples through every
   test stub and Plan 007/011/015's pending edits). Instead, a
   `preflightAgents([]ModelConfig, judge JudgeConfig) error` runs at the
   top of each entry point, after config/model resolution and before any
   iteration directory, lock, or cost prompt is created. It validates,
   per distinct agent: allowlist membership (`ValidateAgent`), binary
   presence (`exec.LookPath` on the runner's binary name), and model
   string validity (`validateModel`) — returning one aggregated,
   actionable error:

   ```
   agent runtime preflight failed:
     - agent "claud": unknown agent (valid: pi, claude, codex) — check defaults.agent in ~/.config/skill-eval/config.yaml
     - judge agent "pi": binary not found in PATH — install pi or set judge.agent
   ```

2. **Expose binary names on the runner.** `AgentRunner` gains
   `BinaryName() string` (three one-line implementations). Preflight uses
   `newAgentRunner(agent)` + `LookPath(runner.BinaryName())` — one source
   of truth, no parallel name table.
3. **Make the fallback loud and honest.** The `buildAgentCmd` fallback
   becomes unreachable in normal flow but stays as defense-in-depth —
   change it to print to **stderr** unconditionally (not just
   `logger.Warn`) before returning a failing command. On the command
   itself: `exec.CommandContext(ctx, "sh", "-c", ...)` shells out, and
   the current `false` relies on a POSIX binary that does not exist on
   Windows. The genuinely portable failing command is a deliberately
   **nonexistent binary name**:
   `exec.CommandContext(ctx, "skill-eval-invalid-agent-see-stderr")` —
   `Start` fails on every platform with "executable file not found", and
   the name itself points at the stderr explanation. Same treatment for
   the `validateModel` warn: upgrade to stderr.
4. **`--dry-run` synergy**: if Plan 020 landed, its dry-run output calls
   the same preflight and reports per-agent status (`agent pi: ok
   (/usr/local/bin/pi)`) — one function, two consumers.
5. **No network/auth probing.** Preflight checks name + binary presence
   only; whether the CLI is authenticated is the runtime's business
   (Plan 029's FAQ covers it). Do not invoke the binaries.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Manual | `skill-eval run` with `defaults.agent: nope` | one preflight error, no iteration dir created |

## Scope

**In scope**:
- `agent.go` (`BinaryName()` on the interface + implementations; new `preflightAgents`)
- `runner.go` (loud fallback per Design decision 3)
- `cmd_run.go`, `cmd_grade.go`, `cmd_loop.go` (`runFixPhase`), `report.go` (call preflight)
- Tests: `agent_test.go` (or alongside existing agent tests — check where `newAgentRunner` is tested), `runner_test.go`
- `docs/guides/troubleshooting.md` entry update IF Plan 029 landed (its missing-agent entry changes from "cryptic" to "preflight catches this")

**Out of scope**:
- Do NOT change the `CmdBuilder` signature.
- Do NOT add `copilot` (or any agent) to the allowlist — separate decision.
- Do NOT probe authentication or run version checks against the binaries.
- Do NOT preflight inside `import-agit` (it uses agit, which already has
  its own LookPath error) except the judge when `--author-assertions`
  (Plan 016) is in play — add that call in whichever plan lands second.

## Git workflow

- Branch: `advisor/031-fail-fast-runtime-preflight`
- Commit message style: `fix: preflight agent runtimes at startup instead of silently running false`
- Do NOT push unless instructed.

## Steps

### Step 1: `BinaryName()` and `preflightAgents`

In `agent.go`: add `BinaryName() string` to the `AgentRunner` interface
and the three runners (`"pi"`, `"claude"`, `"codex"` — read each
`BuildContext` to confirm the literal). Then:

```go
// preflightAgents verifies every distinct agent (run + judge) resolves to
// an allowlisted runner whose binary is on PATH, and that model strings
// are valid. Returns one aggregated error listing every problem, or nil.
func preflightAgents(models []ModelConfig, judge JudgeConfig) error
```

Dedupe agents before checking; label judge problems as such; include the
config-file hint in each line (Design decision 1). Make `LookPath`
swappable for tests (package-level `var lookPath = exec.LookPath`).

**Verify**: `go test ./... -run TestPreflightAgents` — ok path; unknown
agent; known-but-missing binary (swap lookPath); bad model; judge-only
failure; multiple failures aggregated in one error.

### Step 2: Call it at every entry point

- `cmdRun`: after `resolveModels` (`cmd_run.go:47`), before `ws`/lock
  work.
- `cmdGrade`: after config/models resolution (`:41`) — judge included.
- `runFixPhase`: after its config load (`cmd_loop.go:99-109`).
- `cmdReport`: only when `--llm-suggestions` (judge only).
- `cmdLoop` needs no direct call (delegates to run/grade), but verify the
  error surfaces cleanly through its phase wrapping (`"run phase: %w"`).

**Verify**: `TestRunPreflightBlocksBeforeIteration` — invalid agent →
error mentions the agent AND no `iteration-N` directory exists;
`TestGradePreflightCoversJudge`.

### Step 3: Loud fallback

Rework `buildAgentCmd` per Design decision 3 (stderr on both the model
and agent failure paths; keep returning a failing command). Update the
misleading comment.

**Verify**: `TestBuildAgentCmdFallbackIsLoud` — capture stderr (swap
`os.Stderr` via a package-level writer var if needed — simplest: route
through `fmt.Fprintf(errWriter, ...)` with `var errWriter io.Writer =
os.Stderr`); invalid agent produces a message containing the agent name
without `--verbose`.

### Step 4: Docs touch + final checks

If Plan 029 landed, update its missing-agent troubleshooting entry (the
symptom string changes). If not, nothing — 029's plan already documents
the post-031 world as a STOP-condition check.

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–3 tests, plus the regression guard: a fully valid config runs
exactly as before (stub lookPath returning success; assert zero
preflight-related output).

## Done criteria

- [ ] Invalid agent, missing binary, or bad model string fails ONCE, at startup, with an actionable aggregated message — before any workspace mutation or cost prompt.
- [ ] The judge is preflighted wherever it is used (grade, fix, report --llm-suggestions).
- [ ] The `false` fallback path prints the real reason to stderr unconditionally.
- [ ] `CmdBuilder` signature unchanged; all existing tests green.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Adding `BinaryName()` to the interface breaks an external or test
  implementation you can't see (grep for `AgentRunner` implementations
  first — including test doubles).
- Any entry point resolves agents lazily per-job in a way a single upfront
  preflight can't cover (re-read `cmd_run.go:211-218`'s per-job
  `runCfg` construction — it uses the same resolved agents, so it
  shouldn't, but confirm).
- The stderr-writer seam for testing gets invasive — prefer asserting on
  behavior (command fails + message once) over perfect capture.

## Maintenance notes

- Any new agent runtime must implement `BinaryName()` — the compiler now
  enforces what the allowlist used to only imply.
- Any new judge call site (Plans 013/016/027 add them) must call
  `preflightAgents` or route through an entry point that does — reviewers
  should check this on every new subcommand.
- Follow-up candidate: `skill-eval doctor` aggregating preflight + config
  validation + agit presence into one diagnostic command.
