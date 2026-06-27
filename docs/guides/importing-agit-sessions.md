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

## Customizing the Output 🎨

Want to put the evals somewhere else? Use the `--out` flag to specify exactly where you want that shiny new JSON file:

```bash
skill-eval import-agit --out ./my-custom-evals.json
```

If you aren't currently inside your skill directory, you can also point the tool straight to it using `--skill`:

```bash
skill-eval import-agit --skill path/to/my-skill
```

## Next Steps ✨

Once you've imported your evals, open up that `evals.json`! You'll probably want to tweak the generated assertions to make them even sharper. Replace some simple `file_exists` checks with spicy LLM judge assertions, and you'll have an incredible baseline in no time.

Happy importing! 🎉
