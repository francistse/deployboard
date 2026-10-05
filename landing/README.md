# Deployboard landing page

Next.js / React / TypeScript marketing site for **Deployboard** — the fork of
[launch-pilot](https://github.com/RoboZephyr/launch-pilot) that shows your own
launchd deployments instead of 500 system jobs.

Exported as static HTML (`output: "export"`), so it needs no server runtime and
can be hosted anywhere that serves files — including GitHub Pages, for free.

## Development

Use npm and the committed `package-lock.json` (Node.js 20.9 or newer):

```sh
npm ci
npm run dev
npm run lint
npm run build
npm start        # preview the static export at http://127.0.0.1:4174
```

`npm run build` writes the static site to `out/`. The preview serves only
`out/` and binds to localhost; if port 4174 is occupied it exits instead of
silently switching ports.

All copy lives in **`src/content.ts`** — hero, live-panel mock rows, why,
statuses, fork additions, metrics sample, alert rules, install steps, FAQ,
footer. `src/app/page.tsx` only decides layout, so content edits rarely need it.

Links point at `github.com/francistse/deployboard` (the `REPO` constant in
`src/content.ts`) — change it there if the repo moves.

## GitHub Pages

`.github/workflows/pages.yml` at the repository root builds and deploys on every
push that touches `landing/`:

1. **Settings → Pages → Source: GitHub Actions** (once, in the GitHub repo).
2. Push to `main` — the workflow runs `npm ci`, derives the base path from the
   repository name, builds, and uploads `landing/out`.
3. The site appears at `https://<user>.github.io/<repo>/`.

How the base path works: a Pages *project* page is served from `/<repo>/`, so
Next needs to know. `next.config.ts` reads `NEXT_PUBLIC_BASE_PATH` and applies
it to both `basePath` and `assetPrefix`. The workflow sets it to `/<repo>` — and
leaves it empty for a *user* page (`<user>.github.io`), which is served from the
root. To reproduce a project page locally:

```sh
NEXT_PUBLIC_BASE_PATH=/deployboard npm run build
```

Notes:

- Pages is free for **public** repositories. On a private repo you need a paid
  plan, or host `out/` elsewhere (Cloudflare Pages, Netlify, S3) — the export
  has no server-side dependencies.
- `public/.nojekyll` stops Jekyll from swallowing Next's `_next/` directory.

## Structure

- `src/content.ts`: shared copy, metadata, links, install command, mock rows
- `src/app/`: page, root layout, and global styles
- `src/kit/`: reusable visual components and design tokens (7 statuses,
  including the fork's `disabled`)
- `src/page/`: providers and shared hooks
- `public/`: static assets (`favicon.svg`, `.nojekyll`)
- `scripts/preview.mjs`: serves `out/` for a local check

Keep generated output (`.next/` and `out/`) out of version control.

## Credit

Forked from the launch-pilot landing page (MIT). Copy, fork sections, metrics
sample, alert rules and the seven-status palette are this fork's.
