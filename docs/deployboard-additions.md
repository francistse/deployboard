# Deployboard — fork additions (v2, supersedes v1)

Base: `RoboZephyr/launch-pilot` @ `5d9c07d` (MIT). This document is the **authoritative scope**
for the fork's additions. Upstream behaviour stays unless stated here. Go 1.26, module path
`github.com/A404coder/deployboard`.

## R0 — The problem with upstream's default view (why this fork exists)

Upstream classifies by domain: "Mine" = user domain and label doesn't start with `com.apple.`.
On a typical workstation that means Dropbox, Google updaters, Spotify, Chrome, CopilotForXcode,
Tailscale, Docker, todesktop all appear under "Mine" — 556 jobs, mostly noise. What he wants to
answer is: **"which of MY deployed applications are up, which are broken, which are
intentionally off"** — Example App, Example Web, and the local infra that lives outside the project tree.

So the default view becomes an **explicit inventory allowlist**, not a domain heuristic.

## R1 — Inventory classification (new, highest priority)

`internal/inventory/inventory.go`:

```go
type Category string
const (
    CategoryOurs  Category = "ours"   // matched inventory.ours
    CategoryNoise Category = "noise"  // matched inventory.hidden
    CategoryOther Category = "other"  // everything else
)
```

- Glob matching with `path.Match`-style semantics over the label (support `*` only; a bare
  label matches itself). Rules are evaluated in order: `ours` first, then `hidden`, else `other`.
- Job JSON gains: `"category"` (`ours|noise|other`), `"group"` (human group name, see below),
  `"runs"` (int), `"disabled"` (bool), `"printState"` (string), `"restartWarn"` (bool).
- Grouping config (order matters, first match wins) — default `config.json`:

```json
"inventory": {
  "show_noise_default": false,
  "restart_warn_threshold": 50,
  "groups": [
    {"name": "Example App", "match": ["com.example.app.*"]},
    {"name": "Infra",       "match": ["com.example.infra.*", "com.ollama.*"]}
  ],
  "ours":   ["com.example.*"],
  "hidden": ["com.apple.*", "application.*", "com.google.*", "com.dropbox.*", "com.spotify.*",
             "com.docker.*", "com.github.CopilotForXcode.*", "io.tailscale.*", "com.todesktop.*",
             "com.microsoft.*", "com.openssh.*", "org.virtualbox.*"],
  "derive_roots": ["~/Projects", "~/src", "~/code", "~/Developer", "~/work"]
}
```

- Jobs that match neither list are `other` and are shown only when the user asks.
- `/api/jobs` keeps returning ALL jobs (each tagged), so nothing becomes invisible to
  automation — the filtering is a view concern, plus a query param `?category=ours` filter.
- New `GET /api/inventory` → `{"groups":[{"name":..., "labels":[...], "ourCount":n}], "ours":n,
  "other":n, "noise":n, "unmatched":[...]}` so the UI never hardcodes counts.

### UI (web/)
- **Default view = Ours.** Chips become: `Ours (n)` · `Other (n)` · `All (n)`, with Ours
  selected on first load. Persist the choice in `localStorage` (`lp.categoryFilter`).
- A `Show system & vendor jobs` switch (persisted) that reveals `noise`; noise rows render at
  60% opacity with a `SYSTEM`/`VENDOR` badge so they read as background.
- Group headers (R1 groups) with a per-group health line: `3 running · 1 crashed · 1 disabled`.
- A new **`Runs`** column showing `job.runs`, red when `job.restartWarn` (>= threshold) — this
  is how the over 12k-restart web app becomes obvious at a glance.
- Default sort: `ours` first, then failing jobs, then by group.

## R2 — `launchctl print` enrichment (true state, runs, disabled)

Upstream uses `launchctl list`. `launchctl list` has no restart counter and no disabled flag.
Add `internal/launchd/print.go`:

- `PrintInfo(domain, label) (*PrintInfo, error)` runs `launchctl print gui/<uid>/<label>` and
  parses (tab/space tolerant, `key = value`):
  - `state = running|not running` → `PrintState`
  - `pid = N`
  - `runs = N` → restart counter
  - `last exit code = N` (absent when the job never exited → `HasExit bool`)
  - `path = /…plist`
  - A "Could not find service" / non-zero exit means not loaded → `ErrNotLoaded`.
- `DisabledLabels(domain) (map[string]bool, error)` runs **once per sweep**
  `launchctl print-disabled gui/<uid>` and parses `"<label>" => disabled|enabled`.
  Never call it per label (subprocess storm — upstream already memoises sweeps for this reason).
- Cost control: `PrintInfo` is one extra subprocess per *inventory* label only (`ours` +
  `other`), never for `noise` jobs. Cache in the same TTL memo as the sweep
  (`--print-ttl`, default 15s). Batch per sweep, not per request.

### Status layering (must be exact)

New status const `StatusDisabled JobStatus = "disabled"`.

Order of determination for a job:
1. listed by `launchctl list` **and** disabled map says `disabled` **and** pid == 0 → `disabled`
2. pid > 0 → `running`
3. not listed, but plist on disk → `offline`
4. otherwise upstream `DeriveStatus` (scheduled / completed / stopped / error) unchanged

`disabled` must NOT be reported as `error`/`offline` — that is the whole point (5
`com.example.worker.*` plists are retired on purpose). If a disabled job is nonetheless running,
keep `running` and set `disabled: true` (a flag, not a status).

- UI: new `Disabled` status tab + pill colour (grey/amber, not red). Row shows a `disabled`
  badge and, by default, alerting is **off** for `disabled` jobs (no bell).

## R3 — Prometheus `/metrics`

`internal/metrics/metrics.go` + route `GET /metrics` (plain text,
`Content-Type: text/plain; version=0.0.4; charset=utf-8`, `Cache-Control: no-store`).

Metric names (prefix `deployboard_` so a Grafana board never collides with other exporters):

```
deployboard_up{version}                                   1
deployboard_scrape_duration_seconds                       gauge
deployboard_jobs_total{status="running"}                  gauge  (one line per observed status)
deployboard_jobs_total{category="ours"}                   gauge  (one line per category)
deployboard_job_up{label,category,group}                  1|0   (status == running)
deployboard_job_runs{label,category,group}                gauge (restart counter)
deployboard_job_last_exit_code{label,category,group}      gauge (omitted when never exited)
deployboard_job_disabled{label,category,group}            1|0
deployboard_job_probe_reachable{label,port}               1|0   (omitted when no port)
deployboard_job_probe_status{label,port}                  gauge (omitted when no port)
deployboard_job_probe_latency_seconds{label,port}         gauge (omitted when no port)
deployboard_alerts_enabled{label,category}                1|0
```

- Label values escaped (`\` → `\\`, `"` → `\"`, newline → `\n`). One `# HELP` + `# TYPE` per
  family, families sorted, exactly one blank line between, output ends with a single `\n`,
  no duplicate series.
- Port probe: infer ports from `ProgramArguments` (`--port N`, `-p N`, `--hostname` aware),
  probe `http://127.0.0.1:<port>/` **and** `http://[::1]:<port>/` (some services bind IPv6 only),
  3s timeout, **any HTTP status < 500 = reachable**, TTL cache (`--probe-ttl`, default 10s).
  Disable with `--no-probe`. Probe only `ours` + `other` labels.
- `/metrics` must work with `--no-probe` (probe families simply absent).

## R4 — Telegram alerts with a per-application toggle

`internal/alerts/`:

- **State file** `~/.config/deployboard/alerts.json` (path configurable), atomic write
  (temp + rename), schema:
```json
{"version":1,
 "labels":{"com.example.app.uat.web":{"enabled":true,"mutedUntil":null,"lastStatus":"running",
                                 "lastSentAt":1790944000,"lastRuns":12207}},
 "updatedAt":1790944000}
```
  A label absent from the file uses `alerts.default_enabled`; defaults: **true** for `ours`,
  **false** for `other`, never alert for `disabled`/`noise`.
- **Transition engine**, evaluated on each sweep:
  - fire on entering `error` / `offline`, or when a probe flips reachable → unreachable
  - fire a **restart storm** alert when `runs` increases by ≥ `alerts.run_storm_delta`
    (default 25) since the last sweep — how `com.example.app.uat.web` (over 12k runs) gets flagged
    instead of silently churning
  - `notify_on_recovery` (default true) → one message when it returns to `running`
  - dedupe: never send the same (label, status) twice in a row; `cooldown_seconds`
    (default 900) per label; `quiet_hours` (default `23:30–08:00` Asia/Hong_Kong) suppresses
    new alerts but never suppresses a recovery (and never queues them)
- **Telegram sender**: `POST https://api.telegram.org/bot<token>/sendMessage` with
  `{"chat_id": ..., "text": ..., "disable_notification": false}`; no `parse_mode` (avoid
  escaping bugs); 10s timeout; log only the HTTP status — **never the token or the full URL**.
- **Token from macOS Keychain, never from a file or env**:
  `exec.Command("/usr/bin/security", "find-generic-password", "-s", svc, "-a", acc, "-w")`
  with `context.WithTimeout(5s)`. Service/account from config
  (`alerts.telegram.keychain_service`, default `deployboard-telegram`,
  `keychain_account`, default `bot_token`). On a missing item → `alerts.telegram.enabled`
  degrades to false, one WARN line `telegram alerts disabled: keychain item not found`, server
  keeps running (no crash, no retry storm). `chat_id` is an identifier, not a secret → config
  (`alerts.telegram.chat_id`).
- **CLI**: `--telegram-test` → sends one test message and exits 0/1 with the HTTP result;
  `--telegram-status` → prints whether the keychain item is readable (yes/no, never the value).
- **API**:
  - `GET  /api/alerts` → `{"enabled":n,"disabled":n,"labels":{label:{enabled,mutedUntil,lastStatus,lastSentAt}}}`
  - `POST /api/alerts/{label}` body `{"enabled":bool}` → 200 with the new entry
  - `POST /api/alerts` body `{"labels":[...],"enabled":bool}` → bulk
  - `POST /api/alerts/{label}/test` → sends a test alert for that label
  - `GET /api/alerts/history?limit=50` → in-memory ring buffer of the last 50 sends
    (label, status, sentAt, ok) — no payload storage
- **UI**: a bell switch per row (`Alerts` column), a group-header "toggle all" control, a header
  summary `🔔 12 on · 3 off`, a `Test` action in the row menu, and an `Alerts` panel listing
  recent sends. Disabled/noise rows show no bell. Toggling must not require a page reload
  (optimistic update + rollback on non-200).

## R5 — Easy macOS install

- `install.sh` (bash, idempotent, **no sudo**, `set -euo pipefail`):
  - flags: `--prefix <dir>` (default `$HOME/bin`), `--port <n>` (default 9410),
    `--no-agent`, `--binary <file>` (skip build), `--uninstall`, `--purge`, `--dry-run`, `--help`
  - builds via `make build` when Go is present, else requires `--binary`
  - installs the binary to `$PREFIX/deployboard`
  - writes `~/Library/LaunchAgents/com.deployboard.agent.plist`:
    `RunAtLoad` + `KeepAlive` true, `ProgramArguments` all `<string>` elements (an
    `<integer>` makes `launchctl bootstrap` fail with error 5), `WorkingDirectory $HOME`,
    stdout/stderr `$HOME/Library/Logs/deployboard.log`
  - `plutil -lint`, then `launchctl bootout gui/$UID/<label> 2>/dev/null || true` and
    `launchctl bootstrap gui/$UID/<plist>`; print
    `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:<port>/` as the post-install
    health check plus the dashboard URL
  - `--uninstall`: `bootout`, `launchctl disable gui/$UID/<label>`, remove the plist, keep the
    binary unless `--purge`
  - ends by printing (never executing) the one-liner to store the Telegram token:
    `security add-generic-password -U -s deployboard-telegram -a bot_token -w "$TOKEN" -T /usr/bin/security`
- `Makefile`: add `install-macos` / `uninstall-macos` targets wrapping `install.sh`
  (keep upstream's `build`/`test`/`run`/`e2e`/`clean` intact).
- `Formula/deployboard.rb`: Homebrew formula that builds from source
  (`depends_on "go" => :build`, `system "go", "build", …`, `service do run [...] end`).
- `docs/INSTALL-macos.md`: the three install paths (script / brew local formula / manual
  `make build` + plist), plus uninstall and where logs live.

## R6 — Read-only mode

`--read-only` (+ config `read_only`): all mutating routes (`reload`/`start`/`stop`) return
`403 {"error":"read-only mode"}` server-side, and the UI disables those buttons and shows the
mode in the header badge. Default **false** (upstream behaviour). Guards the "monitoring box
must not be able to kill a service" case.

Two levels, so the guard survives contact with real use:

- `--read-only` (flag) is a **hard lock**: `GET/POST /api/settings/access` report
  `locked: true`, `source: "flag"`, and a POST returns `403 {"error":"write mode is locked by
  --read-only", "hint": …}` without touching `config.json`.
- `read_only: true` (config) is a **soft default**: the settings switch POSTs
  `{"read_only": false}`, which writes the key atomically (`inventory.SetReadOnly`, every other
  key preserved, temp+rename) and applies immediately through the same path as a hand edit.

The guard is evaluated per request against a live `AccessControl` interface (not a bool captured
at router construction), which is what makes the switch take effect on the next click rather than
the next restart. `GET /api/settings/access` returns
`{read_only, write_mode, locked, lock_reason, source}` and is what the UI reads on boot.

## R6b — Honest Stop, and a retirement log

`launchctl kill SIGTERM` on a KeepAlive job is not a stop, it is a restart: launchd
respawns it immediately (that is how a dev server reaches thousands of `runs`). So
`Stop` chooses by job shape:

| Job | Stop does | Result |
|---|---|---|
| `KeepAlive` (running, or between restarts with pid 0) | `launchctl bootout` | unloaded; stays down until Start or the next login |
| no `KeepAlive`, running | `launchctl kill SIGTERM` | process stops, job stays loaded (upstream behaviour) |
| no `KeepAlive`, loaded but idle | nothing | no-op, not a failure |
| not loaded | nothing | no-op |

Nothing disappears: the plist is untouched, so the job keeps its row and reads
`offline`; `Start` (`enable` + `kickstart`, or `bootstrap` when not loaded) brings it back.
`Disable` = `launchctl disable` + bootout, which survives login/reboot, and shows as the
fork's `disabled` status; `Enable` clears the flag without starting the job.

Because launchd stores only the override, the decision is recorded locally in
`~/.config/deployboard/retirements.json` (`internal/retire`) and surfaced as `retiredAt`,
so the row can say "retired 3 days ago" — and says nothing when the dashboard was not the
one that disabled it.

Actions also drop the `launchctl print` / `print-disabled` / `ps` memos before and after
running, or the UI reports the pre-click world for up to `printTTL` (a Start that kicks a
label launchd no longer knows; a Disable that still reads `offline`).

## R6c — Verified actions, group bulk actions, restart-loop detection

- `POST /api/jobs/{label}/{action}` answers with `verified: {ok, status, pid, loaded,
  disabled, keepAlive, verdict}` — a fresh read taken after the action. A 200 from
  launchctl means the command ran, not that the job is in the state you asked for. The UI
  additionally re-reads the job after 2s and reports "verified / not verified" in a toast.
- `POST /api/inventory/group-action {group, action}` applies reload/start/stop/disable/
  enable to every non-noise job in a group, returning per-label `{ok, note, error}` plus
  `succeeded/skipped/failed` counts. Jobs with nothing to do are *skipped*, not failed. It
  is read-only-guarded like every other mutating route.
- Restart churn is measured by the dashboard itself (`internal/launchd/history.go`): one
  observation per label per minute, 30 kept, negative deltas clamped to 0 (a reload resets
  launchd's counter). `restartsRecent` + `windowMinutes` + `runsSeries` ride along in the
  job JSON; the banner only fires on window data, never on the cumulative counter, so a
  job with 1207 lifetime restarts does not raise a permanent alert.
- Uptime comes from `ps -o etime=` (`internal/launchd/proc.go`), memoised per pid for
  `printTTL`, and only resolved for jobs that are not noise.

## R7 — Tests (must pass: `make test`)

Extend the Go suites (table-driven, no sleeps, no real launchctl/network — inject fakes):
1. `PrintInfo` parser: running-with-runs+pid, not-running-with-exit-code, never-exited
   (no `last exit code` line), and the `Could not find service` failure → `ErrNotLoaded`.
2. `DisabledLabels` parser on a real `print-disabled` sample (mixed enabled/disabled).
3. Status layering truth table for all 7 statuses incl. `disabled` + "disabled but running".
4. Inventory: glob matching, first-match ordering, `ours`/`hidden`/`other`, grouping, counts.
5. Alerts: default-on for ours / off for other & disabled; transition fires once; repeat status
   does not re-fire; cooldown suppresses; quiet hours suppress new but allow recovery;
   run-storm detection at the delta boundary; mute/unmute persists (temp state file).
6. Telegram: payload JSON shape, no `parse_mode`, token never in logs (assert the redaction
   helper), missing keychain item → disabled + no error returned.
7. `/metrics`: valid exposition (HELP+TYPE per family, single trailing newline, no duplicate
   series, escaping with a `"` in a label), `--no-probe` omits probe families.
8. Handlers: `POST /api/alerts/{label}` toggles + persists; bad label → 400; `/api/alerts`
   counts; `--read-only` → 403 on reload/start/stop; `/api/inventory` counts;
   `GET/POST /api/settings/access` (status, toggle, refused-when-locked, malformed body → 400),
   and the toggler's live effect (stop refused → toggle → the same stop goes through);
   `inventory.SetReadOnly` preserves unrelated keys and creates a missing file;
   group-action forwarding, summary counts, malformed body, read-only guard, and
   `verifyAction`'s verdicts; `Stop` semantics per job shape (KeepAlive → bootout,
   plain → SIGTERM, idle → no-op); the stale-cache regressions after Stop and Disable;
   the restart-history window (deltas, counter reset, no invented rate) and `ps`
   elapsed parsing; the retirement log (round trip, re-enable clears it, corrupt file
   tolerated).
9b. **The keychain self-test must not prompt.** `internal/alerts` writes to a throwaway
   `security create-keychain` file (`DEPLOYBOARD_KEYCHAIN`), never the login keychain:
   each `go test` run is a new process to an item's ACL, and macOS answers that with a
   password dialog (and a 10s stall) every single time. `-T` is also only passed when
   *creating* an item — re-sending it on update makes macOS authorise an ACL change,
   which is the same dialog during a token rotation.
9. `install.sh --dry-run` prints the plist it would write and exits 0 (Go test via
   `exec.Command("bash","install.sh","--dry-run")`).

## R8 — Docs

- `README.md`: the public front page is the one-liner plus pictures of Ours / Other / Noise, `/metrics`, and Telegram. Maintainer detail lives in `docs/REFERENCE.md` and `docs/DEVELOPMENT.md`, and the fork's scope is this file.
- `docs/UPSTREAM.md`: base commit, MIT licence, `git remote add upstream …` + how to rebase.
- `docs/METRICS.md`: metric table + `prometheus.yml` scrape snippet + a Grafana query that
  graphs `deployboard_job_runs` to catch restart churn.
- `docs/ALERTS.md`: token → Keychain steps, chat id, per-app toggles, cooldown, quiet hours,
  run-storm threshold, `--telegram-test`.

## R9 — Path derivation, config hot reload, classify controls (added after review)

The `ours` allowlist alone drifts: a new deployment lands in `Other` until someone edits
config. Three additions remove that:

1. **`derive_roots` derivation.** A job whose plist `StandardOutPath` / `StandardErrorPath` /
   `WorkingDirectory` / `Program` / `ProgramArguments` live under a configured project root is
   classified `ours` automatically, with `categorySource: "derived_path"` so the UI can show
   "auto (path)" instead of "listed". Matching order stays: explicit `ours` → explicit
   `hidden` → derived → `other`. Roots accept a leading `~/` (expanded at match time —
   `filepath.Clean` does not). Degenerate roots (`/`, `.`, empty) are ignored so a typo cannot
   match every job. Measured on a developer Mac: **most deployments derived, 0 false positives
   across the vendor/job vendor bucket**, which is why the shipped `ours` list only needs
   `com.example.app.*`, `com.example.infra.*`, `com.deployboard.*`, `com.ollama.*`
   (infra that lives outside the project tree).
   `inventory.New` fills `DeriveRoots` from the defaults when a config file lacks the key, so
   an older `config.json` cannot silently disable derivation.
2. **`config.json` hot reload** — the server stats the file every 2s (`mtime + size`) and
   re-applies the classifier in place; no restart. `POST /api/inventory/reload` forces it.
   `SaveConfigFile` writes atomically and preserves every other key (`port`, `alerts`,
   `read_only`, …).
3. **Classify controls** — `POST /api/inventory/classify {"label","category"}` where category
   is `ours` | `noise` | `auto` (`auto` removes the manual pin so derivation decides again).
   It writes `config.json` and applies immediately. In the UI: a per-row
   `Classification: Auto (by path) / Ours (pin) / Hide (vendor/OS)` select in the expanded row,
   plus a provenance badge (`listed` / `auto (path)` / `unclassified`) next to the category.
   Clicking the locked `Noise` chip flips the "Show system & vendor jobs" toggle instead of
   being inert.

Upstream parity is preserved: the `All` chip still lists every launchd job (every job on the machine), and the
noise bucket is only *hidden by default*, never dropped from `/api/jobs`.

## R10 — Settings UI (Telegram token) + theme switch

1. **Telegram token management in the UI.** `GET/POST/DELETE /api/settings/telegram` and
   `POST /api/settings/telegram/test` back a **⚙ Settings** drawer with a password-type token
   field, chat-id field, `Save`, `Send test`, `Forget token`, and a stored/not-stored status.
   Security contract:
   - the token only ever travels IN (browser → loopback → Keychain); no response, log line or
     error message contains it (`alerts.Redact` is applied to every error path, and the
     handler test asserts the response has no `token`/`bot_token`/`secret` field);
   - `StoreKeychainToken` writes with `security add-generic-password -U -w … -T /usr/bin/security`
     (idempotent rotation, reader pinned so later headless reads never prompt);
   - `Engine.Configure` re-resolves the sender on every save, and the alert loop retries every
     ~60s while alerts are unavailable, so a token stored out-of-band (CLI) also starts working
     without a restart;
   - a **locked keychain** (security(1) waits for a password) is reported as
     `keychain is locked — unlock it … and retry` instead of hanging silently.
2. **Theme switch** — a three-way `System / Light / Dark` control in the header, persisted in
   `localStorage` under `deployboard:theme` and applied as `data-theme` on `<html>`, with the
   choice applied pre-paint by an inline script in `index.html` (no flash). `system` follows
   `prefers-color-scheme` live via a `matchMedia` listener. Light tokens become the CSS
   default; dark is the Gatus/shadcn palette under `[data-theme="dark"]` and inside the
   `prefers-color-scheme: dark` block for `[data-theme="system"]`.

## Non-goals (do NOT build in this pass)

- No multi-host, no auth/login, no database, no React rewrite of `web/` (keep upstream's
  vanilla JS + Preact Signals structure), no changes to `landing/`, no upstream feature removal.
