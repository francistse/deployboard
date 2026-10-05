# Contributing to Deployboard

Thanks for helping. Deployboard is a fork of
[`RoboZephyr/launch-pilot`](https://github.com/RoboZephyr/launch-pilot) (MIT); upstream
features are never removed — the fork only adds. Read
[`docs/UPSTREAM.md`](docs/UPSTREAM.md) before you change anything that upstream also owns.

## Development setup

Requirements: macOS 13+, Go (see `go.mod`), Node 20.9+ for the frontend and the landing site.

```sh
git clone https://github.com/francistse/deployboard.git
cd deployboard
make build          # -> ./deployboard  (version from Makefile, default 0.0.1)
make test           # Go tests, explicit package list
npm test            # frontend tests (node --test, no install needed)
npm ci && npm run e2e   # Playwright end-to-end (Chromium)
```

The dashboard is a single Go binary with the Preact UI embedded via `go:embed`.
`make build` embeds `web/`; changing `web/` needs a rebuild to reach the running
binary. `make run` starts it on a random port and opens the browser.

The landing site is separate:

```sh
cd landing
npm ci
npm run build              # static export -> landing/out
npm start                  # preview the export at http://127.0.0.1:4174
```

Build the landing with an **empty** `NEXT_PUBLIC_BASE_PATH` for the local preview;
only the Pages workflow sets it to `/<repo>`.

## Working in a worktree

This checkout is shared with other sessions, so do your work in a git worktree and
leave `main` clean:

```sh
git worktree add .worktrees/<task> -b <branch> main
# ... edit, commit, push from the worktree ...
git worktree remove .worktrees/<task>
```

`.worktrees/` is gitignored. A `landing/node_modules` symlink will not build in a
worktree — run `npm ci` inside the worktree's `landing/`.

## Before you open a pull request

Run all of these and make them pass:

```sh
make vet && make test
npm test
cd landing && npm run lint && npm run build
```

Then open the PR against `main`. Small, focused PRs with a real reproduction or
before/after evidence get reviewed fastest. Note that **the published `main` is a
single squashed commit on the upstream base** — don't be surprised that the public
history is one commit; keep working normally on your branch and it is folded in at
release time.

## Conventions

- The Go module path is `github.com/A404coder/deployboard`. New files must import
  that path — a leftover `github.com/A404coder/launch-pilot/...` import still builds
  (`go build` skips test files) but breaks `make test`.
- `Makefile` uses an explicit package list, not `./...` (landing `node_modules` ships
  stray Go files); add new packages to `GO_PKGS` if you create one.
- Keep the 7-status model, the two read-only levels, and the “never edit a plist”
  invariant intact — they are documented in `docs/deployboard-additions.md`.
- GitHub Actions workflows must be guarded with
  `if: github.server_url == 'https://github.com'` where they use GitHub-only
  features, so the project's self-hosted mirror does not queue a failing run.

## Adding or changing UI strings

Dashboard copy lives in `web/lib/i18n.js` as five-column rows:
`[key, en, ja, zh-Hant, zh-Hans]`. Every locale must have the same keys, the same
`{name}` placeholders, and the same `[[code]]` marks — `npm test` asserts that.

1. Add the row to `ROWS` in `web/lib/i18n.js`.
2. Call `t('your.key')` (or `t('your.key', { name })`) from the component.
3. For landing-site copy, edit the matching file under `landing/src/content/`
   (`en.ts`, `ja.ts`, `zh-Hant.ts`, `zh-Hans.ts`) and keep the `Content` shape in
   `types.ts` in sync.
4. Prefer short native wording over literal translations; language names stay in
   their own script in every catalog (`lang.ja` is always `日本語`).

## Reporting bugs and security issues

Open an issue for bugs. For anything security-related see [`SECURITY.md`](SECURITY.md) —
please do not file it publicly.
