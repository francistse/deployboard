# Installing Deployboard on macOS

Three supported paths. All of them can end with the same LaunchAgent
(`com.deployboard.agent`, `RunAtLoad` + `KeepAlive`) serving
`http://127.0.0.1:9410` — Homebrew gives you the binary; `install.sh` /
`install-release.sh` also wire the agent.

How maintainers cut releases for all three paths: [`RELEASE.md`](RELEASE.md).

## Option 1 — Homebrew (recommended if you use brew)

```bash
brew install --cask francistse/tap/deployboard
deployboard --version
```

The cask is published into [`francistse/homebrew-tap`](https://github.com/francistse/homebrew-tap) by GoReleaser on each `v*` tag. Upgrade with `brew upgrade --cask deployboard`.

To also install the LaunchAgent (config + KeepAlive), clone or curl the installer and point it at the brew binary:

```bash
bash install.sh --binary "$(brew --prefix)/bin/deployboard"
# or, without a clone:
curl -fsSL https://raw.githubusercontent.com/francistse/deployboard/main/install-release.sh \
  | bash -s -- --binary "$(brew --prefix)/bin/deployboard"
```

**Do not** run `brew install RoboZephyr/tap/launch-pilot` — that tap is upstream's and installs upstream launch-pilot, not Deployboard.

The in-repo [`Formula/deployboard.rb`](../Formula/deployboard.rb) is a **local source-build helper only** (`brew install --build-from-source ./Formula/deployboard.rb`). The published install is the cask.

## Option 2 — GitHub Release binary (no Go)

```bash
curl -fsSL https://raw.githubusercontent.com/francistse/deployboard/main/install-release.sh | bash
```

What it does:

1. downloads `install.sh` + `config.example.json` from this repo
2. downloads `deployboard_<ver>_darwin_<amd64|arm64>.tar.gz` from the latest GitHub Release
3. runs `install.sh --binary` (LaunchAgent, health check)

Pin a version: `VERSION=v0.0.3 bash install-release.sh` or
`bash -s -- --from-release v0.0.3` after the curl pipe.

Manual path: download the archive from
[Releases](https://github.com/francistse/deployboard/releases), extract, then
`bash install.sh --binary ./deployboard`.

## Option 3 — install.sh from a clone

```bash
cd ~/Projects/deployboard   # or wherever you cloned
make install-macos          # = bash install.sh
```

What it does:

1. builds the binary (`make build`) unless you pass `--binary <file>` or `--from-release [tag]` (Go not required then)
2. installs it to `~/bin/deployboard` (override with `--prefix`) and leaves `~/bin/launch-pilot` as a symlink to that binary for one release
3. creates `~/.config/deployboard/config.json` from the repo default if absent (after moving `~/.config/launch-pilot/` when the new directory does not already exist)
4. writes `~/Library/LaunchAgents/com.deployboard.agent.plist`
   (`RunAtLoad` + `KeepAlive`, `WorkingDirectory` = `$HOME`, logs to
   `~/Library/Logs/deployboard.log`)
5. `plutil -lint`, then `launchctl bootout … || true` + `launchctl bootstrap gui/$UID …`
6. prints a health check (`curl … /`) and the dashboard URL

Flags: `--prefix <dir>`, `--port <n>`, `--config <file>`, `--binary <file>`,
`--from-release [tag]`, `--no-agent`, `--dry-run`, `--uninstall`, `--purge`.

No `sudo` is ever needed — everything is user-scope.

Uninstall:

```bash
make uninstall-macos           # keeps the binary
bash install.sh --uninstall --purge   # also removes ~/bin/deployboard and the ~/bin/launch-pilot symlink
```

Uninstall does all three of: `bootout`, `launchctl disable`, remove the plist. `disable`
persists across reboots, so remove-then-re-add needs `launchctl enable` (the install script
handles it). `--uninstall` removes `com.deployboard.agent`. It also retires
`com.deployboard.launch-pilot` when that older label is still loaded.

## Migrating from launch-pilot

`bash install.sh` is safe to re-run on a machine that already has the previous install.
Before it bootstraps `com.deployboard.agent` it:

1. `launchctl bootout` and `launchctl disable` `com.deployboard.launch-pilot` when that label is loaded, then deletes `~/Library/LaunchAgents/com.deployboard.launch-pilot.plist` and any stale `com.deployboard.launch-pilot.plist.tmp`
2. moves `~/.config/launch-pilot/` to `~/.config/deployboard/` when the new directory does not already exist (an existing new directory is left untouched)
3. installs `~/bin/deployboard` and points `~/bin/launch-pilot` at it with a symlink for one release
4. prints each of those steps

`--dry-run` prints the same steps and does not write under `~/Library/LaunchAgents/` (including no plist temp file there). Alert state and the retirement log also keep working if you start the new binary before the directory move: when `~/.config/deployboard/alerts.json` or `retirements.json` is missing and the file still exists under `~/.config/launch-pilot/`, the process reads the old file and logs that path once.

## Manual run (no LaunchAgent)

```bash
make build
./deployboard --config config.json --port 9410
```

Useful flags: `--port 0` (random free port), `--no-open` (do not open a browser),
`--no-probe` (skip port probes), `--probe-ttl 10s`, `--print-ttl 15s`, `--read-only`,
`--config <path>`.

`--read-only` makes `reload`/`start`/`stop` return `403 {"error":"read-only mode"}` — the right
mode for a box whose job is to watch, not to be able to kill.

Read-only has two levels, and the difference matters:

| Where it comes from | Effect | Can the UI undo it? |
|---|---|---|
| `--read-only` on the command line | hard lock; the server reports `locked: true` and `source: "flag"` | **No.** The settings switch is disabled and says why. |
| `"read_only": true` in `config.json` | soft default; `source: "config"` | **Yes** — Settings → Access → *Allow start / stop / reload*. |

The switch writes the `read_only` key back to `config.json` (atomically, every other key
preserved) and the running server picks it up through the same watcher that handles a hand
edit — about two seconds, no restart — after a confirmation dialog. Enabling write mode is
the one thing this dashboard can do to itself, so it is deliberately explicit; the header
badge (`✎ write mode` / `👁 read-only` / `🔒 read-only (locked)`) always shows which mode the
server is in, and the row buttons are disabled while it is read-only.

## Configuration

`config.json` (see the repo copy for the full default):

```json
{
  "port": 9410,
  "read_only": true,          // true = monitoring only; flip it in Settings → Access
  "print_ttl_seconds": 15,
  "probe_ttl_seconds": 10,
  "inventory": {
    "groups": [{"name": "Example App — UAT", "match": ["com.example.app.uat.*"]}],
    "ours": ["com.example.app.*", "com.ollama.*", "com.example.infra.one"],
    "hidden": ["com.apple.*", "com.dropbox.*", "com.google.*"],
    "derive_roots": ["~/Projects", "~/Projects/demo", "~/Projects/infra"],
    "restart_warn_threshold": 50
  },
  "alerts": {"enabled": true, "cooldown_seconds": 900, "run_storm_delta": 25},
  "desired": {
    "jobs": [
      {"match": "com.example.app.*", "status": "running", "max_restart_rate": 5}
    ],
    "groups": [
      {"name": "Example App — UAT", "status": "running", "probe_port": 8080}
    ]
  }
}
```

**Desired state** (`desired`): declare what Ours jobs *should* be. Matching uses the same
`path.Match` globs as inventory. Job rules win over group rules. Open drifts appear in the
dashboard banner and at `GET /api/drift`; `POST /api/drift/align` runs start/stop/enable/disable
behind the same verified-action + read-only gates. See [`ROADMAP.md`](../ROADMAP.md) Phase 1.

**This file is hot-reloaded** — an edit is applied within ~2s, no restart. The UI's classify
dropdown writes to it too (`POST /api/inventory/classify`), so pinning a new app never
requires restarting anything.

Logs: `~/Library/Logs/deployboard.log` (LaunchAgent) or the foreground terminal.

## Verifying the install

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9410/         # 200
curl -s http://127.0.0.1:9410/healthz                                   # ok
curl -s http://127.0.0.1:9410/api/inventory | python3 -m json.tool      # counts + sources
curl -s http://127.0.0.1:9410/metrics | head                            # Prometheus text
launchctl print gui/$(id -u)/com.deployboard.agent | grep state  # running
kill -9 "$(launchctl print gui/$(id -u)/com.deployboard.agent | awk '/pid =/{print $3}')"
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9410/         # 200 again (self-heal)
```
