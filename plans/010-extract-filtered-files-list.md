# Plan 010: Extract the agent-instruction-file filter list into a shared constant

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/pages/\[...slug\].astro docs/site/src/utils/routing.ts`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: 009 (dead code removal — same file, reduces merge conflict risk)
- **Category**: tech-debt
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The `[...slug].astro` page filters out agent instruction files (AGENTS.md, CLAUDE.md, CONTEXT.md) from the glob results before building nav links. The filter list is an inline array at `:10-13` that `buildNavLinks` has no knowledge of. If a new agent instruction file is added (e.g., `copilot-instructions.md`), someone must update the filter in the page template AND understand that `buildNavLinks` is opaque to this filtering. Extracting the filter to a shared constant in `routing.ts` (where `buildNavLinks` already lives) makes the intent explicit and centralizes future additions.

## Current state

Filter in `docs/site/src/pages/[...slug].astro:9-17`:
```ts
const allFiles = Object.keys(allFilesRaw).reduce((acc, key) => {
  const lowerKey = key.toLowerCase();
  if (
    !lowerKey.endsWith('agents.md') && 
    !lowerKey.endsWith('claude.md') && 
    !lowerKey.endsWith('context.md')
  ) {
    acc[key] = allFilesRaw[key];
  }
  return acc;
}, {});
```

`buildNavLinks` is defined in `docs/site/src/utils/routing.ts:8` and receives `Object.keys(allFiles)` (post-filter). It doesn't know about the filter.

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
- `docs/site/src/utils/routing.ts` — add `EXCLUDED_DOCS` constant array
- `docs/site/src/pages/[...slug].astro` — import and use `EXCLUDED_DOCS` instead of inline filter

**Out of scope**:
- Changing the set of filtered files (keep the same three: agents.md, claude.md, context.md)
- Modifying `buildNavLinks` behavior
- Any changes to test files

## Git workflow

- Branch: `advisor/010-extract-filtered-files`
- Commit message: `refactor(docs): extract agent-instruction filter list to shared constant`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add `EXCLUDED_DOCS` constant to routing.ts

In `docs/site/src/utils/routing.ts`, after the existing imports/exports, add:

```ts
/** File names (lowercase) excluded from the documentation nav — agent instruction files. */
export const EXCLUDED_DOCS = ['agents.md', 'claude.md', 'context.md'];
```

### Step 2: Update `[...slug].astro` to use the constant

In `docs/site/src/pages/[...slug].astro`, change the import line (currently `:3`):
```ts
import { buildNavLinks, cleanPath } from '../utils/routing';
```
to:
```ts
import { buildNavLinks, cleanPath, EXCLUDED_DOCS } from '../utils/routing';
```

Then replace the inline filter block (`:9-17`) with:
```ts
const allFiles = Object.keys(allFilesRaw).reduce((acc, key) => {
  const lowerKey = key.toLowerCase();
  if (!EXCLUDED_DOCS.some(doc => lowerKey.endsWith(doc))) {
    acc[key] = allFilesRaw[key];
  }
  return acc;
}, {});
```

### Step 3: Verify

**Verify**:
- `pnpm lint` → exit 0
- `pnpm test` → exit 0, all tests pass
- `pnpm build` → exit 0
- `pnpm check` → exit 0
- Confirm the build output still excludes the filtered files: `grep -l 'agents\|claude\|context' docs/site/dist/*.html 2>/dev/null` should NOT match any files whose names come from AGENTS.md, CLAUDE.md, or CONTEXT.md. (The word "context" may appear in prose content — that's fine. The check is that no page URL like `/agents/` exists.)

## Test plan

Existing routing tests verify `cleanPath` and `buildNavLinks`. The constant is a value addition, not a logic change. No new tests needed — the build verification gates the behavior.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `grep 'EXCLUDED_DOCS' docs/site/src/utils/routing.ts` returns match
- [ ] `grep 'EXCLUDED_DOCS' docs/site/src/pages/\[...slug\].astro` returns match
- [ ] `grep 'endsWith.*agents.md.*claude.md.*context.md' docs/site/src/pages/\[...slug\].astro` returns no matches (inline filter is gone)
- [ ] `pnpm test` exits 0
- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm check` exits 0
- [ ] `plans/README.md` status row updated

## STOP conditions

- The inline filter in `[...slug].astro` no longer matches the excerpt above (someone already changed it).
- Adding the import breaks the Astro build (unlikely, but verify).
- Any test fails.

## Maintenance notes

- To exclude additional files from the docs nav, add them to `EXCLUDED_DOCS` in `routing.ts`. No changes needed in the page template.
- The constant uses `endsWith` matching (case-insensitive via `toLowerCase()`). Make sure new entries match the patterns: `'filename.md'` (lowercase).
