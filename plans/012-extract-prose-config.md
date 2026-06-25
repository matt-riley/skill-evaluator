# Plan 012: Extract inline Tailwind prose config into a shared CSS class

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/pages/\[...slug\].astro docs/site/src/styles/global.css`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: 007 (format:check — formatting changes may collide with the prose block)
- **Category**: tech-debt
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The prose typography configuration in `[...slug].astro:79-99` is a 20-line inline Tailwind class string. It's hard to scan, hard to diff, and impossible to reuse if another page needs prose styling. Extracting it to a named CSS class in `global.css` (or a Tailwind `@utility`) makes it readable, diffable, and reusable.

## Current state

The inline prose classes at `docs/site/src/pages/[...slug].astro:79-99` (on the `<article>` element):
```astro
<article class="prose max-w-none 
  prose-headings:scroll-mt-32 
  prose-h1:text-5xl md:prose-h1:text-7xl prose-h1:font-black prose-h1:tracking-tight prose-h1:text-black prose-h1:mb-12
  prose-h2:text-4xl prose-h2:font-extrabold prose-h2:tracking-tight prose-h2:text-black prose-h2:mt-16 prose-h2:mb-8 prose-h2:bg-neo-yellow prose-h2:inline-block prose-h2:px-4 prose-h2:py-2 prose-h2:rounded-xl prose-h2:border-4 prose-h2:border-black prose-h2:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] prose-h2:-rotate-1
  prose-h3:text-3xl prose-h3:font-bold prose-h3:text-black prose-h3:mt-10 prose-h3:underline prose-h3:decoration-neo-pink prose-h3:decoration-4 prose-h3:underline-offset-4
  prose-p:text-gray-800 prose-p:leading-relaxed prose-p:text-xl prose-p:font-bold
  prose-a:text-black prose-a:font-black prose-a:bg-neo-blue/30 prose-a:px-1 prose-a:rounded hover:prose-a:bg-neo-blue prose-a:transition-colors hover:prose-a:text-white
  prose-code:bg-neo-pink/20 prose-code:text-black prose-code:px-2.5 prose-code:py-1 prose-code:rounded-xl prose-code:border-2 prose-code:border-black prose-code:before:content-none prose-code:after:content-none prose-code:font-black
  prose-pre:bg-gray-900 prose-pre:text-white prose-pre:border-4 prose-pre:border-black prose-pre:rounded-3xl prose-pre:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] hover:prose-pre:-translate-y-1 hover:prose-pre:-translate-x-1 hover:prose-pre:shadow-[12px_12px_0px_0px_rgba(0,0,0,1)] transition-all prose-pre:p-6 prose-pre:text-lg
  [&_pre_code]:bg-transparent [&_pre_code]:text-white [&_pre_code]:p-0 [&_pre_code]:border-none [&_pre_code]:font-normal
  prose-blockquote:border-l-0 prose-blockquote:border-4 prose-blockquote:border-black prose-blockquote:bg-neo-green/20 prose-blockquote:py-5 prose-blockquote:px-8 prose-blockquote:rounded-3xl prose-blockquote:text-gray-900 prose-blockquote:not-italic prose-blockquote:font-black prose-blockquote:text-xl prose-blockquote:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] hover:prose-blockquote:-translate-y-1 transition-transform
  prose-img:rounded-3xl prose-img:border-4 prose-img:border-black prose-img:shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] hover:prose-img:-translate-y-1 hover:prose-img:-translate-x-1 hover:prose-img:shadow-[12px_12px_0px_0px_rgba(0,0,0,1)] transition-all prose-img:w-full
  prose-strong:text-black prose-strong:font-black
  prose-ul:text-gray-800 prose-li:marker:text-neo-pink prose-ul:font-bold prose-ul:text-xl
  prose-hr:border-black prose-hr:border-4 prose-hr:rounded-full
  prose-table:border-4 prose-table:border-black prose-table:rounded-3xl prose-table:overflow-hidden prose-th:bg-neo-purple prose-th:px-6 prose-th:py-5 prose-th:text-left prose-th:font-black prose-th:text-xl prose-th:text-black prose-td:px-6 prose-td:py-4 prose-td:border-t-4 prose-td:border-black prose-td:font-bold prose-td:text-lg prose-td:text-gray-800">
```

`docs/site/src/styles/global.css` already imports Tailwind and the typography plugin:
```css
@import "tailwindcss";
@plugin "@tailwindcss/typography";
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
- `docs/site/src/styles/global.css` — add a `.docs-prose` class with the prose overrides
- `docs/site/src/pages/[...slug].astro` — replace inline prose classes with `docs-prose`

**Out of scope**:
- Changing any visual appearance — the rendered output must look identical
- Adding new prose styles or modifying existing ones
- Modifying the `@tailwindcss/typography` plugin config
- Any other `.astro` files

## Git workflow

- Branch: `advisor/012-extract-prose-config`
- Commit message: `refactor(docs): extract prose typography config to CSS class`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add `.docs-prose` class to global.css

In `docs/site/src/styles/global.css`, after the existing body styles, add:

```css
.docs-prose {
  @apply prose max-w-none;

  & h1 {
    @apply text-5xl font-black tracking-tight text-black mb-12;
    @apply md:text-7xl;
  }
  & h2 {
    @apply text-4xl font-extrabold tracking-tight text-black mt-16 mb-8;
    @apply bg-neo-yellow inline-block px-4 py-2 rounded-xl border-4 border-black;
    @apply shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] -rotate-1;
  }
  & h3 {
    @apply text-3xl font-bold text-black mt-10;
    @apply underline decoration-neo-pink decoration-4 underline-offset-4;
  }
  & p {
    @apply text-gray-800 leading-relaxed text-xl font-bold;
  }
  & a {
    @apply text-black font-black bg-neo-blue/30 px-1 rounded;
    @apply transition-colors hover:bg-neo-blue hover:text-white;
  }
  & code {
    @apply bg-neo-pink/20 text-black px-2.5 py-1 rounded-xl border-2 border-black;
    @apply before:content-none after:content-none font-black;
  }
  & pre {
    @apply bg-gray-900 text-white border-4 border-black rounded-3xl;
    @apply shadow-[8px_8px_0px_0px_rgba(0,0,0,1)];
    @apply hover:-translate-y-1 hover:-translate-x-1 hover:shadow-[12px_12px_0px_0px_rgba(0,0,0,1)];
    @apply transition-all p-6 text-lg;
  }
  & pre code {
    @apply bg-transparent text-white p-0 border-none font-normal;
  }
  & blockquote {
    @apply border-l-0 border-4 border-black bg-neo-green/20 py-5 px-8 rounded-3xl;
    @apply text-gray-900 not-italic font-black text-xl;
    @apply shadow-[6px_6px_0px_0px_rgba(0,0,0,1)];
    @apply hover:-translate-y-1 transition-transform;
  }
  & img {
    @apply rounded-3xl border-4 border-black;
    @apply shadow-[8px_8px_0px_0px_rgba(0,0,0,1)];
    @apply hover:-translate-y-1 hover:-translate-x-1 hover:shadow-[12px_12px_0px_0px_rgba(0,0,0,1)];
    @apply transition-all w-full;
  }
  & strong {
    @apply text-black font-black;
  }
  & ul {
    @apply text-gray-800 font-bold text-xl;
    & li::marker {
      color: #ff6b9e; /* neo-pink */
    }
  }
  & hr {
    @apply border-black border-4 rounded-full;
  }
  & table {
    @apply border-4 border-black rounded-3xl overflow-hidden;
  }
  & th {
    @apply bg-neo-purple px-6 py-5 text-left font-black text-xl text-black;
  }
  & td {
    @apply px-6 py-4 border-t-4 border-black font-bold text-lg text-gray-800;
  }

  /* scroll-mt for heading anchors */
  h1, h2, h3, h4, h5, h6 {
    scroll-margin-top: 8rem;
  }
}
```

### Step 2: Replace inline prose classes in [...slug].astro

In `docs/site/src/pages/[...slug].astro`, replace the `<article>` element's class attribute (lines 79–99, the entire block from `class="prose max-w-none` to the closing `"`) with:

```astro
<article class="docs-prose">
```

**Verify**: `grep 'prose max-w-none' docs/site/src/pages/\[...slug\].astro` → no matches (inline class blob is gone). `grep 'docs-prose' docs/site/src/pages/\[...slug\].astro` → match found.

### Step 3: Verify visual equivalence

**Verify**:
- `pnpm build` → exit 0
- `pnpm lint` → exit 0
- `pnpm test` → exit 0
- `pnpm check` → exit 0

For visual verification, compare the built dist output before and after. Since this is a mechanical translation of Tailwind classes to CSS with `@apply`, the generated CSS selectors should produce identical styling. The build output should be structurally similar (same HTML elements, different CSS).

## Test plan

No new tests. Existing tests verify routing logic, not visual output. Manual visual check: build the site and browse the rendered docs page — headings, code blocks, tables, and links should look identical to before.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `.docs-prose` class exists in `docs/site/src/styles/global.css`
- [ ] `<article class="docs-prose">` in `docs/site/src/pages/[...slug].astro`
- [ ] Inline prose class blob is removed from `[...slug].astro`
- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm test` exits 0
- [ ] `pnpm check` exits 0
- [ ] `plans/README.md` status row updated

## STOP conditions

- Tailwind `@apply` with `prose` plugin selectors fails to compile (the `@tailwindcss/typography` plugin may not support `@apply` for all variants). If the build fails with "Cannot apply unknown utility class", revert to inline classes and report the limitation.
- Visual output differs significantly (missing borders, wrong colors) — re-check the CSS translation.
- The `h1, h2, h3` scroll-margin shorthand doesn't work (Tailwind v4 may handle this differently). If it fails, use individual `scroll-mt-32` classes on each heading level.

## Maintenance notes

- To add a new prose style (e.g., styling for `<details>` elements), add it to `.docs-prose` in `global.css`.
- The `@apply` approach relies on Tailwind v4's CSS-first configuration. If the project migrates away from Tailwind, these styles become plain CSS — which is an advantage over inline Tailwind classes.
- If another page (e.g., a blog or changelog) needs prose styling, just add `class="docs-prose"` to its `<article>` instead of copying inline classes.
