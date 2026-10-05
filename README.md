# Deployboard

[![CI](https://github.com/francistse/deployboard/actions/workflows/ci.yml/badge.svg)](https://github.com/francistse/deployboard/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Latest release](https://img.shields.io/github/v/release/francistse/deployboard)](https://github.com/francistse/deployboard/releases)
[![Platform: macOS 13+](https://img.shields.io/badge/platform-macOS%2013%2B-lightgrey.svg)](docs/INSTALL-macos.md)
[![Live briefing](https://img.shields.io/badge/live%20briefing-github%20pages-22D3EE)](https://francistse.github.io/deployboard/)

**Launchd has no dashboard. This is the console for managing your own launchd services in the browser, and it exports Prometheus metrics.**

**[Open the live briefing](https://francistse.github.io/deployboard/)** — the color version of this page. Click Ours, Other, Noise, then `/metrics` and Telegram.

launchd is hard to read from the terminal. One command installs the console. The browser shows status, logs, and alerts.

```bash
brew install --cask francistse/tap/deployboard
```

<img src="docs/images/demo.gif" alt="Deployboard preview: switch Ours, Other, and Noise, then Prometheus /metrics and a Telegram alert" width="960">

## Ours / Other / Noise

Your deployments are the default view. Other is unclassified. Noise is Dropbox, Chrome, Spotify, and the rest — one click away, never in the way.

**Ours**

<img src="docs/images/ours.png" alt="Ours view: your own launchd services" width="960">

**Other**

<img src="docs/images/other.png" alt="Other view: one unclassified job" width="960">

**Noise**

<img src="docs/images/noise.png" alt="Noise view: vendor and system jobs" width="960">

## The only launchd → Prometheus exporter

The same process serves `GET /metrics`. No sidecar.

<img src="docs/images/metrics.png" alt="Prometheus /metrics for launchd jobs" width="960">

## Telegram alerts

One message when a job breaks, one when it comes back. A switch per application.

<img src="docs/images/telegram.png" alt="Telegram alert for a restart storm" width="960">

Same console, live: [francistse.github.io/deployboard](https://francistse.github.io/deployboard/).

## Other ways to install

```bash
curl -fsSL https://raw.githubusercontent.com/francistse/deployboard/main/install-release.sh | bash
```

Or from a clone: `bash install.sh`. Flags, LaunchAgent, and uninstall: [`docs/INSTALL-macos.md`](docs/INSTALL-macos.md).

## Docs

| | |
|---|---|
| Install, config, uninstall | [`docs/INSTALL-macos.md`](docs/INSTALL-macos.md) |
| Prometheus series | [`docs/METRICS.md`](docs/METRICS.md) |
| Telegram alerts | [`docs/ALERTS.md`](docs/ALERTS.md) |
| Statuses, API, architecture | [`docs/REFERENCE.md`](docs/REFERENCE.md) |
| Development | [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) |
| Releases | [`docs/RELEASE.md`](docs/RELEASE.md) |
| Fork and rebase | [`docs/UPSTREAM.md`](docs/UPSTREAM.md) |
| Next steps | [`ROADMAP.md`](ROADMAP.md) |
| Product facts | [`PRODUCT-STATE.md`](PRODUCT-STATE.md) |

MIT. Fork of [launch-pilot](https://github.com/RoboZephyr/launch-pilot). Upstream behaviour stays; the fork only adds.
