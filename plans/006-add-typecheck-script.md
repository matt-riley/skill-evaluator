# Plan 006: Add `astro check` typecheck script to catch type errors in CI

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/package.json docs/site/tsconfig.json`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The `tsconfig.json` extends `astro/tsconfigs/strict`, which enables full strict type checking. But the `package.json` scripts have no `check` or `typecheck` command. `oxlint` catches lint issues but not type errors — a mismatched prop type or wrong import in an `.astro` file goes undetected. Adding `"check": "astro check"` gives a one-command CI gate that catches type errors before they ship.

## Current state

- `docs/site/package.json` — scripts block, currently: `dev`, `build`, `preview`, `lint`, `format`, `format:check`, `test`, `test:watch`
- `docs/site/tsconfig.json`:
```json
{
  "extends": "astro/tsconfigs/strict",
  "include": [".astro/types.d.ts", "**/*"],
  "exclude": ["dist"]
}
```

Repo conventions: script names use lowercase (e.g., `format:check` not `formatCheck`). The Go host project has `go vet ./...` as its typecheck analogue — this project should have the equivalent.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Install | `pnpm install` | exit 0 |
| Typecheck (new) | `pnpm check` | exit 0, no errors |
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass |

## Scope

**In scope**:
- `docs/site/package.json` — add `"check": "astro check"` to scripts

**Out of scope**:
- Any change to `tsconfig.json`
- Fixing any type errors that `astro check` reveals (if there are pre-existing errors, that's a separate plan)
- Any other script additions

## Git workflow

- Branch: `advisor/006-add-typecheck-script`
- Commit message: `chore(docs): add astro check typecheck script`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add `"check"` script to package.json

In `docs/site/package.json`, add `"check": "astro check"` to the `"scripts"` block. Insert it alphabetically between `"build"` and `"dev"`:

```json
"check": "astro check",
```

**Verify**: `grep '"check"' docs/site/package.json` → matches `"check": "astro check"`.

### Step 2: Run the new typecheck

**Verify**: `cd docs/site && pnpm check` → exit 0, no errors.

If `astro check` reports type errors that pre-exist in the codebase, record them and STOP — don't fix them here, they're out of scope.

### Step 3: Confirm existing scripts still work

**Verify**:
- `pnpm build` → exit 0
- `pnpm test` → exit 0, all 3 tests pass
- `pnpm lint` → exit 0

## Test plan

No new tests. This is a configuration change. The existing test suite must continue to pass.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `pnpm check` exits 0
- [ ] `pnpm build` exits 0
- [ ] `pnpm test` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `grep '"check": "astro check"' docs/site/package.json` returns a match
- [ ] No files outside the in-scope list modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- The `scripts` block in `package.json` doesn't match the expected shape (unexpected keys, different ordering) — the executor should insert `"check"` adjacent to the existing block structure.
- `astro check` reports pre-existing type errors that require source changes (out of scope).
- Any existing script (`build`, `test`, `lint`) fails after the change.

## Maintenance notes

- If a CI pipeline is added later, include `pnpm check` in the lint/test gate.
- `astro check` depends on `astro` being installed (it is, via `dependencies`). If `astro` is ever moved to devDependencies, this script still works in CI where devDeps are installed.
