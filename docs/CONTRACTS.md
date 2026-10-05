# Health contracts & incidents

Phase 2 of the Deployboard wedge: prove *your* stack is healthy beyond “PID > 0”,
and keep a local timeline when it is not.

## Contracts

Add a top-level `contracts` array to `config.json` (hot-reloaded):

```json
"contracts": [
  {"group": "Example App", "http": {"port": 8080, "path": "/health", "expect_status": 200}},
  {"group": "Infra", "tcp": {"port": 5432}},
  {"match": "com.example.app.api", "exec": {"command": ["./scripts/check.sh"], "timeout_seconds": 5}}
]
```

| Kind | Behaviour |
|---|---|
| `http` | GET `127.0.0.1` then `[::1]` on `port`+`path`; status must equal `expect_status` (default 200) |
| `tcp` | Dial loopback on `port` |
| `exec` | Run `command[0]` with remaining args; **path must resolve under a `derive_roots` entry** or the contract fails closed |

Only contracts whose `group` / `match` intersects **Ours** jobs are evaluated.

- UI: pass/fail pill on group headers
- API: `GET /api/contracts`
- Metrics: `deployboard_contract_ok{kind,group|match}`

## Incidents

Append-only JSONL at `~/.config/deployboard/incidents.jsonl` (rotated past ~5 MiB).

Event kinds: `status`, `probe`, `contract`, `action`, `drift`, `fingerprint`.

Crash fingerprints hash last exit code + last ~20 stderr lines so identical flaps
group as “same failure N× since …”.

- API: `GET /api/incidents?hours=24`
- UI: **Incidents** drawer in the header
