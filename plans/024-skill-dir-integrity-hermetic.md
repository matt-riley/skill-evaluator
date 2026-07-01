# Plan 024: Skill-directory integrity guard and always-hermetic runs

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go cmd_run.go workspace.go config.go schema/config-schema.json`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S-M
- **Risk**: MED (Part B changes where agents execute; Part A is read-only)
- **Depends on**: Part A: none. Part B: plans/007-agit-fixture-import-hermetic-runs.md (reuses `prepareWorkdir` and the workdir layout)
- **Category**: measurement validity
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Every eval run executes with the agent's working directory set to the
**live skill directory** (`runner.go:44`, `runner.go:309`). Nothing stops
a run from mutating the thing being measured:

- An agent told to "save outputs to <dir>" routinely also scatters
  scratch files in its cwd — into the skill folder.
- Worse, an agent can **edit SKILL.md itself** (agents asked to "improve
  X" have done stranger things), after which *every subsequent eval in the
  same iteration runs a different skill than the one benchmarked* — and
  the with-skill/baseline comparison, the snapshot provenance (Plan 017),
  and the corpus fingerprint (Plan 018) are all silently invalidated.
- Concurrent runs (`--parallel`, default 2, `cmd_run.go:161`) share that
  cwd, so runs can also contaminate *each other* through stray files.

Plan 007 introduces hermetic workdirs but only for evals that declare
input `files`. This plan (A) detects contamination for every run mode —
cheap, read-only, immediately shippable — and (B) closes the hole by
letting users run *all* evals hermetically.

## Current state

- `runner.go:43-44`:

```go
	cmd := cmdFn(ctx, agent, model, task, skillPath)
	cmd.Dir = skillDir
```

  (After Plan 007: `runDir` is the workdir only when `len(eval.Files) > 0`;
  otherwise still `skillDir`.)
- `runner.go:309` — `fixEval` same pattern.
- `workspace.go:56-62` — `snapshotSkill` copies the skill per iteration
  (unconditional after Plan 017) — the natural "before" reference.
- No hashing/integrity code exists.
- `config.go:27-30` — `DefaultsConfig` is where a `hermetic` knob belongs;
  `schema/config-schema.json` must mirror it.

## Design decisions (read before coding)

1. **Part A — detect, don't block.** Hash the skill tree at iteration
   start and after all runs complete; on mismatch, print a loud warning
   naming the changed/added/removed paths, and record
   `skill_dirty: true` + the changed path list in the iteration's
   `.lock.json` (extending `IterationLock` — additive field). Benchmark
   (Plan 018's loader or directly) surfaces it:
   `BenchmarkFile.SkillDirty bool` + report banner "the skill changed
   during this iteration — results are unreliable". Detection must not
   fail the run: measurement validity is the user's call.
2. **Tree hash**: walk the skill dir (skip `evals/` **workspace is outside
   the skill dir already**; skip nothing else; follow the same walk rules
   as `snapshotSkill`'s `os.CopyFS` view), sha256 of each file, then
   sha256 over sorted `path\x00hash` lines. Symlinks: hash the link target
   path string, do not follow (agents creating symlink loops must not hang
   the hasher).
3. **Part B — hermetic mode for everything**: config
   `defaults.hermetic: true` or flag `--hermetic` extends Plan 007's rule
   from "when eval has Files" to "always": every run gets
   `prepareWorkdir` (with zero fixture files it is just an empty scratch
   dir) and `cmd.Dir` = workdir. The skill path passed to the runtime
   (`skillPath` arg, `runner.go:43`) is already absolute — unaffected.
4. **Precedence**: flag `--hermetic` > config `defaults.hermetic` >
   default (off, matching Plan 007's Files-only behavior). Baseline runs
   follow the same rule (their snapshot skill path is also absolute).
5. **Interaction**: hermetic mode makes Part A's warning *rare* rather
   than redundant — the skill dir can still change via the user editing
   mid-iteration, which is exactly what the author needs to hear.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- New `integrity.go` + `integrity_test.go` (tree hash + diff description)
- `cmd_run.go` (hash before/after; lock field; `--hermetic` flag)
- `eval.go` (`IterationLock.SkillDirty/SkillChanges`, `BenchmarkFile.SkillDirty`)
- `runner.go` (Part B: hermetic condition change)
- `config.go` + `schema/config-schema.json` (`defaults.hermetic`)
- `benchmark.go`/`report.go` (surface the flagging)
- `eval-workflow.md` (hermetic mode + integrity warning docs)

**Out of scope**:
- Do NOT make the skill dir read-only at the OS level (chmod games break
  user editors and are not portable).
- Do NOT auto-restore the skill from the snapshot on contamination
  (destructive; the warning names the paths, the user decides).
- Do NOT hash the workspace (it is supposed to change).
- Do NOT extend hermetic mode to the fix phase differently than Plan 007
  did — same rule applies there via the shared code path.

## Git workflow

- Branch: `advisor/024-skill-dir-integrity-hermetic`
- Commit message style: `feat: detect skill mutation during runs; add always-hermetic mode`
- Do NOT push unless instructed.

## Steps

### Step 1: Tree hash (Part A core)

New `integrity.go`:

```go
// treeHash returns a stable digest of a directory's contents and the
// per-file digests it was computed from (path -> hash), for diffing.
func treeHash(dir string) (string, map[string]string, error)

// describeTreeDiff renders added/removed/changed paths between two
// per-file digest maps, capped at 20 entries.
func describeTreeDiff(before, after map[string]string) []string
```

Per Design decision 2. Deterministic across runs and platforms
(sort paths; forward-slash them).

**Verify**: `go test ./... -run TestTreeHash` — stable across two calls;
changes on content edit, file add, file remove, rename; symlink handled
without following; empty dir works.

### Step 2: Hook into the run lifecycle (Part A)

In `cmd_run.go`:
- After the iteration is established (post-lock, ~line 113): compute
  `beforeHash, beforeFiles`. Cost note: skills are small; if hashing takes
  >100ms on real skills, log at Debug and move on — never abort.
- After the final lock write (`cmd_run.go:250-254`): recompute; on
  mismatch:
  - print the warning block with `describeTreeDiff` output;
  - set `lock.SkillDirty = true`, `lock.SkillChanges = diff`, rewrite the
    lock (fields added to `IterationLock`, `eval.go:69-75`, additive JSON).
- Resume path: do NOT recompute "before" on resume (the original state is
  gone); compare against the previous "after" is impossible — instead, on
  resume, hash at resume-start and compare at end, and OR the dirty flag
  with any already recorded. Comment why: partial coverage beats false
  assurance.

**Verify**: `TestRunDetectsSkillMutation` — stub `CmdBuilder` whose
command writes a file into the skill dir → lock carries
`skill_dirty: true` with the path named; clean run → field absent.

### Step 3: Surface it (Part A)

- `benchmark.go`: when the iteration's lock has `SkillDirty`, set
  `bf.SkillDirty = true` (loader already reads the lock after Plan 018's
  Step 3; otherwise read it here via `readLock`).
- `report.go`: red banner when set: "SKILL.md or bundled files changed
  while evals were running — treat this iteration's numbers as invalid.
  Changed: <paths>". Changed paths: read from the lock via `ReportData`.

**Verify**: `TestBenchmarkCarriesSkillDirty`, `TestReportRendersDirtyBanner`.

### Step 4: Always-hermetic mode (Part B — requires Plan 007)

- `config.go`: `DefaultsConfig` gains `Hermetic *bool \`yaml:"hermetic"\``;
  merge; schema entry.
- `cmd_run.go`: `hermetic := fs.Bool("hermetic", false, "Run every eval in an isolated scratch directory (implies per-run workdirs even without input files)")`;
  resolve precedence (flag set → true; else config; else false) and thread
  a single `hermetic bool` into `runEval`.
- `runner.go`: the Plan 007 condition `len(eval.Files) > 0` becomes
  `hermetic || len(eval.Files) > 0`. `fixEval` receives the same flag
  (thread through `runFixPhase` → check its call chain,
  `cmd_loop.go:88-163`).
- `cmd_loop.go`: forward `--hermetic`.

**Verify**: `TestHermeticFlagForcesWorkdir` — file-less eval with
`--hermetic` runs in `.../workdir` (stub `CmdBuilder` capturing `cmd.Dir`);
without it, in `skillDir`; config-only setting honored; flag overrides
config false→true.

### Step 5: Docs and final checks

`eval-workflow.md`: short "Keeping runs honest" section — what the dirty
warning means, when to use `--hermetic` (recommendation: always, once
comfortable), and that hermetic + `--parallel` also isolates concurrent
runs from each other.

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–4 carry the tests. Cross-cutting: `TestHermeticCleanIntegrity` —
a hermetic run whose agent writes into its cwd leaves the skill dir hash
unchanged (the two features composing correctly is the point of the plan).

## Done criteria

- [ ] Every run hashes the skill tree before/after; mutation produces a loud warning, lock fields, and a report banner naming changed paths.
- [ ] Detection never fails or blocks a run.
- [ ] `--hermetic` / `defaults.hermetic` route every eval (and fix attempts) through per-run workdirs; default behavior unchanged.
- [ ] Resume semantics documented in code (partial coverage, OR-ed flag).
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Plan 007 has not landed and Part B is wanted — Part A ships alone (it
  has independent value); do not reimplement workdirs here.
- Tree hashing on a real-world skill takes long enough to be user-visible
  (>1s) — add a size-based bail-out (hash only SKILL.md + file list) and
  note the reduced coverage, rather than slowing every run.
- Any agent runtime turns out to resolve the skill *relative to cwd*
  (Plan 007's STOP condition rechecked here for the hermetic-always case)
  — hermetic mode must not silently break that runtime; gate it per-agent
  if needed.

## Maintenance notes

- The integrity warning's value is its loudness; resist demoting it to a
  Debug log when it fires in someone's noisy setup — a firing warning
  means the numbers are wrong.
- Once hermetic mode has mileage, consider flipping the default to on in
  a major release; the `Files`-only behavior then becomes the special case.
- Plan 023 (prune) already treats `workdir/` as bulk evidence — more
  workdirs from hermetic mode increase prune's value; no action needed.
