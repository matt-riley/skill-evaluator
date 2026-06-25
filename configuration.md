---
title: Configuration
description: Configure skill-evaluator through a global YAML file and per-skill overrides, setting defaults for the run agent, judge agent, and optional model names.
---

# ⚙️ Configuration

We use a simple two-tier YAML setup (with a handy JSON schema for autocomplete):

**🌍 Global** (`~/.config/skill-eval/config.yaml`):
```yaml
defaults:
  agent: pi
  model: claude-sonnet-4-5
judge:
  model: gpt-4o-mini
```

**📁 Per-skill** (`.skill-eval.yaml`): Drop this in your skill root to override global defaults!

Supported agents: `pi`, `claude`, `copilot`, `codex`.

Configs are validated against [`schema/config-schema.json`](https://github.com/matt-riley/skill-evaluator/blob/main/schema/config-schema.json) on load. Invalid files fail immediately with a clear error rather than being silently ignored.
