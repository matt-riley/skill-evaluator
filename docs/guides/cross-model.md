---
title: Cross-Model Benchmarking
description: Run your evals against multiple agents at once — pi, claude, copilot — and see which one your skill helps the most. Spot weak spots before your users do!
---

# 🔬 Cross-Model Benchmarking

Your skill works great in **pi**. But does it help **claude**? What about **copilot**? If you're building a skill meant to be runtime-agnostic, you need to know it pulls its weight *everywhere*.

Cross-model benchmarking runs every eval against every agent you specify — in one command. You get a clean per-model breakdown so you can spot exactly where to focus your tuning.

---

## How it works 🎯

Pass `--models` with a comma-separated list of `agent:model` pairs:

```bash
skill-eval loop --models pi:claude-sonnet,claude,copilot
```

skill-eval runs the full `run → grade → benchmark` cycle for each model. Each one gets its own with-skill and baseline run, and the results land side-by-side in `benchmark.json`.

<video src="/guides/cross-model.mp4" controls muted width="100%" class="rounded-3xl border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]" />

---

## What you get 📊

The benchmark tells you at a glance who's winning:

```json
{
  "models": {
    "pi-claude-sonnet": { "delta": { "pass_rate": 0.33 } },
    "claude":           { "delta": { "pass_rate": 0.17 } },
    "copilot":          { "delta": { "pass_rate": 0.33 } }
  },
  "best_model":  "pi-claude-sonnet",
  "worst_model": "claude"
}
```

Three signals jump out:

1. **`best_model` / `worst_model`** — no scanning required, skill-eval tells you
2. **Per-model delta** — is your skill helping pi more than claude? You'll see it
3. **The gap** — if one model's delta is half the others, that's where to tune your skill next

---

## The iteration loop 🔄

Here's the real power move. Run with `--models` across iterations:

```
Run 1: pi +33%,  claude +17%,  copilot +33%
       → Claude's weak at chart-related assertions. Tweak your SKILL.md.

Run 2: pi +34%,  claude +28%,  copilot +32%
       → Claude improved! Copilot slipped slightly. Another tweak.

Run 3: pi +34%,  claude +31%,  copilot +36%
       → All above 30%. Converging nicely. Ship it! 🚀
```

Over time, you'll spot patterns in how different models interpret your skill instructions — and learn to write skills that work well *universally*.

---

## Composing with `--fix` 🪄

Cross-model and auto-fix work together seamlessly:

```bash
skill-eval loop --models pi,claude --fix --max-fix-attempts 2
```

Each model gets its own independent fix phase. If pi passes on the first attempt but claude needs two fix rounds before converging, you'll see that in the output — and in the benchmark. That tells you claude's baseline needs more attention than pi's.

---

## Config for persistence 🛠️

Tired of typing the same `--models` every time? Set it once in your config:

```yaml
# .skill-eval.yaml
models:
  - agent: pi
    model: claude-sonnet-4-5
  - agent: claude
  - agent: copilot
```

Config models become the default for every `run`, `grade`, and `loop` command. CLI `--models` always wins when both are present.

> ⚠️ **Cost warning!** skill-eval counts your total agent invocations ahead of time. If you're about to launch 18 runs (3 evals × 3 models × 2 configs), it'll ask for confirmation first. No surprise bills!

---

## What's next?

- **[Auto-Fixing](/guides/auto-fixing/)** — Combine cross-model with auto-refinement for the ultimate iteration loop.
- **[Reading Your Results](/guides/reading-results/)** — Make sense of all those deltas and per-eval breakdowns.
- **[Eval Workflow](/eval-workflow/)** — The full picture of how everything connects.
