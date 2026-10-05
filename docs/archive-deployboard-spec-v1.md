# Deployboard — SPEC (v1)

**Goal**: one localhost page that answers "what is deployed on a developer Mac, what is running
right now, and what is intentionally disabled" — plus a Prometheus `/metrics` endpoint so
Grafana or any scrape target can graph history later.

**Non-goals (v1)**: alerting, uptime history storage, auth, remote access, editing/starting
services from the UI (read-only).

## Why not Gatus

Gatus probes HTTP/ICMP/TCP/DNS/TLS/UDP/SSH only — it has **no shell/exec monitor**, so it
cannot see launchd state, exit codes, restart counts, disabled plists, or services that
listen on no port at all (e.g. `com.example.app.uat.collector`). Deployboard reads
`launchctl` as the source of truth. Gatus stays the alerting layer (Telegram) if added later.

## Source of truth (exact commands)

1. **Plists**: `~/Library/LaunchAgents/*.plist` and `*.plist.disabled`
   (`*.plist.disabled` = intentionally retired; report as `disabled-file`).
   Parse with stdlib `plistlib` → `Label`, `ProgramArguments`, `WorkingDirectory`,
   `KeepAlive`, `RunAtLoad`, `StandardOutPath`, `StandardErrorPath`.
2. **Live state per label**: `launchctl print gui/<uid>/<label>` — parse these lines
   (leading tabs/spaces, `key = value`):
   - `state = running|not running`
   - `pid = 1234`
   - `runs = 28`
   - `last exit code = 0`  ← absent when the job has never exited (report as `null`)
   - `path = /path/to.plist`
   - `working directory = ...`
   Verify a known sample: `com.example.app.uat.api` → `state = running`, `runs = 28`,
   `pid = 73087`, no `last exit code` line.
3. **Disable intent**: `launchctl print-disabled gui/<uid>` → lines
   `"<label>" => disabled|enabled`. A label listed `disabled` = intentionally off.
   A `print` failure with "Could not find service" = **not loaded** (distinct state).
4. **Ports**: infer from `ProgramArguments` (patterns: `--port <N>`, `-p <N>`, `:PORT` in a
   `--host`-less arg, `--hostname`). Support multiple ports per service. No port → `probe: none`.

## States (each row gets exactly one)

| state | meaning |
|---|---|
| `running` | launchd `state = running` + pid alive |
| `loaded` | loaded, not running, no exit code yet (waiting for fire) |
| `crashed` | loaded, no live pid, `last exit code != 0` |
| `restarting` | `KeepAlive` + high `runs` but no pid right now |
| `disabled` | `print-disabled` says disabled, or `*.plist.disabled` on disk |
| `not-loaded` | plist exists but `launchctl print` cannot find it |
| `unreachable` | running but the port probe fails |

## Probes

- HTTP: `GET http://127.0.0.1:<port>/` and `GET http://[::1]:<port>/` (some services bind
  IPv6 only) with 3s timeout, `urllib.request`. **Any HTTP status < 500 = reachable** —
  `404` on `/` is normal for FastAPI (`/docs` is 200). Record status code + latency ms.
- Non-HTTP ports (e.g. a service that answers 403 to an unauthenticated probe, Chrome CDP
  9222, Tailscale, Dropbox): still HTTP-probe; treat `403`/`401`/`302` as reachable.
- Cache all probe results for `--probe-ttl` seconds (default 10) so `/metrics` scrapes do
  not hammer services. State (`launchctl`) cache: 5s.

## Endpoints

| path | content |
|---|---|
| `/` | HTML dashboard (below) |
| `/api/state` | full JSON snapshot (`generated_at`, `groups[]`, `services[]`, `summary{}`) |
| `/metrics` | Prometheus text exposition v0.0.4, `Content-Type: text/plain; version=0.0.4` |
| `/healthz` | `200 ok` (plain text) |

Metrics (label cardinality must stay bounded; no timestamps/paths as labels):

```
deployboard_up                                 1
deployboard_scrape_duration_seconds            gauge
deployboard_services_total{state="running"}    gauge  (one line per state value)
deployboard_service_up{label,app}              1|0     (launchd running)
deployboard_service_disabled{label,app}        1|0
deployboard_service_restarts{label,app}        counter (from `runs`)
deployboard_service_last_exit_code{label,app}  gauge   (omit when null)
deployboard_service_probe_reachable{label,port} 1|0
deployboard_service_probe_status{label,port}   gauge   (HTTP code; omit when none)
deployboard_service_probe_latency_seconds{label,port} gauge (omit when none)
```

Escape label values (`\`, `"`, newline). Every metric must have `# HELP` + `# TYPE`, one
blank line between families, file ends with a single newline. No duplicate series.

## UI — reference the Gatus look (dark, developer status board)

Match Gatus's design language (its CSS is Tailwind + shadcn HSL vars). Use these exact tokens:

```css
--background: 222.2 84% 4.9%;   --foreground: 210 40% 98%;
--card: 222.2 84% 4.9%;         --card-foreground: 210 40% 98%;
--muted: 217.2 32.6% 17.5%;     --muted-foreground: 215 20.2% 65.1%;
--border: 217.2 32.6% 17.5%;    --radius: 0.5rem;
--ok: 34 197 94;  --bad: 239 68 68;  --warn: 234 179 8;  --idle: 107 114 128;
font-family: Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
```

Layout (mirrors Gatus's results list, not a copy of its assets or code):

- Sticky header: title `Deployboard`, subtitle `Local machine · <n> services`, right side: a
  `127.0.0.1:9400` badge, last-refresh time, `auto` toggle.
- Summary strip: 4-5 stat cards (running / crashed / disabled / not-loaded / probed OK).
- One card (`border: 1px solid hsl(var(--border)); border-radius: var(--radius)`) **per group**
  (e.g. `Example App — UAT`, `Example App — Demo`, `Example Web`, `Infra`), with the group
  name as the card header and a per-group OK/failed count.
- Inside a group, one row per service: 8px status dot (green/red/amber/gray), label (mono),
  friendly name, port chips, `state` pill, `runs` and `last exit` (mono, red if non-zero),
  probe latency + status code, and a `log` link (`file://` to `StandardOutPath`) when known.
- Row background tint on `crashed` (`rgba(239,68,68,.08)`) and `disabled` (reduced opacity).
- `/metrics` and `/api/state` links in the footer; footer also shows the config/discovery
  timestamp.
- Auto-refresh: `fetch('/api/state')` every 15s (`?refresh=5..60`), no full page reload.
- Responsive down to phone width (rows stack: dot+name on line 1, meta on line 2).
- Light theme via `@media (prefers-color-scheme: light)` overriding the same vars (Gatus does
  this too) — dark is the default.

## Grouping config

`config.toml` (stdlib `tomllib`), fallback to built-in defaults if absent:

```toml
[[group]]
name = "Example App — UAT"
match = ["com.example.app.uat.*"]
[[group]]
name = "Example App — Demo"
match = ["com.example.app.demo.*"]
[[group]]
name = "Example Web"
match = ["com.example.web.*"]
[[group]]
name = "Local Agent"
match = ["com.example.agent.*"]
[[group]]
name = "Infra"
match = ["com.example.infra.*", "com.ollama.*", "org.virtualbox.*"]
```

Unmatched labels go to a final `Ungrouped` group. Group titles are per-service overridable.

## CLI

```
deployboard.py [--host 127.0.0.1] [--port 9400] [--config config.toml]
               [--probe-ttl 10] [--state-ttl 5] [--no-probe] [--json] [--once]
```

- `--json` prints the snapshot to stdout and exits (no server) — used by tests and cron.
- `--once` prints the human table and exits.
- Stdlib only (`http.server`, `plistlib`, `subprocess`, `urllib.request`, `tomllib`, `json`,
  `argparse`, `threading`). No third-party deps. Python 3.11+.

## Tests (tests/test_deployboard.py, pytest-style, runnable with `python -m unittest`)

1. `parse_print_output()` on a captured real `launchctl print` sample (running w/ runs+pid,
   not-running w/ exit code, never-exited) → exact dicts.
2. `extract_ports()` on real `ProgramArguments` arrays (uvicorn `--port 8001`; next
   `-H localhost -p 3100`; `--hostname 127.0.0.1 --port 3201`; no-port collector) → expected.
3. `parse_disabled()` on real `print-disabled` output → set of disabled labels.
4. `classify_state()` truth table for all 7 states.
5. `render_metrics()` output parses as valid exposition: every family has HELP+TYPE, ends
   with exactly one `\n`, no duplicate series, label escaping works with a quote in a label.
6. `build_snapshot()` with a fake plist dir + fake launchctl runner → groups and summary
   counts correct, `*.plist.disabled` file reported `disabled`.
7. HTTP: start the server on an ephemeral port in a thread; assert `/healthz` 200,
   `/api/state` valid JSON with `summary`, `/metrics` 200 + `text/plain`.

## Definition of done

- `python3 -m unittest discover -s tests -v` all green.
- `python3 deployboard.py --json | python3 -m json.tool` valid JSON listing ≥15 services.
- `python3 deployboard.py --once` prints all groups incl. Example App UAT/Demo, Example Web, Infra.
- Live server: `/`, `/api/state`, `/metrics`, `/healthz` all 200 (verified with curl).
- `/metrics` valid Prometheus text (verified by parsing every line).
- launchd agent installed, `KeepAlive`, `RunAtLoad`, listening on
  `127.0.0.1:9400`; `kill -9` the PID → it comes back within 10s (self-heal verified).
- No service edited, restarted, or written to by this project (read-only by design).
