---
title: Auto-Fixing with --fix
description: Let skill-eval automatically re-run failing evals — the judge's feedback gets fed back to the agent as a critique until assertions pass or the score stops improving.
---

# 🪄 Auto-Fixing Failing Evals

Sometimes your skill is *almost* there — one assertion keeps failing, and you know exactly what the agent got wrong. Instead of tweaking your SKILL.md and re-running the whole loop by hand, let skill-eval handle it for you!

Meet the `--fix` flag. It turns `skill-eval loop` from a single-pass pipeline into an iterative refinement loop. The judge's feedback becomes the agent's critique, and the agent gets another crack at it — automatically!

---

## How it works 🔄

Normally, `skill-eval loop` does **run → grade → benchmark** and stops. With `--fix`, it adds an extra phase:

```
run → grade → fix (repeat until passes) → benchmark
```

Here's the play-by-play:

1. **Run & grade as usual** — all evals execute, the judge scores them.
2. **Find the failures** — any with-skill eval that didn't pass every assertion gets flagged.
3. **Extract the critique** — the judge's `evidence` from each FAIL becomes the agent's "here's what you got wrong" note.
4. **Re-run with feedback** — the agent tries again, this time with the critique in its prompt.
5. **Re-grade** — the judge evaluates the new attempt.
6. **Keep going until...**
   - All assertions pass ✅
   - The same assertions fail twice in a row (plateau detected — time to try a different approach!)
   - The attempt budget runs out (default: 3 attempts)

The best attempt wins, and its grading replaces the original!

<video src="/guides/auto-fixing.mp4" controls muted width="100%" class="rounded-3xl border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"></video>

---

## Using it 🚀

It couldn't be simpler:

```bash
skill-eval loop --fix
```

That's it! skill-eval will refine each failing eval up to 3 times by default. Want more patience?

```bash
skill-eval loop --fix --max-fix-attempts 5
```

The baseline path is **never fixed** — it's your control group. Only the with-skill side gets the auto-refinement treatment.

---

## What you'll see 👀

During the fix phase, skill-eval reports every attempt:

```
[3/4] Auto-fixing failed evals...
  eval 1: already passing, skipping
  eval 2: 1/2 failed — fixing...
    attempt 1: 1/2 passed
    attempt 2: 2/2 passed ✓
    best: attempt 2 (100% pass)
```

And inside your workspace, each fix attempt gets its own directory:

```
iteration-1/
  eval-2/
    with_skill/
      outputs/          ← initial attempt
      grading.json      ← best result (promoted from fix-2)
      fix-2/
        outputs/        ← first fix attempt
        grading.json
        timing.json
      fix-3/
        outputs/        ← second fix attempt (the winner!)
        grading.json
        timing.json
      fix-results.json  ← full trajectory log
```

> 💡 The initial grading is recorded as attempt 1, so fix directories start at `fix-2`.

---

## When to use `--fix` vs doing it yourself 🤔

| Use `--fix` when... | Iterate manually when... |
|---|---|
| The fix is obvious — the agent just needs a nudge | The core skill instructions are wrong |
| You're iterating fast and want a quick improvement | You need to rethink your assertions |
| One or two assertions keep failing predictably | The whole approach needs re-architecting |
| You want to see if a small prompt tweak resolves it | You're adding new test cases or changing expected output |

Think of `--fix` as your fast-feedback shortcut. It's not a replacement for careful skill design — it's a turbo boost for those "almost there" moments! 🏎️

---

## Where the critique comes from 🔍

The critique is extracted directly from the judge's `evidence` field. If your grading prompt produces clear, specific evidence (like *"Chart shows 5 months instead of the requested 3"*), the agent gets actionable feedback. Vague evidence (*"Output was incorrect"*) leads to vague fixes.

That's another reason to write good assertions and a solid grading prompt — the fix loop is only as smart as your judge!

---

## What's next?

- **[Giving Feedback](/guides/giving-feedback/)** — For when you want to add *human* notes to the judge's context.
- **[Eval Workflow](/eval-workflow/)** — The full picture of how all the pieces connect.
