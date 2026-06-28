# Plan 003: Add deterministic assertion matchers to reduce judge cost and noise

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat c6dac63..HEAD -- eval.go grader.go docs/guides/*.md docs/site/src/content/docs/guides/*.md`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/001-go-test-coverage.md
- **Category**: direction
- **Planned at**: commit `c6dac63`, 2026-06-25

## Why this matters

Today every assertion is a free-text string sent to an LLM judge (`grader.go:33-42`). Common checks like "a file named `results.csv` exists" or "the output contains `revenue`" are deterministic, cost tokens unnecessarily, and can give inconsistent verdicts across prompts. Adding optional prefix-based matchers keeps the existing evals file format intact while making cheap checks cheap and reliable.

## Current state

- `eval.go:11-18` — `Eval.Assertions` is `[]string`.
- `grader.go:37-42` — `gradeEval` calls `buildAgentCmd` for the judge for every assertion.
- `eval-workflow.md` documents assertions as plain strings only.

Relevant excerpt from `eval.go:11-18`:

```go
type Eval struct {
	ID             int      `json:"id"`
	Prompt         string   `json:"prompt"`
	ExpectedOutput string   `json:"expected_output"`
	Files          []string `json:"files,omitempty"`
	Assertions     []string `json:"assertions,omitempty"`
}
```

Relevant excerpt from `grader.go:33-54` (gradeEval snippet):

```go
	if len(eval.Assertions) == 0 {
		return nil, fmt.Errorf("eval %d has no assertions to grade", eval.ID)
	}

	outputContents := readOutputContents(outDir)
	prompt := buildGradingPrompt(eval, outputContents)

	// Shell out to judge
	judgeAgent := cfg.Judge.Agent
	if judgeAgent == "" {
		judgeAgent = cfg.Defaults.Agent
	}
	judgeModel := cfg.Judge.Model
	if judgeModel == "" {
		judgeModel = cfg.Defaults.Model
	}

	cmd := buildAgentCmd(judgeAgent, judgeModel, prompt, "")
```

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w *.go` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Docs build | `cd docs/site && pnpm build` | exit 0 |

## Scope

**In scope**:
- `eval.go`
- `grader.go`
- `eval-workflow.md`
- Tests in `grader_test.go`

**Out of scope**:
- Do NOT change the JSON schema of `evals/evals.json` (assertions stay as strings).
- Do NOT remove LLM judge fallback.
- Do NOT add matchers that require parsing structured formats (JSON path, XML) in v1.

## Git workflow

- Branch: `advisor/003-deterministic-assertion-matchers`
- Commit message style: `feat: add deterministic file/text assertion matchers`.
- Do NOT push unless instructed.

## Steps

### Step 1: Define matcher types and parser

In `eval.go`, add:

```go
type MatcherType string

const (
	MatcherLLM          MatcherType = "llm"
	MatcherFileExists   MatcherType = "file_exists"
	MatcherContainsText MatcherType = "contains_text"
	MatcherMatchesText  MatcherType = "matches_text"
)

type ParsedAssertion struct {
	Original string
	Type     MatcherType
	File     string
	Arg      string
}
```

Add a function `parseAssertion(s string) ParsedAssertion` in `grader.go` that:

- If `s` starts with `file_exists:`, extracts path after colon → `file_exists`.
- Else if `s` starts with `contains_text:`, splits on `:` into file and substring. Require exactly two colons after prefix.
- Else if `s` starts with `matches_text:`, same split for regex.
- Else → `llm` with the full string as original.

Trim whitespace.

**Verify**: `go test ./... -run TestParseAssertion` → pass (add tests in `grader_test.go`).

### Step 2: Run deterministic matchers before the judge

In `gradeFromOutput` (or `gradeEval` if Plan 001 hasn't landed), partition `eval.Assertions`:

1. Run `parseAssertion` for each.
2. For non-LLM matchers, evaluate against `outputContents` (map from `readOutputContents`) and the output dir.
   - `file_exists`: `os.Stat` on `outDir/<File>`.
   - `contains_text`: file content contains `Arg`.
   - `matches_text`: compile regex and match against file content.
3. Collect LLM assertions separately.

Produce `AssertionResult` values for matchers with evidence like `file outputs/results.csv exists`.

**Verify**: `go test ./... -run TestMatch*` → pass.

### Step 3: Judge only LLM assertions

Build the grading prompt with only the LLM assertions (or all of them, but the deterministic ones are already decided and excluded from the judge prompt to save tokens). The simplest approach:

- Create a temporary `Eval` copy containing only LLM assertions for `buildGradingPrompt`.
- Merge judge results back into the full `AssertionResult` list preserving original assertion order.

Compute `Summary.Passed/Failed/Total` from the merged list.

**Verify**: `go test ./... -run TestGradeMixedMatchers` → pass (mixed LLM + deterministic).

### Step 4: Update workflow docs

In `eval-workflow.md`, add examples:

```json
"assertions": [
  "file_exists: results.csv",
  "contains_text: summary.txt:Total revenue",
  "matches_text: output.md:## Summary",
  "The chart uses a sensible color palette and is visually clear"
]
```

Explain that prefixed assertions are evaluated locally and the rest go to the LLM judge.

**Verify**: docs build passes.

### Step 5: Final checks

```bash
gofmt -w *.go
go test ./...
go vet ./...
golangci-lint run
cd docs/site && pnpm build
```

All pass.

## Test plan

- `TestParseAssertion`: file_exists, contains_text, matches_text, llm fallback, malformed strings.
- `TestMatcherFileExists`: file present, file missing.
- `TestMatcherContainsText`: contains, does not contain.
- `TestMatcherMatchesText`: regex matches, invalid regex fails gracefully with evidence.
- `TestGradeMixedMatchers`: one deterministic pass, one LLM assertion, ensure total/failed are computed from merged results.

## Done criteria

- [ ] `file_exists:`, `contains_text:`, and `matches_text:` assertions are evaluated without calling the judge.
- [ ] Plain-string assertions still go to the LLM judge unchanged.
- [ ] Assertion order is preserved in `grading.json`.
- [ ] `go test ./...` and `golangci-lint run` pass.
- [ ] `eval-workflow.md` documents the new prefixes.
- [ ] `docs/site pnpm build` passes.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The code at `grader.go:33-54` no longer matches the excerpt (drift).
- Any matcher requires reading files outside the eval output directory.
- Preserving assertion order turns out to require changing `Eval` struct JSON tags.

## Maintenance notes

- Future matchers should be added to `parseAssertion` and the evaluator switch; keep each matcher pure and output-directory scoped.
- Reviewers should ensure the LLM judge prompt never receives assertions that were already decided locally, or the cost savings are lost.
- Follow-up v1.x: JSON path matcher for structured outputs.
