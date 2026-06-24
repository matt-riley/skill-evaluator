# Skill Evaluator

A standalone, provider-agnostic CLI tool that automates the eval-driven skill iteration loop: defining test cases, spawning agent runs with and without a skill, LLM-grading assertion results, and aggregating benchmarks.

## Language

**Skill**:
A set of instructions and optional scripts that guide an LLM agent's behavior, packaged as a `SKILL.md` file.
_Avoid_: plugin, extension, prompt

**Eval**:
A test case consisting of a prompt, an expected output description, optional input files, and a set of assertions.
_Avoid_: test, test case, scenario

**Assertion**:
A verifiable pass/fail statement about what an eval's output should contain or achieve.
_Avoid_: check, validation rule

**Run**:
A single execution of an eval by an agent, producing outputs, timing data, and token counts.
_Avoid_: execution, invocation

**Grading**:
The process of evaluating each assertion against actual run outputs (PASS/FAIL with evidence), typically by an LLM judge.
_Avoid_: scoring, evaluation (reserved for the broader eval process)

**Benchmark**:
Aggregated statistics (mean pass rate, time, tokens, stddev) across all runs in an iteration, with computed deltas between with-skill and without-skill configurations.
_Avoid_: score, report

**Iteration**:
One full pass through all evals (run, grade, benchmark), producing an `iteration-N/` directory in the workspace.
_Avoid_: cycle, round

**Workspace**:
The directory structure where eval results live, organized by iteration and eval, containing outputs, timing, grading, benchmarks, and human feedback.
_Avoid_: results directory, output directory

**Feedback**:
Human-written notes on eval outputs, capturing qualitative issues that assertions miss.
_Avoid_: review notes, comments

**Agent Runtime**:
The external CLI tool that executes runs (e.g., `pi`, `claude`, `copilot`, `codex`). The tool shells out to it rather than embedding an LLM client.
_Avoid_: provider, backend, executor

**Global Config**:
User-wide defaults for agent runtime and model, stored at `~/.config/skill-eval/config.yaml`. A JSON Schema is provided for LSP autocomplete.
_Avoid_: user config

**Skill Config**:
Per-skill overrides at `.skill-eval.yaml` in the skill directory, overriding global defaults for that skill. Same YAML format as global config. Both share a JSON Schema.
_Avoid_: project config, local config

**Judge**:
The LLM configuration used for grading assertions — typically a cheaper/faster model than the run model, since grading is read-only. Implemented by shelling out to the same agent runtime with a different model and a grading prompt.
_Avoid_: grader, evaluator (ambiguous with the tool itself)

**Run Failure**:
A run that produces no outputs (agent crash, timeout) — recorded as `status: "failed"` and counted as 0% pass rate at aggregation. Does not abort the iteration.
_Avoid_: error, crash (these are causes, not the result)

**Baseline**:
The comparison configuration for a run — either no-skill (default) or a snapshot of a previous skill version. The workspace uses `baseline/` instead of `without_skill/` to cover both cases.
_Avoid_: without_skill, old_skill, control

**Snapshot**:
A copy of a skill directory taken before an iteration, used as the baseline for the next iteration.
_Avoid_: backup, clone

## Relationships

- A **Skill** is evaluated across multiple **Evals**
- An **Eval** produces two **Runs** (with-skill and baseline)
- A **Run** is **Graded** to produce assertion results
- **Grading** results are aggregated into a **Benchmark**
- An **Iteration** contains **Runs**, **Grading**, and a **Benchmark**
- **Feedback** is attached per **Eval** within an **Iteration**
- Everything lives in a **Workspace**

## Flagged ambiguities

- None yet.
