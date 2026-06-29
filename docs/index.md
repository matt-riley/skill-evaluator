---
title: Home
description: skill-eval — automate eval-driven iteration for AI skills. Run, grade, and benchmark your agent outputs.
---

# skill-eval

CLI tool for eval-driven AI skill iteration. Define test cases, run your agent with and without a skill, have an LLM grade the results, and generate benchmark reports.

Inspired by the workflow from [agentskills.io](https://agentskills.io/skill-creation/evaluating-skills).

![Skill Evaluator in action](../assets/cli-demo.gif)

## Quick start

```bash
brew install matt-riley/tools/skill-eval
# or
go install github.com/matt-riley/skill-evaluator@latest

# Scaffold evals in a skill directory
cd your-skill/
skill-eval init

# Run the full eval loop
skill-eval loop

# View the report
skill-eval report
```

Head over to the [Quick Start](/quick-start) guide for a walkthrough, or jump straight to [Writing Evals](/guides/writing-evals).
