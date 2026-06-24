# Plan 002: Missing 404 page leads to unhandled redirects

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 8b20fcd..HEAD -- docs/site/src/pages/[...slug].astro`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: bug
- **Planned at**: commit `8b20fcd`, 2026-06-24

## Why this matters

In `[...slug].astro`, if a user requests a URL that doesn't map to an existing markdown file, the logic issues an `Astro.redirect('/404')`. However, there is no `404.astro` page in the `src/pages/` directory. This results in the user receiving an unhandled default browser 404 error instead of staying within the styled documentation site. Adding a custom 404 page ensures the user experience remains consistent, friendly, and visually aligned with the rest of the site.

## Current state

- `docs/site/src/pages/[...slug].astro` — Contains the fallback redirect.
  - Line 39:
    ```astro
    if (!matchKey || !allFiles[matchKey]) return Astro.redirect('/404');
    ```
- The `docs/site/src/pages/` directory currently only contains `[...slug].astro`.
- The site uses the "Bubbly Neobrutalist" aesthetic. A basic page uses `Layout.astro` with heavy borders, neo-pink/neo-yellow colors, and rounded elements.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `cd docs/site && pnpm build` | exit 0, outputs to `dist/` |

## Scope

**In scope** (the only files you should modify):
- `docs/site/src/pages/404.astro` (create)

**Out of scope** (do NOT touch, even though they look related):
- `docs/site/src/pages/[...slug].astro` — the redirect logic is correct; the target simply doesn't exist.

## Git workflow

- Branch: `advisor/002-add-404-page`
- Commit per step or per logical unit; message style: `fix(docs): add custom 404 error page`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Create 404.astro
Create a new file `docs/site/src/pages/404.astro`.
Use the existing `Layout.astro` component just like a normal page.
Include a playful message indicating the page wasn't found (e.g., "Oops! This page wandered off. 🕵️‍♂️") and provide a prominent button/link back to `/` (Home) styled with the Neobrutalist classes (e.g., `bg-neo-pink text-white font-black px-6 py-3 rounded-2xl border-4 border-black shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] hover:-translate-y-1 transition-transform`).

**Verify**: `test -f docs/site/src/pages/404.astro` → exits 0

### Step 2: Verify the build
Run the Astro build process to ensure the new page compiles successfully without syntax errors.

**Verify**: `cd docs/site && pnpm build` → success.

## Test plan

- Manual verification of the 404 page rendering locally.
- Verification: `cd docs/site && pnpm build` → all pass.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `cd docs/site && pnpm build` exits 0
- [ ] `docs/site/src/pages/404.astro` exists
- [ ] No files outside the in-scope list are modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The build fails because `Layout` is incorrectly imported.
- You are unsure of the Neobrutalist CSS classes to use (in this case, refer to the classes generated in `[...slug].astro` for inspiration).

## Maintenance notes

- If new layouts are added in the future, ensure the 404 page is updated to use the appropriate global layout so it doesn't look orphaned.
