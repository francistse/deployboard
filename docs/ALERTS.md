# Telegram alerts

Deployboard sends Telegram messages on launchd state **transitions**, per application,
with a per-app toggle in the UI.

## 1. Store the bot token (UI or CLI)

### Option A — the Settings panel in the dashboard (recommended)

Open the dashboard → **⚙ Settings** → paste the token into the *Bot token* field → **Save**.

- The token is POSTed to the local server (`127.0.0.1` only) and written straight to the
  macOS Keychain; it is never written to a file, never logged, and **never sent back to the
  page** — the panel only ever shows "Token stored / No token stored".
- **Send test** fires one message with the stored credentials; **Forget token** deletes the
  Keychain item and disables sending.
- A token stored after startup is picked up within ~60s (or immediately on Save), so no
  restart is needed.

### Option B — the CLI

```bash
security add-generic-password -U -s deployboard-telegram -a bot_token \
  -w "$TELEGRAM_BOT_TOKEN" -T /usr/bin/security
./deployboard --telegram-status      # → readable=true
./deployboard --telegram-test        # sends one test message
```

Keychain **service**: `deployboard-telegram`, **account**: `bot_token`
(both configurable under `alerts.telegram` in `config.json`).

The token is read at send time through `/usr/bin/security`. The ACL is pinned to that binary,
so the read keeps working across OS/toolchain updates and never prompts again.

Two failure modes worth knowing:

- **Missing item** → sending disabled, one `telegram alerts disabled: keychain item not found`
  line on stderr, server keeps running.
- **Locked keychain** → `security(1)` waits for a password no headless caller can supply. The
  settings panel reports `keychain is locked — unlock it (Keychain Access, or
  'security unlock-keychain') and retry`. A launchd-launched server runs inside your GUI
  session, where the login keychain is normally already unlocked, so this mostly affects
  terminal runs.

The **chat id** is an identifier, not a secret, so it lives in `config.json`
(`alerts.telegram.chat_id`) and is editable in the same panel.

## 2. Which applications alert

| Category | Default |
|---|---|
| `ours` (your deployments) | **on** |
| `other` (unclassified) | off |
| `noise` (vendor/OS) | never |
| `disabled` jobs | never |

An explicit toggle always wins over the default.

## 3. Toggling per application

- **In the UI**: the 🔔 switch on each row (and 🧪 for a one-off test alert). Toggling is
  optimistic and rolls back if the request fails.
- **API**:
  ```bash
  curl -X POST http://127.0.0.1:9410/api/alerts/com.example.app.uat.web \
       -H 'Content-Type: application/json' -d '{"enabled": false}'
  curl -X POST http://127.0.0.1:9410/api/alerts \
       -H 'Content-Type: application/json' \
       -d '{"labels": ["com.example.app.uat.api","com.example.app.uat.web"], "enabled": true}'
  curl -X POST http://127.0.0.1:9410/api/alerts/com.example.app.uat.web/test
  curl  http://127.0.0.1:9410/api/alerts          # enabled/disabled counts + per-label state
  curl  http://127.0.0.1:9410/api/alerts/history  # last 50 sends
  ```
- **State file**: `~/.config/deployboard/alerts.json` (atomic write). It records the
  explicit toggle (`explicit: true|false`) separately from the observed status, so a
  recorded status never masquerades as your decision.

## 4. When it fires

| Trigger | Kind |
|---|---|
| status → `error` | `error` |
| status → `offline` (plist on disk, not loaded) | `offline` |
| port probe reachable → unreachable | `unreachable` |
| `runs` increased by ≥ `run_storm_delta` (default 25) between sweeps | `run_storm` |
| back to `running` / probe reachable again | `recovery` |
| desired-state drift opens (Ours + `desired` rule) | `drift_opened` |
| desired-state drift clears | `drift_cleared` |

Delivery rules:

- **Transition only** — the same `(label, status)` never fires twice in a row.
- **Cooldown** `cooldown_seconds` (default 900) per label suppresses repeats.
- **Quiet hours** `23:30–08:00` Asia/Hong_Kong suppress *new* alerts; **recovery and
  `drift_cleared` are never suppressed** and nothing is queued for later.
- Messages are plain text (no `parse_mode`) — a launchd label or log path can never break
  delivery.
- Only the HTTP status is logged; the token is redacted everywhere (`alerts.Redact`).

`run_storm` is the one that catches silent churn: `com.example.app.uat.web` sat at **over 12k
restarts** without anyone noticing until it became a counter.
