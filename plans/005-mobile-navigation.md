# Plan 005: Add mobile navigation (hamburger menu) so phone users can navigate between docs

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.

> **Drift check (run first)**: `git diff --stat a573a68..HEAD -- docs/site/src/pages/[...slug].astro docs/site/src/pages/404.astro docs/site/src/layouts/Layout.astro`
> If any in-scope file changed, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `a573a68`, 2026-06-24

## Why this matters

The sidebar navigation is hidden on screens below `md` (768px) in `[...slug].astro:38`:
`<aside class="... hidden md:flex ...">`. Mobile users see only a fixed header bar with the site logo and title. They cannot navigate between documentation pages. This is the only user-facing gap in an otherwise polished docs site. Adding a hamburger-triggered slide-out drawer or overlay menu gives mobile users the same navigation capability as desktop users.

## Current state

- `docs/site/src/pages/[...slug].astro` — doc page with sidebar (`:38` sidebar hidden on mobile, `:53` mobile header bar with just logo + title)
- `docs/site/src/pages/404.astro` — 404 page, no navigation (no sidebar) — out of scope
- `docs/site/src/layouts/Layout.astro` — shared layout wrapper — out of scope
- Design system: neo-brutalist (thick borders, shadows, rounded corners), Tailwind v4 with custom neo-* colors (`global.css`)
- Site uses static output mode (`astro.config.mjs:8`), so no JS framework is wired — any interactivity (hamburger toggle) can be done with a vanilla `<script>` or a small Astro island.

Current mobile header (`[...slug].astro:53-59`):
```astro
<div class="md:hidden fixed top-0 left-0 right-0 h-20 border-b-4 border-black bg-neo-yellow z-30 flex items-center px-6">
  <div class="size-12 rounded-2xl bg-neo-pink border-4 border-black flex items-center justify-center shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] mr-4">
    <span class="text-white font-black text-lg -rotate-6">SE</span>
  </div>
  <a href="/" class="text-3xl font-black text-black tracking-tight">Skill Eval</a>
</div>
```

Sidebar (starts at `:38`):
```astro
<aside class="w-80 bg-neo-yellow border-r-4 border-black p-6 hidden md:flex flex-col shrink-0 overflow-y-auto h-screen sticky top-0 z-20">
  ... navigation links ...
</aside>
```

Repo conventions: conventional commits (e.g. `feat(docs): add hamburger menu for mobile nav`). All source in `docs/site/`. This sub-project has no internal design docs beyond what's in the code. The ADR at `docs/adr/0001-shell-out-to-agent-runtimes.md` does not constrain the docs site design.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Install | `pnpm install` | exit 0 |
| Lint | `pnpm lint` | exit 0 |
| Tests | `pnpm test` | all pass (3 tests) |
| Build | `pnpm build` | exit 0, builds all pages with no errors |
| Format check | `pnpm format:check` | exit 0 (but NOTE: this currently fails per plan 007 — don't block on it) |

## Scope

**In scope**:
- `docs/site/src/pages/[...slug].astro` — add hamburger button to mobile header + slide-out menu

**Out of scope**:
- `docs/site/src/pages/404.astro` — this page has no nav links; don't add a menu here
- `docs/site/src/layouts/Layout.astro` — don't modify the shared layout
- Any CSS framework changes; stick to Tailwind v4 utilities already in use
- Server-side rendering or JS framework islands; use a small inline `<script>` for toggle behavior

## Git workflow

- Branch: `advisor/005-mobile-navigation`
- Commit message style: `feat(docs): add hamburger menu for mobile navigation`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add hamburger button to the mobile header

In `docs/site/src/pages/[...slug].astro`, inside the mobile header `<div class="md:hidden fixed ...">`, add a hamburger button at the right end (before the closing `</div>`). The button should:
- Be styled consistently: `p-2 bg-white border-4 border-black rounded-xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]`
- Have an `id="menu-toggle"` and `aria-label="Open menu"`
- Show a ☰ (hamburger) icon using an inline `<span>`
- Use `ml-auto` so it pushes to the right

Example button:
```astro
<button id="menu-toggle" aria-label="Open menu" class="ml-auto p-2 bg-white border-4 border-black rounded-xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] hover:-translate-y-1 transition-all">
  <span class="text-2xl font-black">☰</span>
</button>
```

**Verify**: `pnpm build` → exit 0, the index.html in `dist/` contains `id="menu-toggle"`.

### Step 2: Add the slide-out mobile menu overlay

Add a mobile menu drawer after the mobile header (still inside the outer `<div class="flex min-h-screen...">`). The menu should:
- Be `id="mobile-menu"` with classes: `hidden fixed inset-0 z-40 md:hidden`
- Have a semi-transparent backdrop (`bg-black/50`) for the overlay
- The actual drawer panel slides in from the left: `bg-neo-yellow border-r-4 border-black w-80 h-full overflow-y-auto p-6 transform -translate-x-full transition-transform duration-300` (the JS will toggle `translate-x-0`)
- Contain a close button (✕) in the top-right, `id="menu-close"`
- Replicate the navigation links from the desktop sidebar (same list of docs + ADR details/summary)

Copy the nav link structure from the desktop sidebar (`:44-89` in `[...slug].astro`), including the two sections: "Documentation 📚" and "Arch Decisions 🏗️". The active link highlighting should match.

**Verify**: `pnpm build` → exit 0, `dist/` output contains `<div id="mobile-menu"`.

### Step 3: Add inline `<script>` for toggle behavior

Add a `<script>` block at the bottom of the page (just before the closing `</body>` equivalent — in Astro, after the last `</div>` of the main content). This script should:
- Listen for clicks on `#menu-toggle` → remove `hidden` from `#mobile-menu`, add `translate-x-0` (remove `-translate-x-full`) after a `requestAnimationFrame`
- Listen for clicks on `#menu-close` → remove `translate-x-0`, add `-translate-x-full`, then add `hidden` after the transition (300ms timeout)
- Listen for clicks on the backdrop (the `#mobile-menu` element itself, if the click target is the backdrop div not the drawer) → same close behavior

```astro
<script>
  const toggle = document.getElementById('menu-toggle');
  const menu = document.getElementById('mobile-menu');
  const close = document.getElementById('menu-close');
  const drawer = menu?.querySelector('.w-80'); // the drawer panel inside

  toggle?.addEventListener('click', () => {
    menu.classList.remove('hidden');
    requestAnimationFrame(() => {
      drawer?.classList.remove('-translate-x-full');
      drawer?.classList.add('translate-x-0');
    });
  });

  function closeMenu() {
    drawer?.classList.remove('translate-x-0');
    drawer?.classList.add('-translate-x-full');
    setTimeout(() => menu.classList.add('hidden'), 300);
  }

  close?.addEventListener('click', closeMenu);
  menu?.addEventListener('click', (e) => {
    if (e.target === menu) closeMenu();
  });
</script>
```

**Verify**: `pnpm build` → exit 0. `grep -c 'addEventListener' dist/index.html` returns at least 3.

### Step 4: Final verification

**Verify**:
- `pnpm lint` → exit 0
- `pnpm build` → exit 0, all pages built
- `pnpm test` → all 3 tests pass
- Manual check: open `dist/index.html` in a browser at viewport < 768px, confirm hamburger shows, menu opens/closes, nav links work

## Test plan

No automated test changes are required. The toggle behavior is DOM-level interactivity best verified with a browser. Existing routing tests (`tests/routing.test.ts`) must continue to pass.

**Verify**: `pnpm test` → 3 tests pass.

## Done criteria

- [ ] `pnpm build` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm test` exits 0 (all 3 existing tests pass)
- [ ] `grep 'id="menu-toggle"' docs/site/src/pages/\[...slug\].astro` returns a match
- [ ] `grep 'id="mobile-menu"' docs/site/src/pages/\[...slug\].astro` returns a match
- [ ] `grep 'closeMenu' docs/site/src/pages/\[...slug\].astro` returns a match
- [ ] No files outside the in-scope list are modified
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The mobile header at `[...slug].astro:~53` doesn't match the excerpt in "Current state" (codebase drifted).
- The desktop sidebar at `[...slug].astro:~38` doesn't contain the `hidden md:flex` pattern.
- `pnpm build` fails after step 1 or 2 with an Astro compilation error you can't resolve in one fix attempt.
- The nav link structure in the sidebar has changed significantly (new sections, different markup) — the mobile menu must replicate whatever the desktop sidebar shows.

## Maintenance notes

- If new doc pages are added or the sidebar structure changes (e.g., a third section), the mobile menu must be updated in lockstep with the desktop sidebar. Consider extracting the nav links into a reusable Astro component later.
- This plan adds an inline `<script>` for simplicity. If the site later adopts a JS framework (React, Preact, Solid), the toggle behavior should be migrated to that framework's component model.
- The menu uses Tailwind utility classes for transitions. If the Tailwind config changes (especially `duration` or `transform` classes), verify the menu animation still works.
