# Plan 009: Remove dead `findMatchKey` function from routing.ts

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/utils/routing.ts`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: tech-debt
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

`findMatchKey` at `routing.ts:32` is exported but has zero callers anywhere in the codebase. Dead exports mislead maintainers (they look for callers and wonder if they're missing something) and increase the surface area of the module. Removing it is a one-line deletion with no behavioral impact.

## Current state

- `docs/site/src/utils/routing.ts:32-42`:
```ts
export function findMatchKey(keys: string[], slug: string): string | undefined {
  let matchKey = keys.find(key => {
    const relativePath = cleanPath(key);
    return relativePath.toLowerCase() === slug.toLowerCase();
  });

  if (!matchKey && slug === 'README') {
    matchKey = keys.find(k => k.toLowerCase().endsWith('readme.md'));
  }

  return matchKey;
}
```

Confirmed dead: `grep -rn 'findMatchKey' docs/site/src/` only returns the definition in `routing.ts`. It is not imported or called from any `.astro`, `.ts`, or `.tsx` file.

The only imported exports from `routing.ts` are `buildNavLinks` and `cleanPath`, both imported at `[...slug].astro:3`:
```ts
import { buildNavLinks, cleanPath } from '../utils/routing';
```

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Install | `pnpm install` | exit 0 |
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass (3 tests) |
| Build | `pnpm build` | exit 0 |
| Typecheck | `pnpm check` | exit 0 |

## Scope

**In scope**:
- `docs/site/src/utils/routing.ts` — remove `findMatchKey` function (lines 32–42)

**Out of scope**:
- Any other changes to `routing.ts`
- Adding new functions or tests for `findMatchKey`
- Any `.astro` or test file changes

## Git workflow

- Branch: `advisor/009-remove-dead-findMatchKey`
- Commit message: `chore(docs): remove unused findMatchKey from routing.ts`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Delete `findMatchKey` from routing.ts

Open `docs/site/src/utils/routing.ts` and remove lines 32–42 (the entire `findMatchKey` function definition, including the blank line before it if present). The file should end after the `buildNavLinks` function's closing brace.

The resulting file should look like:
```ts
export function cleanPath(key: string): string {
  return key
    .replace(/^(\.\.\/)+/, '')
    .replace(/^docs\//, '')
    .replace('.md', '');
}

export function buildNavLinks(keys: string[], currentSlug: string) {
  // ... (unchanged)
}

// findMatchKey removed — had zero callers
```

**Verify**: `grep 'findMatchKey' docs/site/src/utils/routing.ts` → no matches.

### Step 2: Verify no regressions

**Verify**:
- `pnpm test` → exit 0, all 3 tests pass (no tests reference `findMatchKey`)
- `pnpm lint` → exit 0
- `pnpm build` → exit 0
- `pnpm check` → exit 0

## Test plan

No new tests. The existing routing tests (`tests/routing.test.ts`) test `cleanPath` and `buildNavLinks`, which are unchanged. No existing test references `findMatchKey`.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `grep 'findMatchKey' docs/site/src/utils/routing.ts` returns no matches
- [ ] `pnpm test` exits 0
- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm check` exits 0
- [ ] `plans/README.md` status row updated

## STOP conditions

- `grep -rn 'findMatchKey' docs/site/` returns matches outside `routing.ts` or outside `plans/` — someone added a caller since this plan was written. Do not delete.
- Any existing test fails after removal.
- `pnpm build` fails after removal.

## Maintenance notes

- If `findMatchKey` is ever needed again (e.g., for search or URL matching logic), re-implement it from git history rather than guessing at the signature. The original paired with `getStaticPaths` to resolve ambiguous slugs.
