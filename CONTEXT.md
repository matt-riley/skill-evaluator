# 📚 Skill Evaluator Context

Welcome to the context guide for the **Skill Evaluator**! This is a standalone, provider-agnostic CLI tool that automates your eval-driven skill iteration loop. It handles everything from defining test cases and spawning agent runs, to LLM-grading and aggregating your benchmarks. 

To keep everyone on the same page, we use a specific, friendly vocabulary. Here's a quick guide! 📖

---

## 🗣️ Our Language Guide

### **Skill** 🛠️
A set of instructions and optional scripts that guide an LLM agent's behavior, beautifully packaged in a `SKILL.md` file.
> *Let's avoid: plugin, extension, prompt*

### **Eval** 🧪
A single test case! It includes a prompt, an expected output description, optional input files, and some assertions.
> *Let's avoid: test, test case, scenario*

### **Assertion** ✅
A verifiable pass/fail statement that checks exactly what an eval's output should contain or achieve.
> *Let's avoid: check, validation rule*

### **Run** 🏃
A single execution of an eval by an agent. This produces outputs, timing data, and token counts.
> *Let's avoid: execution, invocation*

### **Grading** 🎓
The process of evaluating each assertion against the actual run outputs (giving a PASS/FAIL with evidence). This is usually done by our trusty LLM judge!
> *Let's avoid: scoring, evaluation (we save that word for the whole process!)*

### **Benchmark** 📊
Your aggregated statistics! This includes the mean pass rate, time, tokens, and standard deviations across all runs in an iteration, complete with deltas.
> *Let's avoid: score, report*

### **Iteration** 🔄
One full, glorious pass through all your evals (run, grade, benchmark). This creates a shiny new `iteration-N/` directory.
> *Let's avoid: cycle, round*

### **Workspace** 🗂️
The cozy home for all your eval results! It's organized by iteration and eval, keeping all your outputs, timings, and feedback tidy.
> *Let's avoid: results directory, output directory*

### **Feedback** 📝
Your personal, human-written notes on eval outputs. This captures all the qualitative nuances that assertions might miss.
> *Let's avoid: review notes, comments*

### **Agent Runtime** 🤖
The external CLI tool that actually executes your runs (like `pi`, `claude`, `copilot`, or `codex`). We shell out to it to keep things flexible!
> *Let's avoid: provider, backend, executor*

### **Global Config** 🌍
Your user-wide defaults for the agent runtime and model, safely stored at `~/.config/skill-eval/config.yaml`.
> *Let's avoid: user config*

### **Skill Config** 🎯
Your per-skill overrides, found in `.skill-eval.yaml`. These lovingly override the global defaults just for that specific skill.
> *Let's avoid: project config, local config*

### **Judge** ⚖️
The LLM configuration used for grading your assertions. We usually pick a cheaper/faster model for this since grading is a read-only task!
> *Let's avoid: grader, evaluator*

### **Run Failure** 💔
A run that sadly produced no outputs (like a crash or timeout). It's recorded as `status: "failed"` and counts as a 0% pass rate, but don't worry—it won't abort your iteration!
> *Let's avoid: error, crash (those are causes, not results!)*

### **Baseline** 📏
The comparison point for a run! It can be either no-skill (the default) or a snapshot of a previous skill version. 
> *Let's avoid: without_skill, old_skill, control*

### **Snapshot** 📸
A quick copy of a skill directory taken right before an iteration begins. It serves as the baseline for your next iteration!
> *Let's avoid: backup, clone*

---

## 🔗 How Everything Connects

- A **Skill** is evaluated across multiple **Evals**.
- An **Eval** produces two **Runs** (with-skill and your baseline).
- A **Run** is **Graded** to produce assertion results.
- **Grading** results are bundled up into a **Benchmark**.
- An **Iteration** holds **Runs**, **Grading**, and a **Benchmark**.
- **Feedback** is lovingly attached per **Eval** inside an **Iteration**.
- And everything lives happily together in a **Workspace**! 🏡

---

## 🤔 Flagged Ambiguities

- None yet! We're perfectly clear. ✨
