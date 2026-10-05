# Deployboard

[![CI](https://github.com/francistse/deployboard/actions/workflows/ci.yml/badge.svg)](https://github.com/francistse/deployboard/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Latest release](https://img.shields.io/github/v/release/francistse/deployboard)](https://github.com/francistse/deployboard/releases)
[![Platform: macOS 13+](https://img.shields.io/badge/platform-macOS%2013%2B-lightgrey.svg)](#requirements)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg)](go.mod)

Visual control console for macOS launchd services. View status, read logs, diagnose issues, and manage user-domain launch agents — all from your browser.

> **This is a fork.** Upstream: [`RoboZephyr/launch-pilot`](https://github.com/RoboZephyr/launch-pilot) (MIT).
> Base commit `5d9c07d`; upstream history is preserved so it can be rebased. Nothing upstream is removed.
> See [`docs/UPSTREAM.md`](docs/UPSTREAM.md) for the full fork rationale and the rebase procedure.

## Fork additions — Deployboard

Upstream classifies jobs by domain, so "Mine" includes Dropbox, Chrome, Spotify, Tailscale and
every updater — on a developer Mac, 545 jobs, mostly noise. The fork answers the question that matters
instead: **which of MY deployed applications are up, broken, or intentionally off.**

Committed next steps stay on that wedge (desired state, health contracts + incidents, project-native
registration, Ours-scoped agent MCP) — see [`ROADMAP.md`](ROADMAP.md). Product facts:
[`PRODUCT-STATE.md`](PRODUCT-STATE.md).

| Addition | What it does | Docs |
|---|---|---|
| **Ours / Other / Noise view** | Default view is your deployments only. Anything whose plist log/working dir/args live under `derive_roots` is auto-classified as yours (most of them here, 0 false positives across 528 vendor jobs); a short allowlist covers infra that lives outside the project tree. `Hide`/`Auto` per row from the UI. | `docs/INSTALL-macos.md` (config) |
| **`launchctl print` truth** | `runs` (restart counter) and `print-disabled`, so a **disabled** plist stops looking like a failure. Upstream reads `launchctl list`, which has neither field. | below |
| **Prometheus `/metrics`** | launchd state as time series — `deployboard_job_up`, `_runs`, `_last_exit_code`, `_disabled`, `_probe_*`. The only launchd→metrics exporter in this niche. | [`docs/METRICS.md`](docs/METRICS.md) |
| **Telegram alerts + per-app toggles** | Transition-based alerts (error / offline / port unreachable / restart storm / recovery), 🔔 switch per application, cooldown + quiet hours, token from the macOS Keychain. | [`docs/ALERTS.md`](docs/ALERTS.md) |
| **One-command macOS install** | Homebrew cask, `install.sh` / `install-release.sh`, or GitHub Release binaries — LaunchAgent with `KeepAlive`. | [`docs/INSTALL-macos.md`](docs/INSTALL-macos.md) · [`docs/RELEASE.md`](docs/RELEASE.md) |
| **Write access you control** | Read-only mode refuses `reload`/`start`/`stop` server-side and disables the row buttons; the settings switch turns it back on (writes `read_only` to `config.json`, applied in ~2s, behind a confirmation). `--read-only` on the command line stays a **hard lock** the UI cannot undo, so a monitoring box still cannot be talked into killing a service. | `docs/INSTALL-macos.md` |
| **Multi-language UI** | English / 日本語 / 繁體中文 / 简体中文 for the embedded dashboard and the landing site. Auto-detects from the browser, persists the choice, and exposes a header + Settings switcher. | below |
| **`config.json` hot reload** | Edit the file (or click a classify action) → applied in ~2s, no restart. | `docs/INSTALL-macos.md` |
| **Honest Stop, Disable/Enable** | Stop picks the mechanism that holds: `bootout` for a KeepAlive job (a signal would just restart it — the job stays listed as `offline` and Start bootstraps it back), `SIGTERM` otherwise. `Disable` retires a job across logins and reboots; `Enable` undoes it. | below |
| **Verified actions** | Every action re-reads launchd before answering (`verified.ok` + a `verdict` when it did not take) and the UI re-checks two seconds later, so "succeeded" never means "we sent a command". | below |
| **Group bulk actions** | Restart / Start / Stop / Retire every job in a group in one click, with the affected labels listed in the confirmation. | below |
| **Restart-loop banner** | Churn measured over the dashboard's own window (not the cumulative counter), surfaced above the table with Stop/Retire actions and a per-row sparkline. | below |
| **Uptime + retirement history** | `ps`-derived uptime per running job, and a local log of when each job was retired and by whom — launchd remembers the override, not the decision. | `~/.config/deployboard/retirements.json` |

Seventh status value, on top of upstream's six:

| Status | Meaning |
|--------|---------|
| `disabled` | listed by launchd but `launchctl print-disabled` says disabled and it has no live pid — retired on purpose, not a failure |

## Upstream features

## Features

**Time-aware job status** — Six status values distinguish scheduled jobs waiting for their next trigger, recently-completed runs, truly offline plists, and classic running / stopped / error states. The backend derives these from PID, exit code, plist schedule shape, and a configurable recent-completion window.

| Status | Meaning |
|--------|---------|
| running | PID > 0 |
| scheduled | PID = 0, clean exit, has `StartInterval` / `StartCalendarInterval` / `RunAtLoad`, no recent log mtime |
| completed | PID = 0, clean exit, log mtime within `--recent-window` (default 10m) |
| stopped | PID = 0, clean exit, no schedule and no recent log mtime |
| error | Non-zero last exit status |
| offline | plist file exists but `launchctl list` does not return the label |

**Next / last run heuristics** — Each job carries optional `nextRunAt` (computed from `StartCalendarInterval` or `StartInterval`) and `lastRunAt` (newer of stdout / stderr mtimes). Status badges show both on hover.

**Real-time monitoring** — SSE (`/api/events`) pushes the full job list every 5 seconds; the frontend refreshes via Preact Signals without a manual reload.

**Service control** — Start, stop, and reload LaunchAgents with one click. Confirmation dialogs prevent accidental operations.

**Log viewer** — Tail stdout / stderr log files directly in the browser. Load up to 10,000 lines per file.

**Diagnostics** — 6 automated health checks per job:

| Check | What it verifies |
|-------|-----------------|
| Exit Code | Maps exit code to human-readable explanation (e.g. 127 = command not found) |
| Program Exists | Executable path exists on disk |
| Program Executable | File has execute permission |
| Plist Owner | Plist owned by current user |
| Plist Permissions | No group/world write bits (security) |
| Log Path | Parent directories for log files exist |

**Job classification** — Each job is automatically categorized based on label prefix and domain:

| Category | Rule | Badge color |
|----------|------|-------------|
| Mine | `domain=user` and label does not start with `com.apple.` | Blue |
| System | Label starts with `com.apple.` | Gray |
| 3rd-party | `domain=global` and label does not start with `com.apple.` | Purple |

**Multi-dimensional filtering** — Four filter dimensions that compose as AND:

- **Category chips** — All / Mine / System / 3rd-party, each showing count badge
- **Status tabs** — All / Running / Scheduled / Completed / Stopped / Error / Offline, each showing count
- **Only Mine toggle** — One-click shortcut to show only user-created jobs (persisted to localStorage)
- **Search** — Label substring filter, composable with all above

**Multi-language UI** — Dashboard and marketing site ship in English, Japanese, Traditional Chinese, and Simplified Chinese. The dashboard picks a locale from `localStorage` (`deployboard:locale`), then `navigator.language`, then English; switch anytime from the header control or Settings → Language. The landing site mirrors the same four locales at `/`, `/ja/`, `/zh-Hant/`, and `/zh-Hans/`.

## Install

**v0.0.2** ships three supported paths. Maintainers: how releases stay in sync is in [`docs/RELEASE.md`](docs/RELEASE.md).

### Homebrew (recommended if you already use brew)

```bash
brew install --cask francistse/tap/deployboard
```

Tap: [`francistse/homebrew-tap`](https://github.com/francistse/homebrew-tap) (cask updated automatically on each tagged release).

**Do not run `brew install RoboZephyr/tap/launch-pilot`** — that tap is upstream and installs launch-pilot, not Deployboard.

### GitHub Release binary (no Go, no clone)

```bash
curl -fsSL https://raw.githubusercontent.com/francistse/deployboard/main/install-release.sh | bash
```

Downloads the darwin archive for your CPU from the latest GitHub Release, then runs `install.sh --binary` (LaunchAgent + health check). Pin a version with `VERSION=v0.0.2` or pass `--from-release v0.0.2` after `bash -s --`.

You can also download `deployboard_*_darwin_*.tar.gz` from the [Releases](https://github.com/francistse/deployboard/releases) page and run `bash install.sh --binary ./deployboard`.

### From a clone (`install.sh`)

```bash
git clone https://github.com/francistse/deployboard.git
cd deployboard
bash install.sh                      # = make install-macos; builds with Go
# or, use a published binary without building:
bash install.sh --from-release       # latest
bash install.sh --from-release v0.0.2
```

Installs to `~/bin/deployboard`, writes LaunchAgent `com.deployboard.agent` (`RunAtLoad` + `KeepAlive`), health-checks. No `sudo`. Flags: `--dry-run`, `--uninstall`, `--purge`, `--prefix`, `--port`, `--config`, `--binary`, `--from-release`, `--no-agent`. Full detail: [`docs/INSTALL-macos.md`](docs/INSTALL-macos.md).

### Build from source only

```bash
git clone https://github.com/francistse/deployboard.git
cd deployboard
make build
```

Produces `./deployboard` with the frontend embedded — no separate UI build.

## Usage

```bash
deployboard                          # random port, auto-opens browser
deployboard --port 8080              # listen on explicit port
deployboard --no-open                # start server without opening browser
deployboard --recent-window 30m      # mark jobs as "completed" if they ran in the last 30m
deployboard --version                # print version and exit
```

The server binds to `127.0.0.1` (localhost only). Press `Ctrl+C` to shut down gracefully (5-second timeout).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port` | int | `0` (random) | Listen port on `127.0.0.1` |
| `--no-open` | bool | `false` | Skip auto-opening the browser |
| `--recent-window` | duration | `10m` | How long after `lastRunAt` a job still shows as `completed`. Valid range: `1m`–`24h`. Accepts any Go duration string (`30m`, `1h30m`, `24h`). Outside the range exits with a clear stderr message. |
| `--version` | bool | `false` | Print version and exit |

## API

All endpoints return JSON. Labels must match `[a-zA-Z0-9._-]+`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/jobs` | List all jobs including offline plists |
| GET | `/api/jobs/{label}` | Get single job details |
| POST | `/api/jobs/{label}/start` | `launchctl kickstart` |
| POST | `/api/jobs/{label}/stop` | `launchctl kill SIGTERM` |
| POST | `/api/jobs/{label}/reload` | `launchctl bootout` + `bootstrap` |
| GET | `/api/jobs/{label}/logs?lines=200` | Tail stdout / stderr (max 10,000 lines) |
| GET | `/api/jobs/{label}/diagnose` | Run the 6 diagnostic checks |
| GET | `/api/events` | SSE stream — pushes full job list every 5s |

### Job JSON shape

```json
{
  "label": "com.example.backup",
  "pid": 0,
  "lastExitStatus": 0,
  "status": "scheduled",
  "plistPath": "/Users/you/Library/LaunchAgents/com.example.backup.plist",
  "program": "/usr/local/bin/backup",
  "programArgs": ["/usr/local/bin/backup", "--daily"],
  "standardOutPath": "/tmp/backup.out",
  "standardErrPath": "/tmp/backup.err",
  "runAtLoad": false,
  "keepAlive": false,
  "domain": "user",
  "nextRunAt": "2026-04-19T03:00:00Z",
  "lastRunAt": "2026-04-18T03:00:04Z",
  "startInterval": 0,
  "startCalendarInterval": [{ "Hour": 3, "Minute": 0 }]
}
```

`nextRunAt`, `lastRunAt`, `startInterval`, and `startCalendarInterval` are optional (omitted when empty). Old clients that ignore these fields continue to work.

## Architecture

```
Browser (Preact + Signals, no build step)
    |
    +-- SSE (/api/events)     <- real-time job status push
    +-- REST (/api/jobs/...)  <- actions, logs, diagnostics
    |
    v
Go HTTP Server (net/http, embedded frontend via go:embed)
    |
    +-- Service layer         <- merges launchctl list + plist data
    |     NextCalendarFire / NextIntervalFire / LastRunAt
    |     DeriveStatus(pid, exit, plist, lastRunAt, now, window)
    +-- Diagnose engine       <- 6 read-only health checks
    |
    v
launchctl CLI + plist files
    +-- ~/Library/LaunchAgents     (user domain)
    +-- /Library/LaunchAgents      (global domain)
```

**Frontend stack**: Preact + Signals + htm, vendored as ESM modules via import map. No bundler, no transpiler — browser-native ES modules.

**Plist scanning**: Reads `~/Library/LaunchAgents` and `/Library/LaunchAgents` with mtime-based caching. Plists that exist on disk but are absent from `launchctl list` are appended as synthetic `offline` jobs so the UI can surface unloaded plists.

**Single binary**: All frontend assets (HTML, JS, CSS) are embedded in the Go binary via `go:embed`. No external files needed at runtime.

**Client-side filtering**: All filter/search logic runs in the browser using Preact Signals `computed()`. The backend pushes the full job list (~50 KB for 300 jobs) via SSE; the filter pipeline (onlyMine → category → status → search) runs in < 1ms on 300 items.

**Zero new Go deps**: calendar / interval next-fire calculation uses the standard-library `time` package only. No cron library.

## Development

### Prerequisites

- macOS (uses `launchctl`)
- Go 1.26.2+ (matches `go.mod`)
- Node.js 22+ for frontend and browser tests

### Commands

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

### Project structure

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
copy lives in `landing/src/content.ts`.

```bash
cd landing
npm ci
npm run dev          # local preview
npm run build        # static export -> landing/out
```

Enable it once per repository: **Settings → Pages → Source: GitHub Actions**.
See [`landing/README.md`](landing/README.md) for the base-path details and the
non-GitHub hosting options.

## License

MIT
