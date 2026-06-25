# Plan 015: Create AGENTS.md for the docs/site sub-project

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: Not applicable — this plan creates a new file. No in-scope files exist to drift.
> Confirm the repo root `AGENTS.md` still exists and the `docs/site/` directory is present before proceeding.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: 006 (typecheck), 007 (format) — so the instructions reference working commands
- **Category**: dx
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The repo root has an `AGENTS.md` with project-wide conventions, but the `docs/site/` sub-project has none. Agents working on the Astro site get no guidance on the tech stack, commands, or conventions specific to this sub-project. A short AGENTS.md reduces onboarding friction and prevents agents from guessing at commands or conventions.

## Current state

- Repo root `AGENTS.md` covers the Go CLI tool — it doesn't mention the docs site.
- `docs/site/` has no `AGENTS.md`, `CLAUDE.md`, or `CONTEXT.md`.
- The sub-project is an Astro 6 SSG site with Tailwind v4, vitest, oxlint, and pnpm as package manager.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass |
| Build | `pnpm build` | exit 0 |
| Typecheck | `pnpm check` | exit 0 |

## Scope

**In scope**:
- `docs/site/AGENTS.md` (create) — agent instructions for this sub-project

**Out of scope**:
- Modifying the repo root `AGENTS.md`
- Any source file changes

## Git workflow

- Branch: `advisor/015-add-agents-md`
- Commit message: `docs: add AGENTS.md for docs/site sub-project`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Create docs/site/AGENTS.md

Create `docs/site/AGENTS.md` with the following content:

```markdown
# Agent Instructions — docs/site

This is the documentation site for skill-evaluator, built with **Astro 6** (static output).

## Tech stack

- **Astro 6** — SSG, `output: "static"` in `astro.config.mjs`
- **Tailwind CSS v4** — styling via `@tailwindcss/vite` plugin
- **@tailwindcss/typography** — prose styling for rendered markdown
- **Cloudflare Pages** — deployment target (`wrangler.toml`)
- **pnpm** — package manager

## Commands

| Command | What it does |
|---------|-------------|
| `pnpm install` | Install dependencies |
| `pnpm dev` | Start dev server (hot reload) |
| `pnpm build` | Build static site to `dist/` |
| `pnpm check` | Typecheck with `astro check` |
| `pnpm lint` | Lint with oxlint |
| `pnpm test` | Run vitest tests |
| `pnpm format` | Format with oxfmt |
| `pnpm format:check` | Check formatting |

## Project structure

```
docs/site/
  src/
    pages/
      [...slug].astro    — main doc page (renders markdown from repo root and docs/adr/)
      404.astro          — custom 404 page
    layouts/
      Layout.astro       — shared HTML shell (head, body, fonts)
    utils/
      routing.ts         — cleanPath, buildNavLinks (path → nav link conversion)
    styles/
      global.css         — Tailwind v4 imports, theme colors, base styles
  tests/
    routing.test.ts      — unit tests for routing utilities
  public/
    favicon.ico, favicon.svg
  astro.config.mjs
  tsconfig.json (extends astro/tsconfigs/strict)
  vitest.config.ts
  wrangler.toml
```

## Conventions

- **TypeScript strict mode** — all `.ts` and `.astro` files type-checked. Run `pnpm check` before committing.
- **Conventional commits** — e.g., `feat(docs):`, `fix(docs):`, `chore(docs):`, `test(docs):`.
- **Formatting**: oxfmt with config in `.oxfmtrc.json`. Run `pnpm format` before committing.
- **Linting**: oxlint. Run `pnpm lint`.
- **Neo-brutalist design**: thick black borders, colored backgrounds (neo-pink, neo-yellow, neo-blue, neo-green, neo-purple), bold shadows, playful transforms. Defined in `global.css` as Tailwind theme colors.
- **Markdown source**: the site renders `.md` files from the repo root (4 dirs up) and `docs/adr/`. Agent instruction files (AGENTS.md, CLAUDE.md, CONTEXT.md) are filtered out.
- **Branch naming for plans**: `advisor/NNN-short-slug` (where NNN is the plan number from `plans/README.md`).

## Testing

Tests live in `tests/` and use **vitest**. Follow the pattern in `tests/routing.test.ts`:

```ts
import { describe, test, expect } from 'vitest';
// import thing under test
describe('Thing', () => {
  test('does X', () => {
    expect(actual).toBe(expected);
  });
});
```

Run all tests: `pnpm test`. Run specific tests: `pnpm test -- -t "pattern"`.

## Important notes

- Do NOT import or reference `@cloudflare/astro` or any SSR adapter. This site is static-only.
- The `[...slug].astro` page uses `import.meta.glob` to find markdown files at build time. These paths are relative to the source file (`../../../../*.md` = repo root).
- When modifying `Layout.astro`, keep `<slot />` intact — it's where page content renders.
```

### Step 2: Verify the file is well-formed

**Verify**:
- `wc -l docs/site/AGENTS.md` → shows a file with content
- `grep 'Agent Instructions' docs/site/AGENTS.md` → match found
- `grep 'pnpm check' docs/site/AGENTS.md` → match found

### Step 3: Verify no impact on build

**Verify**:
- `pnpm build` → exit 0 (adding a .md file outside the glob pattern doesn't affect the site — AGENTS.md is at `docs/site/`, the glob looks at repo root)
- `pnpm lint` → exit 0
- `pnpm test` → exit 0
- `pnpm check` → exit 0

## Test plan

No new tests. This is a documentation-only addition.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `docs/site/AGENTS.md` exists and contains the section "Agent Instructions — docs/site"
- [ ] `pnpm lint` exits 0
- [ ] `pnpm test` exits 0
- [ ] `pnpm build` exits 0
- [ ] `pnpm check` exits 0
- [ ] `plans/README.md` status row updated

## STOP conditions

- `docs/site/AGENTS.md` already exists (someone created one since this plan was written). If so, compare content and only add missing sections.
- Adding the file causes `pnpm build` to fail (unlikely — it's outside the glob pattern, but verify).
- The repo root `AGENTS.md` doesn't exist (then we'd need to reference a different pattern).

## Maintenance notes

- Keep the commands table in sync with `package.json` scripts. If a new script is added, update this file.
- If the project structure changes (new directories, renamed files), update the structure section.
- This file is intentionally excluded from the docs site by the filter in `routing.ts` (`EXCLUDED_DOCS` after plan 010) — it won't appear in the sidebar.
