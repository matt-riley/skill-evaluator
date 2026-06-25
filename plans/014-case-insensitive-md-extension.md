# Plan 014: Make `.md` extension removal case-insensitive in cleanPath

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/utils/routing.ts docs/site/tests/routing.test.ts`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: 009 (dead code removal in same file), 010 (extracted filter in same file)
- **Category**: correctness
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

`cleanPath` at `routing.ts:4` uses `.replace('.md', '')` which only matches lowercase `.md`. If a file named `CHANGELOG.MD` is added to the repo root, its slug would become `CHANGELOG.MD` instead of `CHANGELOG` — breaking the URL and the nav link. While all current markdown files use lowercase `.md`, making the regex case-insensitive (`/\.md$/i`) prevents a future surprise. This is especially relevant because the glob pattern `import.meta.glob('../../../../*.md')` is case-sensitive on macOS but case-insensitive on Linux (Cloudflare deployment), so behavior across environments could differ.

## Current state

`docs/site/src/utils/routing.ts:1-6`:
```ts
export function cleanPath(key: string): string {
  return key
    .replace(/^(\.\.\/)+/, '')
    .replace(/^docs\//, '')
    .replace('.md', '');
}
```

The `.replace('.md', '')` call uses a plain string, which is case-sensitive. It also replaces `.md` anywhere in the string, not just at the end — though in practice the glob paths always end with `.md`.

Current test (`tests/routing.test.ts:15-16`):
```ts
test('given ../../../../docs/adr/0001-record.md returns adr/0001-record', () => {
  expect(cleanPath('../../../../docs/adr/0001-record.md')).toBe('adr/0001-record');
});
```

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Install | `pnpm install` | exit 0 |
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass (3 tests + new) |
| Build | `pnpm build` | exit 0 |
| Typecheck | `pnpm check` | exit 0 |

## Scope

**In scope**:
- `docs/site/src/utils/routing.ts` — change `.replace('.md', '')` to use a case-insensitive regex anchored to end
- `docs/site/tests/routing.test.ts` — add a test case for `.MD` extension

**Out of scope**:
- Changing any other regex in `cleanPath`
- Changing `buildNavLinks` or any other function
- Astro page template changes

## Git workflow

- Branch: `advisor/014-case-insensitive-md`
- Commit message: `fix(docs): make .md extension removal case-insensitive in cleanPath`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Update cleanPath to use case-insensitive regex

In `docs/site/src/utils/routing.ts`, change line 4:
```ts
    .replace('.md', '');
```
to:
```ts
    .replace(/\.md$/i, '');
```

The `/i` flag makes it case-insensitive. The `\.` escapes the dot (literal dot, not regex "any char"). The `$` anchors to end of string so it only removes the extension, not `.md` appearing mid-path.

### Step 2: Add test case for uppercase .MD

In `docs/site/tests/routing.test.ts`, add a new test to the `cleanPath` describe block:

```ts
test('given a path ending in .MD (uppercase) returns path without extension', () => {
  expect(cleanPath('../../../../CHANGELOG.MD')).toBe('CHANGELOG');
});
```

### Step 3: Verify all tests pass

**Verify**: `pnpm test` → exit 0, 4 tests pass (3 existing + 1 new `CHANGELOG.MD` test).

### Step 4: Verify no regressions

**Verify**:
- `pnpm build` → exit 0
- `pnpm lint` → exit 0
- `pnpm check` → exit 0

## Test plan

Add 1 new test case as described in Step 2. No existing test behavior changes.

**Verify**: `pnpm test` → 4 tests pass.

## Done criteria

- [ ] `grep '\.md\$\/i' docs/site/src/utils/routing.ts` returns a match (regex written correctly)
- [ ] `grep 'CHANGELOG.MD' docs/site/tests/routing.test.ts` returns a match (new test)
- [ ] `pnpm test` exits 0 with 4 passing tests
- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm check` exits 0
- [ ] `plans/README.md` status row updated

## STOP conditions

- The `cleanPath` function at `routing.ts:1-6` doesn't match the excerpt (already modified, possibly by plan 009 or 010).
- Existing tests fail after the change (indicates the regex isn't equivalent for existing paths).
- `pnpm build` fails — check if the regex change affects slug generation in unexpected ways.

## Maintenance notes

- The regex anchor `/\.md$/i` also means a path like `docs/something.md/extra` won't lose its extension — the `.md` must be at the end. This matches the actual glob behavior (files always end with `.md`).
- If other file extensions are ever supported (e.g., `.mdx`), extend the regex: `/\.(md|mdx)$/i`.
