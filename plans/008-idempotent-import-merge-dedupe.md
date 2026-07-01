# Plan 008: Make `import-agit --merge` idempotent by deduplicating on step hash

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- cmd_import_agit.go eval.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: —
- **Category**: correctness
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §2.6

## Why this matters

Running `skill-eval import-agit --merge` twice duplicates every imported
eval: the merge path appends unconditionally and renumbers. Users who treat
`import-agit --all-sessions --merge` as "sync my corpus" (the natural
mental model) silently double their eval count, inflate benchmark run
costs, and skew pass-rate aggregation toward duplicated prompts.

Every imported eval already carries a stable identity:
`EvalSource.AgitSessionID` + `EvalSource.AgitStepHash` (BLAKE3,
content-addressed, deterministic per the agit `steps-v1` format). Dedup is
a set lookup — no schema change needed.

## Current state

- `cmd_import_agit.go:202-217` — the merge path appends everything:

```go
	if *merge {
		existing, err := readEvalsFile(*outPath)
		if err == nil {
			// Append new evals after existing ones, renumbering.
			nextID := 1
			for _, e := range existing.Evals {
				if e.ID >= nextID {
					nextID = e.ID + 1
				}
			}
			for i := range evalFile.Evals {
				evalFile.Evals[i].ID = nextID + i
			}
			evalFile.Evals = append(existing.Evals, evalFile.Evals...)
			fmt.Printf("Merging with %d existing evals (next ID: %d)\n", len(existing.Evals), nextID)
		}
	}
```

- `eval.go:23-31` — `EvalSource` already stores `AgitSessionID` and
  `AgitStepHash` (with `omitempty`, so hand-written evals have a nil
  `Source` and must never be dropped by dedupe).
- Note the silent `err == nil` swallow above: if the existing file is
  unreadable, merge currently degrades to overwrite semantics. Fix that
  while here (see Step 2).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `cmd_import_agit.go` (merge path only)
- Tests (new `cmd_import_agit_test.go` or extend `main_test.go`, matching
  where existing import tests live — check first)

**Out of scope**:
- Do NOT dedupe on prompt text or any fuzzy similarity — identity is
  exactly `(AgitSessionID, AgitStepHash)`.
- Do NOT dedupe hand-written evals (nil/empty `Source`).
- Do NOT change non-merge (`--force`/fresh) behavior.
- Do NOT renumber *existing* evals — their IDs are referenced by workspace
  directories from prior iterations.

## Git workflow

- Branch: `advisor/008-idempotent-import-merge-dedupe`
- Commit message style: `fix: dedupe merged agit imports by session and step hash`
- Do NOT push unless instructed.

## Steps

### Step 1: Extract an identity helper

In `cmd_import_agit.go` add:

```go
// evalIdentity returns a stable identity for an imported eval, or ("", false)
// for hand-written evals (no agit source), which are never deduplicated.
func evalIdentity(e Eval) (string, bool) {
	if e.Source == nil || e.Source.AgitStepHash == "" {
		return "", false
	}
	return e.Source.AgitSessionID + "\x00" + e.Source.AgitStepHash, true
}
```

Use `\x00` as separator so a crafted session ID cannot collide with a hash
boundary.

**Verify**: `go build ./...`.

### Step 2: Filter incoming evals in the merge path

Rewrite the merge block:

1. If `readEvalsFile` fails for any reason other than `os.IsNotExist`,
   **return the error** instead of silently appending to nothing (this
   fixes the current unreadable-file → overwrite hazard).
2. Build `seen := map[string]bool{}` from `existing.Evals` via `evalIdentity`.
3. Partition `evalFile.Evals` into `fresh` (identity absent or
   hand-written-source-free — imported evals always have a source, so in
   practice: identity absent) and `skipped`.
4. Renumber only `fresh` starting at `nextID`, append, and print both
   counts:

```go
fmt.Printf("Merging: %d new, %d duplicate(s) skipped (already imported), %d existing\n",
	len(fresh), skipped, len(existing.Evals))
```

5. If `len(fresh) == 0`, still write the file unchanged is wasteful —
   print "No new evals to merge" and return nil **without** rewriting the
   file (preserves mtime; makes repeated syncs cheap and diff-free).

**Verify**: `go test ./... -run TestMergeDedupe` (Step 3).

### Step 3: Tests

Table-driven, in the same file that tests `cmdImportAgit` today (check for
existing coverage; if none, create `cmd_import_agit_test.go` writing temp
`evals.json` files):

- `TestMergeDedupeSkipsExistingStepHash`: existing file has eval with
  source `(sess-a, hash-1)`; incoming batch has `(sess-a, hash-1)` and
  `(sess-a, hash-2)` → result has 2 evals total, new one gets `ID 2`.
- `TestMergeDedupeDifferentSessionsSameHashKept`: `(sess-a, hash-1)` vs
  `(sess-b, hash-1)` → both kept (hashes are per-step but belt-and-braces).
- `TestMergeKeepsHandWrittenEvals`: existing eval with `Source: nil` is
  untouched and never treated as a duplicate of anything.
- `TestMergeAllDuplicatesLeavesFileUntouched`: incoming all-duplicates →
  file bytes unchanged (compare content or mtime), exit nil.
- `TestMergeUnreadableExistingFileErrors`: existing file is invalid JSON →
  command returns an error mentioning the path (no overwrite).
- `TestMergeIdempotent`: run the merge logic twice with the same incoming
  batch → second run adds 0.

### Step 4: Documentation

Update `docs/guides/importing-agit-sessions.md` (merge section): state
that merge is idempotent, keyed on the recorded step hash, and that
re-running `import-agit --all-sessions --merge` is the supported way to
keep a corpus in sync.

### Step 5: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

All pass.

## Test plan

See Step 3. The idempotency test (`TestMergeIdempotent`) is the contract;
the unreadable-file test locks in the error-handling fix.

## Done criteria

- [ ] Re-running `import-agit --merge` with the same sessions adds zero evals.
- [ ] Duplicate skip count is printed; all-duplicate merges do not rewrite the file.
- [ ] Hand-written evals (nil `Source`) are never deduplicated or renumbered.
- [ ] Unreadable existing `evals.json` is an error, not a silent overwrite.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Guide updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Existing evals in the wild are found to share `(session, step-hash)`
  pairs across *different* skill dirs in a way the identity misses — that
  would mean the identity needs the origin too (`AgitOrigin` is available;
  adding it is a one-line change but decide deliberately, not silently).
- The merge path turns out to be exercised by other commands (grep for
  `readEvalsFile` callers) — coordinate before changing its error behavior.

## Maintenance notes

- Any future import source (Plan 014's `--between` / `--bundle`) must set
  `AgitStepHash` so dedupe keeps working; reviewers should check this on
  every new source.
- If `EvalSource` ever gains a non-agit source type, generalize
  `evalIdentity` rather than adding a parallel dedupe path.
