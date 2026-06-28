---
title: Giving Feedback
description: Learn how to write useful feedback in feedback.json after a grading run, and how skill-eval passes that context to the judge on the next iteration.
---

# 💬 Giving Feedback (and What Happens Next)

After grading, skill-eval leaves a `feedback.json` in your workspace. It's easy to overlook — but filling it in is one of the most powerful things you can do to improve your skill quickly.

Here's exactly how it works, and what happens to your notes after you write them.

---

## Where feedback lives 📍

After every `skill-eval grade` or `skill-eval loop`, your workspace gets a `feedback.json`:

```
<skill-name>-workspace/
  iteration-1/
    grading.json      ← what the judge decided
    feedback.json     ← your turn to react
    benchmark.json    ← the headline numbers
    outputs/          ← the raw agent responses
```

Open `feedback.json` — it's pre-populated with one key per eval ID, all empty:

```json
{
  "eval-1": "",
  "eval-2": "",
  "eval-3": ""
}
```

---

## Step 1 — Read the grading first 🔍

Before you write anything, check `grading.json` to understand what the judge saw:

```bash
cat <skill-name>-workspace/iteration-1/grading.json | jq .
```

Look for `"result": "FAIL"` entries. The `reasoning` field tells you *why* the judge failed an assertion — that's your cue for what to address in feedback.

<video src="/guides/feedback-loop.mp4" controls muted width="100%" class="rounded-3xl border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"></video>

---

## Step 2 — Write targeted feedback ✏️

Edit `feedback.json` and fill in the key for any eval that surprised you — whether it passed unexpectedly or failed in a confusing way.

```json
{
  "eval-1": "Chart shows all months, not just the top 3 by revenue. The skill should filter before plotting.",
  "eval-2": "",
  "eval-3": "Output format was correct but the summary paragraph was missing. Needs a one-line narrative."
}
```

**What makes feedback useful:**

| ✅ Good feedback | 🚫 Unhelpful feedback |
|---|---|
| "Chart shows all months, not just top 3" | "It was wrong" |
| "Summary paragraph is missing entirely" | "Needs improvement" |
| "File was saved to /tmp instead of the output dir" | "File issue" |

Keep it specific and observable. The judge — and your future self — will thank you!

> 💡 **Leave it empty if it was fine.** An empty string means "nothing to add here." You don't need to write feedback for every eval, every time.

---

## What happens next? 🔮

Your feedback travels with the eval on the **next grading run**. When you run:

```bash
skill-eval grade
# or
skill-eval loop
```

skill-eval passes your feedback notes directly to the judge agent alongside the agent's output and your assertions. The judge uses them as extra context — like a reviewer's margin notes — when deciding whether each assertion passes.

This means:
- **Failed evals with clear feedback** tend to produce more precise FAIL reasoning, making it easier to trace the root cause back to your skill instructions.
- **Feedback survives across iterations.** If you run `skill-eval loop --baseline previous`, your notes carry forward into the new iteration.
- **Empty feedback signals confidence.** Once an eval is consistently passing and you've cleared its feedback, that's a signal your skill has nailed that scenario!

---

## Clearing feedback ♻️

Once an issue is resolved and your assertions are passing cleanly, just empty the string back out:

```json
{
  "eval-1": "",
  "eval-2": ""
}
```

A completely empty `feedback.json` is the goal — it means every eval is behaving exactly as expected. 🎉

---

## What's next?

- **[Reading Your Results](/guides/reading-results/)** — make sense of `benchmark.json` and spot where your skill is winning.
- **[Eval Workflow](/eval-workflow/)** — the full picture of how all the pieces connect.
