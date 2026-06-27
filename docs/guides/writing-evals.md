---
title: Writing Useful Evals
description: How to design evals and assertions that show whether your skill actually helps the model.
---

# 🎯 Writing Useful Evals

Evals are the heart of `skill-eval`. A well-written eval doesn't just check that an agent *can* do a task — it checks whether your **skill makes the agent better at it**. 🚀

This guide shows you how to write evals that produce trustworthy, actionable numbers.

---

## The golden rule ✨

> A good eval is one the **baseline fails more often than the with-skill run**.

If both the baseline and the skill sail through every assertion, you can't tell whether the skill is helping. The eval might be too easy, or your assertions might be checking generic correctness instead of the specific behavior your skill teaches.

### The two possible readings

When the baseline scores 100%:

1. **Your evals are too easy** — the task doesn't need the skill's guidance. Time to make them harder or more specific.
2. **Your skill isn't needed for this task** — the base model already knows how to do it. That's useful information too! Focus your skill on a narrower or harder problem.

The benchmark alone can't tell you which is true. You have to look at the actual outputs and judge whether the skill is producing noticeably better work.

---

## Start with the skill's promise 📝

Before you write a single assertion, finish this sentence:

> "My skill helps the agent do ______ better than it would on its own."

Every eval should target that gap. If your skill teaches a migration from Mocha to Jest, don't just ask "do the tests pass?" — ask "did the agent replace `sinon.stub()` with `jest.spyOn()` in the way the skill describes?"

---

## Anatomy of a strong eval

A strong eval has three parts:

1. **A focused prompt** that gives just enough context.
2. **Input files** that exercise a real corner case.
3. **Assertions that check the skill-specific behavior**, not just success.

Example prompt:

```json
{
  "prompt": "Migrate evals/fixtures/sinon-sandbox.spec.js from Sinon to Jest. Write the migrated file to outputs/sinon-sandbox.spec.js. Do not run the tests."
}
```

Good assertions for this prompt might be:

- `file_exists:sinon-sandbox.spec.js`
- `contains_text:sinon-sandbox.spec.js:jest.spyOn(`
- `contains_text:sinon-sandbox.spec.js:jest.restoreAllMocks()`
- `The output file does not import sinon or call require('sinon')`
- `jest.spyOn calls appear inside beforeEach, not at module scope`

Notice how the last few assertions verify the *shape* of the migration — the very thing the skill is trying to teach.

---

## Assertion types and when to use them 🧰

`skill-eval` gives you four built-in assertion styles. Mix them wisely!

### 1. Deterministic matchers ✅

Use these whenever you can. They're fast, cheap, and don't vary between runs.

- `file_exists:<name>` — Checks that an output file was produced.
- `contains_text:<file>:<text>` — A simple, reliable substring check.
- `matches_text:<file>:<regex>` — When you need pattern matching.

These are great for structural checks: imports, function names, config keys, and exact strings.

### 2. LLM judge assertions 🧠

Use these for subjective or semantic checks that a regex can't capture.

Examples:

- `"The explanation is clear and actionable.`
- `"The refactor preserves the original behavior.`
- `"The output follows the repository's existing style.`

LLM assertions are powerful but slower and more expensive. Keep them focused — 1–2 per eval is usually enough. Always pair them with at least one deterministic assertion so a trivial output can't pass by accident.

### 3. Negative assertions 🚫

Don't just assert what *should* be there — assert what *shouldn't*.

- `"The output does not import sinon.`
- `"No placeholder text like 'TODO' or 'Lorem ipsum' appears.`

Negative assertions catch lazy outputs that technically pass a positive check but miss the point.

---

## Common eval-writing traps 🪤

### 1. The "any correct answer" trap

🚫 **Bad:** `"The output is a valid JSON file.`"

✅ **Better:** `"The JSON contains a 'users' key with at least three entries, each with 'id' and 'email'.`"

### 2. The "check the wrong thing" trap

🚫 **Bad:** For a code-migration skill, only checking that the tests pass.

✅ **Better:** Checking that the old pattern is gone and the new pattern is present in the right place.

### 3. The "single golden path" trap

If every eval uses the same simple input, the model can coast on prior knowledge. Vary the inputs:

- Include an edge case.
- Include a deliberately messy file.
- Include a case where the naive answer is subtly wrong.

### 4. The "forty assertions" trap

More assertions don't always mean better signal. A smaller set of sharp assertions beats a wall of fuzzy ones. Aim for 3–7 assertions per eval.

---

## Make your evals fail first 🧪

This sounds backwards, but it's the fastest way to know they're working:

1. Write your evals and assertions.
2. Run the **baseline** (no skill) and watch them fail.
3. Run the **with-skill** version and watch more pass.

If the baseline already passes, tighten the assertions until it doesn't. If you can't, the task may not be a good target for your skill.

---

## Prompt-writing tips for eval authors 💡

- **Be explicit about output location.** Tell the agent exactly where to save its work.
- **Forbid shortcuts.** Add phrases like "Do not ask questions" or "Do not run the tests" if those would derail the eval.
- **Keep files bite-sized.** Huge inputs blow up tokens and obscure signal.
- **Use realistic fixtures.** The closer the input looks to real user data, the more useful the results.

---

## Iterate on your evals like you iterate on your skill 🔄

Evals aren't set-and-forget. As your skill improves, revisit your evals:

- Remove assertions the skill now trivially passes.
- Add new ones that stretch the skill further.
- Delete evals that no longer measure anything meaningful.

A healthy eval suite gets harder over time — that's how you know your skill is genuinely improving.

---

## Quick checklist ✅

Before you run `skill-eval loop`, ask yourself:

- [ ] Can I explain what gap this eval is measuring?
- [ ] Will the baseline plausibly fail at least one assertion?
- [ ] Do my assertions check the skill-specific behavior, not just correctness?
- [ ] Are at least some assertions deterministic?
- [ ] Did I include a negative assertion to catch lazy outputs?
- [ ] Is the prompt clear about where outputs should go?

If you can check all six, you're in great shape. Happy evaluating! 🎉
