# Plan 013: Add Content-Security-Policy meta tag to Layout.astro

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/layouts/Layout.astro docs/site/astro.config.mjs`
> If any in-scope file changed, compare the "Current state" excerpts against the live code; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: 011 (font loading — both touch the `<head>`, sequential avoids merge conflicts)
- **Category**: security
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The site renders user-authored markdown content as HTML. While Astro's markdown pipeline sanitizes input, defense in depth matters. A CSP header restricts which scripts, styles, fonts, and images can load, limiting the blast radius if markdown content somehow contains injected tags. Since this is a static site with no external scripts (only Google Fonts for CSS + inline scripts for menu toggle after plan 005), a CSP can be tight and low-maintenance.

## Current state

`docs/site/src/layouts/Layout.astro:11-23` (head section):
```astro
<head>
  <meta charset="utf-8" />
  <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
  <link rel="icon" href="/favicon.ico" />
  <meta name="viewport" content="width=device-width" />
  <meta name="generator" content={Astro.generator} />
  <title>{title}</title>
  
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Nunito:ital,wght@0,200..1000;1,200..1000&display=swap" rel="stylesheet">
</head>
```

External resources used:
- `https://fonts.googleapis.com` — font CSS (style-src)
- `https://fonts.gstatic.com` — font files (font-src)
- Inline styles and scripts from Astro's build process and future mobile nav toggle

The site is `output: "static"` (SSG), deployed to Cloudflare Pages. A CSP can be delivered as a `<meta>` tag (which works for static sites) or as an HTTP header via Cloudflare's `_headers` file.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Install | `pnpm install` | exit 0 |
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass |
| Build | `pnpm build` | exit 0 |
| Typecheck | `pnpm check` | exit 0 |

## Scope

**In scope**:
- `docs/site/src/layouts/Layout.astro` — add a CSP `<meta>` tag in `<head>`

**Out of scope**:
- Cloudflare `_headers` file for HTTP-level CSP (can be added later)
- CSP reporting (`report-uri` or `report-to`)
- Modifying the Astro build to inject content hashes for inline scripts/styles
- Any other security headers

## Git workflow

- Branch: `advisor/013-add-csp-meta`
- Commit message: `security(docs): add Content-Security-Policy meta tag`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add CSP meta tag to Layout.astro

In `docs/site/src/layouts/Layout.astro`, add a CSP `<meta>` tag after the existing `<meta>` tags and before the `<title>`:

```astro
<meta http-equiv="Content-Security-Policy" content="default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; script-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';" />
```

Policy breakdown:
- `default-src 'self'` — allow only same-origin by default
- `style-src 'self' 'unsafe-inline' https://fonts.googleapis.com` — allow inline styles (Astro generates these) + Google Fonts CSS
- `font-src 'self' https://fonts.gstatic.com` — allow Google Font files
- `img-src 'self' data:` — allow same-origin images + data URIs
- `script-src 'self' 'unsafe-inline'` — allow inline scripts (mobile nav toggle, Astro's build output)
- `connect-src 'self'` — no external XHR/fetch needed
- `frame-ancestors 'none'` — prevent clickjacking embedding
- `base-uri 'self'` — prevent base tag injection
- `form-action 'self'` — no external form submissions

### Step 2: Verify build still works

**Verify**:
- `pnpm build` → exit 0
- `grep 'Content-Security-Policy' docs/site/dist/index.html` → match found
- `pnpm lint` → exit 0
- `pnpm test` → exit 0
- `pnpm check` → exit 0

### Step 3: Verify no resource loading is blocked

Open the built site locally (`pnpm preview`) and check the browser console for CSP violation reports. No resources should be blocked. If any are, adjust the policy directives accordingly (see STOP conditions).

## Test plan

No new tests. The CSP is a configuration addition that doesn't affect logic. Verify visually that no resources are blocked.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `grep 'Content-Security-Policy' docs/site/src/layouts/Layout.astro` returns a match
- [ ] `grep 'Content-Security-Policy' docs/site/dist/index.html` returns a match
- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm test` exits 0
- [ ] `pnpm check` exits 0
- [ ] `plans/README.md` status row updated

## STOP conditions

- The `<head>` section in `Layout.astro` has changed significantly from the current state excerpt.
- `pnpm build` produces a CSP-related warning or strips the `http-equiv` attribute (unlikely, but verify).
- Browser testing reveals blocked resources (CSP violations in console) — report which directives need adjustment.
- If Astro injects inline scripts with nonces/hashes in a future version, the `'unsafe-inline'` allowance becomes a weakness. That's for the maintainer to tighten later.

## Maintenance notes

- If the site adds external images (e.g., from a CDN), add the domain to `img-src`.
- If analytics or monitoring scripts are added, add their domains to `script-src` and `connect-src`.
- For stronger CSP, replace `'unsafe-inline'` with nonces or hashes — but this requires Astro build integration. This plan keeps it simple.
- Cloudflare Pages can also deliver CSP as an HTTP header via a `_headers` file, which is stronger than a meta tag. Consider adding that as a follow-up.
