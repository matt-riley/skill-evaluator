# Plan 002: Validate `.skill-eval.yaml` against the existing JSON schema

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat c6dac63..HEAD -- config.go schema/config-schema.json go.mod go.sum`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: plans/001-go-test-coverage.md
- **Category**: robustness
- **Planned at**: commit `c6dac63`, 2026-06-25

## Why this matters

`LoadConfig` in `config.go` unmarshals YAML but never validates it. A typo like `defautls:` or an unsupported agent name (`agenta: bad`) is silently ignored or passed straight to the agent CLI, producing confusing runtime errors. A config schema already exists at `schema/config-schema.json`; v1 should use it and fail fast with a clear message when config is invalid.

## Current state

- `config.go:37` — `LoadConfig` currently does only `yaml.Unmarshal` and field-by-field merge. It returns no validation errors for bad values.
- `schema/config-schema.json` exists and defines `agent` enums (`pi`, `claude`, `copilot`, `codex`) and required `defaults.agent`.
- `go.mod` is on Go 1.26.4; the `embed` package is available.

Relevant excerpt from `config.go:37-58`:

```go
func LoadConfig(skillDir string) (*Config, error) {
	cfg := &Config{
		Defaults: DefaultsConfig{Agent: "pi"},
		Judge:    JudgeConfig{Agent: "pi"},
	}

	// Load global config
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".config", "skill-eval", "config.yaml")
		if data, err := os.ReadFile(globalPath); err == nil {
			var globalCfg Config
			if err := yaml.Unmarshal(data, &globalCfg); err != nil {
				return nil, fmt.Errorf("global config %s: %w", globalPath, err)
			}
			mergeConfig(cfg, &globalCfg)
		}
	}
	// ... skill-level config block is identical pattern
}
```

Relevant excerpt from `schema/config-schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "properties": {
    "defaults": {
      "properties": {
        "agent": { "enum": ["pi", "claude", "copilot", "codex"] },
        "model": { "type": "string" }
      },
      "required": ["agent"]
    }
  }
}
```

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Format    | `gofmt -w *.go`          | exit 0              |
| Lint      | `golangci-lint run`      | exit 0              |
| Tests     | `go test ./...`          | exit 0              |

## Suggested executor toolkit

- Use `go-build-and-test` skill if available for any test failures.
- JSON Schema validator: `github.com/santhosh-tekuri/jsonschema/v6` supports draft 2020-12.

## Scope

**In scope**:
- `config.go`
- `schema/config-schema.json`
- `go.mod` / `go.sum` (dependency add only)

**Out of scope**:
- Do NOT change the YAML format itself or force users to JSON.
- Do NOT validate `evals/evals.json` in this plan.
- Do NOT change how unknown fields are handled in the config structs beyond schema enforcement.

## Git workflow

- Branch: `advisor/002-config-schema-validation`
- Commit message style: `feat: validate config against JSON schema`.
- Do NOT push unless instructed.

## Steps

### Step 1: Add JSON Schema validator dependency

Run:

```bash
go get github.com/santhosh-tekuri/jsonschema/v6
```

**Verify**: `go.mod` and `go.sum` updated; `go build ./...` still passes.

### Step 2: Embed the schema

At the top of `config.go`, add:

```go
import _ "embed"

//go:embed schema/config-schema.json
var configSchemaJSON []byte
```

**Verify**: `go build ./...` passes.

### Step 3: Add validation helper

Add a function `validateConfigYAML(data []byte, path string) error` in `config.go` that:

1. Unmarshals `data` into `map[string]any` using `yaml.Unmarshal`.
2. Marshals that map to JSON bytes.
3. Compiles the embedded schema with `jsonschema/v6`.
4. Validates the JSON and returns an error formatted as `config %s: <validation error>`.

Call this helper for both global and skill configs after reading the file and **before** the `yaml.Unmarshal` into `Config`.

If validation fails, return immediately.

**Verify**: code compiles.

### Step 4: Strengthen the schema (optional but recommended)

Add `"additionalProperties": false` to the root object, `defaults`, `judge`, and each item in `models` so typos fail fast. Also mark `models` as an array of objects with `agent` (required) and `model`.

**Verify**: existing `.skill-eval.yaml` in the repo (if any) still validates. If the repo does not contain one, create a temporary one for manual testing.

### Step 5: Add tests

In `config_test.go` (created in Plan 001), add tests for `LoadConfig` validation:

- Valid config loads successfully.
- Unknown agent name returns error mentioning the invalid value.
- Missing `defaults.agent` returns error.
- Unknown top-level key returns error after `additionalProperties: false`.

Use `t.TempDir()` and `os.WriteFile` to write temp config files.

**Verify**: `go test ./... -run TestLoadConfigValidation` → pass (and deliberately failing cases are caught).

### Step 6: Update docs

In `configuration.md`, add a paragraph stating that config is validated against `schema/config-schema.json` on load and invalid configs fail immediately.

**Verify**: docs build still passes (`cd docs/site && pnpm build`).

### Step 7: Final checks

Run:

```bash
gofmt -w *.go
go test ./...
go vet ./...
golangci-lint run
cd docs/site && pnpm build
```

All should pass.

## Test plan

- Extend `config_test.go` from Plan 001.
- Cover happy path + three invalid cases.
- Do not test the JSON Schema library itself; test the integration in `LoadConfig`.

## Done criteria

- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Invalid config files cause `LoadConfig` to return a clear error.
- [ ] `golangci-lint run` exits 0.
- [ ] `docs/site pnpm build` exits 0.
- [ ] `schema/config-schema.json` still describes the current config shape.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The schema file is draft 2020-12 and the chosen validator cannot parse it.
- Validation of a known-good global config fails (check `~/.config/skill-eval/config.yaml` if it exists).
- The only way to validate unknown fields requires breaking existing valid configs.

## Maintenance notes

- Future config keys must be added to both the Go structs and `schema/config-schema.json`. A reviewer should check both.
- If the schema drifts from the structs, the tests added here are the early warning.
- This pairs with Plan 001; run it after or alongside tests so validation behavior is covered.
