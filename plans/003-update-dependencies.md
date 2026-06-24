# Plan 003: High-severity vulnerabilities in build dependencies

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 8b20fcd..HEAD -- docs/site/package.json docs/site/pnpm-lock.yaml`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `8b20fcd`, 2026-06-24

## Why this matters

The `pnpm audit` command currently reports 15 vulnerabilities (6 high, 5 moderate, 4 low), primarily in `undici` and `esbuild`. These are transitively imported by `wrangler` and `vitest` in the `docs/site` package. While these dependencies are strictly for build/dev-time, leaving known high-severity CVEs in the lockfile risks supply-chain exploitation during local development or CI. Updating the vulnerable dependencies patches these security holes.

## Current state

- `docs/site/package.json` — Defines the devDependencies.
  - Lines 21-27:
    ```json
      "devDependencies": {
        "@tailwindcss/typography": "^0.5.20",
        "oxfmt": "^0.55.0",
        "oxlint": "^1.0.0",
        "vitest": "^4.0.0",
        "wrangler": "^4.0.0"
      },
    ```

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Update    | `pnpm update`            | exit 0              |
| Audit     | `pnpm audit`             | exit 0 or significantly fewer CVEs |
| Build     | `pnpm build`             | exit 0              |

## Scope

**In scope** (the only files you should modify):
- `docs/site/package.json`
- `docs/site/pnpm-lock.yaml`

**Out of scope** (do NOT touch, even though they look related):
- Root-level Go files or module files.
- Application code in `docs/site/src/`.

## Git workflow

- Branch: `advisor/003-update-dependencies`
- Commit per step or per logical unit; message style: `chore(docs): update dependencies to fix audit vulnerabilities`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Update dependencies
Run the update command specifically for the `docs/site` workspace to bump minor/patch versions of the dependencies to secure versions.

**Verify**: `cd docs/site && pnpm up --latest vitest wrangler` → exit 0

### Step 2: Verify audit
Check that the audit results are now clean (or at least the high severity `undici`/`esbuild` vulnerabilities are resolved).

**Verify**: `cd docs/site && pnpm audit` → exit 0 (or no high severities)

### Step 3: Verify build
Ensure the dependency bump didn't break the Astro build.

**Verify**: `cd docs/site && pnpm build` → success.

## Test plan

- Test coverage relies on the build succeeding.
- Verification: `cd docs/site && pnpm build` → all pass.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `cd docs/site && pnpm build` exits 0
- [ ] `cd docs/site && pnpm audit` shows 0 high vulnerabilities.
- [ ] No files outside the in-scope list are modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The build fails after the dependency update.
- The `pnpm audit` command still reports the exact same high severity vulnerabilities even after `pnpm up --latest`.

## Maintenance notes

- Regular `pnpm audit` checks should be added to the CI pipeline in the future to prevent regression.
