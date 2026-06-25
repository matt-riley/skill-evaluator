# Plan 005: Add iteration lockfile and `--resume` for safe partial runs

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat c6dac63..HEAD -- workspace.go main.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/001-go-test-coverage.md
- **Category**: robustness
- **Planned at**: commit `c6dac63`, 2026-06-25

## Why this matters

`skill-eval run` can fire many agent invocations and take minutes. If it crashes or is cancelled after eval 7 of 10, the current iteration directory is half-written but there's no record of what completed. `loop` also assumes `run` and `grade` run atomically in the same process. A per-iteration lockfile records progress, lets the user resume safely, and prevents `grade` from running on incomplete iterations.

## Current state

- `workspace.go:27` — `nextIteration` returns `max + 1` without considering whether the latest iteration finished.
- `main.go:357` — `cmdGrade` uses `iter := nextIteration(ws) - 1`, assuming the highest-numbered iteration is ready to grade.
- `main.go:183-184` — `cmdRun` creates the iteration directory but writes no progress marker.

Relevant excerpt from `workspace.go:27-37`:

```go
func nextIteration(workspace string) int {
	max := 0
	entries, _ := os.ReadDir(workspace)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "iteration-")); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}
```

Relevant excerpt from `main.go:357-366` (grade iteration selection):

```go
	ws := workspacePath(skillDir)
	iter := nextIteration(ws) - 1
	if iter < 1 {
		return fmt.Errorf("no iterations found — run 'skill-eval run' first")
	}

	fmt.Printf("Grading iteration %d\n", iter)
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
- `workspace.go`
- `main.go` (`cmdRun`, `cmdGrade`, `cmdLoop`, `runFixPhase`)
- `eval.go` (new lockfile struct types only if needed)
- Tests in `workspace_test.go`

**Out of scope**:
- Do NOT change the iteration directory layout.
- Do NOT resume grading; `grade` remains a validation gate, not a resume target.
- Do NOT implement distributed locking or FS-based mutexes; this is single-machine, single-process safety.

## Git workflow

- Branch: `advisor/005-iteration-lockfile-resume`
- Commit message style: `feat: add iteration lockfile and --resume`.
- Do NOT push unless instructed.

## Steps

### Step 1: Define the lockfile type

In `eval.go`, add:

```go
type IterationLock struct {
	Iteration int           `json:"iteration"`
	Status    string        `json:"status"` // "running" | "complete"
	Completed []RunIdentity `json:"completed"`
	StartedAt time.Time     `json:"started_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type RunIdentity struct {
	EvalID int    `json:"eval_id"`
	Model  string `json:"model"`
	Config string `json:"config"`
}
```

**Verify**: `go build ./...` passes.

### Step 2: Add lockfile helpers in workspace.go

Add functions:

```go
func lockPath(workspace string, iter int) string
func readLock(workspace string, iter int) (*IterationLock, error)
func writeLock(workspace string, lock *IterationLock) error
func isCompleted(lock *IterationLock, evalID int, model, config string) bool
```

Use `json.MarshalIndent` to `iteration-N/.lock.json`.

**Verify**: Add unit tests in `workspace_test.go` covering read/write/completed checks.

### Step 3: Write lock at start of run

In `cmdRun`, after computing `iter` and `nextIteration`, if `--resume` is not set, call `nextIteration` normally. If `--resume` is set, find the latest iteration whose lock status is `"running"` and resume it.

Add `--resume` flag to `cmdRun`. When resuming:
- Read lock; if none found, error.
- Set `iter` to locked iteration.
- Skip eval/config combinations already in `Completed`.

At the start of run (whether new or resumed), write a lock with status `"running"`, `StartedAt` now, empty `Completed` (for new) or existing `Completed` (for resume).

**Verify**: `go test ./... -run TestLockfile` passes.

### Step 4: Mark completion after each run

In `cmdRun`, inside the goroutine after `runEval` returns (or after `wg.Wait` loop), mark each completed job in the lock. Use a mutex to protect concurrent writes to the lock, or build the completed list in memory and write once after `wg.Wait`.

Simplest safe approach: after `wg.Wait`, iterate `jobs` and `results` and append every completed job to `lock.Completed`, then write the lock. This may include previously completed items on resume; deduplicate by checking `isCompleted` first.

At the end of `cmdRun`, write the lock with status `"complete"`.

**Verify**: build passes.

### Step 5: Guard grade against incomplete iterations

In `cmdGrade`, before grading, read the lock for the target iteration. If status is `"running"`, return an error listing the missing eval/config pairs and suggesting `--resume`.

Also check that expected output directories exist; if missing, list them in the error.

**Verify**: unit test with a mock incomplete lock returns error.

### Step 6: Pass resume through loop and fix

In `cmdLoop`, add `--resume` flag and pass it to `cmdRun` via `runArgs`. `runFixPhase` should be unaffected.

**Verify**: `go run . loop --resume --help` compiles and shows the flag.

### Step 7: Final checks

```bash
gofmt -w *.go
go test ./...
go vet ./...
golangci-lint run
```

All pass.

## Test plan

- `TestLockReadWrite`: write a lock, read it back, fields match.
- `TestIsCompleted`: completed items recognized, non-completed not.
- `TestNextIterationWithLock`: not required unless you change `nextIteration`; if you do, cover it.
- `TestGradeBlocksIncompleteLock`: `cmdGrade` returns error on `"running"` lock.

## Done criteria

- [ ] `skill-eval run` writes `.lock.json` to the iteration directory.
- [ ] `.lock.json` lists every completed eval/model/config and final status `"complete"`.
- [ ] `skill-eval run --resume` continues the latest running iteration, skipping completed work.
- [ ] `skill-eval grade` refuses to grade an iteration whose lock status is `"running"`.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- `nextIteration` semantics need to change in a way that breaks existing usage.
- Adding the lockfile requires changing the workspace directory layout beyond one new file.
- Resume logic cannot be implemented without duplicating the run scheduling code significantly.

## Maintenance notes

- Any new subcommand that reads or writes iterations must respect the lockfile.
- Reviewers should ensure lockfile writes are atomic enough for the single-process case (write temp file + rename is ideal).
- Follow-up v1.x: expose a `skill-eval status` command that prints the latest iteration's lockfile.
