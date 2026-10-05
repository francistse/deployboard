# Prometheus metrics

`GET /metrics` on the dashboard port (default `http://127.0.0.1:9410/metrics`) exposes
launchd state as time series. Nothing else in this niche does this: the comparable
projects (launch-pilot, ground-control, launchd-svc-panel, mac-agents-manager-ai,
justinpaulson/status, agent-deck) contain zero references to Prometheus, `/metrics` or
Grafana — they are all imperative control panels. A panel can tell you "down now"; only a
series tells you "started churning at 03:10".

## Series

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `deployboard_up` | gauge | — | 1 when this exporter served the scrape |
| `deployboard_version` | gauge | `version` | constant 1, carries the build |
| `deployboard_scrape_duration_seconds` | gauge | — | time to build the exposition |
| `deployboard_jobs_total` | gauge | `status`, `category` | job counts per status and per category |
| `deployboard_job_up` | gauge | `label`, `category`, `group` | 1 when launchd reports the job running |
| `deployboard_job_runs` | gauge | `label`, `category`, `group` | restart counter (`launchctl print` → `runs`) |
| `deployboard_job_last_exit_code` | gauge | `label`, `category`, `group` | omitted when the job never exited |
| `deployboard_job_disabled` | gauge | `label`, `category`, `group` | 1 when explicitly disabled (`print-disabled`) |
| `deployboard_alerts_enabled` | gauge | `label`, `category` | 1 when Telegram alerts are on for the job |
| `deployboard_job_probe_reachable` | gauge | `label`, `port` | 1 when the port answered |
| `deployboard_job_probe_status` | gauge | `label`, `port` | HTTP status observed |
| `deployboard_job_probe_latency_seconds` | gauge | `label`, `port` | probe latency |

Notes:

- Probe series only exist for jobs that expose a port (`--port N`, `-p N`, `--http-port N`
  in `ProgramArguments`). Ports are probed on `127.0.0.1` **and** `[::1]` (some services bind
  IPv6 only). Any HTTP status **< 500** counts as reachable — a `404` on `/` is normal
  for FastAPI, `403`/`302` means up-but-guarded.
- Results are cached for `--probe-ttl` (default 10s) so a 15s scrape cannot hammer services.
- Disable entirely with `--no-probe` (probe families simply disappear).
- `deployboard_job_runs` is scraped from `launchctl print`; `launchctl list` (what the
  upstream project uses) has no `runs` field, so this metric does not exist anywhere else.

## prometheus.yml

```yaml
scrape_configs:
  - job_name: deployboard
    scrape_interval: 15s
    static_configs:
      - targets: ["127.0.0.1:9410"]
```

## Grafana

Restart-churn panel — the query that would have caught `com.example.app.uat.web`:

```promql
# restart rate per job over the last 6h (counter is a gauge here, so use delta/deriv)
deriv(deployboard_job_runs{category="ours"}[6h]) * 3600

# jobs that are not running
deployboard_job_up{category="ours"} == 0

# anything explicitly disabled
deployboard_job_disabled == 1

# port stopped answering
deployboard_job_probe_reachable == 0
```

Suggested alert rule:

```yaml
- alert: DeployboardJobDown
  expr: deployboard_job_up{category="ours"} == 0
  for: 5m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.label }} is not running"
```
