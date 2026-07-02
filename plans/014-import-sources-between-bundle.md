# Plan 014: New import sources — `--between` git ranges and `--bundle` archives

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- cmd_import_agit.go internal/agit/client.go internal/agit/types.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S (each source; both in one plan because they share the flag/validation plumbing)
- **Risk**: MED (both depend on agit interfaces that must be probed first)
- **Depends on**: plans/008-idempotent-import-merge-dedupe.md (bundles re-import the same steps repeatedly; dedupe must exist first)
- **Category**: ergonomics / corpus workflows
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §2.4, §2.5

## Why this matters

Two workflows the current session-oriented import cannot express:

1. **"Turn this PR into evals."** agit records which git commit each step
   landed on; `agit between <rev1> <rev2>` returns the steps whose captured
   commit falls in a revision range. That maps a feature branch or PR —
   a human-reviewed, merged unit of work — directly onto eval candidates,
   which is a better curation boundary than "whatever happened in session X".
2. **Sharing corpora.** Sessions live in the local `.agit/` store of the
   machine where they were recorded. `agit export <PATH>` writes a portable
   bundle (`bundle-v1`: session refs + reachable objects); `agit import`
   ingests one. Supporting `import-agit --bundle <path>` lets a teammate's
   golden session, or a CI artifact, become evals anywhere — and (with
   Plan 007) carries the blob objects that fixtures need.

## Current state

- `cmd_import_agit.go:38-49` — flag set: `--session`, `--skill`, `--out`,
  `--force`, `--merge`, `--all-sessions`, `--eval-filter`. Target
  collection at `:68-92` builds `[]sessionTarget` from either
  `FetchSessions()` or the single `--session` value.
- `internal/agit/client.go` — wrappers exist for `log`, `show`, `diff`,
  `steps`, `sessions`, `eval`; nothing for `between`, `export`, `import`.
- `internal/agit/convert.go:87-154` — `ConvertSteps` consumes a `*Steps`
  (origin + session_id + step rows); a `between` result set spans sessions,
  so conversion needs the per-step origin/session (probe the wire shape).
- Plan 008 gives merge dedupe on `(session, step-hash)` — the property that
  makes bundle re-imports safe.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Probe between | `agit between --help` && `agit between <rev1> <rev2> --json \| head -c 4000` | JSON step rows with session attribution |
| Probe import | `agit import --help` | flags for ingesting a bundle |
| Probe export | `agit export --help` | flags for producing one (needed for tests/docs) |
| Build | `go build ./...` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `internal/agit/client.go` (+`types.go` if `between` rows differ from `StepRow`)
- `cmd_import_agit.go` (two new flags, mutual-exclusion validation, target routing)
- Tests: `internal/agit/client_test.go`, import-level tests
- `docs/guides/importing-agit-sessions.md`

**Out of scope**:
- Do NOT implement bundle *creation* (`agit export` stays a user-run step;
  document it).
- Do NOT import bundles into a temporary isolated store unless `agit
  import` forces it — prefer whatever agit's native ingest does, and note
  the consequence (bundle sessions join the local store).
- Do NOT resolve git revisions ourselves — pass them through to agit
  verbatim; agit owns rev-parse semantics.
- Do NOT add fixture handling here (Plan 007 owns fixtures; if both land,
  bundles feed it for free through the shared conversion path).

## Git workflow

- Branch: `advisor/014-import-sources-between-bundle`
- Commit message style: `feat: import evals from git revision ranges and agit bundles`
- Do NOT push unless instructed.

## Steps

### Step 1: Probe both agit interfaces

Record actual output of the three probe commands above. Answer:
- `between`: arg order, `--json` support, and whether rows carry
  `origin`/`session_id` per step (they must, since ranges span sessions).
  Does it return full step objects + diffs, or hashes needing `steps`/
  `show` follow-ups?
- `import`: does it take a path positionally? Does it print the ingested
  session refs (needed to scope the subsequent conversion to *just the
  bundle's* sessions rather than `--all-sessions`)? Is it idempotent when
  re-importing the same bundle (content-addressed stores usually are)?

If `between` has no `--json`, STOP (report the interface). If `import`
doesn't report which sessions it ingested, fall back to: list sessions
before, list after, diff the sets — note this in the code.

### Step 2: Client wrappers

In `internal/agit/client.go` (shapes adjusted per Step 1):

```go
// FetchBetween returns recorded steps whose captured git commit falls
// between two revisions, across sessions.
func FetchBetween(rev1, rev2 string) (*BetweenResult, error) {
	out, err := runAgit("between", "--json", rev1, rev2)
	if err != nil {
		return nil, fmt.Errorf("agit between %s %s: %w", rev1, rev2, err)
	}
	return decodeEnvelope[BetweenResult](out)
}

// ImportBundle ingests an agit export bundle into the local store and
// returns the session refs it contained.
func ImportBundle(path string) ([]SessionRow, error)
```

Validate `path` exists and is a regular file before shelling out; cap
accepted bundle size (e.g. 500 MB) with a clear error. Add `BetweenResult`
(and a per-row type embedding origin/session/hash + optional `*Step`/`*Diff`)
to `types.go` per the probed shape.

**Verify**: `client_test.go` with swapped `runAgit` — arg shapes, size/
existence validation, envelope decoding from a canned fixture.

### Step 3: Flag routing in cmdImportAgit

- New flags:

```go
between := fs.String("between", "", "Import steps whose git commit falls in a range: <rev1>..<rev2>")
bundle := fs.String("bundle", "", "Import sessions from an agit export bundle file first")
```

- Mutual exclusion: `--between`, `--all-sessions`, and `--session` are
  exclusive of each other (`--bundle` combines with any, since it only
  *adds* sessions before selection). Return a clear error on conflicts.
- `--between` parsing: split on `".."` (require exactly one occurrence,
  both sides non-empty).
- Routing:
  - `--bundle` set → `agit.ImportBundle(path)` first; if `--session`/
    `--all-sessions`/`--between` are absent, default the target set to
    exactly the bundle's returned sessions.
  - `--between` set → `agit.FetchBetween(rev1, rev2)`; group returned steps
    by `(origin, session_id)` and feed each group through the existing
    conversion path (reusing `ConvertSteps` by synthesizing a `*Steps` per
    group, or a small adapter — pick whichever avoids duplicating filter
    logic). Fetch `agit eval` per distinct session for quality metadata,
    same as today (`cmd_import_agit.go:108-111`).
- `EvalSource` provenance must be populated identically (origin, session,
  step hash) so Plan 008's dedupe works across all sources.

**Verify**: `go build ./...`; conflict combinations return errors
(`TestImportSourceFlagConflicts`).

### Step 4: Tests

- `TestFetchBetween` / `TestImportBundle` (client level, canned envelopes).
- `TestBetweenRangeParsing`: `a..b` ok; `a..`, `..b`, `a...b`, `a` →
  errors with actionable messages.
- `TestImportSourceFlagConflicts` (Step 3).
- `TestBetweenGroupsBySession`: a canned between-result spanning two
  sessions produces evals with correct per-session `EvalSource` and
  session-scoped eval-report enrichment.
- `TestBundleThenMergeIsIdempotent`: bundle sessions imported twice with
  `--merge` add zero the second time (leans on Plan 008; skip with a clear
  t.Skip if 008 hasn't landed — but then this plan's Depends-on was
  violated, which is a STOP).

### Step 5: Documentation

`docs/guides/importing-agit-sessions.md`, two new sections:
- **From a git range**: `skill-eval import-agit --between main..feature-x
  --merge` — turn a reviewed PR into evals; note that agit must have been
  recording when that work happened.
- **From a bundle**: teammate runs `agit export corpus.agitbundle`
  (exact flags per Step 1 probe), you run
  `skill-eval import-agit --bundle corpus.agitbundle --merge`; note that
  the bundle's sessions join your local agit store, and re-imports are
  deduped.

### Step 6: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

See Step 4. The grouping test (`TestBetweenGroupsBySession`) is the one
that guards conversion correctness; the rest are plumbing.

## Done criteria

- [ ] `import-agit --between <rev1>..<rev2>` imports steps from a git range, grouped and enriched per session.
- [ ] `import-agit --bundle <path>` ingests a bundle and imports exactly its sessions by default.
- [ ] Conflicting source flags error out clearly.
- [ ] Provenance fields populate identically across all sources (dedupe-compatible).
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Guide updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- `agit between` lacks `--json` or omits per-step session attribution.
- `agit import` cannot be scoped/observed well enough to know what a bundle
  added (and the before/after session-set diff is racy on a busy store).
- Reusing `ConvertSteps` for between-groups requires changing its
  signature in ways that conflict with Plan 010's pending signature change
  — coordinate the two plans' order first.

## Maintenance notes

- Every import source must funnel through the same conversion + provenance
  path; reviewers should reject source-specific eval construction.
- When Plan 007 lands, verify bundles carry blob objects (bundle-v1 says
  "reachable objects") so fixtures restore from bundle-imported sessions
  too — add a test then.
