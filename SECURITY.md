# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for a security problem.

Use GitHub's private reporting instead: **Security → Report a vulnerability** on
<https://github.com/francistse/deployboard/security/advisories/new>. That opens a
private advisory only the maintainers can see.

Include: the version (`deployboard --version`), macOS version, what you expected,
what actually happened, and the smallest reproduction you can manage. You will get
an acknowledgement on GitHub; a fix or a written explanation follows.

## Supported versions

The latest release on `main` is supported. There are no maintenance branches yet.

## Threat model — what Deployboard is and is not

Deployboard is a **localhost-only** console for the current user's launchd agents.
Its security posture is deliberate and narrow:

- The HTTP server binds **127.0.0.1 only**. It is not designed to be exposed to a
  LAN or the internet, and there is no authentication. **Do not put it behind a
  reverse proxy or port-forward it.**
- It reads `launchctl` state and asks `launchctl` to start / stop / reload / disable
  user-domain (`gui/<uid>`) jobs. It never touches the `system` domain, never edits a
  plist, and does not use a privileged helper or `sudo`.
- `--read-only` is a **hard lock**: mutating endpoints return `403` and the UI
  controls are disabled. Use it for a box that must be visible but not controllable.
- The Telegram bot token lives in the **macOS Keychain** (`security add-generic-password`).
  It is never written to `config.json`, a log line, or an API response.
- The only outbound traffic Deployboard can make is the Telegram messages you turn
  on. There is no telemetry, no analytics and no update check.
- Job labels are validated against `[a-zA-Z0-9._-]+` before any shell call.

## Out of scope

- Anything that requires the attacker to already have local access to the machine or
  the user's Keychain.
- Exposing the port and expecting it to be safe. It is a local console by design.
- Reports that only show a dependency advisory without a reachable path in this code.
