# Plan 001: Convert Astro configuration from SSR to SSG

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 8b20fcd..HEAD -- docs/site/astro.config.mjs docs/site/src/pages/[...slug].astro`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: perf
- **Planned at**: commit `8b20fcd`, 2026-06-24

## Why this matters

Currently, the documentation site is configured for SSR (Server-Side Rendering) using the Cloudflare adapter, and explicitly disables prerendering on the `[...slug].astro` route. Because the site globs all markdown files locally using `import.meta.glob(..., { eager: true })`, every page view invokes a Cloudflare Worker that contains the entire documentation payload in memory. As docs grow, this worker will exceed the 1MB script size limit and deployments will fail. Static Site Generation (SSG) pre-renders all paths at build time, completely eliminating this risk and serving the site instantly from Cloudflare's edge cache.

## Current state

- `docs/site/astro.config.mjs` — Configures the server output and cloudflare adapter.
  - Lines 7-9:
    ```js
    export default defineConfig({
      output: "server",
      adapter: cloudflare(),
    ```
- `docs/site/src/pages/[...slug].astro` — The dynamic route that renders all markdown files.
  - Line 4:
    ```astro
    export const prerender = false;
    ```
- Conventions: The site uses the "Bubbly Neobrutalist" aesthetic, but this is a purely architectural configuration change. No visual styles should be altered.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Install   | `pnpm install`           | exit 0              |
| Build     | `pnpm build`             | exit 0, outputs to `dist/` |

## Scope

**In scope** (the only files you should modify):
- `docs/site/astro.config.mjs`
- `docs/site/src/pages/[...slug].astro`
- `docs/site/package.json`

**Out of scope** (do NOT touch, even though they look related):
- Any CSS or styling files in `src/styles/` or `src/layouts/`.
- The actual markdown files in the repository root.

## Git workflow

- Branch: `advisor/001-convert-ssr-to-ssg`
- Commit per step or per logical unit; message style: `chore(docs): convert astro site to SSG`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Remove Cloudflare adapter and set output to static
In `docs/site/astro.config.mjs`, remove `output: "server"` and change it to `output: "static"` (or remove the `output` key entirely as it defaults to static). Remove the `adapter: cloudflare(),` line, and remove the import for `@astrojs/cloudflare`.

**Verify**: `cat docs/site/astro.config.mjs | grep cloudflare` → (No output)

### Step 2: Uninstall cloudflare adapter dependency
Run the uninstallation command for the unused package.

**Verify**: `cd docs/site && pnpm remove @astrojs/cloudflare` → exit 0

### Step 3: Implement getStaticPaths in the dynamic route
In `docs/site/src/pages/[...slug].astro`, remove `export const prerender = false;`.
Instead, you must export an asynchronous `getStaticPaths()` function. 
Move the globbing logic and the path cleaning logic inside `getStaticPaths()`. The function must return an array of objects in the shape: `{ params: { slug: "..." }, props: { doc, navLinks, ... } }`.
Then, in the top level of the component script, grab `const { doc, navLinks } = Astro.props;` and render as usual.

**Verify**: `cd docs/site && pnpm build` → success, and it should show multiple `.html` files generated in the build output.

## Test plan

- There are no unit tests for this; the verification is that the site compiles statically via `pnpm build` without errors, and that `dist/` contains `.html` files for the routes.
- Verification: `cd docs/site && pnpm build` → success.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `cd docs/site && pnpm build` exits 0
- [ ] `grep "export const prerender = false" docs/site/src/pages/[...slug].astro` returns no matches
- [ ] No files outside the in-scope list are modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The build fails because of an unresolved variable after implementing `getStaticPaths`.
- The site relies on Server-Side functionality (like headers or API routes) that break upon removing the SSR adapter.

## Maintenance notes

- With SSG, `import.meta.glob` is executed exclusively at build time. Dynamic content that requires runtime evaluation cannot be easily injected without client-side fetching.
