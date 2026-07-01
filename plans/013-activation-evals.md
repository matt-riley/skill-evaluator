# Plan 013: Activation evals — test skill discovery, not just execution

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- eval.go grader.go cmd_run.go cmd_grade.go benchmark.go cmd_import_agit.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (introduces a second eval type through run/grade/benchmark; must not disturb task evals)
- **Depends on**: plans/009-skill-md-validate-command.md (reuses `parseSkillMD`)
- **Category**: direction (spec alignment)
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §3.2

## Why this matters

The Agent Skills spec's progressive-disclosure model means an agent decides
whether to load a skill from its **name + description alone** — the body is
read only after activation. A skill with a perfect body and a description
that never triggers is a broken skill, and no current eval can detect it,
because `run` always injects the skill path into the prompt
(`runner.go:243-245`). Symmetrically, an over-broad description that
triggers on unrelated tasks pollutes agent context everywhere.

Activation evals close this: given only the frontmatter, would a model
select this skill for prompt X? Positives are cheap (imported agit prompts
are real tasks the skill served); negatives are prompts that must NOT
trigger it. The benchmark then reports activation precision/recall
alongside output pass rates — a second axis unique to skill quality.

## Current state

- `eval.go:12-19` — `Eval` has no type concept; every eval is a task eval.
- `cmd_run.go:172-190` — the run loop executes every eval in
  `ef.Evals` with agent invocations.
- `cmd_grade.go:75-117` — grading walks the same list expecting
  `outputs/` dirs.
- `benchmark.go:12-80` — `computeBenchmark` aggregates
  with_skill/baseline `RunResult`s only.
- `eval.go:151-168` — `BenchmarkFile` has room for additive fields.
- `grader.go:59-64` — judge shell-out pattern to reuse
  (`cfg.Judge`, `cmdFn`, `parseGradingOutput`-style JSON extraction at
  `grader.go:335-347`).
- Plan 009 provides `parseSkillMD` returning frontmatter (name, description).

## Design decisions (read before coding)

1. **Schema**: additive fields on `Eval`:

```go
	// Type is "task" (default, empty) or "activation".
	Type string `json:"type,omitempty"`
	// ShouldActivate is the expected discovery verdict for activation
	// evals. Nil defaults to true (a positive case).
	ShouldActivate *bool `json:"should_activate,omitempty"`
```

   Activation evals need no `expected_output`, `files`, or `assertions`;
   `readEvals` validation must not reject them for that.
2. **Phase**: activation evals are judged during **grade** (they need no
   agent run — `run` skips them). This keeps the expensive phase untouched
   and lets `loop` work unchanged.
3. **Judged, not simulated**: v1 asks the configured judge "given this
   skill's name and description, would an agent handling this task load
   it?" — a judgment of the description's routing quality. Actually probing
   a specific runtime's real router is out of scope (runtime-dependent,
   not portable across the shell-out agents).
4. **Multiple samples**: activation verdicts are cheap; ask the judge for a
   single verdict per eval in v1 (repetitions arrive with Plan 011's
   `--runs` if ever needed — do not build it here).
5. **Metrics**: over activation evals — TP (should=true, verdict=yes),
   FP (should=false, verdict=yes), FN (should=true, verdict=no),
   TN (should=false, verdict=no). Report precision TP/(TP+FP), recall
   TP/(TP+FN), accuracy, and counts. Guard division by zero (report as 0
   with the counts visible).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `eval.go` (Eval fields, `ActivationResult`, `ActivationSummary`, `BenchmarkFile.Activation`)
- New `activation.go` + `activation_test.go` (judge prompt, verdict parse, metrics)
- `cmd_run.go` (skip activation evals; exclude from cost count)
- `cmd_grade.go` (judge activation evals; write `activation.json` per eval)
- `benchmark.go` (aggregate `ActivationSummary`)
- `report.go` (activation section in HTML report)
- `cmd_import_agit.go` (optional `--as-activation` flag, see Step 6)
- `eval-workflow.md`, new `docs/guides/activation-evals.md`

**Out of scope**:
- Do NOT run activation evals against the baseline config — there is no
  skill to activate in a baseline; activation is a property of the skill's
  frontmatter only.
- Do NOT probe real runtime skill routers in v1.
- Do NOT auto-generate negative prompts in this plan (Plan 016's authoring
  machinery is the right home; note it there).
- Do NOT change task-eval behavior in any way.

## Git workflow

- Branch: `advisor/013-activation-evals`
- Commit message style: `feat: add activation evals grading skill discovery precision and recall`
- Do NOT push unless instructed.

## Steps

### Step 1: Schema and helpers

- Add the two `Eval` fields (Design decision 1) and:

```go
// isActivation reports whether an eval tests discovery rather than execution.
func (e Eval) isActivation() bool { return e.Type == "activation" }

// expectedActivation returns the expected verdict, defaulting to true.
func (e Eval) expectedActivation() bool {
	return e.ShouldActivate == nil || *e.ShouldActivate
}
```

- New result types in `eval.go`:

```go
// ActivationResult is the judged discovery verdict for one activation eval.
type ActivationResult struct {
	EvalID        int    `json:"eval_id"`
	Expected      bool   `json:"expected"`
	WouldActivate bool   `json:"would_activate"`
	Reason        string `json:"reason"`
}

// ActivationSummary aggregates activation verdicts for a benchmark.
type ActivationSummary struct {
	Total     int     `json:"total"`
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
	TN        int     `json:"tn"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	Accuracy  float64 `json:"accuracy"`
}
```

- `BenchmarkFile`: `Activation *ActivationSummary \`json:"activation,omitempty"\``.

**Verify**: `go build ./...`.

### Step 2: The activation judge

New `activation.go`:

```go
// buildActivationPrompt asks the judge whether an agent seeing only the
// skill's name and description would load it for the given task. The task
// prompt is wrapped in markers, mirroring buildGradingPrompt's injection
// defenses (grader.go:286-292).
func buildActivationPrompt(name, description, taskPrompt string) string
```

Prompt contract (return ONLY JSON, same style as `grader.go:294-303`):

```
{"would_activate": true, "reason": "..."}
```

The prompt must instruct the judge to consider ONLY the name/description
(progressive disclosure), to answer for a *general* coding agent, and to
say no when the task is merely adjacent. Reuse `sanitizeAssertionText` on
the task prompt.

```go
// judgeActivation runs one activation eval through the judge agent.
func judgeActivation(ctx context.Context, cfg *Config, name, description string, eval Eval, cmdFn CmdBuilder) (*ActivationResult, error)
```

Follow `gradeFromOutput`'s judge plumbing (`grader.go:49-64`): judge
agent/model fallback to defaults, `cmdFn` defaulting to `buildAgentCmd`,
first-`{` JSON extraction with a size limit.

**Verify**: `go test ./... -run TestBuildActivationPrompt` and
`TestJudgeActivationParsesVerdict` (stub `CmdBuilder` returning canned JSON).

### Step 3: Run phase skips activation evals

In `cmd_run.go`:
- The eval-count loop (`:130-135`) and the job-building loop (`:172-190`)
  skip `eval.isActivation()` (with a one-line notice the first time:
  `N activation eval(s) will be judged during grade`).
- They must not enter the lockfile either (they have no run identity).

**Verify**: `go test ./... -run TestRunSkipsActivationEvals` (stub
`CmdBuilder` counting invocations).

### Step 4: Grade phase judges them

In `cmd_grade.go`, after the existing task-eval loop:
- If any activation evals exist: read frontmatter once via Plan 009's
  `parseSkillMD` (`os.ReadFile(filepath.Join(skillDir, "SKILL.md"))`);
  on parse error, fail activation grading with a pointer to
  `skill-eval validate`.
- For each activation eval (respect `--eval` filter): call
  `judgeActivation`, print
  `  eval 7 activation... would_activate=yes (expected yes)`, and write
  `activation.json` (the `ActivationResult`) to
  `evalPath(ws, iter, eval.ID, "")/activation.json` — model-independent,
  no config subdir.
- Collect results and pass them to benchmark: extend `computeBenchmark`'s
  signature with `activations []ActivationResult` (variadic or new param —
  check both call sites: `cmd_grade.go:120` and `cmd_benchmark.go`).

**Verify**: `go test ./... -run TestGradeActivation` (temp workspace, stub
judge; `activation.json` written; benchmark receives results).

### Step 5: Benchmark + report

- `benchmark.go`: new `func summarizeActivation(results []ActivationResult) *ActivationSummary`
  implementing the metrics (Design decision 5); `computeBenchmark` sets
  `bf.Activation` when results exist. `cmd_benchmark.go` (standalone
  benchmark command) must also load persisted `activation.json` files from
  the iteration so re-benchmarking works without re-grading — mirror how it
  loads `grading.json` (read that file first to match its pattern).
- `report.go`: add an "Activation" section to the template when
  `.Activation` is non-nil: precision/recall/accuracy plus the FP/FN eval
  IDs (misrouting cases are exactly what the author must fix in the
  description). Extend `ReportData` accordingly.

**Verify**: `go test ./... -run TestSummarizeActivation` (hand-computed:
2 TP, 1 FP, 1 FN, 1 TN → precision 0.667, recall 0.667, accuracy 0.6);
`TestReportRendersActivation`.

### Step 6: Import synergy (small, optional-flag)

In `cmd_import_agit.go`: add
`asActivation := fs.Bool("as-activation", false, "Import prompts as activation evals (positives) instead of task evals")`.
When set, emitted evals get `Type: "activation"`, no assertions/expected
output, `ShouldActivate: nil` (positive). Use case: sessions recorded in
*other* repos imported with `--as-activation` + hand-flipped
`should_activate: false` become negatives. Document that workflow in the
guide rather than automating negative labeling.

**Verify**: `go run . import-agit --as-activation --skill <tmp> --force`
produces type-tagged evals (or unit-test the conversion mapping).

### Step 7: Docs and final checks

- `docs/guides/activation-evals.md`: what activation testing is (spec's
  progressive disclosure), JSON examples of a positive and a negative,
  the precision/recall reading guide, and the "import negatives from other
  repos" recipe.
- `eval-workflow.md`: short section + link.

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

- `TestEvalActivationDefaults` (`expectedActivation` nil→true, false→false).
- `TestBuildActivationPrompt` (contains name/description/task, markers, JSON contract).
- `TestJudgeActivationParsesVerdict` (+ malformed JSON → error).
- `TestRunSkipsActivationEvals` (zero agent invocations for them; cost count excludes them).
- `TestGradeActivation` (writes `activation.json`; respects `--eval`).
- `TestSummarizeActivation` (metric math incl. zero-division guards: all-positive corpus → precision with TP+FP=0 handled).
- `TestReportRendersActivation`.
- Regression: full existing test suite green — task evals untouched.

## Done criteria

- [ ] `"type": "activation"` evals are skipped by `run`, judged by `grade`, summarized in `benchmark.json`, and rendered in the report.
- [ ] Negative cases via `"should_activate": false` work and feed FP/TN.
- [ ] Task-eval behavior is byte-identical when no activation evals exist.
- [ ] `import-agit --as-activation` imports prompts as positives.
- [ ] Judge prompt reuses the existing injection defenses.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Guides updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Plan 009 has not landed (no `parseSkillMD`) — implement 009 first or
  extract just the frontmatter parser with 009's tests.
- `computeBenchmark`'s signature change touches more call sites than
  `cmd_grade.go` and `cmd_benchmark.go`.
- `readEvals` performs validation that rejects assertion-less evals —
  loosen it for activation type only, and STOP if that loosening would
  also accept broken *task* evals.

## Maintenance notes

- The activation prompt is policy; version it in a code comment and note
  in the guide that verdicts are judge-model-dependent (pin a cheap,
  consistent judge model for comparable numbers across iterations).
- Follow-up candidates: N-sample activation verdicts with majority vote;
  probing real runtime routers where a runtime exposes one; auto-generated
  negatives via Plan 016's authoring call.
