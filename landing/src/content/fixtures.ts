import type { Content, PanelJob } from "./types";

/** Publishing note: every link below points at github.com/francistse/deployboard. */
export const REPO = "https://github.com/francistse/deployboard";

export const logStream: Content["logStream"] = [
  { ts: "14:02:11.402", level: "info", service: "com.example.app.api", body: "Uvicorn running on http://127.0.0.1:8001" },
  { ts: "14:02:11.889", level: "stream", service: "com.example.app.web", body: "compiled / in 412ms (1102 modules)" },
  { ts: "14:02:12.014", level: "warn", service: "com.example.app.web", body: "respawn 1206 — next dev exited before ready" },
  { ts: "14:02:12.311", level: "info", service: "com.example.app.collector", body: "collected 42 items (no listening port, still tracked)" },
  { ts: "14:02:12.744", level: "error", service: "com.example.worker.nightly", body: "exit status 1 — KeepAlive throttle: 10s" },
  { ts: "14:02:13.210", level: "info", service: "com.example.infra.one", body: "[I] started; 3 workers" },
  { ts: "14:02:13.558", level: "stream", service: "com.example.app.api", body: "GET /api/inventory 200 in 3ms" },
];

export const metricsSample: Content["metrics"]["sample"] = [
  "# HELP deployboard_job_up 1 when the launchd job is running.",
  "# TYPE deployboard_job_up gauge",
  'deployboard_job_up{label="com.example.app.api",category="ours",group="Example App"} 1',
  'deployboard_job_runs{label="com.example.app.web",category="ours",group="Example App"} 1207',
  'deployboard_job_disabled{label="com.example.worker.old",category="ours"} 1',
  'deployboard_job_probe_reachable{label="com.example.app.api",port="8001"} 1',
  'deployboard_job_probe_latency_seconds{label="com.example.app.api",port="8001"} 0.003',
  'deployboard_jobs_total{category="ours"} 16',
  'deployboard_jobs_total{category="noise"} 528',
];

export const metricsQuery: Content["metrics"]["query"] = [
  "# restart churn, per job, over 6h",
  'deriv(deployboard_job_runs{category="ours"}[6h]) * 3600',
  "# anything of yours that is not running",
  'deployboard_job_up{category="ours"} == 0',
];

/**
 * The hero panel's machine facts, spread into every locale's `livePanel`.
 *
 * Identical in all four locales on purpose: launchd labels, pids, run counters,
 * schedule expressions and the inventory totals are not translated — only the
 * surrounding copy (column headers, provenance labels, group names, action
 * verbs) is, and that lives in the locale files.
 *
 * The counts describe the whole machine, the way the dashboard computes them,
 * which is why the panel's chips say 545 while its table shows eight rows.
 * 545 = 16 ours + 528 noise + 1 other, matching the hero headline and the
 * `/metrics` sample above.
 */
export const heroPanel: Pick<Content["livePanel"], "counts" | "groups"> = {
  counts: {
    categories: { all: 545, ours: 16, noise: 528, other: 1 },
    statuses: {
      all: 545,
      running: 163,
      scheduled: 2,
      completed: 0,
      stopped: 239,
      error: 139,
      offline: 1,
      disabled: 1,
    },
  },
  groups: [
    {
      key: "app",
      health: { running: 3, error: 1, disabled: 0 },
      total: 5,
      rows: [
        { label: "com.example.app.api", status: "running", category: "ours", source: "listed", pid: 73087, exit: 0, runs: 28, alert: true, uptime: "2h 14m" },
        { label: "com.example.app.web", status: "running", category: "ours", source: "path", pid: 73091, exit: 0, runs: 1207, alert: true, uptime: "6d 3h", keepAlive: true, churn: "churn" },
        { label: "com.example.app.collector", status: "running", category: "ours", source: "path", pid: 73104, exit: 0, runs: 21, alert: true, uptime: "18m" },
        { label: "com.example.worker.nightly", status: "error", category: "ours", source: "listed", pid: null, exit: 1, runs: 3, alert: true },
        { label: "com.example.backup.weekly", status: "scheduled", category: "ours", source: "path", pid: null, exit: 0, runs: 0, alert: true, when: "Sun 03:00" },
      ],
    },
    {
      key: "ops",
      health: { running: 0, error: 0, disabled: 1 },
      total: 2,
      rows: [
        { label: "com.example.worker.old", status: "disabled", category: "ours", source: "listed", pid: null, exit: 0, runs: 0, alert: false, retired: true },
        { label: "com.example.legacy.agent", status: "offline", category: "ours", source: "path", pid: null, exit: 0, runs: 0, alert: false },
      ],
    },
    {
      key: "infra",
      health: { running: 1, error: 0, disabled: 0 },
      total: 1,
      rows: [
        { label: "com.example.infra.one", status: "running", category: "other", source: "unclassified", pid: 816, exit: 0, runs: 1, alert: false, uptime: "3d 7h" },
      ],
    },
  ],
};

/**
 * Vendor and system jobs for the Noise scene of the hero preview.
 * Labels are machine facts, same as `heroPanel`, so they are not translated.
 */
export const noiseJobs: readonly PanelJob[] = [
  { label: "com.dropbox.DropboxUpdater", status: "running", category: "noise", source: "unclassified", pid: 412, exit: 0, runs: 88, alert: false, uptime: "11d" },
  { label: "com.google.GoogleUpdater.wake", status: "scheduled", category: "noise", source: "unclassified", pid: null, exit: 0, runs: 640, alert: false, when: "hourly" },
  { label: "com.spotify.client.helper", status: "running", category: "noise", source: "unclassified", pid: 2201, exit: 0, runs: 14, alert: false, uptime: "4h" },
  { label: "com.apple.SafariBookmarksSyncAgent", status: "running", category: "noise", source: "unclassified", pid: 88, exit: 0, runs: 3, alert: false, uptime: "2d" },
];
