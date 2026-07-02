# Plan 012: Persist run transcripts and mine tool calls into process assertions

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go grader.go eval.go internal/agit/convert.go internal/agit/types.go schema/config-schema.json config.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (writes previously-discarded agent output to disk — secret-hygiene sensitive)
- **Depends on**: — (pairs naturally with Plan 007; no hard dependency)
- **Category**: signal quality
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §4.2 + §2.3

## Why this matters

Two connected gaps:

1. **The judge can only see output files.** `runEval` captures the agent's
   combined stdout/stderr, extracts a token count, and discards it
   (`runner.go:47`, `runner.go:68`). `buildGradingPrompt` includes only
   `outputs/` contents (`grader.go:269-284`). Any assertion about *process*
   — "the agent ran the tests", "the agent did not ask questions" — is
   ungradeable.
2. **The best assertion source is thrown away at import.** agit records
   `tool_calls` per step (`internal/agit/types.go:39-43`), and the recorded
   session shows exactly how the human-approved work was verified
   (`go test ./...`, `npm run lint`). Instead of using that,
   `keyTermFromSummary` (`convert.go:397-426`) guesses a verb phrase from
   the assistant summary and hopes it reappears — the weakest link in
   imported eval quality. `buildAssertionsWithSignals` even has a stub
   admitting this (`convert.go:322-327`).

Fix both with one artifact: persist a redacted transcript per run, add
deterministic `transcript_contains:` / `transcript_matches:` matchers, and
have the importer emit transcript assertions from recorded verification
tool calls.

## Current state

- `runner.go:46-47` — capture point:

```go
	// ponytail: capture combined stdout+stderr — token counts may be on stderr
	output, err := cmd.CombinedOutput()
```

- `runner.go:54-61` — the security stance to honor: full agent output is
  deliberately never *logged* ("may contain API keys, PII, or other
  secrets"). Persisting to the workspace is a different, user-local sink,
  but redaction + opt-out are mandatory.
- `grader.go:163-197` — `parseAssertion` prefix dispatch; the extension point.
- `grader.go:222-263` — `evaluateMatcher` operates on `outDir` +
  `outputContents`; transcript matchers need the config dir (parent of
  `outDir`).
- `internal/agit/convert.go:310-347` — `buildAssertionsWithSignals` has the
  verification-signal stub:

```go
		// If verification signals show verification commands, extract them from
		// the tool_calls context. We can't reconstruct tool_calls here, but we
		// can note the verification signal in the assertion quality metadata.
		if dims.Verification.Signals.VerificationCommands > 0 {
			_ = dims.Verification // used for signal enrichment below
		}
```

- `ConvertSteps` (`convert.go:87-154`) has `step.ToolCalls` in hand
  (fetched via `--include-step-objects`, `client.go:71`) and never reads it.
- `schema/config-schema.json` — config schema; a new `defaults` key must be
  added there or validation (`config.go:85-124`) rejects it.
- `fixEval` (`runner.go:310`) also captures output — same treatment.

## Design decisions (read before coding)

1. **Location**: `transcript.txt` is written to the **config dir**
   (`.../<config>/transcript.txt`), sibling of `outputs/` — NOT inside
   `outputs/`, so `readOutputContents` never feeds it to file matchers or
   balloons the judge prompt, and `file_exists:` assertions can't
   accidentally match it.
2. **Redaction before write**: strip common credential shapes. v1 pattern
   set (case-insensitive, keep conservative):
   `(?i)(api[_-]?key|token|secret|password|authorization|bearer)\s*[:=]\s*\S+`
   → `$1: [REDACTED]`; AWS access keys `AKIA[0-9A-Z]{16}`; PEM blocks
   (`-----BEGIN [^-]+ PRIVATE KEY-----` through END). Cap the file at 1 MB
   (head-truncate with a marker line).
3. **Opt-out**: config `defaults.save_transcript: false` disables writing
   entirely (schema + `DefaultsConfig` + merge). Default **on** — the
   artifact is local to the user's own workspace.
4. **Matchers**: `transcript_contains: <literal>` and
   `transcript_matches: <regex>` — no file component (unlike
   `contains_text:`), since there is exactly one transcript per run.
5. **Judge stays file-based.** LLM assertions do NOT get the transcript
   appended in this plan (prompt-size and injection surface); deterministic
   transcript matchers cover the import use case. Revisit later if needed.
6. **Import mining**: a tool call is a *verification command* if its tool
   name suggests shell execution (`bash`, `shell`, `run`, `exec`,
   `terminal` — substring match, lowercased) and its args match a
   conservative command list: `go test`, `go vet`, `pytest`, `npm test`,
   `npm run lint`, `yarn test`, `cargo test`, `make test`, `make lint`,
   `golangci-lint run`. Emit `transcript_contains: <matched command>` (the
   canonical short form, e.g. `go test`), max 2 per eval, deduped.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `runner.go` (write transcript in `runEval` + `fixEval`), new `redact.go` + `redact_test.go`
- `grader.go` + `eval.go` (two new matcher types)
- `internal/agit/convert.go` (verification-call mining)
- `config.go`, `schema/config-schema.json` (`save_transcript`)
- Tests: `runner_test.go`, `grader_test.go`, `internal/agit/convert_test.go`, `config_test.go`
- `eval-workflow.md` (matcher table), `docs/guides/importing-agit-sessions.md`

**Out of scope**:
- Do NOT log transcript content (the `runner.go:54-61` stance stands).
- Do NOT feed transcripts to the LLM judge prompt.
- Do NOT execute anything from tool calls — mining is string matching only
  (a `command_succeeds:` matcher is deliberately deferred; see research
  §4.3 — it executes untrusted commands and needs its own security design).
- Do NOT attempt perfect secret detection — redaction is best-effort
  defense-in-depth plus an opt-out, documented as such.

## Git workflow

- Branch: `advisor/012-transcript-artifact-toolcall-assertions`
- Commit message style: `feat: persist redacted run transcripts and mine tool calls into transcript assertions`
- Do NOT push unless instructed.

## Steps

### Step 1: Redaction helper

New `redact.go`:

```go
// redactSecrets masks common credential patterns in agent output before it
// is persisted. Best-effort defense in depth — see docs; disable persistence
// entirely with defaults.save_transcript: false.
func redactSecrets(s string) string
```

Compile the three pattern groups from Design decision 2 as package-level
`regexp.MustCompile` vars.

**Verify**: `go test ./... -run TestRedactSecrets` — api_key/token/bearer
forms masked; AWS key masked; PEM block masked; ordinary prose untouched;
idempotent (`redact(redact(x)) == redact(x)`).

### Step 2: Config knob

- `config.go` `DefaultsConfig`: add
  `SaveTranscript *bool \`yaml:"save_transcript"\`` (pointer so absence ≠
  false); `mergeConfig` copies when non-nil; helper
  `func (c *Config) saveTranscript() bool { return c.Defaults.SaveTranscript == nil || *c.Defaults.SaveTranscript }`.
- `schema/config-schema.json`: add `save_transcript` (boolean) under the
  `defaults` properties.

**Verify**: `go test ./... -run 'TestMergeConfig|TestValidateConfig'` —
config with `save_transcript: false` validates and merges.

### Step 3: Write the transcript

In `runEval` after `output, err := cmd.CombinedOutput()` and status
handling (`runner.go:47-62`):

```go
	if cfg.saveTranscript() {
		tPath := filepath.Join(evalDir, configLabel, "transcript.txt")
		if werr := os.WriteFile(tPath, []byte(redactSecrets(truncateBytes(string(output), maxTranscriptBytes))), 0o600); werr != nil {
			logger.Warn("failed to write transcript", "path", tPath, "error", werr)
		}
	}
```

with `const maxTranscriptBytes = 1 << 20` and a `truncateBytes` that
appends `"\n[transcript truncated]"` when cut. Mirror in `fixEval`
(transcript per fix attempt: `fix-<n>/transcript.txt`).

**Verify**: `runner_test.go` — with a stub `CmdBuilder` echoing known
output, `transcript.txt` appears beside `timing.json`, is redacted, and is
absent when config disables it.

### Step 4: Transcript matchers

- `eval.go`: add

```go
	// MatcherTranscriptContains checks the run transcript for a literal substring.
	MatcherTranscriptContains MatcherType = "transcript_contains"
	// MatcherTranscriptMatches checks the run transcript against a regexp.
	MatcherTranscriptMatches MatcherType = "transcript_matches"
```

- `grader.go` `parseAssertion`: two new prefixes; the remainder after the
  prefix is `Arg` (no file component).
- `evaluateMatcher` needs the transcript. Thread it in via
  `gradeFromOutput`: read `filepath.Join(filepath.Dir(outDir), "transcript.txt")`
  once (size-capped), pass as a new parameter to `evaluateMatcher`.
  Missing transcript → FAIL with evidence
  `"no transcript captured (save_transcript disabled or run predates transcripts)"`.
- Evidence on pass quotes a ±80-char window around the match.

**Verify**: `go test ./... -run TestTranscriptMatchers` — contains/absent,
regex valid/invalid, missing file, evidence windows.

### Step 5: Mine verification tool calls at import

In `internal/agit/convert.go`:

```go
// verificationCommands are the conservative command list recognized as
// verification activity in recorded tool calls. Order matters: first match wins.
var verificationCommands = []string{
	"go test", "go vet", "golangci-lint run", "pytest", "npm test",
	"npm run lint", "yarn test", "cargo test", "make test", "make lint",
}

// minedVerificationAssertions returns transcript assertions for recorded
// verification tool calls, capped and deduped.
func minedVerificationAssertions(toolCalls []Tool) []string
```

Wire into `ConvertSteps`: pass `step.ToolCalls` into
`buildAssertionsWithSignals` (signature gains the parameter), which
appends the mined assertions (cap 2, before the trailing LLM assertion).
Replace the dead stub at `convert.go:322-327` with the real call. Tool
`Args` is a string (`types.go:41`) — match with `strings.Contains` on the
lowercased args.

**Verify**: `go test ./internal/agit -run TestMinedVerificationAssertions`
— bash tool with `go test ./...` args → `transcript_contains: go test`;
non-shell tool with same args → nothing; two `go test` calls → one
assertion; `rm -rf` → nothing.

### Step 6: Docs and final checks

- `eval-workflow.md` matcher table: add both prefixes with examples and
  the note that they check the agent's transcript, not output files.
- `docs/guides/importing-agit-sessions.md`: mined verification assertions +
  the redaction/opt-out story (be explicit that redaction is best-effort).

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–5 each carry their tests; the cross-cutting one to add:
`TestGradeMixedWithTranscript` — an eval with `file_exists:`,
`transcript_contains:`, and one LLM assertion grades correctly end-to-end
with a stub judge (extend the existing mixed-matcher test pattern from
Plan 003).

## Done criteria

- [ ] Every run (and fix attempt) writes a redacted, size-capped `transcript.txt` beside `timing.json`; `save_transcript: false` disables it.
- [ ] Transcript content never appears in logs or in the judge prompt.
- [ ] `transcript_contains:` / `transcript_matches:` evaluate deterministically with useful evidence.
- [ ] `import-agit` emits transcript assertions from recorded verification tool calls (cap 2, conservative list).
- [ ] Schema + config validation accept the new key.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Anyone proposes writing the transcript inside `outputs/` — that changes
  judge input and matcher semantics (see Design decision 1).
- The `buildAssertionsWithSignals` signature change ripples beyond
  `convert.go` + its tests + `ConvertSession` fallback.
- Redaction needs to run on outputs too (scope creep — file outputs are
  agent-authored deliverables, different threat model; raise it as a
  separate finding).

## Maintenance notes

- `verificationCommands` is an allowlist by design; additions should be
  commands whose presence in a transcript is near-unambiguous.
- If a `command_succeeds:` matcher lands later, mined assertions should
  upgrade from `transcript_contains:` to it behind a flag.
- Reviewers: check every new matcher keeps the "no reads outside the eval
  directories" invariant from Plan 003's maintenance notes.
