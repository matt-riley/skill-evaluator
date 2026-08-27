# Design System — skilleval.mattriley.tools

**"Precision Instrument"** — a dark glass technical workspace. The site reads like a
measurement instrument for AI skills, not a marketing page: near-black surfaces, hairline
rails, frosted panels, and one accent color used exactly the way the tool uses it — as a
_verdict signal_ (pass / eval / warn / fail).

Supersedes the neo-brutalist theme (mint dot grid, thick black borders, emoji nav).

## Tokens

Defined in `src/styles/global.css` via Tailwind v4 `@theme` + CSS custom properties.

| Token                 | Value     | Role                     |
| --------------------- | --------- | ------------------------ |
| `--color-base`        | `#0a0c10` | page background          |
| `--color-base-2`      | `#0d1016` | stage background         |
| `--color-raised`      | `#12151c` | panels / sidebar         |
| `--color-ink`         | `#eef1f6` | primary text             |
| `--color-ink-dim`     | `#98a2b3` | secondary text           |
| `--color-ink-faint`   | `#5c677a` | tertiary / labels        |
| `--color-line`        | `#1d2330` | hairlines                |
| `--color-line-bright` | `#2b3345` | hover hairlines          |
| `--color-accent`      | `#3ddc97` | pass / active nav / CTAs |
| `--color-accent-2`    | `#54d3f2` | eval / info              |
| `--color-warn`        | `#f2b84b` | baseline / warnings      |
| `--color-danger`      | `#f26d8d` | fails                    |

**Contrast** (verified against WCAG): ink/base ≈ 15:1 · ink-dim/base ≈ 7:1 ·
accent on base ≈ 8:1 · CTA text `#05281a` on accent ≈ 8:1.

## Typography

| Role                   | Family                      | Notes                                                 |
| ---------------------- | --------------------------- | ----------------------------------------------------- |
| Display & headings     | Space Grotesk (700/650/600) | `--font-display`, tight tracking                      |
| Body & UI              | Geist (400-700)             | `--font-sans`                                         |
| Code, labels, metadata | IBM Plex Mono (400/500/600) | `--font-mono`, uppercase track 0.12-0.16em for labels |

Loaded via Google Fonts with `media="print"` swap; CSP allows `fonts.googleapis.com` +
`fonts.gstatic.com` only.

## Layout

- **Topbar** (sticky, 4rem, blur) — brand mark (emerald compass-star on dark glass tile),
  version chip, GitHub link, scroll progress hairline.
- **Sidebar** (≥lg, fixed under topbar, 16rem) — grouped nav: Start here / Documentation /
  Guides / Architecture / Releases. Active item = emerald tint + left glow dot. Mobile:
  glass drawer with backdrop.
- **Doc pages** — centered `max-w-3xl` content with page-hero (kicker + h1 + description),
  prose col, prev/next pager. Rail lines + corner marks frame the stage at ≥1320px.
- **Homepage** — custom `src/pages/index.astro`: hero (giant Space Grotesk headline),
  live-typing terminal demo, 3-step loop panels, baseline-vs-with-skill chart, install
  panel, command surface, guide cards, footer.

## Motion

- Scroll reveals: `IntersectionObserver`, fade+rise 16px, `cubic-bezier(0.22,1,0.36,1)`,
  elements already in view resolve immediately; hard fallback on `load` so content is
  never hidden.
- Hero terminal types a real loop transcript; loops; `prefers-reduced-motion` renders the
  full transcript statically.
- All motion collapses under `prefers-reduced-motion` (0.01ms overrides).
- No external animation libraries. No WebGL.

## Content conventions applied

- The `docs/index.md` markdown no longer renders as a page — Home is a custom Astro page.
  `index.md` content overlaps with it; keep them in sync if you edit either.
- Page titles moved out of markdown (`# ...` h1 removed from every doc) — the layout's
  page-hero owns the h1 from frontmatter `title`.
- Inline `<video>`/`<img>` tags in guides drop the old brutalist class soup; global
  `docs-prose` styles media now.

## Testing

- `pnpm build` → must pass; `pnpm test` (vitest, reads built dist) → must pass;
  `pnpm check` → no errors.
- The tests assert the rendered design contract (hero headline, terminal, nav groups,
  drawer ids). Change deliberately, not silently.
