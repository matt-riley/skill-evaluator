# Plan 007: Seed eval input files from agit blobs and run evals in hermetic workdirs

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go eval.go cmd_import_agit.go internal/agit/client.go internal/agit/convert.go internal/agit/types.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: HIGH (touches the runner's working-directory contract and shells out to an agit interface that must be probed first)
- **Depends on**: —
- **Category**: correctness / replay fidelity
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §2.1

## Why this matters

Imported agit evals usually cannot be replayed. A recorded prompt like
"clean up `internal/auth/session.go`" references files from the *original
repository*, but:

1. `cmd_import_agit.go` never populates `Eval.Files` — the field exists in
   the schema (`eval.go:16`) and `buildPrompt` mentions it
   (`runner.go:247-249`), but imported evals always have it empty.
2. `runEval` executes the agent with `cmd.Dir = skillDir` (`runner.go:44`),
   so even hand-authored `Files` entries are only *named* in the prompt and
   resolved against the skill directory — never copied anywhere.

The with-skill and baseline runs then both fail for the same missing-context
reason, their delta collapses toward zero, and the eval measures nothing.
agengit stores content-addressed BLAKE3 blobs for every file it captures
(`steps-v1` diffs carry blob hashes and sizes; `agit restore` materializes
recorded state), so the pre-step file contents are recoverable. This plan
(a) makes the importer write those files as fixtures and (b) makes the
runner copy `Eval.Files` into a per-run scratch directory so runs are
hermetic — agents cannot dirty the skill directory or leak state between
the with-skill and baseline runs.

## Current state

- `eval.go:12-19` — `Eval.Files []string` exists but is write-only today.
- `runner.go:44` — `cmd.Dir = skillDir` for every run.
- `runner.go:233-253` — `buildPrompt` only *lists* files:

```go
	if len(eval.Files) > 0 {
		fmt.Fprintf(&b, "- Input files: %s\n", strings.Join(eval.Files, ", "))
	}
	fmt.Fprintf(&b, "- Save outputs to: %s\n", outDir)
```

- `cmd_import_agit.go:20-36` — `evalFromConverted` maps `agit.ConvertedEval`
  onto `Eval` and never sets `Files`.
- `internal/agit/types.go:50-53` — `Change` currently decodes only
  `kind` and `path`; the `steps-v1` wire format also carries blob hashes
  and sizes that we drop.
- `internal/agit/client.go:30-37` — `runAgit` is the single shell-out
  point; adding a new agit subcommand means adding a `Fetch*` wrapper here.
- `internal/agit/sanitize.go:10-25` — `sanitizeAssertionPath` already
  rejects absolute/traversal paths from agit output; reuse it for fixture
  paths.
- `workspace.go:56-62` — `snapshotSkill` shows the existing pattern for
  copying trees (`os.CopyFS`).
- Docs convention (`eval-workflow.md:22`): input fixtures live under
  `evals/files/…` relative to the skill directory.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Probe agit restore | `agit restore --help` | usage text (see Step 1) |
| Probe agit steps blobs | `agit steps --json \| head -c 2000` | blob hashes visible in `changes` |
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `internal/agit/types.go` (extend `Change` with blob metadata)
- `internal/agit/client.go` (new fetch wrapper for file content at a step)
- `internal/agit/convert.go` (surface restorable-file candidates per step)
- `cmd_import_agit.go` (write fixtures, populate `Files`, new `--fixtures` flag)
- `runner.go` (hermetic workdir: copy `Files`, set `cmd.Dir`)
- `eval.go` (doc comment on `Files` semantics only; no schema change)
- Tests in `internal/agit/convert_test.go`, `internal/agit/client_test.go`, `runner_test.go`
- `docs/guides/importing-agit-sessions.md`, `eval-workflow.md` (document fixtures + workdir)

**Out of scope**:
- Do NOT change the `evals.json` JSON schema (`Files` stays `[]string`; the
  directory-entry convention below is interpretation, not schema).
- Do NOT restore *added* files' post-step content as inputs (they didn't
  exist before the step; they are outcomes, not inputs).
- Do NOT implement fixture download from agit remotes (S3); local store only.
- Do NOT touch grading (`grader.go`) — outputs still go to `outputs/`.

## Git workflow

- Branch: `advisor/007-agit-fixture-import-hermetic-runs`
- Commit message style: `feat: import agit file fixtures and run evals in hermetic workdirs`
- Do NOT push unless instructed.

## Design decisions (read before coding)

1. **Which files become fixtures**: for each converted step, every diff
   change with `kind == "modified"` or `kind == "deleted"` existed *before*
   the step — restore its **pre-step** content. Cap at 20 files / 1 MB
   total per eval (constants `maxFixtureFiles`, `maxFixtureBytes`).
2. **Fixture layout**: `<skill>/evals/files/eval-<id>/<original-relative-path>`.
3. **`Files` directory convention**: an entry ending in `/` is a *directory
   of fixtures*; the runner copies its **contents** into the workdir root,
   preserving relative paths. This lets the workdir reproduce the original
   repo-relative paths the prompt mentions (`internal/auth/session.go`)
   without inventing a new schema. A plain file entry is copied preserving
   its own relative path (so the documented `evals/files/sales_2025.csv`
   example keeps working: prompt says that path, cwd is the workdir, the
   file is at that path inside the workdir).
4. **Workdir activation**: hermetic mode triggers **only when
   `len(eval.Files) > 0`**. Evals without files keep today's
   `cmd.Dir = skillDir` behavior — zero regression risk for existing users.
5. **Pre-step content source**: `agit steps --json` gives *post-step* blob
   hashes. The pre-step content of a file modified in step N is its blob in
   the **latest earlier step that touched it**, or the workspace snapshot
   agit captured. Step 1 probes which retrieval command exposes this
   (`agit restore`, `agit show <blob>`, or `agit between`); the plan's code
   isolates retrieval behind one function so only Step 2 depends on the
   probe result.

## Steps

### Step 1: Probe the agit retrieval interface (no code yet)

Run and record the output of:

```bash
agit restore --help
agit show --help
agit steps --json | head -c 4000
```

Answer in a scratch note:
- Does `agit restore` accept a step hash + path and write to a target
  location (e.g. `agit restore <step-hash> --path <file> --out <dir>` or
  similar)? What are the actual flags?
- Do `steps-v1` diff changes include pre-image blob hashes (e.g.
  `old_blob`/`blob_before`) or only post-image?
- Can `agit show <blob-hash>` (or an equivalent like `agit cat`) print raw
  blob content?

If **no** command can produce pre-step file content by hash or by
step+path, this is a STOP condition — report the actual interface so the
plan can be revised (a fallback is restoring the *post*-step content of the
*previous* step's snapshot).

**Verify**: you can manually reconstruct one file's pre-step content from a
real recorded session before writing any Go.

### Step 2: Extend agit types and client

In `internal/agit/types.go`, extend `Change` with the blob fields the probe
confirmed (names must match the actual wire format — do not guess):

```go
type Change struct {
	Kind string `json:"kind"` // "added" | "modified" | "deleted"
	Path string `json:"path"`
	// Blob fields per the agengit repo's docs/format/steps-v1.md — confirm exact JSON keys in Step 1.
	Blob     string `json:"blob,omitempty"`
	BlobSize int64  `json:"blob_size,omitempty"`
}
```

In `internal/agit/client.go`, add one retrieval wrapper following the
existing `Fetch*` pattern (exact args from Step 1), e.g.:

```go
// FetchBlob returns the raw content of a recorded blob.
func FetchBlob(hash string) ([]byte, error) {
	out, err := runAgit("show", "--raw", hash) // adjust to probed interface
	if err != nil {
		return nil, fmt.Errorf("agit blob %s: %w", hash, err)
	}
	return out, nil
}
```

Note: blob content is **raw bytes, not a cli-json-v1 envelope** — do not
route it through `decodeEnvelope`. Enforce a size cap (reuse the 50 MB
limit pattern or tighter: reject blobs > 1 MB for fixtures).

**Verify**: `go build ./...`; add a `client_test.go` case that swaps
`runAgit` (it is already a swappable var, `client.go:30`) and checks the
args passed and the size cap.

### Step 3: Surface fixture candidates from conversion

In `internal/agit/convert.go`, add to `ConvertedEval`:

```go
type FixtureFile struct {
	Path string // original repo-relative path (sanitized)
	Blob string // content hash of the pre-step version
}

type ConvertedEval struct {
	// ... existing fields ...
	Fixtures []FixtureFile
}
```

In `ConvertSteps`, for each emitted eval, walk the step's diff changes:
- `kind == "modified"` or `"deleted"` → candidate.
- Sanitize with `sanitizeAssertionPath` (reject unsafe, dot-prefixed, and
  lockfile paths — same skip rules as `buildAssertions`, `convert.go:366-370`).
- Resolve the pre-step blob (per Step 1's answer: either the change's
  pre-image blob field, or look back through earlier `StepRow`s for the
  most recent post-image blob of the same path).
- Enforce `maxFixtureFiles = 20`; skip candidates whose `BlobSize` exceeds
  `maxFixtureBytes = 1 << 20` cumulative.

Keep this **pure** (no I/O): conversion returns hashes; the importer
fetches content. That preserves the existing fixture-based unit testing.

**Verify**: `go test ./internal/agit -run TestConvertStepsFixtures` — a
fixture session where step 2 modifies a file created in step 1 yields the
step-1 blob as step-2's fixture.

### Step 4: Importer writes fixtures and populates Files

In `cmd_import_agit.go`:
- Add `fixtures := fs.Bool("fixtures", true, "Restore pre-step input files as eval fixtures")`.
- After conversion, for each eval with fixtures (when `*fixtures`):
  1. `dir := filepath.Join(skillDir, "evals", "files", fmt.Sprintf("eval-%d", eval.ID))`
  2. For each fixture: `content, err := agit.FetchBlob(f.Blob)`; on error,
     log a warn and continue (fixtures are best-effort).
  3. Validate the join with the same containment logic as
     `grader.go:isPathWithin` before writing; write with `0o600`, dirs `0o700`.
  4. Set `eval.Files = []string{fmt.Sprintf("evals/files/eval-%d/", eval.ID)}`
     (trailing slash = directory-contents convention).
- Fixture IDs must be assigned **after** merge renumbering
  (`cmd_import_agit.go:202-217`) or the directory names desync from eval
  IDs. Restructure: convert → merge/renumber → write fixtures → write JSON.

**Verify**: `go build ./...`; with a recorded session available, run
`go run . import-agit --skill <tmp-skill> --force` and confirm
`evals/files/eval-1/...` exists and `evals.json` has the `files` entry.
If no live agit store is available, cover via the unit tests in Step 6.

### Step 5: Hermetic workdir in the runner

In `runner.go`:

1. Add:

```go
// prepareWorkdir copies eval input files into a per-run scratch directory.
// Entries ending in "/" are fixture directories whose contents are copied
// to the workdir root (reproducing original repo-relative paths); plain
// entries are copied preserving their own relative path.
func prepareWorkdir(skillDir string, eval Eval, evalConfigDir string) (string, error)
```

   - Workdir path: `filepath.Join(evalConfigDir, "workdir")` (sibling of
     `outputs/`, i.e. `iteration-N/eval-M/<model>/<config>/workdir/`).
   - Reject `Files` entries that are absolute or escape `skillDir` after
     cleaning (reuse `isSafeAssertionPath` + `isPathWithin` semantics).
   - Use `os.CopyFS` for directory entries (pattern at `workspace.go:58`).
   - Remove any pre-existing workdir first (resume/fix reruns must start clean).

2. In `runEval`, replace the fixed dir:

```go
	cmd := cmdFn(ctx, agent, model, task, skillPath)
	cmd.Dir = skillDir
```

   with:

```go
	runDir := skillDir
	if len(eval.Files) > 0 {
		wd, err := prepareWorkdir(skillDir, eval, filepath.Join(evalDir, configLabel))
		if err != nil {
			return nil, fmt.Errorf("preparing workdir for eval %d: %w", eval.ID, err)
		}
		runDir = wd
	}
	cmd := cmdFn(ctx, agent, model, task, skillPath)
	cmd.Dir = runDir
```

3. Apply the same change to `fixEval` (`runner.go:309`, `cmd.Dir = skillDir`).
4. In `buildPrompt`, when files are present, state that they are in the
   current working directory: change the line to
   `- Input files (already present in your working directory): %s`.
   For directory entries, list the copied contents (walk the workdir) so
   the agent sees concrete paths, capped at 20 entries.

**Verify**: `go test ./... -run TestPrepareWorkdir` and
`go test ./... -run TestRunEvalHermetic` (see test plan).

### Step 6: Tests

Add table-driven tests:

- `internal/agit/convert_test.go` — `TestConvertStepsFixtures`: modified
  file → fixture with earlier blob; added file → no fixture; unsafe path
  (`../x`) → skipped; cap enforcement (21 files → 20 fixtures).
- `internal/agit/client_test.go` — `TestFetchBlob`: args shape, size cap,
  error propagation (swap `runAgit`).
- `runner_test.go` — `TestPrepareWorkdir`: plain-file entry preserves
  relative path; `dir/` entry copies contents to root; traversal entry
  errors; rerun cleans stale workdir. `TestRunEvalHermetic`: with a stub
  `CmdBuilder` that records `cmd.Dir`, an eval with `Files` runs in
  `.../workdir` and one without runs in `skillDir`.
- `main_test.go` or `cmd_import_agit` test — fixture write path stays
  within the skill dir; IDs match post-merge numbering.

### Step 7: Documentation

- `docs/guides/importing-agit-sessions.md`: new "Input file fixtures"
  section — what gets restored, the `evals/files/eval-N/` layout,
  `--fixtures=false` opt-out.
- `eval-workflow.md`: document the workdir behavior ("when an eval lists
  `files`, the run executes in a scratch working directory seeded with
  those files") and the trailing-slash directory convention.

### Step 8: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

All pass.

## Test plan

Covered in Step 6; the two highest-value cases are
`TestRunEvalHermetic` (behavioral contract of `cmd.Dir`) and
`TestConvertStepsFixtures` (pre-step blob resolution), because both guard
against silent replay-fidelity regressions.

## Done criteria

- [ ] `import-agit` writes pre-step file fixtures under `evals/files/eval-<id>/` and sets `files` in `evals.json`.
- [ ] Runs for evals with `files` execute in a per-run `workdir/` seeded with those files; evals without `files` behave exactly as before.
- [ ] `fixEval` uses the same workdir logic.
- [ ] All fixture/workdir paths are containment-checked; traversal entries fail loudly.
- [ ] `--fixtures=false` skips fixture restoration.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Guides updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Step 1 finds no agit command that yields pre-step file content (report
  the actual `agit restore`/`show` interface verbatim).
- `steps-v1` blob field names differ from the probe in a way that requires
  a second agit call per file *per step* (N+1 regression) — propose a batch
  alternative instead of coding it.
- Changing `cmd.Dir` breaks any agent runtime that resolves the skill path
  relative to cwd (check `newAgentRunner` implementations before merging).
- Fixture restoration would require >1 MB per eval routinely in real
  sessions — revisit the caps rather than raising them silently.

## Maintenance notes

- The retrieval wrapper (`FetchBlob` or equivalent) is the only place that
  knows the agit restore interface; when agit changes, only
  `internal/agit/client.go` should need edits.
- Reviewers: reject any change that copies fixtures *into the skill
  directory itself* at run time — the workdir exists precisely to keep the
  skill dir pristine.
- Follow-up (see Plan 014): fixtures inside `agit export` bundles make eval
  corpora fully portable.
