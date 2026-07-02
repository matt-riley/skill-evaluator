# Plan 023: `skill-eval prune` — workspace retention without losing history

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- workspace.go main.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: MED for a small plan — it deletes user data; the safety rules below are the plan
- **Depends on**: plans/017-fix-baseline-previous.md (soft — prune must know snapshots are load-bearing once 017 lands; the rule is written defensively either way)
- **Category**: hygiene
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Workspaces only grow. Every iteration accumulates full agent outputs for
every eval × model × config (× runs, after Plan 011; + transcripts after
Plan 012; + workdir copies after Plan 007), and nothing in the tool ever
deletes anything. Iterating seriously — the tool's whole point — means tens
of iterations; with file-producing evals that is easily gigabytes of stale
outputs whose *verdicts* are already distilled into small JSON files. Users
will eventually `rm -rf` by hand and destroy exactly the artifacts that
long-term trends (Plan 006's deltas, Plan 016's bucket history) depend on.
The tool should own its retention story: delete bulky evidence, keep every
verdict.

## Current state

- No deletion code exists anywhere (`grep -rn "RemoveAll" *.go` → nothing
  outside tests, verify at execution time).
- Workspace layout (CLAUDE.md + `workspace.go:15-33`):
  `<skill>-workspace/iteration-N/eval-M[/<model>]/<config>/{outputs/,timing.json,grading.json}`,
  plus `iteration-N/benchmark.json`, `.lock.json` (`workspace.go:64-67`),
  `skill-snapshot/` (after Plan 017), `fix-*/` attempt dirs
  (`runner.go:299-300`), and later `run-<r>/` (Plan 011), `workdir/`
  (Plan 007), `transcript.txt` (Plan 012), `process_quality.json`
  (Plan 015), `activation.json` (Plan 013).
- `findRunningIteration` / `readLock` (`workspace.go:82-93, 182-210`) give
  the tools to identify unsafe-to-touch iterations.

## Design decisions (read before coding)

1. **Two tiers, one default.** Default prune removes **bulk evidence** in
   old iterations: `outputs/` dirs (including inside `fix-*/` and
   `run-*/`), `workdir/` dirs, `transcript.txt` files. It **always keeps**:
   every `*.json` artifact (grading, timing, benchmark, lock, fix-results,
   process_quality, activation) and — see rule 3 — skill snapshots.
   `--all-artifacts` additionally removes entire old iteration directories
   except `benchmark.json` (moved up? no — keep the directory with only
   `benchmark.json` and `.lock.json` inside; simplest and preserves paths).
2. **Retention window**: `--keep N` (required, no default — deletion never
   happens by omission; `prune` with no flags prints usage + current size
   report and exits 0). The newest N iterations are untouched.
3. **Snapshots are load-bearing** (Plan 017's `--baseline previous`):
   never delete the newest snapshot outside the kept window; older
   snapshots go only under `--all-artifacts`.
4. **Never touch** an iteration whose lock status is `"running"`
   (`readLock`), regardless of age — even under `--all-artifacts`.
5. **`--dry-run` first-class**: prints exactly what would be deleted with
   per-iteration byte counts; the real run prints the same plus
   `freed 1.4 GB`. Both walk the same plan structure so dry-run cannot lie.
6. **Cache interaction** (Plan 021): pruning `outputs/` makes cached
   gradings un-re-verifiable but not invalid — the verdict JSONs remain;
   re-grading a pruned iteration is impossible anyway (no evidence). No
   special handling needed; document it.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Manual | `go run . prune --keep 3 --dry-run` | plan printed, nothing deleted |

## Scope

**In scope**:
- New `cmd_prune.go` + `cmd_prune_test.go`
- `main.go` (dispatch + usage)
- `workspace.go` (only if a shared iteration-listing helper is extracted)
- CLAUDE.md workspace section + `docs/guides/reading-results.md` (retention note)

**Out of scope**:
- Do NOT compress instead of delete (no tar/zip archiving in v1).
- Do NOT prune automatically from `run`/`loop` (explicit command only;
  auto-retention config is a follow-up once trust is earned).
- Do NOT touch anything outside the workspace directory, ever.

## Git workflow

- Branch: `advisor/023-workspace-prune`
- Commit message style: `feat: add prune subcommand for workspace retention`
- Do NOT push unless instructed.

## Steps

### Step 1: Plan builder (pure)

New `cmd_prune.go`:

```go
type pruneTarget struct {
	Path  string
	Bytes int64
}

type prunePlan struct {
	Iteration int
	Targets   []pruneTarget
	Skipped   string // non-empty reason: "running", "within --keep window", ...
}

// buildPrunePlan walks the workspace and decides, per iteration, what the
// given retention settings would delete. Pure planning: no deletion here.
func buildPrunePlan(ws string, keep int, allArtifacts bool) ([]prunePlan, error)
```

- Enumerate iterations by the same convention as `nextIteration`
  (`workspace.go:41-53`); sort descending; the first `keep` are marked
  skipped ("within --keep window").
- Per prunable iteration: read the lock; `"running"` → skipped
  ("running"). Then walk collecting targets per Design decision 1
  (match by base name: `outputs`, `workdir` dirs; `transcript.txt` files);
  under `--all-artifacts`, targets = everything in the iteration dir
  except `benchmark.json` and `.lock.json`, honoring rule 3 for the
  newest out-of-window `skill-snapshot`.
- Byte counts via `filepath.WalkDir` summing file sizes.

**Verify**: `go test ./... -run TestBuildPrunePlan` — synthetic workspace
(5 iterations, one running, one with snapshot); keep=2; assert per-tier
target sets, the running skip, and the snapshot preservation rule.

### Step 2: Executor + command

```go
func cmdPrune(ctx context.Context, args []string) error
```

- Flags: `--keep N` (int, required — error with usage when absent or < 1),
  `--dry-run`, `--all-artifacts`, `--skill <dir>`.
- No `--keep` → print the size report (per-iteration totals from the plan
  builder with keep=∞) and exit nil.
- Confirmation: unless `--dry-run` or global `--yes`
  (`skipsPrompts`, `main.go:12-14`), prompt
  `Delete 1.4 GB across 3 iterations? [y/N]` mirroring the run-phase
  prompt style (`cmd_run.go:143-151`).
- Execute with `os.RemoveAll` per target; collect errors, continue on
  failure, report failures at the end (partial prune is fine — targets are
  independent).
- Output format (both modes): per-iteration lines
  `iteration-2: outputs 312 MB, transcripts 4 MB  [would delete|deleted]`,
  skipped lines with reasons, total line.

**Verify**: `TestPruneDeletesPlanned` (temp workspace; JSONs survive,
outputs gone), `TestPruneDryRunDeletesNothing` (byte-identical tree
after), `TestPruneRefusesRunning`, `TestPruneNoKeepPrintsReport`.

### Step 3: Wire-up, docs, final checks

- `main.go`: `case "prune":` + usage line
  (`skill-eval prune --keep N     Delete bulky artifacts from old iterations (verdict JSONs are kept)`).
- CLAUDE.md: one line in the workspace section noting retention via prune.
- `docs/guides/reading-results.md`: what survives a prune and why trends
  keep working (benchmark/grading JSONs are never deleted by default).

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–2 carry it. The safety-critical assertions, stated once more
because this command deletes data: dry-run leaves the tree byte-identical;
default tier never deletes a `*.json`; running iterations are untouchable;
`--keep` absent deletes nothing.

## Done criteria

- [ ] `prune --keep N --dry-run` prints an accurate, sized deletion plan; without `--dry-run` it deletes exactly that plan after confirmation.
- [ ] Default tier removes only outputs/workdirs/transcripts; all JSON verdicts survive; `--all-artifacts` preserves `benchmark.json`, `.lock.json`, and the newest out-of-window snapshot.
- [ ] Running iterations and the newest N are never touched.
- [ ] `prune` with no flags is a size report, not a deletion.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Any target path computation can escape the workspace directory (add an
  `isPathWithin(ws, target)` assertion before every `RemoveAll` — if that
  assertion can fire in tests, the plan builder has a bug; fix before
  shipping, never suppress).
- The workspace contains unrecognized directory shapes in real usage
  (users put their own files there) — default tier must skip unknown
  names, not delete them; if that rule conflicts with `--all-artifacts`
  expectations, resolve toward preservation.

## Maintenance notes

- Every plan that adds a new bulky artifact (007 workdirs, 011 run dirs,
  012 transcripts) must add it to the default tier's match list and to
  `TestBuildPrunePlan` — reviewers should check prune coverage on any new
  workspace artifact.
- Follow-up: `retention:` block in config for auto-prune after `loop`,
  once the manual command has real-world mileage.
