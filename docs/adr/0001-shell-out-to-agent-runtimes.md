# Shell out to agent runtimes rather than embedding an LLM SDK

The tool orchestrates the eval loop by shelling out to existing agent CLIs (`pi`, `claude`, `copilot`, `codex`) rather than embedding its own LLM client library. The alternative was to include an OpenAI-compatible SDK and manage API keys internally, giving the tool full control over model selection, retries, and structured output parsing.

We chose shelling out because: (a) skills are authored for specific agent runtimes — evaluating them in the actual runtime they're designed for produces faithful results, (b) users already have these CLIs installed and authenticated, so the tool has zero credential management, (c) the tool stays a thin orchestrator (~hundreds of lines) rather than an AI client product (~thousands of lines). The trade-off is that the tool depends on the agent CLI's output format for extracting timing and token data, which may vary across runtimes.

**Status**: accepted
