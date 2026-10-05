# Installing Deployboard on macOS

Two paths — pick one. Both of them end with the same thing: a `launchd` LaunchAgent
(`com.deployboard.agent`) that starts at login and self-heals (`KeepAlive`), serving
`http://127.0.0.1:9410`.

## Option 1 — the install script (recommended)

```bash
cd ~/Projects/10_deployboard
make install-macos                 # = bash install.sh
```

What it does:

1. builds the binary (`make build`) unless you pass `--binary <file>` (Go not required then)
2. installs it to `~/bin/deployboard` (override with `--prefix`) and leaves `~/bin/launch-pilot` as a symlink to that binary for one release
3. creates `~/.config/deployboard/config.json` from the repo default if absent (after moving `~/.config/launch-pilot/` when the new directory does not already exist)
4. writes `~/Library/LaunchAgents/com.deployboard.agent.plist`
   (`RunAtLoad` + `KeepAlive`, `WorkingDirectory` = `$HOME`, logs to
   `~/Library/Logs/deployboard.log`)
5. `plutil -lint`, then `launchctl bootout … || true` + `launchctl bootstrap gui/$UID …`
6. prints a health check (`curl … /`) and the dashboard URL

Flags: `--prefix <dir>`, `--port <n>`, `--config <file>`, `--binary <file>`, `--no-agent`,
`--dry-run`, `--uninstall`, `--purge`.

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

## Option 2 — manual

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

## Homebrew — planned, not yet available

Phase 1 ships `install.sh` only. `Formula/deployboard.rb` is the starting point for a later phase,
not a working install: its `url`/`homepage` point at `github.com/francistse/deployboard`, but
there is no released tarball or tap yet, so
`brew install --build-from-source ./Formula/deployboard.rb` fails at fetch.

Do **not** run `brew install RoboZephyr/tap/launch-pilot` — that tap is upstream's and installs
upstream launch-pilot, not Deployboard.

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
  "alerts": {"enabled": true, "cooldown_seconds": 900, "run_storm_delta": 25}
}
```

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
