# Plan 009: Add `skill-eval validate` — lint SKILL.md against the Agent Skills spec

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- main.go cmd_init.go config.go go.mod`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S-M
- **Risk**: LOW (new command; only additive touch points elsewhere)
- **Depends on**: —
- **Category**: direction (spec alignment)
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §3.1

## Why this matters

The tool exists to help authors write top-quality skills per the
[Agent Skills spec](https://agentskills.io/specification), yet its only
contact with the spec is an existence check on `SKILL.md`
(`cmd_init.go:28-31`). The most common real-world skill failure is not a
bad instruction — it is a skill that **never loads**: a `name` that doesn't
match its directory, malformed frontmatter, or a `description` that never
triggers activation. Those are deterministic, checkable, and free to catch
before a single judge token is spent. A `validate` subcommand (also run as
a non-fatal preflight inside `run`) closes that gap.

## Current state

- `cmd_init.go:27-31` — the entire spec surface today:

```go
	// Verify we're in a skill directory
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		return fmt.Errorf("SKILL.md not found — run from a skill directory")
	}
```

- `main.go:36-57` — subcommand dispatch switch; `printUsage` at
  `main.go:60-91` lists commands. Both need a new entry.
- `go.mod` already depends on `gopkg.in/yaml.v3` (used by `config.go:13`),
  so frontmatter parsing needs no new dependency.
- `runner.go:171-189` — `detectSkillDir` walks upward to find `SKILL.md`;
  reuse it.

## Spec rules to enforce (from agentskills.io/specification)

Errors (exit non-zero):
- **E1** Frontmatter block present: file starts with `---\n`, has a
  closing `---`, and the block parses as YAML.
- **E2** `name` present, non-empty.
- **E3** `name` format: lowercase letters, digits, hyphens only
  (`^[a-z0-9]+(-[a-z0-9]+)*$`), max 64 chars (this regex also rejects
  leading/trailing hyphens).
- **E4** `name` equals the containing directory's base name (skills
  silently fail to load otherwise).
- **E5** `description` present, non-empty, max 1024 chars.
- **E6** `compatibility`, when present, max 500 chars.
- **E7** `metadata`, when present, is a map with string values.
- **E8** Unknown frontmatter keys outside the spec set
  {`name`, `description`, `license`, `compatibility`, `metadata`,
  `allowed-tools`, `version`} → error listing them (typos like
  `descripton` should not pass silently). If the spec set drifts, this is
  the one list to update.

Warnings (printed, exit zero):
- **W1** `description` lacks trigger phrasing — heuristic: contains none of
  `use when`, `use this when`, `use for`, `when the user`, `when you` (case-insensitive).
  The spec requires the description to state *what* the skill does **and
  when to use it**; this is the activation surface.
- **W2** Body (content after frontmatter) exceeds 500 lines — spec guidance
  is to keep SKILL.md lean and push detail into `references/`.
- **W3** A relative path referenced by the body does not exist. Detection:
  markdown link targets `](<path>)` plus inline-code tokens matching
  `^(scripts|references|assets)/` — resolve against the skill dir.
- **W4** `license` absent (recommended, not required).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Manual | `go run . validate --skill <dir>` | findings printed, exit code per severity |

## Scope

**In scope**:
- New `cmd_validate.go` + `cmd_validate_test.go`
- `main.go` (dispatch case + usage text)
- `cmd_run.go` (non-fatal preflight call)
- New guide `docs/guides/validating-skills.md`; mention in `quick-start.md`

**Out of scope**:
- Do NOT validate `evals.json` here (config validation already exists via
  the JSON schema; eval-file linting is a separate concern).
- Do NOT fetch or embed the remote spec — the rules above are hardcoded
  with a comment linking the spec version/date.
- Do NOT make `run` fail on validation errors in this plan (warn only);
  flipping to hard-fail is a follow-up decision for the user.
- Do NOT lint SKILL.md prose quality with an LLM (that is `report
  --llm-suggestions` territory).

## Git workflow

- Branch: `advisor/009-skill-md-validate-command`
- Commit message style: `feat: add validate subcommand linting SKILL.md against the agentskills spec`
- Do NOT push unless instructed.

## Steps

### Step 1: Finding model and frontmatter parser

Create `cmd_validate.go`:

```go
// Finding is a single validation result.
type Finding struct {
	Rule     string // e.g. "E3", "W1"
	Severity string // "error" | "warning"
	Message  string
}

// skillFrontmatter is the spec's frontmatter schema.
// Spec: https://agentskills.io/specification (rules snapshot 2026-07-01).
type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
	Version       string            `yaml:"version"`
}

// parseSkillMD splits SKILL.md into frontmatter and body.
func parseSkillMD(data []byte) (fm map[string]any, body string, err error)
```

Parse the frontmatter **twice**: once into `map[string]any` (for E8
unknown-key detection — compare keys against the allowed set) and once into
`skillFrontmatter` (for typed checks). Frontmatter extraction: content
between the first line `---` and the next line that is exactly `---`;
anything else is an E1 error.

**Verify**: `go test ./... -run TestParseSkillMD` (frontmatter present /
missing / unterminated / non-YAML).

### Step 2: Rule checks

```go
// validateSkill runs all spec checks against a skill directory.
func validateSkill(skillDir string) ([]Finding, error)
```

Implement E1–E8 and W1–W4 exactly as specified above. Keep each rule a
small function returning `[]Finding` so the test table maps 1:1 to rules.
For W3, cap scanning at the first 200 path candidates and ignore URLs
(`://`) and absolute paths.

**Verify**: `go test ./... -run TestValidateSkill` (table per rule, see
test plan).

### Step 3: The subcommand

```go
func cmdValidate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	skillFlag := fs.String("skill", "", "Skill directory (default: detect upward)")
	quiet := fs.Bool("quiet", false, "Print errors only")
	// ...
}
```

- Resolve dir via `--skill` or `detectSkillDir()`.
- Print findings as `ERROR [E4] name "foo" does not match directory "bar"` /
  `WARN  [W2] ...`, errors first.
- Exit contract: errors present → return an error (CLI exits 1 via
  `main.go:17-20`); warnings only → print and return nil.
- Success with no findings prints `SKILL.md OK (name: <name>)`.

Wire into `main.go`: `case "validate": return cmdValidate(ctx, args)` and a
usage line `skill-eval validate            Lint SKILL.md against the Agent Skills spec`.

**Verify**: `go run . validate` inside a scratch skill dir shows expected
output for a good and a broken SKILL.md.

### Step 4: Preflight in `run`

In `cmd_run.go`, after `detectSkillDir()` succeeds (line ~32), call
`validateSkill(skillDir)` and print findings as warnings prefixed
`preflight:` — never fail the run. Skip entirely when zero findings.

**Verify**: `go test ./... -run TestRunPreflight` — a run against a skill
with a bad name prints the preflight warning (stub `CmdBuilder` so no real
agent is invoked; follow existing `runner_test.go` patterns).

### Step 5: Tests

Table-driven `cmd_validate_test.go`, one temp skill dir per case:

- E1: no frontmatter; unterminated frontmatter; tabs-in-YAML parse error.
- E2/E5: missing name; missing description; empty strings.
- E3: uppercase, underscore, leading hyphen, trailing hyphen, 65 chars → error;
  `csv-analyzer`, `a`, 64 chars → pass.
- E4: name ≠ dir base → error; equal → pass.
- E5/E6: 1025-char description, 501-char compatibility → error.
- E7: metadata with nested map value → error; flat string map → pass.
- E8: `descripton` (typo) → error naming the key.
- W1: description without trigger phrasing → warning; with "Use when…" → none.
- W2: 501-line body → warning.
- W3: body referencing `references/missing.md` → warning; existing file → none.
- Exit contract: errors → non-nil return; warnings only → nil.

### Step 6: Documentation

- `docs/guides/validating-skills.md`: what is checked, the E/W rule table,
  spec link, and the note that `run` performs the same checks as warnings.
- `quick-start.md`: add `skill-eval validate` after `init` in the flow.

### Step 7: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

All pass.

## Test plan

See Step 5 — the rule table is the test table. Additionally one
integration-style test running `validateSkill` against this repo's own
docs fixtures if any exist (skip if not).

## Done criteria

- [ ] `skill-eval validate` exists, is listed in usage, and exits non-zero on any E-rule violation.
- [ ] All E1–E8 and W1–W4 rules implemented with per-rule tests.
- [ ] `skill-eval run` prints the same findings as non-fatal preflight warnings.
- [ ] Rules cite the spec URL and snapshot date in a code comment.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] New guide added; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The live spec at agentskills.io materially disagrees with the E-rules
  above (e.g. field renamed, limits changed) — update the rule list in this
  plan first, then implement.
- Frontmatter in real-world skills commonly carries extra runtime-specific
  keys (e.g. Claude Code's `disable-model-invocation`) such that E8 as an
  *error* would break valid skills — downgrade E8 to a warning and note why.
- `detectSkillDir` semantics need changing to support `--skill` (they
  shouldn't — the flag bypasses detection).

## Maintenance notes

- The allowed-key set (E8) is the single point to update when the spec
  evolves; keep the spec snapshot date comment current.
- Plan 013 (activation evals) reuses `parseSkillMD` to extract
  name/description — keep that function exported-in-package and free of
  CLI concerns.
- Follow-up candidate: `validate --json` for CI consumption.
