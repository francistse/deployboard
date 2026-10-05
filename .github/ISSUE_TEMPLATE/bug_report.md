---
name: Bug report
about: Something is broken or wrong
title: ""
labels: bug
assignees: ""
---

**What happened**
A clear description of the bug.

**What you expected**
What you thought would happen instead.

**Steps to reproduce**
1.
2.
3.

**Environment**
- Deployboard version (`deployboard --version`):
- macOS version (`sw_vers`):
- Installed via: `bash install.sh` / built from source / other
- Running with: `--read-only`? `--port`? (paste the `deployboard ...` command line)

**Job status output (if relevant)**
If the bug is about a specific launchd job, paste the output of:

```sh
launchctl print gui/$(id -u)/<your.label> | head -40
launchctl print-disabled gui/$(id -u)
```

**Anything else**
Logs from `~/Library/Logs/…`, a screenshot, or a `/api/jobs` snippet.

> Please redact real labels, usernames and paths if you'd rather not share them —
> `com.example.app.*` style labels are fine.
