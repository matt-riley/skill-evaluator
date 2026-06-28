# Repository Guidelines

## Project Structure & Module Organization
This repository is a single Go module (`github.com/matt-riley/skill-evaluator`) with the CLI entrypoint and core logic at the repo root.

- `main.go`: subcommand dispatch for `init`, `run`, `grade`, `benchmark`, and `loop`
- `config.go`, `runner.go`, `grader.go`, `benchmark.go`, `workspace.go`, `eval.go`: config loading, agent execution, grading, aggregation, and shared types
- `schema/config-schema.json`: shared JSON Schema for config validation
- `docs/adr/`: architecture decisions
- `CONTEXT.md`: project-specific operational notes

Generated eval artifacts live outside the skill directory in `<skill-name>-workspace/iteration-N/...`.

## Build, Test, and Development Commands
- `go build -o skill-eval .`: build the local CLI binary
- `go install github.com/matt-riley/skill-evaluator@latest`: install from source
- `go test ./...`: run the test suite; this repo currently has no `*_test.go` files
- `go vet ./...`: run Go static analysis

## Coding Style & Naming Conventions
Use standard Go formatting and idioms:

- Run `gofmt` on all Go files before committing.
- Prefer short, descriptive names for packages, funcs, and files.
- Keep config keys and workspace directories consistent with existing names such as `with_skill`, `baseline`, and `iteration-N`.
- Do not introduce new abstractions unless they reduce real duplication or match an existing pattern.

## Testing Guidelines
There is no established automated test suite yet. When adding coverage, prefer table-driven Go tests in `*_test.go` files alongside the code they exercise.

Test names should describe behavior, for example `TestMergeConfig` or `TestWorkspacePath`. Run focused tests with `go test ./... -run '^TestName$'`.

## Commit & Pull Request Guidelines
The history currently shows only an initial commit (`feat: initial commit`), so no strong convention is established. Use clear, imperative commit messages; conventional commits are acceptable if kept consistent.

Pull requests should include:

- a short summary of the change and why it exists
- commands used to verify it
- notes about config, workspace, or schema changes
- screenshots only if you change user-facing CLI output or docs examples

## Security & Configuration Tips
Global config lives at `~/.config/skill-eval/config.yaml`; per-skill overrides live in `.skill-eval.yaml` beside the skill. Treat generated output and agent responses as untrusted input, and keep the JSON output contract strict.
