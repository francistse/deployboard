# Deployboard roadmap

> **Wedge:** which of *MY* deployed applications are up, broken, or intentionally off —
> not another Lingon/LaunchControl-style plist editor for every launchd job on the Mac.

Competitors in this niche ([LaunchControl](https://www.soma-zone.com/LaunchControl/), Lingon,
[mac-dash](https://talhaorak.github.io/mac-dash/), [houston](https://github.com/quantizor/houston),
[launchr](https://github.com/scoult/launchr), [launchd-ui](https://github.com/azu/launchd-ui),
upstream [launch-pilot](https://github.com/RoboZephyr/launch-pilot)) optimize for **browse / edit /
control every launchd job**. Deployboard doubles down on a local **deployment control plane** for
the Ours inventory. See [`docs/deployboard-additions.md`](docs/deployboard-additions.md) (R0) for
why the fork exists.

**Rule for every item below:** if LaunchControl or mac-dash would ship it as a Lingon-parity
checkbox (visual plist editor, system LaunchDaemons, process explorer, AI plist rewrite), skip it.

Product facts live in [`PRODUCT-STATE.md`](PRODUCT-STATE.md). This file is the committed future work.

---

## Moat already shipped (protect and deepen)

| Capability | Why it stays unique |
|---|---|
| Path-derived **Ours / Other / Noise** + groups | Default view answers “my deploys,” not “every agent on the Mac” |
| Prometheus **`/metrics`** | Time series for launchd state — see [`docs/METRICS.md`](docs/METRICS.md) |
| Telegram transition alerts + per-app toggles | Error / offline / probe / run-storm / recovery — [`docs/ALERTS.md`](docs/ALERTS.md) |
| Verified actions + KeepAlive `bootout` + retirement history | “Succeeded” means state changed, not “command sent” |
| Soft UI `read_only` vs hard `--read-only` | Monitoring box cannot be talked into killing a service |

---

## Competitor contrast

| Concern | Typical launchd GUI | Deployboard (today → roadmap) |
|---|---|---|
| Default question | What jobs exist / how do I edit this plist? | Which of *my* apps are healthy? |
| Inventory | Domain / type / tags | Ours allowlist + `derive_roots` path derivation |
| Observability | Live status, logs | Prometheus series + (Phase 1) drift metrics |
| Alerts | Rare / desktop notifications | Transition Telegram + (Phase 1) drift kinds |
| Desired state | None | (Phase 1) expected vs actual + align |
| Health beyond “is PID live?” | Optional HTTP / logs | (Phase 2) HTTP / TCP / exec contracts + incident timeline |
| Project registration | Global app config | (Phase 3) per-repo `deployboard.yaml` |
| Agent / MCP | Audit *everything* (e.g. launchd-audit) | (Phase 4) Ours + desired + verified actions only |

---

## Phase 1 — Desired state for Ours

**Status:** shipped

**Gap:** every competitor shows live state. None treat “these labels must be running / disabled /
probing” as a reconcile target (the systemd/Kubernetes pattern, absent from launchd GUIs).

### Ship

1. **`desired` block in `config.json`** (hot-reload like inventory) — per label or group: expected
   status (`running` | `disabled` | `scheduled`), optional probe URL/port, optional max restart rate.
2. **Drift panel** on the dashboard — Ours only: expected vs actual, one-click **align**
   (start/stop/enable/disable) behind verified actions and both read-only gates.
3. **Metrics:** `deployboard_job_desired_up`, `deployboard_drift_total{reason}` so Grafana can alert
   on drift without scraping the UI.
4. **Alerts:** transition kinds `drift_opened` / `drift_cleared` (reuse the Telegram engine in
   `internal/alerts`).

### Touchpoints

`internal/desired`, `cmd/deployboard`, `internal/server`, `web/`, `docs/`

### Success

A developer with 5–15 self-deployed LaunchAgents can answer “what should be up that isn’t?” in one
glance, and Prometheus can alert on drift — not only `job_up`.

---

## Phase 2 — Health contracts + local incident timeline

**Status:** shipped

**Gap:** probes today are HTTP-only and ephemeral; Apple exposes no job history. Competitors show
logs or live state; none give an **ops timeline for YOUR stack**.

### Ship

1. **Per-group health contracts** — HTTP path + expect status, TCP connect, and optional `exec`
   probe (user-owned script under `derive_roots` only). Pass/fail on group headers.
2. **Local transition log** — append-only JSONL under `~/.config/deployboard/` for status / probe /
   action / drift events (Ours only); UI “Incident” drawer for the last N hours. No DB; no claim of
   Apple’s internal job history.
3. **Crash fingerprints** — on error / restart-storm, hash last exit + last ~20 stderr lines; group
   identical flaps (“same failure 14× since 03:10”). Deterministic, not LLM (LaunchControl’s AI is
   edit-oriented; this is ops RCA).

### Success

Prometheus/Grafana can alert on **contract failure**; the UI can reconstruct “what happened to this
group since 03:00” without an external TSDB.

---

## Phase 3 — Project-native registration + brew/cron under Ours

**Status:** shipped

**Gap:** inventory is global config. Competitors manage plists; brew-only menu apps ignore non-brew
deploys; cron+launchd aggregators are usually read-only notes, not Deployboard.

### Ship

1. **`deployboard.yaml` in project roots** under `derive_roots` — declares labels, group, desired
   state, probes. On scan, merge into inventory (file wins for that project’s labels). “Install
   app → it appears as Ours” becomes automatic.
2. **Optional Ours sources:** `homebrew.mxcl.*` allowlisted into an Infra group; user crontab lines
   matching declared commands as **read-only companion rows** (heuristics only — no cron rewrite).
   Still filtered by Ours, not a second Lingon.

### Success

A new side project registers itself by dropping a YAML next to the code; brew infra and matching
cron companions show up beside LaunchAgents without drowning the default view in vendor noise.

---

## Phase 4 — Agent surface that inherits the wedge

**Status:** shipped

**Gap:** tools like [launchd-audit](https://pypi.org/project/launchd-audit/) MCP audit *everything*.
Deployboard should expose an MCP (stdio) that only speaks **Ours + desired + verified actions +
metrics snapshot**.

### Ship (illustrative tools)

| Tool | Role |
|---|---|
| `ours_status` | Inventory-scoped status for Ours / groups |
| `drift_list` | Expected vs actual from Phase 1 |
| `job_incident` | Recent timeline / fingerprints from Phase 2 |
| `job_action` | Mutating; dry-run by default; `confirm=true` required; respects soft + hard read-only |
| `metrics_snapshot` | Point-in-time scrape of the same series as `/metrics` |

Same safety model as the HTTP API; no system-daemon scope.

### Success

An AI coding agent can safely query and act on **Ours only**, with dry-run and hard `--read-only`
intact.

---

## Later (deferred)

**Observe-only remote** — not committed. If revisited: push-only Ours status + drift + recent
incidents to a read-only view or webhook; no remote start/stop; control stays on the Mac. Distinct
from authenticated full remote control (e.g. mac-dash token).

---

## Explicitly not on this roadmap (parity traps)

- Visual / expert plist editor (Lingon / LaunchControl / mac-dash already own this)
- System LaunchDaemons / privileged helper
- Full process explorer / system-load dashboard (mac-dash)
- Exact launchd internal history or a full cron expression DSL (Apple / API limits)
- Generic “AI rewrite my plist” (LaunchControl already ships LLM edit)

---

## Marketing claim to keep true

*The only launchd console that treats your deployments as a desired-state inventory with metrics
and incidents — not another plist GUI.*
