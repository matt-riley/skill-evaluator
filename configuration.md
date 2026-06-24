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
