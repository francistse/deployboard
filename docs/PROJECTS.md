# Project-native registration

Drop a `deployboard.yaml` (or `deployboard.json`) in a project under any
`inventory.derive_roots` entry (the root itself or one child directory). On
scan / config hot-reload, Deployboard merges it so the project appears as Ours
without editing global `config.json`.

```yaml
ours:
  - com.myapp.*
group: My App
desired:
  jobs:
    - match: com.myapp.*
      status: running
contracts:
  - group: My App
    http: { port: 8080, path: /health, expect_status: 200 }
cron_match:
  - myapp-nightly
```

**Merge rule:** project file patterns are prepended — they win over the global
file for the labels they declare.

## Homebrew infra

Set `"include_homebrew_infra": true` under `inventory` to classify
`homebrew.mxcl.*` as Ours in group `homebrew_group` (default `Infra`).

## Cron companions

Patterns in project `cron_match` (or later global) select user crontab lines.
Matching lines appear as read-only Ours rows (`source: "cron"`) — no
start/stop/disable.
