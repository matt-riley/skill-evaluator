# Plan 008: Add component-level tests for Astro pages (static render output)

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/ docs/site/tests/ docs/site/package.json docs/site/vitest.config.ts`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW
- **Depends on**: 006 (typecheck working), 007 (format:check passing — optional but nice)
- **Category**: tests
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The only test file is `tests/routing.test.ts`, which tests the two utility functions in `routing.ts`. The three Astro page components — `[...slug].astro`, `404.astro`, and `Layout.astro` — have no automated coverage. If someone accidentally breaks the sidebar rendering, the title prop passing, or the 404 page markup, nothing fails until a human notices. Adding vitest-based component output tests (rendering the Astro components to HTML strings and asserting on the output) catches these regressions cheaply.

## Current state

- `docs/site/tests/routing.test.ts` — 3 tests for `cleanPath` and `buildNavLinks`. Uses `vitest` with `describe`/`test`/`expect` pattern.
- `docs/site/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config"
export default defineConfig({
  test: {
    include: ["tests/**/*.test.{ts,js}"],
  },
})
```
- `docs/site/src/pages/[...slug].astro` — renders docs with sidebar + content
- `docs/site/src/pages/404.astro` — renders 404 page
- `docs/site/src/layouts/Layout.astro` — shared layout wrapper (title prop, Google Fonts, body structure)

Testing pattern to follow: the existing `tests/routing.test.ts` uses `import { describe, test, expect } from 'vitest'`. New tests should match this style — no fixtures, no framework, just imports and assertions.

### How to test Astro components in vitest

Astro components can be rendered to HTML strings using `@astrojs/test-utils` or by directly importing the component and calling its `render()` method. In Astro 6 with vitest, use:

```ts
import { experimental_AstroContainer as AstroContainer } from 'astro/container';
```

The container provides `container.renderToString(Component, { props })` which returns the HTML output as a string. This lets us assert on `expect(html).toContain(...)` without a browser.

If `astro/container` is not available, alternative is `astro/middleware` with a mock context, or simpler: test the static build output files directly (`dist/*.html`). For this plan, prefer container-based tests since they're fast and don't require a build step.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Install | `pnpm install` | exit 0 |
| Typecheck | `pnpm check` | exit 0 |
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass (existing 3 + new tests) |
| Build | `pnpm build` | exit 0 |

For running only the new tests: `pnpm test -- -t "Layout"` and `pnpm test -- -t "pages"`.

## Scope

**In scope**:
- `docs/site/tests/layout.test.ts` (create) — tests for `Layout.astro`
- `docs/site/tests/pages.test.ts` (create) — tests for `[...slug].astro` and `404.astro`

**Out of scope**:
- Modifying `routing.test.ts` (already covered by plan 004)
- E2E/Playwright/browser tests
- `docs/site/src/` source changes (test-only addition)
- Visual regression or screenshot testing

## Git workflow

- Branch: `advisor/008-component-tests`
- Commit message: `test(docs): add component-level tests for Astro pages`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Verify Astro Container is available

```bash
cd docs/site && node -e "require('astro/container')" 2>&1
```

If this fails, run `pnpm install` (astro is already a dependency, the container module ships with it). If it still fails, the container API may differ in Astro 6 — see STOP conditions.

### Step 2: Create `tests/layout.test.ts`

Create `docs/site/tests/layout.test.ts` with tests for `Layout.astro`:

```ts
import { describe, test, expect } from 'vitest';
import { experimental_AstroContainer as AstroContainer } from 'astro/container';
import Layout from '../src/layouts/Layout.astro';

describe('Layout', () => {
  test('renders the provided title in <title> tag', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test Page' },
    });
    expect(html).toContain('<title>Test Page</title>');
  });

  test('renders the slot content', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test' },
      slots: { default: '<p id="slot-content">Hello World</p>' },
    });
    expect(html).toContain('Hello World');
  });

  test('includes meta charset utf-8', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test' },
    });
    expect(html).toContain('<meta charset="utf-8"');
  });

  test('includes viewport meta tag', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test' },
    });
    expect(html).toContain('name="viewport"');
  });

  test('body has font-sans class', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test' },
    });
    expect(html).toContain('font-sans');
  });
});
```

**Verify**: `pnpm test -- -t "Layout"` → all 5 Layout tests pass.

### Step 3: Create `tests/pages.test.ts`

Create `docs/site/tests/pages.test.ts` with tests for `404.astro`:

```ts
import { describe, test, expect } from 'vitest';
import { experimental_AstroContainer as AstroContainer } from 'astro/container';
import NotFound from '../src/pages/404.astro';

describe('404 page', () => {
  test('renders 404 heading text', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(NotFound);
    expect(html).toContain('404');
  });

  test('includes a link back to home', async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(NotFound);
    expect(html).toContain('href="/"');
    expect(html).toContain('Back to Home');
  });
});
```

**Verify**: `pnpm test -- -t "404"` → both 404 tests pass.

### Step 4: Run full test suite

**Verify**: `pnpm test` → all tests pass (3 existing routing tests + 5 layout tests + 2 page tests = 10 total).

### Step 5: Verify lint and build

**Verify**:
- `pnpm lint` → exit 0
- `pnpm build` → exit 0
- `pnpm check` → exit 0

## Test plan

The plan IS the tests. No additional test plan needed — the 7 new tests described above are the deliverable.

**Verify**: `pnpm test` → 10 tests pass.

## Done criteria

- [ ] `pnpm test` exits 0 with at least 10 passing tests
- [ ] `tests/layout.test.ts` exists with at least 5 tests
- [ ] `tests/pages.test.ts` exists with at least 2 tests
- [ ] `pnpm lint` exits 0
- [ ] `pnpm build` exits 0
- [ ] `pnpm check` exits 0
- [ ] No files outside the in-scope list modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- `astro/container` import fails or `experimental_AstroContainer` is not available in the installed Astro 6 version. In that case, fall back to testing the built `dist/*.html` output directly (read the HTML files and assert with `expect(content).toContain(...)`). Report the fallback method used.
- `pnpm test` fails on a test that the existing routing tests already passed — indicates a regression.
- Any type errors from `pnpm check` after adding test files.

## Maintenance notes

- The container API (`experimental_AstroContainer`) is experimental and may change in future Astro releases. When upgrading Astro, verify these tests still compile.
- These tests assert on HTML string output. They don't test CSS or visual layout — for that, add Playwright tests later.
- If the `[...slug].astro` page's `getStaticPaths` pulls in more markdown files (e.g., from a new directory), add tests for those paths.
