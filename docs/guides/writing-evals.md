---
title: Writing Useful Evals
description: How to design evals and assertions that show whether your skill actually helps the model.
---

# 🎯 Writing Useful Evals

Welcome to the heart of `skill-eval`! If you're new to writing evals (short for evaluations), think of them as friendly grading rubrics for your AI. Instead of traditional unit tests that just check if code compiles, an eval checks if the AI made the *right kinds of decisions* when following your skill's instructions.

A well-written eval doesn't just check that an agent *can* do a task — it checks whether your **skill makes the agent better at it**. 

This guide is your trusty companion to writing evals that produce trustworthy, actionable numbers. Let's dive in.

---

## The Golden Rule

> A good eval is one where the **baseline run fails more often than the with-skill run**.

If both the baseline (the AI working all alone) and the skill (the AI armed with your awesome instructions) sail through every assertion, you can't tell if the skill is actually helping. The eval might be a little too easy, or your assertions might just be checking generic correctness instead of the specific superpower your skill provides.

### What if the baseline gets 100%?

When the baseline gets a perfect score, it usually means one of two things:

1. **Your evals are too easy!** The task doesn't need the skill's guidance. Time to crank up the difficulty or make the assertions more specific.
2. **Your skill isn't needed for this task.** The base model already knows how to do it without you. That's super useful information. You can now focus your skill on a narrower or trickier problem.

The benchmark numbers alone can't tell you which one is true. You have to peek at the actual outputs and judge whether your skill is producing noticeably better work.

---

## Start with the Skill's Promise

Before you write a single assertion, try finishing this sentence:

> "My skill helps the agent do ______ better than it would on its own."

Every eval should target that exact gap. If your skill teaches a migration from Mocha to Jest, don't just ask "do the tests pass?" — ask "did the agent replace `sinon.stub()` with `jest.spyOn()` exactly how the skill describes?" 

---

## Anatomy of a Strong Eval

A strong eval has three key ingredients:

1. **A focused prompt** that gives just enough context (don't overwhelm the AI).
2. **Input files** that exercise a real, meaty corner case.
3. **Assertions that check the skill-specific behavior**, not just basic success.

### Let's look at an example prompt:

```json
{
  "prompt": "Migrate evals/fixtures/sinon-sandbox.spec.js from Sinon to Jest. Write the migrated file to outputs/sinon-sandbox.spec.js. Do not run the tests."
}
```

Great assertions for this prompt might be:

- `file_exists:sinon-sandbox.spec.js` (Did it even make the file?)
- `contains_text:sinon-sandbox.spec.js:jest.spyOn(` (Did it use the right tool?)
- `contains_text:sinon-sandbox.spec.js:jest.restoreAllMocks()` (Did it clean up after itself?)
- `The output file does not import sinon or call require('sinon')` (Did it leave the old stuff behind?)
- `jest.spyOn calls appear inside beforeEach, not at module scope` (Did it understand the nuance?)

Notice how those last few assertions verify the *shape* of the migration — the exact behavior your skill is trying to teach.

---

## Assertion Types & When to Use Them

`skill-eval` gives you four built-in assertion styles. Let's mix them wisely.

### 1. Deterministic Matchers

Use these whenever you can. They're blazingly fast, cheap, and never vary between runs.

- `file_exists:<name>` — Checks that an output file was actually produced.
- `contains_text:<file>:<text>` — A simple, reliable substring check.
- `matches_text:<file>:<regex>` — When you need some regex pattern matching magic.

These are your bread and butter for structural checks: imports, function names, config keys, and exact strings.

### 2. LLM Judge Assertions

Use these for subjective or semantic checks that a regex just can't catch.

Examples:
- `"The explanation is clear and actionable."`
- `"The refactor preserves the original behavior."`
- `"The output follows the repository's existing friendly style."`

LLM assertions are super powerful but a bit slower and more expensive. Keep them focused — 1 to 2 per eval is usually perfect. Always pair them with at least one deterministic assertion so a totally empty output doesn't pass by accident.

### 3. Negative Assertions

Don't just assert what *should* be there — assert what *shouldn't*.

- `"The output does not import sinon."`
- `"No placeholder text like 'TODO' or 'Lorem ipsum' appears."`

Negative assertions catch lazy outputs that technically pass a positive check but totally miss the point.

---

## Common Eval-Writing Traps

Watch out for these pesky pitfalls.

### 1. The "Any Correct Answer" Trap

🚫 **Oops:** `"The output is a valid JSON file."`

✅ **Nailed it:** `"The JSON contains a 'users' key with at least three entries, each with an 'id' and 'email'."`

### 2. The "Check the Wrong Thing" Trap

🚫 **Oops:** For a code-migration skill, only checking that the tests pass.

✅ **Nailed it:** Checking that the *old* pattern is gone and the *new* pattern is present in the right place.

### 3. The "Single Golden Path" Trap

If every eval uses the exact same simple input, the model will just coast on prior knowledge. Spice it up!
- Include a tricky edge case.
- Include a deliberately messy file.
- Include a case where the naive answer is subtly wrong.

### 4. The "Forty Assertions" Trap

More assertions don't always mean better signal. A smaller set of sharp, focused assertions beats a giant wall of fuzzy ones. Aim for a sweet spot of 3–7 assertions per eval.

---

## Make Your Evals Fail First

We know this sounds backwards, but it's the absolute fastest way to know they're working.

1. Write your evals and assertions.
2. Run the **baseline** (no skill) and watch them fail beautifully.
3. Run the **with-skill** version and watch more of them turn green!

If your baseline already passes, tighten up those assertions until it doesn't. If you just can't get it to fail, the task might not be a great target for your skill.

---

## Prompt-Writing Tips for Eval Authors

- **Be explicit about output location.** Tell the agent *exactly* where to save its work.
- **Forbid sneaky shortcuts.** Add phrases like "Do not ask questions" or "Do not run the tests" if those would derail your eval.
- **Keep files bite-sized.** Huge inputs blow up tokens and obscure your signal.
- **Use realistic fixtures.** The closer the input looks to real user data, the more useful your results will be.

---

## Iterate on Your Evals Like You Iterate on Your Skill

Evals aren't set-and-forget. As your skill levels up, your evals should too:

- Remove assertions the skill now passes in its sleep.
- Add new ones that stretch the skill even further.
- Delete evals that no longer measure anything meaningful.

A healthy eval suite actually gets harder over time. That's how you know your skill is genuinely improving.

---

## Quick Checklist

Before you run the magic `skill-eval loop` command, ask yourself:

- [ ] Can I explain exactly what gap this eval is measuring?
- [ ] Will the baseline run plausibly fail at least one assertion?
- [ ] Do my assertions check the skill-specific behavior, not just generic correctness?
- [ ] Are at least some assertions deterministic?
- [ ] Did I include a negative assertion to catch lazy outputs?
- [ ] Is the prompt super clear about where outputs should go?

If you can check all six boxes, you're in fantastic shape. Happy evaluating!
