# Plan 017: Fix `--baseline previous` — it currently compares the skill against itself

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- cmd_run.go workspace.go runner.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1 (correctness bug in the tool's core comparison)
- **Effort**: S-M
- **Risk**: LOW-MED (behavior change for a flag that currently produces meaningless numbers)
- **Depends on**: —
- **Category**: bug fix
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

`skill-eval loop --baseline previous` is the documented iteration workflow
(`eval-workflow.md` §6: "run the loop again comparing against your previous
version"). The implementation snapshots the **current** skill directory
into the **current** iteration and uses that snapshot as the baseline — so
the with-skill and baseline runs execute with byte-identical skill content.
Every "delta" produced by this mode is pure run-to-run noise presented as a
skill-improvement signal. Users following the documented workflow are
iterating against a mirage. `prevIter` is computed and validated but never
used to *read* anything.

## Current state

- `cmd_run.go:115-127` — the bug:

```go
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
```

  `snapshotSkill(skillDir, ws, iter)` copies the **current** skill into
  `iteration-<iter>/skill-snapshot` (`workspace.go:56-62`), and that path
  becomes `baselinePath`. `prevIter` is only used for the `< 1` check.

- `workspace.go:56-62`:

```go
func snapshotSkill(skillDir, workspace string, iteration int) (string, error) {
	dst := filepath.Join(iterationPath(workspace, iteration), "skill-snapshot")
	if err := os.CopyFS(dst, os.DirFS(skillDir)); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	return dst, nil
}
```

- `runner.go:80-88` — `resolveSkillPath` returns `baselinePath` for the
  baseline config when it is neither empty nor `"none"`; the baseline
  agent then receives the snapshot as its "skill path". So today's
  `--baseline previous` runs are *with-skill vs same-skill*, not
  *with-skill vs previous-skill*.
- CLAUDE.md states the intended semantics: "baseline can be none, explicit
  path, or **previous-iteration snapshot**".
- Nothing else writes `skill-snapshot` directories, so on existing
  workspaces there is no previous snapshot to read — the fix must
  bootstrap the convention.

## Design decisions (read before coding)

1. **Always snapshot, resolve backward.** At the start of every run
   (new iterations only, not `--resume`), snapshot the current skill into
   `iteration-<iter>/skill-snapshot`. This records "what the skill looked
   like when iteration N ran" — provenance that is independently useful.
   `--baseline previous` then resolves to the **most recent earlier
   iteration that has a snapshot** (walk backward like
   `loadPreviousBenchmark`, `benchmark.go:84-101`), NOT the just-written
   one.
2. **Fail loudly when no earlier snapshot exists** (first run on an old
   workspace): error message explains that snapshots are recorded from now
   on and suggests `--baseline none` or an explicit `--baseline <path>`
   for this iteration.
3. **Print what was resolved**: `Baseline: skill snapshot from iteration 3`
   — silent wrong baselines are how this bug survived.
4. **Snapshot contents**: exclude the `evals/` subdirectory? No — keep the
   full copy (fixtures under `evals/files/` may be *needed* by the
   baseline run once Plan 007 lands, and skills are small). Do exclude
   nothing in v1; note disk growth is handled by Plan 023 (prune keeps
   snapshots).
5. **`--resume` never re-snapshots** — the iteration's snapshot must keep
   describing the skill state at the iteration's start.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `cmd_run.go` (snapshot-always; baseline resolution)
- `workspace.go` (new `findPreviousSnapshot` helper)
- Tests in `workspace_test.go` and a run-level test
- `eval-workflow.md`, `docs/guides/first-eval.md` (semantics note), CLAUDE.md workspace tree (add `skill-snapshot/`)

**Out of scope**:
- Do NOT migrate/repair old workspaces (no snapshots exist to recover).
- Do NOT change `--baseline <explicit-path>` or `--baseline none`.
- Do NOT deduplicate identical consecutive snapshots in v1 (content-hash
  short-circuit is a nice follow-up; note it in maintenance).

## Git workflow

- Branch: `advisor/017-fix-baseline-previous`
- Commit message style: `fix: make --baseline previous use the previous iteration's skill snapshot`
- Do NOT push unless instructed.

## Steps

### Step 1: Snapshot resolution helper

In `workspace.go`:

```go
// findPreviousSnapshot walks backward from iteration-1 below currentIter and
// returns the newest existing skill snapshot, or ("", 0) if none exists.
func findPreviousSnapshot(workspace string, currentIter int) (string, int) {
	for i := currentIter - 1; i >= 1; i-- {
		p := filepath.Join(iterationPath(workspace, i), "skill-snapshot")
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p, i
		}
	}
	return "", 0
}
```

**Verify**: `go test ./... -run TestFindPreviousSnapshot` — none, exact
N-1, gap (N-1 missing but N-3 present → N-3), currentIter 1 → none.

### Step 2: Always snapshot at iteration start

In `cmd_run.go`, in the non-resume branch (after `ensureDir(iterationPath(...))`,
`cmd_run.go:92-95`), snapshot unconditionally:

```go
		if _, err := snapshotSkill(skillDir, ws, iter); err != nil {
			return fmt.Errorf("recording skill snapshot: %w", err)
		}
```

Guard: `snapshotSkill` currently fails if `dst` exists (`os.CopyFS` refuses
overwrite? verify — if it merges silently, add an explicit
`os.Stat` + skip). A resumed iteration must keep its original snapshot.

**Verify**: `go test ./... -run TestRunWritesSnapshot` (stub `CmdBuilder`;
snapshot dir exists after run; resume does not rewrite it — compare a
sentinel file's mtime/content).

### Step 3: Fix the baseline resolution

Replace the buggy block (`cmd_run.go:115-127`) with:

```go
	if baselinePath == "previous" {
		snap, snapIter := findPreviousSnapshot(ws, iter)
		if snap == "" {
			return fmt.Errorf("no previous skill snapshot found in %s — snapshots are recorded from this version onward; use --baseline none for this iteration, then --baseline previous next time", ws)
		}
		baselinePath = snap
		fmt.Printf("Baseline: skill snapshot from iteration %d (%s)\n", snapIter, snap)
	}
```

Note ordering: the current-iteration snapshot (Step 2) is written *before*
this resolution, but `findPreviousSnapshot` starts at `currentIter-1`, so
it can never resolve to the snapshot just written. Add a test asserting
exactly that.

**Verify**: `go test ./... -run TestBaselinePrevious` — with iteration-1
snapshot content "v1" and current skill content "v2", the baseline run's
`skillPath` (captured via stub `CmdBuilder`) points into
`iteration-1/skill-snapshot` and reading it yields "v1", while the
with-skill run's points at the live skill dir.

### Step 4: Documentation

- `eval-workflow.md` §6: explain the corrected semantics — every run
  records a snapshot; `--baseline previous` compares against the newest
  earlier snapshot; first run after upgrading needs one plain run to seed
  a snapshot.
- CLAUDE.md workspace tree: add `iteration-1/skill-snapshot/`.
- CHANGELOG-worthy: this is a behavior fix for numbers that were previously
  meaningless — call it out plainly in the commit body.

### Step 5: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

- `TestFindPreviousSnapshot` (4 cases, Step 1).
- `TestRunWritesSnapshot` + resume-preserves-snapshot.
- `TestBaselinePrevious` (the end-to-end contract, Step 3).
- `TestBaselinePreviousNoSnapshotErrors` (old workspace → actionable error).
- Regression: `--baseline none` and `--baseline <path>` untouched.

## Done criteria

- [ ] Every new iteration records `skill-snapshot/` at start; resume preserves it.
- [ ] `--baseline previous` resolves to the newest earlier iteration's snapshot and says so on stdout.
- [ ] Missing earlier snapshot → clear error, not a self-comparison.
- [ ] Docs updated to the corrected semantics.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- `os.CopyFS` semantics on existing destinations differ from what Step 2
  assumes (check Go stdlib version behavior first — it returns an error if
  files would be overwritten; confirm on the toolchain in `go.mod`).
- Any existing user workflow is discovered to *depend* on the old
  behavior (unlikely — it was measuring noise — but check docs/site
  content for promises).
- Snapshot cost is nontrivial for real skills (e.g. `assets/` with large
  binaries) — add size guard + warning rather than silently copying.

## Maintenance notes

- Plan 023 (prune) must treat snapshots as load-bearing: never prune the
  newest snapshot older than the latest iteration.
- Follow-up: content-hash consecutive snapshots and hardlink/skip when the
  skill didn't change between iterations.
- Follow-up: `skill-eval report` could show *which* skill version (snapshot
  hash) each iteration ran, making trends auditable.
