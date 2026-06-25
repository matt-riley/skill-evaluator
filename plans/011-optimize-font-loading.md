# Plan 011: Make Google Fonts CSS non-render-blocking

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/layouts/Layout.astro`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: perf
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The Google Fonts CSS stylesheet at `Layout.astro:22` is a render-blocking external resource. Browsers pause first paint until this CSS downloads and parses. For users on slow connections, the page sits blank until the font CSS arrives, even though the site uses `font-display: swap` (which only helps after the CSS loads). The standard pattern is to load the font CSS asynchronously using `media="print" onload="this.media='all'"` so the page renders with the fallback font immediately and swaps to Nunito when the font CSS arrives.

## Current state

`docs/site/src/layouts/Layout.astro:18-22`:
```astro
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Nunito:ital,wght@0,200..1000;1,200..1000&display=swap" rel="stylesheet">
```

The fallback font stack (from `docs/site/src/styles/global.css:5`):
```css
--font-sans: "Nunito", "Quicksand", ui-sans-serif, system-ui, sans-serif;
```

Quicksand and system-ui provide acceptable fallback rendering while Nunito loads.

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
- `docs/site/src/layouts/Layout.astro:22` — change the font `<link>` to async loading pattern

**Out of scope**:
- Self-hosting the font files (bigger change, needs font file management)
- Changing the font family or fallback stack
- Any other `<head>` changes
- Any JS framework changes (the `onload` handler is vanilla inline JS)

## Git workflow

- Branch: `advisor/011-optimize-font-loading`
- Commit message: `perf(docs): load Google Fonts CSS asynchronously`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Replace the render-blocking font link with async pattern

In `docs/site/src/layouts/Layout.astro`, replace line 22:
```astro
<link href="https://fonts.googleapis.com/css2?family=Nunito:ital,wght@0,200..1000;1,200..1000&display=swap" rel="stylesheet">
```

with:
```astro
<link href="https://fonts.googleapis.com/css2?family=Nunito:ital,wght@0,200..1000;1,200..1000&display=swap" rel="stylesheet" media="print" onload="this.media='all'">
```

Also add a `<noscript>` fallback immediately after, for browsers with JS disabled:
```astro
<noscript>
  <link href="https://fonts.googleapis.com/css2?family=Nunito:ital,wght@0,200..1000;1,200..1000&display=swap" rel="stylesheet">
</noscript>
```

The pattern works as follows:
- `media="print"` tells the browser this stylesheet is only for print, so it doesn't block rendering
- `onload="this.media='all'"` switches media to `all` after the CSS loads, applying the fonts
- The `<noscript>` fallback loads the font normally when JS is unavailable

### Step 2: Verify no regressions

**Verify**:
- `pnpm build` → exit 0
- `pnpm lint` → exit 0
- `pnpm test` → exit 0 (all 3 tests pass)
- `pnpm check` → exit 0
- `grep 'media="print"' docs/site/dist/index.html` → match found
- `grep 'onload="this.media' docs/site/dist/index.html` → match found

## Test plan

No new automated tests. The font loading behavior is a runtime browser concern verified by inspecting the built HTML and visually confirming the page renders text (in a fallback font) before the Google Font CSS arrives.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `grep 'media="print"' docs/site/src/layouts/Layout.astro` returns a match
- [ ] `grep 'onload="this.media' docs/site/src/layouts/Layout.astro` returns a match
- [ ] `grep '<noscript>' docs/site/src/layouts/Layout.astro` returns a match
- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm test` exits 0
- [ ] `pnpm check` exits 0
- [ ] Built output contains the async font link pattern
- [ ] `plans/README.md` status row updated

## STOP conditions

- The font link at `Layout.astro:22` doesn't match the excerpt (already changed).
- `pnpm build` produces a warning or error about the `onload` attribute (Astro may escape inline JS — if so, use a `<script>` tag instead).
- Any test fails.

## Maintenance notes

- If the font family changes (e.g., Nunito replaced with Inter), update the URL in both the `<link>` and `<noscript>` blocks.
- This optimization matters most for first-time visitors on slow connections. Repeat visitors benefit from browser cache on the font CSS anyway.
- For a more robust solution (no flash of unstyled text at all), consider self-hosting the font files. That's a separate, larger effort.
