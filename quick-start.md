---
title: Quick Start
description: Get started with skill-evaluator by installing the CLI, scaffolding your first skill, defining evals, and running with-skill versus baseline comparisons.
---

# 🌟 Quick Start

Getting started is a breeze. Just run these commands:

```bash
# Install the CLI tool
go install github.com/matt-riley/skill-evaluator@latest

# Set up your global config (just once!)
skill-eval init --global

# Navigate to your skill directory and scaffold your first evals
skill-eval init
# (Go ahead and edit evals/evals.json with your test cases)

# Run the full evaluation loop!
skill-eval loop
```
