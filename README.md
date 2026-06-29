# skill-eval

[![CI](https://github.com/matt-riley/skill-evaluator/actions/workflows/ci.yml/badge.svg)](https://github.com/matt-riley/skill-evaluator/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/matt-riley/skill-evaluator)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/github/license/matt-riley/skill-evaluator)](./LICENSE)

Eval-driven iteration for AI skills. Define test cases, run an agent with and without a skill, have an LLM grade the output, and benchmark the results.

## Install

```bash
brew install matt-riley/tools/skill-eval
go install github.com/matt-riley/skill-evaluator@latest
```

## Quick start

```bash
cd your-skill/
skill-eval init         # scaffold evals/evals.json
skill-eval loop         # run → grade → benchmark
skill-eval report       # open the HTML report
```

## Docs

Full docs at [skilleval.mattriley.tools](https://skilleval.mattriley.tools).
