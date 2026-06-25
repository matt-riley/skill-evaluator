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
cat <skill-name>-workspace/iteration-1/benchmark.json | jq .summary
```

You'll see something like this:

```json
{
  "total_evals": 3,
  "pass_rate_with_skill": 0.89,
  "pass_rate_baseline": 0.56,
  "delta": "+33%",
  "avg_duration_with_skill_s": 18.4,
  "avg_duration_baseline_s": 14.2
}
```

<video src="/guides/reading-results.mp4" controls muted width="100%" class="rounded-3xl border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"></video>

### What each field means

| Field | What it's telling you |
|---|---|
| `pass_rate_with_skill` | Fraction of assertions that passed *when your skill was active* |
| `pass_rate_baseline` | Same thing, but without your skill (the control group) |
| `delta` | The difference — this is the number you're trying to maximise! |
| `avg_duration_*` | Average wall-clock time per eval, with and without skill |

> 💡 **A positive delta is the goal.** If `delta` is `+0%` or negative, your skill isn't helping (or it's actively hurting). Time to revisit your instructions!

---

## Per-eval breakdown 🔬

The `evals` section of `benchmark.json` shows the verdict for every individual eval:

```bash
cat <skill-name>-workspace/iteration-1/benchmark.json | jq '.evals'
```

```json
{
  "eval-1": { "with_skill": "FAIL", "baseline": "FAIL", "delta": "none" },
  "eval-2": { "with_skill": "PASS", "baseline": "PASS", "delta": "none" },
  "eval-3": { "with_skill": "PASS", "baseline": "FAIL", "delta": "+skill" }
}
```

Here's how to read the `delta` column:

| Delta value | What it means |
|---|---|
| `+skill` | 🌟 Your skill turned a FAIL into a PASS — this is what you're looking for! |
| `none` | Neither better nor worse — skill made no difference here. |
| `-skill` | ⚠️ Skill turned a PASS into a FAIL. Your instructions may be over-constraining the agent. |

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
