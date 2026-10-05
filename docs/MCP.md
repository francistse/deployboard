# Deployboard MCP (Ours-scoped)

`deployboard mcp` speaks JSON-RPC 2.0 over stdio for AI coding agents. It only
exposes **Ours** inventory, desired-state drift, incidents, verified actions
(dry-run by default), and a metrics snapshot — never system LaunchDaemons.

## Cursor `mcp.json`

```json
{
  "mcpServers": {
    "deployboard": {
      "command": "/Users/you/bin/deployboard",
      "args": ["mcp", "--config", "/Users/you/.config/deployboard/config.json"]
    }
  }
}
```

## Tools

| Tool | Behaviour |
|---|---|
| `ours_status` | Ours jobs: label, status, group, disabled |
| `drift_list` | Open desired-state drifts |
| `job_incident` | Timeline + crash fingerprints (`hours`, default 24) |
| `job_action` | `label` + `action`; **dry-run unless `confirm: true`**; refuses non-Ours, cron companions, and read-only mode |
| `metrics_snapshot` | Prometheus text from the same renderer as `/metrics` |

Same safety model as the HTTP API: soft `read_only` and hard `--read-only` both block mutations.
