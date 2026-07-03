---
title: Activation Evals
description: Test whether your skill's name and description trigger for the right tasks — not just whether the skill body executes well.
---

# 🎯 Activation Evals

Most evals test **execution**: given the skill path, does the agent produce good
output? But there's a second axis to skill quality that no execution eval can
catch: **does the skill's description trigger for the right tasks?**

The Agent Skills spec uses [progressive disclosure](https://agentskills.io/specification):
an agent decides whether to load a skill from its **name and description alone**.
The body is only read *after* activation. This means:

- A skill with a perfect body but a description that **never triggers** is a
  broken skill — no agent will ever load it.
- A skill with an **over-broad** description triggers on unrelated tasks,
  polluting agent context everywhere.

Activation evals close this gap.

---

## How it works

An activation eval has:

```json
{
  "id": 1,
  "type": "activation",
  "prompt": "Help me write a commit message for staged changes",
  "should_activate": true
}
```

- `type: "activation"` — marks this as a discovery eval, not an execution eval.
- `should_activate: true` — the expected verdict (a **positive** case).
- `should_activate: false` — a **negative** case (this task should NOT trigger
  the skill). Omitting the field defaults to `true`.

When you run `skill-eval grade`, the judge is asked: *"Given only this skill's
name and description, would an agent handling this task load it?"* The verdict
is compared against `should_activate` and recorded in `activation.json`.

---

## What the metrics mean

The benchmark reports four counts:

| Outcome | Expected | Verdict | Meaning |
|---------|----------|---------|---------|
| TP | true  | yes | Correctly activated ✓ |
| FP | false | yes  | Over-triggered (description too broad) ✗ |
| FN | true  | no   | Under-triggered (description too narrow) ✗ |
| TN | false | no   | Correctly did NOT activate ✓ |

From these:

- **Precision** = TP / (TP + FP) — of all activations, how many were correct?
  Low precision means the description is too broad.
- **Recall** = TP / (TP + FN) — of all tasks that should trigger, how many did?
  Low recall means the description is too narrow.
- **Accuracy** = (TP + TN) / Total — overall correctness.

The report also lists the **eval IDs** of false positives and false negatives —
these are exactly the cases where the description needs fixing.

---

## Writing activation evals

### Positive cases

Use real tasks that the skill served — the best source is your existing task
evals' prompts. Each positive says: "this task should trigger the skill."

### Negative cases

Negative cases are prompts that share surface vocabulary with the skill's
domain but should NOT trigger it. For example, if your skill helps with Git
commits, a negative case might be "Help me resolve merge conflicts in my
working tree" — Git-adjacent, but not a commit task.

Negative cases catch over-broad descriptions. Without them, a description that
says "helps with Git" would trigger on every Git question, and you'd never
know.

---

## Importing activation evals from agit sessions

The `--as-activation` flag imports prompts as activation positives:

```bash
skill-eval import-agit --as-activation --skill path/to/my-skill --force
```

This strips assertions and expected output — the prompts become discovery
tests, not execution tests. Each imported eval gets `type: "activation"` with
`should_activate` omitted (defaults to true / positive).

### Creating negatives from other repos

A powerful workflow for generating negatives:

1. Record agent sessions in a **different** repo that uses a different skill.
2. Import them as activation positives:
   ```bash
   skill-eval import-agit --as-activation --skill path/to/my-skill --force
   ```
3. Hand-flip the ones that should NOT trigger your skill by adding
   `"should_activate": false` to each relevant eval in `evals.json`.
4. Merge with your existing positives:
   ```bash
   skill-eval import-agit --as-activation --skill path/to/my-skill --merge --session other-repo/session-id
   ```

This gives you realistic negatives from prompts that actually came up in
adjacent work.

---

## Running activation evals

Activation evals are **judged during grade**, not run:

```bash
skill-eval run     # skips activation evals (no agent invocation needed)
skill-eval grade   # judges activation evals + grades task evals
```

The run phase prints a notice: `N activation eval(s) will be judged during grade`.

---

## Judge model notes

Activation verdicts are judge-model-dependent — different models may judge
routing differently. **Pin a consistent judge model** in your config for
comparable numbers across iterations:

```yaml
judge:
  agent: pi
  model: sonnet
```

The activation prompt explicitly asks the judge to consider only the name and
description (progressive disclosure) and to say NO for merely adjacent tasks.
This policy is versioned in the `buildActivationPrompt` function in
`activation.go`.
