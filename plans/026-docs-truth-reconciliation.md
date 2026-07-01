# Plan 026: Docs truth reconciliation — make every page match the shipped binary

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- docs/guides/ *.md docs/site/src/utils/routing.ts main.go cmd_run.go cmd_grade.go cmd_loop.go cmd_import_agit.go report.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1 (the docs currently describe features that do not exist)
- **Effort**: M
- **Risk**: LOW-MED (mass edits across pages; the drift-guard test is new CI surface)
- **Depends on**: — (coordinates with Plan 027: the feedback.json fiction is *resolved* there; this plan quarantines it — see Step 2)
- **Category**: documentation
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Educational documentation is worthless if readers can't trust it, and
right now several pages are confidently wrong:

1. **An entire guide documents a feature that does not exist.**
   `docs/guides/giving-feedback.md` (116 lines, in the site nav, with a
   produced video) describes `feedback.json`: auto-created after every
   grade, pre-populated with eval keys, "passed directly to the judge
   agent" on the next run, carried across iterations. `grep -rn feedback
   --include='*.go' .` returns **nothing**. `eval-workflow.md` §4 repeats
   the fiction ("Don't forget to add your feedback! You can leave notes in
   feedback.json"). A user who follows the guide writes notes into a file
   nothing will ever read.
2. **Wrong schemas taught as real.** `giving-feedback.md` tells readers to
   look for `"result": "FAIL"` and a `reasoning` field in `grading.json`;
   the actual schema is `assertion_results[].passed` (bool) and
   `evidence` (`eval.go:62-66`). Its workspace diagram puts `grading.json`,
   `feedback.json`, and `outputs/` directly under `iteration-1/` — the
   real layout nests them per eval/config (`workspace.md`, CLAUDE.md).
3. **The flag reference is incomplete.** `commands.md` omits `--timeout`,
   `--parallel`, `--resume`, `--force-unlock`, `--yes/-y`, `--verbose/-v`,
   `--baseline-only` appears but `report --iteration` doesn't, and
   `import-agit`'s six flags are documented only in the guide. Users
   discover capabilities by reading `--help` instead of the site.
4. **Input-file semantics are misdocumented.** `eval-workflow.md`'s
   example prompt says `data/sales_2025.csv` while its `files` entry says
   `evals/files/sales_2025.csv`, and nothing explains what `files`
   actually does today: it is *listed in the prompt* and resolved against
   the skill dir as cwd (`runner.go:44,247-249`) — no copying, no
   isolation. Until Plan 007 lands, docs must teach the real contract
   (fixtures live under the skill dir; prompts must reference the same
   path as the `files` entry).
5. **Nav gaps.** `docs/guides/importing-agit-sessions.md` renders but is
   missing from `ORDERED_PAGES` (`routing.ts:10-23`), so its position is
   accidental.

And structurally: nothing prevents drift from recurring — this plan adds a
cheap self-updating guard.

## Current state

Key excerpts already quoted above; additionally:

- `routing.ts:81` — `EXCLUDED_DOCS = ["agents.md", "claude.md", "context.md", "readme.md"]`
  (so root globbing is intentional and CLAUDE/AGENTS are correctly
  excluded; CHANGELOG is intentionally included — no action).
- `main.go:60-91` — `printUsage` is the closest thing to a flag inventory;
  it too omits `--resume`/`--force-unlock` (fix alongside).
- `workspace.md` — accurate but incomplete: missing `fix-<n>/` attempt
  dirs (`runner.go:299-300`) and `fix-results.json` (`runner.go:363`);
  exit-code table says only 0/1 (fine today; Plan 022 adds 2 — its
  problem, not this plan's).
- `configuration.md` — omits `judge.agent` (supported: `config.go:33-36`)
  and the `models:` list (schema-supported, documented only in
  `commands.md`).
- `docs/guides/reading-results.md` — single-model example shows
  `run_summary` populated: acceptable (current writes populate both
  aggregate and `models`), but it never mentions the `models` map or
  `best_model`/`worst_model` for the single-model case reader — verify
  wording against `benchmark.go:52-57` and adjust only if wrong.
- `docs/guides/auto-fixing.md` — verify its critique-flow description
  against `fixEval` (`runner.go:258-372`): attempts start at 2, best
  attempt overwrites `grading.json`, `fix-results.json` records the
  trajectory, convergence = identical failure text. Fix any mismatch found.

## Design decisions (read before coding)

1. **Quarantine, don't delete, the feedback guide.** Plan 027 implements
   `feedback.json` for real. Until it lands, `giving-feedback.md` gets a
   prominent, honest, on-tone banner at the top:
   > 🚧 **Heads up!** `feedback.json` is a documented-ahead feature — the
   > CLI doesn't create or read it yet (it's coming; the workflow below
   > still works great as a manual note-taking convention). Track progress
   > in the repo.
   plus the schema/layout corrections so the rest of the page is true.
   `eval-workflow.md` §4's feedback paragraph gets one clause added:
   "(a manual convention for now — skill-eval doesn't read this file yet)".
   If Plan 027 has ALREADY landed at execution time, skip the banner and
   instead verify the guide against 027's shipped behavior.
2. **`commands.md` becomes the canonical flag reference**, one table per
   subcommand (not one mega-table), each row: flag, default, what it does,
   one-line example. Global flags (`--yes`, `--verbose`) get their own
   small table. Content generated by reading every `fs.*(` declaration in
   `cmd_*.go` + `parseGlobalArgs` (`main.go:120-137`) — not from memory.
3. **A drift-guard test in Go.** New `docs_test.go` (package main) that:
   - regex-scans `cmd_*.go` sources for `fs.(String|Bool|Int|Duration)\("([a-z-]+)"`
     (read the files with `os.ReadFile`, walk via `filepath.Glob("cmd_*.go")`),
   - asserts every captured flag name appears as `` `--<name>` `` somewhere
     in `commands.md`,
   - with a small allowlist var for intentionally undocumented flags
     (starts empty; additions require a comment justifying).
   Self-updating: a new flag without docs fails `go test ./...` with a
   message naming the flag and the file. Cheap, no build-graph tricks.
4. **Real-output policy**: every JSON/layout example corrected in this
   plan must be verifiable against structs or a real run; add an HTML
   comment above hand-authored examples:
   `<!-- verified against eval.go structs @ <commit> -->` so future
   editors know the provenance convention.
5. **Tone preserved** — corrections keep the voice; a fixed page should
   read like it was always right, except the quarantine banner which is
   deliberately candid.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Go tests | `go test ./...` | exit 0 (incl. new docs_test.go) |
| Flag inventory | `grep -n 'fs\.\(String\|Bool\|Int\|Duration\)(' cmd_*.go` | the list Step 3 documents |
| Site build | `cd docs/site && pnpm build && pnpm test` | exit 0 |
| Fiction check | `grep -rn "feedback" --include="*.go" .` | still empty (or 027 landed) |

## Scope

**In scope**:
- `docs/guides/giving-feedback.md` (banner + schema/layout corrections)
- `eval-workflow.md` (feedback clause; files-path consistency + real `files` semantics note)
- `commands.md` (full per-subcommand flag reference), `main.go` (printUsage gaps only)
- `configuration.md` (`judge.agent`, `models:`), `workspace.md` (fix dirs)
- `docs/guides/auto-fixing.md`, `docs/guides/reading-results.md`,
  `docs/guides/first-eval.md`, `docs/guides/importing-agit-sessions.md`
  (verify-and-correct pass against code; first-eval's evals.json example
  must satisfy Plan 020's lint rules if that landed)
- `docs/site/src/utils/routing.ts` (+ its test) — register
  `guides/importing-agit-sessions` in `ORDERED_PAGES`
- New `docs_test.go`

**Out of scope**:
- Do NOT implement feedback.json (Plan 027).
- Do NOT restructure pages or add new ones (Plans 025/028/029/030).
- Do NOT fix `writing-evals.md` (Plan 025 owns it).
- Do NOT touch the videos even where they show the fictional flow —
  re-recording is Plan 030's scope; the banner covers the gap.

## Git workflow

- Branch: `advisor/026-docs-truth-reconciliation`
- Commit message style: `docs: correct feature fiction, schemas, and flag reference to match the binary`
- Do NOT push unless instructed.

## Steps

### Step 1: Build the truth inventory

Mechanically extract, into a scratch note: every subcommand's flags (grep
above), the real `grading.json`/`benchmark.json` field names (`eval.go`),
the real workspace tree (walk a real or test-generated workspace), and the
real `files` behavior (`runner.go:243-252`). This note is the source for
every edit below — no edit gets made from memory.

### Step 2: Feedback quarantine

Apply Design decision 1 to `giving-feedback.md` (banner; fix the
`grading.json` read instructions to `assertion_results[].passed` /
`evidence` with a real excerpt; fix the workspace diagram) and the
one-clause amendment in `eval-workflow.md`. Check `plans/README.md` for
027's status first.

**Verify**: page renders; no remaining `"result"`/`"reasoning"` references
(`grep -n 'reasoning\|"result"' docs/guides/giving-feedback.md` → empty).

### Step 3: commands.md rewrite + usage gaps

Per Design decision 2. Also add the missing `--resume`/`--force-unlock`/
`--runs`-if-landed lines to `printUsage` (`main.go:74-86`) — keep it terse;
the site page is the full reference.

**Verify**: every flag from the Step 1 inventory appears; examples copy-
paste clean.

### Step 4: The drift-guard test

Implement `docs_test.go` per Design decision 3.

**Verify**: `go test ./... -run TestCommandsDocCoversAllFlags` passes;
then temporarily add a fake flag to a `cmd_*.go` in your working tree and
confirm the test fails naming it; revert.

### Step 5: files semantics + remaining page corrections

- `eval-workflow.md`: make the example prompt and `files` entry use the
  same path, and add a short "How input files work today" callout: listed
  in the prompt, resolved from the skill directory, not copied — keep
  fixtures small and under `evals/files/`. (If Plan 007 landed, document
  the workdir behavior instead — its plan carries the wording.)
- `configuration.md`: add `judge.agent` and the `models:` block (moving
  the canonical YAML example here; `commands.md` keeps a pointer).
- `workspace.md`: add `fix-2/`, `fix-results.json`, and (if landed)
  `skill-snapshot/` to the tree.
- `auto-fixing.md` / `reading-results.md` / `first-eval.md` /
  `importing-agit-sessions.md`: line-by-line verify against the Step 1
  inventory; correct in place.

**Verify**: site build; targeted greps for each corrected claim.

### Step 6: Nav registration + final checks

Add `{ path: "guides/importing-agit-sessions" }` to `ORDERED_PAGES`
(position: after `guides/cross-model`); update `routing.test.ts` if it
snapshots the order.

```bash
go test ./...
cd docs/site && pnpm build && pnpm test
```

## Test plan

- New `TestCommandsDocCoversAllFlags` (+ the deliberate-failure spot check).
- Site vitest suite + build.
- Grep assertions per step (no fictional fields, no path mismatch).
- Manual: click through every corrected page in `pnpm dev` once.

## Done criteria

- [ ] No page describes unimplemented behavior without the quarantine banner; all `grading.json` references use real field names.
- [ ] `commands.md` documents every flag of every subcommand + globals; the drift-guard test enforces it forever.
- [ ] `files` semantics documented as they actually are.
- [ ] `configuration.md`/`workspace.md` complete; importing-agit guide in the nav order.
- [ ] `go test ./...` and site build/tests pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- You find MORE fictional features during Step 1 (beyond feedback.json) —
  inventory them all and report before editing; the fix strategy may need
  its own decision (implement vs quarantine) per feature.
- The drift-guard regex approach can't see flags defined through a helper
  (grep for `fs.Var` or wrapper funcs first) — extend the regex, don't
  allowlist silently.
- Plan 027 landed with behavior that differs from the guide in ways a
  banner can't bridge — hand the guide rewrite to 027's docs step instead
  of patching twice.

## Maintenance notes

- The provenance comment convention (`<!-- verified against ... -->`) plus
  the drift-guard test are the recurrence defenses; reviewers should
  reject doc PRs that state schemas or flags without either.
- When Plans 007/011/017/022 land, each carries its own doc edits — this
  plan establishes the baseline of truth they amend.
