# Plan 004: Placeholder test suite provides zero coverage

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 8b20fcd..HEAD -- docs/site/src/pages/[...slug].astro docs/site/tests/index.test.ts`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `8b20fcd`, 2026-06-24

## Why this matters

The test suite in `docs/site/tests/index.test.ts` currently runs a dummy assertion (`expect("Home").toBe("Home")`). The most complex and brittle part of the documentation site is the routing logic inside `[...slug].astro`, which uses regex to clean file paths (like `../../../README.md` into `readme`) and sorting logic to build the navigation links. If someone breaks this logic, the build will succeed but the site routing will silently break in production. Extracting this logic into a utility module and writing actual unit tests provides structural safety.

## Current state

- `docs/site/src/pages/[...slug].astro` — Contains inline regex mapping and sorting.
  - Lines 44-67:
    ```js
    const navLinks = Object.keys(allFiles).map(key => {
      const path = key
        .replace(/^(\.\.\/)+/, '')
        .replace(/^docs\//, '')
        .replace('.md', '');
      // ... string manipulation ...
    }).sort((a, b) => {
      if (a.isAdr && !b.isAdr) return 1;
      if (!a.isAdr && b.isAdr) return -1;
      return a.title.localeCompare(b.title);
    });
    ```
- `docs/site/tests/index.test.ts` — Contains the dummy test.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Tests     | `pnpm test`              | all pass            |
| Build     | `pnpm build`             | exit 0              |

## Scope

**In scope** (the only files you should modify):
- `docs/site/src/pages/[...slug].astro`
- `docs/site/src/utils/routing.ts` (create)
- `docs/site/tests/routing.test.ts` (create)
- `docs/site/tests/index.test.ts` (delete)

**Out of scope** (do NOT touch, even though they look related):
- Root-level Go tests or logic.

## Git workflow

- Branch: `advisor/004-test-routing-logic`
- Commit per step or per logical unit; message style: `test(docs): unit test navigation routing logic`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Extract logic to utils
Create a new file `docs/site/src/utils/routing.ts`.
Extract the `navLinks` mapping and sorting logic, and the `matchKey` finding logic into pure functions (e.g. `export function cleanPath(key: string): string`, `export function buildNavLinks(keys: string[], currentSlug: string)`).
Refactor `docs/site/src/pages/[...slug].astro` to import and use these functions.

**Verify**: `cd docs/site && pnpm build` → exit 0

### Step 2: Delete dummy test
Remove `docs/site/tests/index.test.ts`.

**Verify**: `test -f docs/site/tests/index.test.ts` → exits 1 (fails, meaning file is gone)

### Step 3: Write real unit tests
Create `docs/site/tests/routing.test.ts`.
Write tests that cover:
1. `cleanPath`: Given `../../../../README.md` returns `README`. Given `../../../../docs/adr/0001-record.md` returns `adr/0001-record`.
2. `buildNavLinks`: Given a mock list of raw keys, verifies that ADRs are sorted correctly at the bottom, titles are capitalized correctly, and `isAdr` is set to true for paths starting with `adr/`.

**Verify**: `cd docs/site && pnpm test` → all pass.

## Test plan

- The entire goal of this plan is testing.
- Verification: `cd docs/site && pnpm test` → all pass, including new tests.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `cd docs/site && pnpm build` exits 0
- [ ] `cd docs/site && pnpm test` exits 0
- [ ] `docs/site/tests/index.test.ts` is deleted.
- [ ] `docs/site/tests/routing.test.ts` exists and has actual expectations.
- [ ] No files outside the in-scope list are modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The refactor into pure functions breaks the Astro build due to TypeScript type mismatch.
- You are unable to run Vitest.

## Maintenance notes

- Any future changes to how Markdown files are stored or categorized must update both the regex in `routing.ts` and the corresponding unit tests in `routing.test.ts`.
