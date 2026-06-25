# Agent Instructions — docs/site

This is the documentation site for skill-evaluator, built with **Astro 6** (static output).

## Tech stack

- **Astro 6** — SSG, `output: "static"` in `astro.config.mjs`
- **Tailwind CSS v4** — styling via `@tailwindcss/vite` plugin
- **@tailwindcss/typography** — prose styling for rendered markdown
- **Cloudflare Pages** — deployment target (`wrangler.toml`)
- **pnpm** — package manager

## Commands

| Command             | What it does                  |
| ------------------- | ----------------------------- |
| `pnpm install`      | Install dependencies          |
| `pnpm dev`          | Start dev server (hot reload) |
| `pnpm build`        | Build static site to `dist/`  |
| `pnpm check`        | Typecheck with `astro check`  |
| `pnpm lint`         | Lint with oxlint              |
| `pnpm test`         | Run vitest tests              |
| `pnpm format`       | Format with oxfmt             |
| `pnpm format:check` | Check formatting              |

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
import { describe, test, expect } from "vitest";
// import thing under test
describe("Thing", () => {
  test("does X", () => {
    expect(actual).toBe(expected);
  });
});
```

Run all tests: `pnpm test`. Run specific tests: `pnpm test -- -t "pattern"`.

## Important notes

- Do NOT import or reference `@cloudflare/astro` or any SSR adapter. This site is static-only.
- The `[...slug].astro` page uses `import.meta.glob` to find markdown files at build time. These paths are relative to the source file (`../../../../*.md` = repo root).
- When modifying `Layout.astro`, keep `<slot />` intact — it's where page content renders.
