---
title: Reading Your Results
description: Learn how to interpret benchmark.json and grading.json after a skill-eval run — understand pass rates, deltas, and what each result type means for your skill.
---

# 📊 Reading Your Results

After `skill-eval loop` finishes, two files tell you everything you need to know: `benchmark.json` for the big picture, and `grading.json` for the detail. Here's how to read them like a pro!

---

## The headline: `benchmark.json` 🏆

This is your scoreboard. Open it with:

```bash
cat <skill-name>-workspace/iteration-1/benchmark.json | jq .
```

For a single-model run, you'll see something like this:

```json
{
  "run_summary": {
    "with_skill": {
      "pass_rate": { "mean": 0.89, "stddev": 0.05 },
      "time_seconds": { "mean": 18.4, "stddev": 2.1 },
      "tokens": { "mean": 1250, "stddev": 120 }
    },
    "baseline": {
      "pass_rate": { "mean": 0.56, "stddev": 0.08 },
      "time_seconds": { "mean": 14.2, "stddev": 1.5 },
      "tokens": { "mean": 980, "stddev": 90 }
    },
    "delta": {
      "pass_rate": 0.33,
      "time_seconds": 4.2,
      "tokens": 270
    }
  },
  "generated_at": "2026-06-25T16:30:00Z"
}
```

<video src="/guides/reading-results.mp4" controls muted width="100%" class="rounded-3xl border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"></video>

### What each field means

| Field | What it's telling you |
|---|---|
| `run_summary.with_skill` | Average stats across all evals *when your skill was active* |
| `run_summary.baseline` | Same stats, but without your skill (the control group) |
| `run_summary.delta` | The difference — this is the number you're trying to maximise! |
| `pass_rate` | Fraction of assertions that passed (mean and stddev) |
| `time_seconds` | Average wall-clock time per eval |
| `tokens` | Average total tokens consumed per eval |

> 💡 **A positive `pass_rate` delta is the goal.** If it's `+0%` or negative, your skill isn't helping (or it's actively hurting). Time to revisit your instructions!

### Tracking progress across iterations 🔄

When a previous iteration exists, `benchmark.json` also includes a `previous_iteration` field and an `iteration_delta`:

```json
{
  "previous_iteration": 1,
  "iteration_delta": {
    "pass_rate": 0.05,
    "time_seconds": 0.2,
    "tokens": -15
  }
}
```

`iteration_delta` is the current iteration's delta minus the previous iteration's delta. A **positive `pass_rate`** means your skill improved relative to baseline since the last run. A negative value means it got worse — a signal that a recent change may have hurt performance. Keep an eye on this to make sure you're moving in the right direction! 📈

---

## Per-eval breakdown 🔬

For the verdict on every individual eval, dig into the per-eval directories rather than `benchmark.json`. Each eval directory has its own `grading.json`:

```bash
cat <skill-name>-workspace/iteration-1/eval-1/with_skill/grading.json | jq .
```

Look for these patterns:

| Pattern | What it means |
|---|---|
| With-skill PASS, baseline FAIL | 🌟 Your skill turned a failure into a success — this is what you're looking for! |
| Both PASS or both FAIL | Neither better nor worse — skill made no visible difference here. |
| With-skill FAIL, baseline PASS | ⚠️ Skill over-constrained the agent. Your instructions may be too prescriptive. |

---

## The detail: `grading.json` 🧑‍⚖️

`benchmark.json` tells you *that* something failed. `grading.json` tells you *why*:

```bash
cat <skill-name>-workspace/iteration-1/grading.json | jq '.["eval-1"]'
```

```json
{
  "overall": "FAIL",
  "assertions": [
    {
      "assertion": "Output includes a bar chart image",
      "result": "PASS"
    },
    {
      "assertion": "Chart shows exactly 3 months",
      "result": "FAIL",
      "reasoning": "Got 5 months — top-3 filter was not applied"
    },
    {
      "assertion": "Both axes are labeled",
      "result": "PASS"
    }
  ]
}
```

The `reasoning` field is gold. It's the judge's explanation for the verdict — read it carefully before updating your skill instructions or your feedback.

---

## Common patterns to look for 🧩

**All evals fail, with and without skill**
Your assertions might be too strict, or your eval prompts might need more context. Try relaxing one assertion at a time to isolate the issue.

**Baseline always passes, with-skill always fails**
Your skill instructions are constraining the agent in a way that's breaking something. Look for overly prescriptive steps in your `SKILL.md`.

**With-skill passes, baseline fails (the dream!)**
This is exactly the signal you want. Note which instructions drove the improvement — that's your skill earning its keep.

**Skill is slower but passes more**
A timing trade-off. Usually fine! But if the extra time is significant, check whether your skill is prompting extra unnecessary steps.

---

## Iterating from here 🔄

Once you've read your results:

1. **Add feedback** for any failing evals — see [Giving Feedback](/guides/giving-feedback/) for how to write notes that actually help.
2. **Tweak your skill** instructions based on what you learned from the `reasoning` fields.
3. **Run again** to compare: `skill-eval loop --baseline previous` creates an `iteration-2/` alongside `iteration-1/` so you can track progress over time.

The numbers will get better. Keep going! 🚀
