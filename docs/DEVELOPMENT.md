# Development

Maintainer notes. The public README is the install path and the pictures.

## Prerequisites

- macOS (uses `launchctl`)
- Go 1.26.2+ (matches `go.mod`)
- Node.js 22+ for frontend and browser tests

## Commands

```bash
make build    # compile binary with version from git tag
make test     # run Go tests in cmd/, internal/, and web/
make run      # build + run
make clean    # remove binary
```

### Frontend tests

Frontend modules are tested with Node.js built-in test runner:

```bash
npm test
```

The test loader maps bare specifiers (`@preact/signals`) to vendored ESM files for Node.js compatibility.

### Browser tests

```bash
npm ci
npx playwright install chromium
npm run e2e
```

To use an installed Google Chrome: `PLAYWRIGHT_CHANNEL=chrome npm run e2e`.
The suite starts an isolated server on `127.0.0.1:18080`.

## Project structure

```
cmd/deployboard/         Go entrypoint (CLI flags, server startup)
internal/
  launchd/              Job model, launchctl parser, service layer, DeriveStatus
  diagnose/             6-check diagnostic engine
  plist/                Plist reader + mtime cache + NextCalendarFire / NextIntervalFire / LastRunAt
  server/               HTTP router, REST handlers, SSE handler
web/
  app.js                Preact app root
  index.html            HTML shell with import map
  components/           UI components (JobRow, FilterBar, SearchBar, job-tooltip, etc.)
  lib/                  State signals, classification logic, SSE client, API client
  styles/               CSS (single main.css, CSS variables for theming)
  vendor/               Vendored ESM: Preact, htm, Signals
landing/                Static marketing site (Next.js export) → GitHub Pages
.github/workflows/      pages.yml — builds landing/ and deploys it to Pages
```

## Landing site

`landing/` is a static Next.js export for the project's public page, deployed to
GitHub Pages by `.github/workflows/pages.yml` on any push that touches it. All
copy lives in `landing/src/content/`.
The hero preview is the interactive console in `landing/src/app/landing-view.tsx`.

```bash
cd landing
npm ci
npm run dev          # local preview
npm run build        # static export -> landing/out
```

Enable it once per repository: **Settings → Pages → Source: GitHub Actions**.
See [`landing/README.md`](../landing/README.md) for the base-path details and the
non-GitHub hosting options.

Screenshots and the GIF on the repository README are captured from that hero
preview (`[data-preview]`): Ours, Other, Noise, the `/metrics` scene, and the
Telegram scene.
