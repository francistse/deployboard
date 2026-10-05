# Upstream relationship

Deployboard is a **fork** of [`RoboZephyr/launch-pilot`](https://github.com/RoboZephyr/launch-pilot)
(MIT), a localhost web console for macOS `launchd` jobs. The fork keeps upstream's history so
it can be rebased, and never removes an upstream feature.

- **Base commit**: `5d9c07d` ("Fix landing tooling, browser tests, and release configuration")
- **Licence**: MIT (see `LICENSE`) — the fork keeps the upstream copyright.
- **Module path** is `github.com/A404coder/deployboard`. Upstream's module was
  `github.com/A404coder/launch-pilot`; import paths will conflict on a rebase.
- **Not built by this fork**: `landing/` (upstream's Cloudflare Pages site).

## Remotes

```bash
git remote -v
# origin     https://github.com/francistse/deployboard.git         (this fork's home)
# upstream   https://github.com/RoboZephyr/launch-pilot.git    (read-only, for rebases)
```

## Taking upstream changes

```bash
git fetch upstream
git log --oneline HEAD..upstream/main          # what landed upstream
git rebase upstream/main                       # or: git merge upstream/main
make test && make build   # or: go build ./cmd/deployboard
git push origin main
```

Conflict hot-spots (where the fork deliberately changed upstream code):

| File | Fork change |
|---|---|
| `internal/launchd/service.go` | offline/online merge sets `WorkingDirectory`; calls `s.enrich(jobs)` |
| `internal/launchd/types.go` | `StatusDisabled`, `Category`, `CategorySource`, `Group`, `Runs`, `Disabled`, `PrintState`, `RestartWarn`, `HasExit`, `WorkingDirectory` |
| `internal/server/router.go` | `NewRouterWithFork` + the fork routes; `NewRouter` still behaves as upstream |
| `web/lib/classify.js` | `ours`/`noise`/`other` instead of `mine`/`system`/`thirdparty` |
| `web/lib/state.js` | `showNoise` replaces `onlyMine` |

## What the fork adds (why it exists)

Upstream classifies by domain: "Mine" = user domain, label not starting with `com.apple.`.
On a real workstation that pulls in Dropbox, Google updaters, Spotify, Chrome,
CopilotForXcode, Tailscale, Docker and todesktop — 545 jobs, mostly noise — so the question
"which of MY deployed applications are up?" is not answerable from it.

The fork answers it, and imports the missing data:

1. **Inventory view** — an allowlist plus *path derivation* (`derive_roots`): a job whose
   plist log/working dir/args live under your project tree is yours, automatically. Measured
   here: most deployments derived, 0 false positives across the vendor/job vendor bucket.
2. **`launchctl print` truth** — `runs` (restart counter) and `print-disabled` (`disabled`
   status, so a retired plist stops looking like a failure). `launchctl list`, upstream's
   source, has neither.
3. **Prometheus `/metrics`** — the only launchd→metrics exporter in this niche
   (`docs/METRICS.md`).
4. **Telegram alerts with per-app toggles** (`docs/ALERTS.md`).
5. **One-command macOS install** (`docs/INSTALL-macos.md`).
6. **`--read-only` mode** for a box that must not be able to kill a service.

Spec of record: [`docs/deployboard-additions.md`](deployboard-additions.md).
