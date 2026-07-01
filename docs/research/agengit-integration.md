---
title: "Research: Deepening the agengit integration"
description: Where the agit import stands today, what agengit capabilities are still unused, and what else the codebase needs to produce top-quality skills per the Agent Skills spec.
---

# Research: Deepening the agengit integration

Date: 2026-07-01. Researched against skill-evaluator v1.4.0 and agengit v1.26.0.

## Goal

skill-eval's purpose is to help write top-quality skills (per the
[Agent Skills spec](https://agentskills.io/specification)) by making the
eval loop cheap and honest. agengit records real agent sessions; those
sessions are the best available source of *realistic* evals. This document
maps what the integration already does, what agengit offers that we do not
yet use, and what other gaps stand between the tool and its goal.

## 1. Where the integration stands today

`skill-eval import-agit` (`cmd_import_agit.go` + `internal/agit`) already:

- Shells out to the `agit` binary (consistent with ADR-0001) and parses
  `cli-json-v1` envelopes.
- Uses the fast path `agit steps --json --include-step-objects` (agit
  v1.26+) with a legacy `log` + `show` + `diff` N+1 fallback.
- Converts substantive turns (prompt length floor, acknowledgement filter,
  ≥1 file change) into evals: first user message → `prompt`, recorded
  outcome/diff/summary → `expected_output`, added/modified files →
  `file_exists:` / `contains_text:` assertions plus one LLM-judged
  assertion.
- Fetches `agit eval --json` per session (best-effort) and stores
  session-level `classification` and a derived 0–100 `quality_score` in
  `EvalSource`; `--eval-filter good,mixed` gates whole sessions.
- Supports `--merge`, `--force`, `--all-sessions`, `--session`, `--out`.

This is a solid v1. The rest of this document is about fidelity, signal
quality, and closing the loop.

## 2. Unused agengit capabilities, in priority order

### 2.1 Replay fidelity: seed eval input files from agit blobs (highest value)

The single biggest weakness of imported evals is that they usually cannot
actually be replayed. Recorded prompts reference files from the original
repo ("clean up `internal/auth/session.go`"), but `runEval` executes the
agent with `cmd.Dir = skillDir` and `Eval.Files` is never populated by the
importer — the referenced files simply don't exist at run time. The
with-skill and baseline runs both fail for the same reason, the delta
collapses toward zero, and the eval measures nothing.

agengit stores content-addressed blobs: `steps-v1` diffs carry blob hashes
and sizes per change, and `agit restore` can materialize recorded file
state. The importer should:

1. For each converted step, resolve the *pre-step* state of the files the
   prompt/diff touches (parent step's blobs, or `agit restore --at <hash>`).
2. Write them under `<skill>/evals/files/eval-<id>/…` (the existing
   fixture convention from `eval-workflow.md`).
3. Populate `Eval.Files` so `buildPrompt` references them.

Follow-on runner change: copy `Eval.Files` into a per-run scratch
directory instead of only naming them in the prompt, so runs are hermetic
and agents can't dirty the skill directory or share state between
with-skill and baseline runs. This is the one item where the importer and
the runner must move together.

### 2.2 Per-step quality: `agit eval --include-steps`

`internal/agit/client.go` calls `agit eval --json` without
`--include-steps`, so one session-level classification is stamped onto
every eval imported from that session. A "mixed" session usually contains
both clean turns and churny ones; today we either import all of them or
none.

`--include-steps` returns `step_assessments` keyed by step hash with the
10 signal counters per turn. Use it to:

- Filter at step granularity (e.g. skip steps with `repeated_failures > 0`
  or zero `verification_commands` when `--eval-filter good` is set).
- Store per-step signals in `EvalSource` instead of the session-wide
  score, so `report` can correlate eval failures with the quality of the
  turn they came from.

### 2.3 Tool calls: mine verification commands into assertions

`Step.ToolCalls` (tool_name, args, result) is parsed into `types.go` and
then ignored. `convert.go` even has a stub acknowledging this
(`buildAssertionsWithSignals`: "We can't reconstruct tool_calls here").
But we *do* have them on the steps fast path. The recorded session shows
how the work was verified — `go test ./...`, `npm run lint`, a curl against
an endpoint. Those are far better assertion candidates than
`keyTermFromSummary`'s verb-phrase heuristic, which is brittle (it grabs up
to 50 chars after "added "/"fixed " and hopes the text reappears).

Proposal: when a step's tool calls include verification commands (agit's
own `verification_commands` signal identifies them), emit an assertion the
judge can check against the run transcript/outputs, e.g.
`"The agent ran the project's tests (go test) and they passed."` — or, once
a `command_succeeds:` deterministic matcher exists (§4.3), emit that.

### 2.4 `agit between`: evals from a git revision range

`agit between <rev1> <rev2>` returns steps whose captured commit falls in
a range. This enables a natural workflow: "everything the agent did on
this PR/feature branch becomes eval candidates." Add
`skill-eval import-agit --between <rev1>..<rev2>` as a thin flag that
swaps the session listing for a `between` call. Cheap to build on the
existing conversion pipeline.

### 2.5 `agit export` bundles: portable, CI-friendly eval corpora

`agit export` writes a portable bundle of session refs + reachable
objects (`bundle-v1`). Supporting `import-agit --bundle <path>` decouples
eval generation from the machine where the session was recorded:
teammates can share "golden sessions", and CI can regenerate/verify an
eval corpus without an `.agit` store. This also gives fixtures (§2.1) a
transport format.

### 2.6 Idempotent import: dedupe by step hash

`--merge` appends and renumbers; running `import-agit --merge` twice
duplicates every eval. Since `EvalSource.AgitStepHash` is already stored
and agit hashes are content-addressed (BLAKE3, deterministic), merge
should skip evals whose `(session_id, step_hash)` already exists.
Similarly, `eval_hash` / `captured_evidence_hash` are deterministic per
scope — record them and make `--all-sessions --merge` a safe, re-runnable
"sync my corpus" operation.

### 2.7 Close the loop: run `agit eval` on the eval runs themselves

Today grading is output-based: the judge sees produced files. But if agit
hooks are installed for the agent runtime being benchmarked, every
`skill-eval run` is *itself* recorded as an agit session. After a run,
shell out to `agit eval` on that fresh session and attach its dimensions
(verification, failure_recovery, churn_risk…) to `RunResult`.

That gives the benchmark a second axis: not just "did the output pass the
assertions" but "did the skill make the agent's *process* better — fewer
repeated failures, more verification, less churn." A skill that produces
the same output with 40% less churn is a better skill, and today the tool
cannot see that. Concretely: correlate runs to sessions by start/end
timestamps (`agit sessions` exposes `updated_at`), best-effort, behind a
`--process-quality` flag. This is the most novel item here and directly
serves the "write better skills" goal.

## 3. Skills-spec alignment (independent of agengit)

The tool's only contact with the Agent Skills spec today is "a file named
SKILL.md exists" (`cmd_init.go`). The spec is small and checkable; the
tool that exists to improve skills should be able to tell you your skill
is malformed before spending judge tokens on it.

### 3.1 `skill-eval validate` (or checks inside `init`/`loop`)

Deterministic lint against the spec:

- Frontmatter parses; `name` and `description` present.
- `name`: ≤64 chars, lowercase letters/digits/hyphens, no leading/trailing
  hyphen, **matches the directory name** (skills silently fail to load
  otherwise — the most common real-world skill bug).
- `description`: ≤1024 chars; warn if it doesn't state both *what* the
  skill does and *when* to use it (heuristic: look for trigger phrasing
  like "use when").
- Optional fields (`license`, `compatibility` ≤500 chars, `metadata`,
  `allowed-tools`) type-checked when present.
- Relative paths referenced from SKILL.md (`scripts/`, `references/`,
  `assets/`) exist.
- Size guidance: warn when the SKILL.md body is large enough to hurt the
  context budget (spec guidance: keep it under ~500 lines, push detail to
  `references/`).

### 3.2 Activation evals: test discovery, not just execution

The spec's progressive-disclosure model means the `description` is the
routing surface — at discovery time an agent sees *only* name +
description and decides whether to load the skill. A skill whose body is
perfect but whose description never triggers is a broken skill, and no
current eval can detect it because `run` always injects the skill.

Proposal: a new eval mode (`"type": "activation"` or a `skill-eval
activation` phase) where the judge is given the skill's name +
description plus each eval prompt and asked "would an agent select this
skill for this task?" — plus negative prompts that should *not* trigger
it. Benchmarks then report activation precision/recall alongside output
pass rate. Imported agit prompts are ideal positive cases (they are real
tasks the skill is meant for), and sessions from *other* skills/repos are
natural negatives — another agengit synergy.

## 4. Other codebase improvements toward the goal

### 4.1 Repeat runs for statistical honesty

`Stats.Stddev` is currently variance *across evals*, not across repeated
runs of the same eval. Agent runs are high-variance; a one-sample delta
between with-skill and baseline is noise-prone, and `iteration_delta` can
show regressions that are just sampling error. Add `--runs N` (default 1)
to repeat each eval/config pair, aggregate per-eval means first, and
report per-eval variance. Even N=3 makes the with-skill delta far more
trustworthy, and the benchmark schema already has room for it.

### 4.2 Give the judge the transcript, not only the output files

`buildGradingPrompt` includes only output-file contents. Assertions about
process ("the agent asked no clarifying questions", "the agent ran the
tests", §2.3) are ungradeable. Persist the captured agent stdout (already
in memory in `runEval`, currently discarded beyond token extraction —
with the existing secret-safety caveats applied) as
`outputs/_transcript.txt` or a sibling artifact, and let assertions opt in
via a prefix (e.g. `transcript: …`).

### 4.3 A `command_succeeds:` deterministic matcher

The matcher family (`file_exists:`, `contains_text:`, `matches_text:`)
covers artifacts but not behavior. A `command_succeeds: go test ./...`
matcher (run in the output/scratch dir, sandboxed, with a timeout) is the
natural target for assertions mined from recorded verification tool calls
(§2.3), and is what "top quality" means for code-producing skills.
Security posture matters here (it executes eval-author-supplied commands),
so it should be opt-in via config.

### 4.4 LLM-assisted assertion authoring at import time

`keyTermFromSummary` is the weakest link in imported eval quality. Since a
judge agent is already configured, `import-agit --author-assertions` could
make one judge call per converted step: given the recorded prompt, diff
file list, and assistant summary, emit 3–5 specific, observable assertions
(mixing deterministic matchers and LLM-judged ones, per the guidance in
`eval-workflow.md`). One cheap call at import time buys much better
grading signal on every subsequent iteration.

### 4.5 Report-level synthesis: from benchmark to skill edits

The loop currently ends at numbers plus `llmCoachNotes`. The highest-value
output for the *author* is: "assertions that pass only with the skill"
(the skill's proven value), "assertions that fail in both configs" (eval
or skill defect), and — with §2.7 — process dimensions where the skill
helps. Structuring the report around those three buckets turns benchmarks
into concrete SKILL.md edit suggestions, which is the tool's stated
purpose.

## 5. Suggested sequencing

| # | Item | Depends on | Effort | Why this order |
|---|------|-----------|--------|----------------|
| 1 | Fixture import from agit blobs + hermetic run dirs (§2.1) | — | M-L | Without replay fidelity, everything downstream grades noise. |
| 2 | Merge dedupe by step hash (§2.6) | — | S | Makes import safe to re-run; prerequisite for corpus workflows. |
| 3 | `skill-eval validate` spec lint (§3.1) | — | S-M | Cheap, deterministic, catches the most common skill-breaking bugs. |
| 4 | Per-step eval filtering (§2.2) | — | S | Client already has the call site; add one flag and per-step gating. |
| 5 | Repeat runs `--runs N` (§4.1) | — | M | Statistical honesty for every benchmark thereafter. |
| 6 | Transcript artifact + tool-call assertions (§4.2, §2.3) | 1 | M | Enables process assertions; feeds matcher work. |
| 7 | Activation evals (§3.2) | 3 | M | New eval axis unique to skills; uses imported prompts as positives. |
| 8 | `--between`, `--bundle` import sources (§2.4, §2.5) | 2 | S each | Corpus ergonomics once import is idempotent. |
| 9 | Process-quality benchmarking via `agit eval` on runs (§2.7) | 5 | M-L | The deepest integration; needs stable timing correlation. |
| 10 | Assertion authoring + report synthesis (§4.4, §4.5) | 6 | M | Quality-of-signal polish on top of everything above. |

## Sources

- agengit README and format docs (`docs/format/steps-v1.md`,
  `eval-v1.md`, `cli-json-v1.md`, `bundle-v1.md`) at
  [github.com/matt-riley/agengit](https://github.com/matt-riley/agengit), v1.26.0.
- [Agent Skills specification](https://agentskills.io/specification)
  ([spec repo](https://github.com/agentskills/agentskills)); the format
  originated with [Anthropic](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview).
- This repo: `cmd_import_agit.go`, `internal/agit/{client,convert,types}.go`,
  `runner.go`, `grader.go`, `cmd_init.go`, `eval.go`, `eval-workflow.md`,
  `plans/README.md`, `docs/adr/0001-shell-out-to-agent-runtimes.md`.
