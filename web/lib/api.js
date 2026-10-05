/**
 * Perform a JSON API request.
 * @param {string} path  — URL path (e.g. "/api/jobs")
 * @param {RequestInit} [opts]
 * @returns {Promise<any>}
 */
export async function apiFetch(path, opts) {
  const res = await fetch(path, opts);
  const body = await res.json();
  if (!res.ok) {
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return body;
}

/** POST an action (reload/start/stop) on a job. */
export function postAction(label, action) {
  return apiFetch(`/api/jobs/${encodeURIComponent(label)}/${action}`, {
    method: 'POST',
  });
}

/** GET inventory summary (category/group counts). */
export function fetchInventory() {
  return apiFetch('/api/inventory');
}

/** GET Prometheus metrics text. */
export function fetchMetrics() {
  return fetch('/metrics').then(r => r.text());
}

/** GET alert snapshot (enabled/disabled counts + per-label state). */
export function fetchAlerts() {
  return apiFetch('/api/alerts');
}

/** POST toggle alert for one label. */
export function postAlertToggle(label, enabled) {
  return apiFetch(`/api/alerts/${encodeURIComponent(label)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  });
}

/** POST bulk alert toggle. */
export function postAlertBulk(labels, enabled) {
  return apiFetch('/api/alerts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ labels, enabled }),
  });
}

/** POST send a test alert for one label. */
export function postAlertTest(label) {
  return apiFetch(`/api/alerts/${encodeURIComponent(label)}/test`, {
    method: 'POST',
  });
}

/** POST pin a label's category in config.json ("ours" | "noise" | "auto"). */
export function postClassify(label, category) {
  return apiFetch('/api/inventory/classify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label, category }),
  });
}

/** POST force a config.json hot reload. */
export function postReloadConfig() {
  return apiFetch('/api/inventory/reload', { method: 'POST' });
}

/** GET write-mode state (read_only / locked / source). */
export function fetchAccess() {
  return apiFetch('/api/settings/access');
}

/**
 * POST the write-mode switch. Passing false enables start/stop/reload.
 * A server started with --read-only refuses with 403 and a lock_reason.
 */
export function setAccess(readOnly) {
  return apiFetch('/api/settings/access', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ read_only: readOnly }),
  });
}

/** POST one action to every job in an inventory group. */
export function postGroupAction(group, action) {
  return apiFetch('/api/inventory/group-action', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ group, action }),
  });
}

/** GET desired-state drifts (Ours only). */
export function fetchDrift() {
  return apiFetch('/api/drift');
}

/** POST align one drifted job toward its desired state. */
export function postDriftAlign(label, action) {
  const body = { label };
  if (action) body.action = action;
  return apiFetch('/api/drift/align', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

/** GET one job (used to re-check what an action actually did). */
export function fetchJob(label) {
  return apiFetch(`/api/jobs/${encodeURIComponent(label)}`);
}

/** GET Telegram settings status (never returns the token). */
export function fetchTelegramSettings() {
  return apiFetch('/api/settings/telegram');
}

/** POST a new bot token (and/or chat id). The token goes straight to the Keychain. */
export function saveTelegramSettings(botToken, chatId) {
  const body = {};
  if (botToken) body.bot_token = botToken;
  if (chatId) body.chat_id = chatId;
  return apiFetch('/api/settings/telegram', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

/** DELETE the stored token from the Keychain. */
export function forgetTelegram() {
  return apiFetch('/api/settings/telegram', { method: 'DELETE' });
}

/** POST send a test message with the stored credentials. */
export function testTelegram() {
  return apiFetch('/api/settings/telegram/test', { method: 'POST' });
}

/** GET alert send history. */
export function fetchAlertHistory(limit = 50) {
  return apiFetch(`/api/alerts/history?limit=${limit}`);
}