---
title: Importing Agit Sessions
description: Learn how to magically turn your interactive agit sessions into a brand new evals.json corpus!
---

# 🪄 Turning Sessions into Evals

Writing evals from scratch can take time. But what if you could just *do the work* with your AI assistant and automatically turn that real-world interaction into an evaluation rubric?

Enter `skill-eval import-agit`! 🚀

This magical command looks at a recorded [agit](https://agit.mattriley.tools) session, picks out all the important steps, and transforms them into shiny new evals. It saves the results straight into your `evals/evals.json` file.

## How it Works 🛠️

`skill-eval` looks at what happened during an `agit` session. Every time you give the agent a substantive prompt, the command turns it into an eval where:
- The **prompt** becomes the `prompt` in your eval.
- The **files modified** by the agent become `file_exists` assertions.
- The **final output** helps shape the expected baseline!

## Getting Started

Let's say you just finished an awesome pairing session where the agent did exactly what you wanted. You want to make sure it can *always* do that.

Just run:

```bash
skill-eval import-agit
```

Boom! 💥 It grabs the most recent session, processes the steps, and outputs an `evals.json`.

### Specific Sessions & Overwriting

Need an older session? No problem! Use the `--session` flag:

```bash
skill-eval import-agit --session my-cool-session-123
```

By default, we play it safe and won't overwrite an existing `evals.json`. But if you're ready to replace it, just tell it to `--force`:

```bash
skill-eval import-agit --force
```

### Merging Into Existing Evals

Want to keep your hand-written evals and add imported ones on top? Use `--merge`:

```bash
skill-eval import-agit --merge
```

This preserves your existing `evals.json` and appends new evals with incremented IDs. Great for building up a corpus over multiple sessions.

### Batch Import All Sessions

Got a bunch of recorded sessions? Import them all at once with `--all-sessions`:

```bash
skill-eval import-agit --all-sessions --merge
```

This iterates over every recorded `agit` session, imports each one, and writes a single combined `evals.json`. Combine with `--merge` to layer them on top of existing evals.

### Filtering by Quality Classification

`agit eval` classifies sessions as `good`, `mixed`, `bad`, or `unknown` based on evidence quality signals. Use `--eval-filter` to import only sessions matching a classification:

```bash
skill-eval import-agit --all-sessions --eval-filter good
```

You can pass a comma-separated list to accept multiple classifications:

```bash
skill-eval import-agit --all-sessions --eval-filter good,mixed
```

Sessions are skipped silently if their classification doesn't match the filter, so you get a clean corpus of only the quality you want.

## Customizing the Output 🎨

Want to put the evals somewhere else? Use the `--out` flag to specify exactly where you want that shiny new JSON file:

```bash
skill-eval import-agit --out ./my-custom-evals.json
```

If you aren't currently inside your skill directory, you can also point the tool straight to it using the `--skill`:

```bash
skill-eval import-agit --skill path/to/my-skill
```

## How Quality Signals Work

When `agit eval` is available (agit v1.26+), `import-agit` automatically fetches quality metadata for each session. This enriches the generated evals with:

- **Smart assertions** derived from eval signals (goal clarity, execution focus, failure recovery, verification, completion signal, churn risk)
- **Quality scores** that influence which evals are prioritised when filtering
- **Source metadata** recording the agit origin, session ID, and step hash for traceability

If `agit eval` isn't available or returns no data for a session, the importer falls back gracefully — it still generates evals from the session steps, just without the quality enrichment.

The importer also tries `agit steps --json` (agit v1.26+) for a single-call fetch of all step data with diffs. If that command isn't available (older agit or if it errors), it falls back to the legacy `log` + `show` + `diff` N+1 pattern automatically.

## Next Steps ✨

Once you've imported your evals, open up that `evals.json`! You'll probably want to tweak the generated assertions to make them even sharper. Replace some simple `file_exists` checks with spicy LLM judge assertions, and you'll have an incredible baseline in no time.

Happy importing! 🎉
