---
title: Running Your First Evaluation
description: A step-by-step walkthrough of setting up evals, configuring your skill, and running the full skill-eval loop for the first time.
---

# 🌱 Running Your First Evaluation

Ready to see your skill put through its paces? This guide walks you through the full journey — from a blank slate to a working eval loop — in just a few minutes.

---

## Before you start 🛠️

Make sure you've got the CLI installed and your global config set up:

```bash
go install github.com/matt-riley/skill-evaluator@latest
skill-eval init --global
```

> 💡 The global config lives at `~/.config/skill-eval/config.yaml`. It's where you set your default agent and model — you only need to do this once!

---

## Step 1 — Scaffold your eval directory 📁

Navigate to your skill directory (the folder where your `SKILL.md` lives) and run:

```bash
skill-eval init
```

This creates an `evals/` folder with a starter `evals.json` file. Open it up — it's your canvas!

Here's what a filled-out `evals.json` looks like:

```json
{
  "skill_name": "csv-analyzer",
  "evals": [
    {
      "id": 1,
      "prompt": "Analyze sales_2025.csv and show me the top 3 months by revenue.",
      "expected_output": "A ranked list of the top 3 months with revenue totals.",
      "assertions": [
        "Output names exactly 3 months",
        "Months are sorted by revenue descending",
        "Revenue totals are shown"
      ]
    }
  ]
}
```

**💡 Quick tips for great evals:**
- **Be specific!** Vague prompts produce vague results.
- **Start small.** Two or three evals is plenty for your first loop.
- **Add your assertions later** — run once without them first so you know what "good" actually looks like!

---

## Step 2 — Check your config ⚙️

Drop a `.skill-eval.yaml` next to your `SKILL.md` to tell skill-eval which agent and model to use:

```yaml
agent: pi
model: claude-sonnet-4-5
```

This overrides your global defaults for just this skill. Handy when you're comparing across models!

---

## Step 3 — Run the loop! 🔄

Here's the magic command:

```bash
skill-eval loop
```

Watch it go! This single command handles the whole cycle:

1. **Run** — executes every eval twice: once with your skill active, once as a plain baseline.
2. **Grade** — asks the judge agent to check each assertion, producing a `grading.json` with PASS/FAIL verdicts.
3. **Benchmark** — rolls all the stats up into a `benchmark.json` so you can see the delta at a glance.

<video src="/guides/first-eval.mp4" controls muted width="100%" class="rounded-3xl border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"></video>

---

## Step 4 — Peek at the workspace 👀

After the loop finishes, your results land in `<skill-name>-workspace/iteration-1/`. Here's what you'll find:

| File | What it tells you |
|------|------------------|
| `outputs/` | The actual agent responses — open them up! |
| `grading.json` | Which assertions passed or failed, with the judge's reasoning. |
| `benchmark.json` | The headline stats: pass rates, timing, and the with-skill vs baseline delta. |
| `feedback.json` | Empty for now — this is where you leave notes for next time. |

---

## What's next? 🚀

- Head over to **[Reading Your Results](/guides/reading-results/)** to learn how to interpret `benchmark.json`.
- When something fails, **[Giving Feedback](/guides/giving-feedback/)** shows you exactly what to write and why it matters.

You're off to a great start — keep iterating! ✨
