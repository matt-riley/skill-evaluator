---
title: Eval Workflow
description: Learn the skill-evaluator workflow, from writing eval prompts and assertions to running agent tasks, grading outputs with an LLM judge, and iterating.
---

# 🎢 Your Eval Workflow

### 1. Design your test cases 🎨

Running `skill-eval init` creates a handy `evals/evals.json` file in your skill directory. Pop open that file and add 2-3 realistic prompts. Don't worry about making it perfect yet—just get a feel for how it works!

```json
{
  "skill_name": "csv-analyzer",
  "evals": [
    {
      "id": 1,
      "prompt": "I have a CSV of monthly sales data in data/sales_2025.csv. Can you find the top 3 months by revenue and make a bar chart?",
      "expected_output": "A bar chart image showing the top 3 months by revenue, with labeled axes and values.",
      "files": ["evals/files/sales_2025.csv"]
    }
  ]
}
```

**💡 Tips for awesome prompts:**
- **Mix it up!** Try casual phrasing ("clean this up") and precise phrasing.
- **Edge cases matter!** Throw in a tricky edge-case prompt to see how your skill handles it.
- **Keep it real!** Mention real file paths, column names, and context.
- **Start small!** Just 2-3 test cases are plenty for your first loop.

### 2. Write your assertions ✅

Assertions are simple pass/fail statements about what your output should look like. It's best to add these *after* your first run, so you know what "good" actually looks like!

```json
"assertions": [
  "The output includes a bar chart image file",
  "The chart shows exactly 3 months",
  "Both axes are labeled",
  "The chart title or caption mentions revenue"
]
```

Keep your assertions specific and observable (like "a file named results.csv exists"). Try to avoid vague statements ("the output is good") or super brittle ones ("it says exactly 'Total Revenue: $X'").

### Deterministic assertion matchers 🤖

For common checks, you can use prefix-based matchers that are evaluated locally instead of being sent to the LLM judge. These are faster, cheaper, and give consistent verdicts:

```json
"assertions": [
  "file_exists: results.csv",
  "contains_text: summary.txt:Total revenue",
  "matches_text: output.md:^## Summary",
  "The chart uses a sensible color palette and is visually clear"
]
```

Supported prefixes:

| Prefix | Example | What it checks |
|--------|---------|----------------|
| `file_exists:` | `file_exists: results.csv` | A file was produced in the output directory. |
| `contains_text:` | `contains_text: summary.txt:Total revenue` | A file contains the given literal text. |
| `matches_text:` | `matches_text: output.md:^## Summary` | A file matches the given regular expression. |

Any assertion without a prefix is sent to the LLM judge unchanged, so you can mix deterministic checks with open-ended judgement in the same eval.

### 3. Run the loop! 🔄

Ready? Let's go!

```bash
skill-eval loop
```

This magic command handles the whole cycle for you:
1. **Run:** Executes every eval twice (once with your skill, once as a baseline). It saves the outputs to `outputs/` and notes the timing.
2. **Grade:** Asks the judge agent to check your assertions against the outputs, generating a nice `grading.json` with PASS/FAIL verdicts.
3. **Benchmark:** Gathers all the stats into `benchmark.json` so you can easily see if your skill is pulling its weight!

> 💡 **Want the tool to fix failures automatically?** Try `skill-eval loop --fix` and watch it re-run failing evals with the judge's feedback as a critique! It'll keep refining until things pass or it hits the attempt limit. Perfect for those "almost there" situations.

> 🔬 **Shipping to multiple runtimes?** `skill-eval loop --models pi:claude-sonnet,claude:opus-4-8,copilot` runs every eval against each agent and tells you which one your skill helps most. Great for runtime-agnostic skills!

### 4. Review the results 🔍

Take a peek inside your workspace. For each eval, you'll find:

| Artifact | What it tells you |
|----------|------------------|
| `outputs/` | The actual files generated—open them up and take a look! |
| `grading.json` | Which assertions passed or failed, along with the judge's reasoning. |
| `benchmark.json` | All your stats and deltas in one place. |

**Don't forget to add your feedback!** You can leave notes in `feedback.json`:

```json
{
  "eval-1": "The chart is missing axis labels and the months are in alphabetical order instead of chronological.",
  "eval-2": ""
}
```
Specific feedback is super helpful; vague feedback, not so much. If you leave it empty, it means everything looked great!

### 5. Spot the patterns 🧩

After grading, dive into the details:
- **Toss out assertions that always pass:** They aren't telling you anything new!
- **Fix assertions that always fail:** The test might be too hard, or the assertion might be a bit off.
- **Celebrate assertions that pass *only* with your skill:** This is where your skill shines! Figure out which instructions made the magic happen.
- **Check the outliers:** If something took way too long, read the transcript to see where it got stuck.

### 6. Keep iterating! 🚀

Now you have the three golden signals: failed assertions, your feedback, and execution transcripts. Use them to tweak your skill!

Once you've made updates, run the loop again comparing against your previous version:

```bash
skill-eval loop --baseline previous
```

This creates an `iteration-2/` directory. Keep iterating until you're thrilled with the results and your feedback is completely empty!
