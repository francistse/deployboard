/**
 * Display helpers. Pure functions on purpose: the formatting rules are the part
 * worth testing, and the components stay concerned with layout.
 * Relative phrases go through t() so they follow the dashboard locale.
 */
import { t } from './i18n.js';

/**
 * formatUptime renders a duration in seconds the way a human reads a process
 * age: "42s", "3m", "2h 14m", "3d 4h". Returns '' for zero/unknown so callers
 * can skip rendering instead of printing "0s".
 */
export function formatUptime(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '';
  const s = Math.floor(seconds);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${m % 60}m`;
  const d = Math.floor(h / 24);
  return `${d}d ${h % 24}h`;
}

/**
 * formatAgo renders a timestamp as a short relative age: "just now", "3 days
 * ago", "2 months ago". Accepts an ISO string or epoch milliseconds, which is
 * what encoding/json produces on both sides.
 */
export function formatAgo(when, now = Date.now()) {
  const ts = typeof when === 'number' ? when : Date.parse(when);
  if (!Number.isFinite(ts)) return '';
  const secs = Math.max(0, Math.floor((now - ts) / 1000));
  if (secs < 90) return t('ago.justNow');
  const mins = Math.floor(secs / 60);
  if (mins < 60) return t('ago.min', { n: mins });
  const hours = Math.floor(mins / 60);
  if (hours < 24) return t(hours === 1 ? 'ago.hour' : 'ago.hours', { n: hours });
  const days = Math.floor(hours / 24);
  if (days < 30) return t(days === 1 ? 'ago.day' : 'ago.days', { n: days });
  const months = Math.floor(days / 30);
  if (months < 12) return t(months === 1 ? 'ago.month' : 'ago.months', { n: months });
  const years = Math.floor(months / 12);
  return t(years === 1 ? 'ago.year' : 'ago.years', { n: years });
}

/**
 * sparkBars turns a series of per-interval restart counts into bar heights for a
 * fixed-height sparkline. Always returns `width` bars so the shape does not jump
 * around as data arrives; missing slots are zero-height.
 */
export function sparkBars(series, width = 24) {
  const list = Array.isArray(series) ? series : [];
  const tail = list.slice(-width);
  const pad = Math.max(0, width - tail.length);
  const max = Math.max(1, ...tail);
  const bars = new Array(pad).fill(0);
  for (const v of tail) {
    const n = Number.isFinite(v) && v > 0 ? v : 0;
    // Floor at 1 for any non-zero value so a single restart is visible.
    bars.push(n === 0 ? 0 : Math.max(1, Math.round((n / max) * 10)));
  }
  return bars;
}

/**
 * stormLevel classifies a job's *recent* churn for the banner and the row badge.
 *
 * Deliberately uses the observed window only, never the cumulative runs counter:
 * a job with 1207 restarts since launchd loaded it is a fact about its history,
 * but if it has restarted twice in the last half hour it is not a problem now —
 * and a banner that is always on trains you to ignore it. The cumulative count
 * still marks the row (restartWarn) so the number itself stays visible.
 *
 * Thresholds: four restarts inside the window is worth a look; ten in a window
 * of half an hour is a loop.
 */
export function stormLevel(job) {
  const restarts = job && Number.isFinite(job.restartsRecent) ? job.restartsRecent : 0;
  const minutes = job && Number.isFinite(job.windowMinutes) ? job.windowMinutes : 0;
  if (minutes <= 0) return null;
  if (restarts >= 10) return 'storm';
  if (restarts >= 4) return 'churn';
  return null;
}

/** Human summary of a job's recent churn, or '' when there is nothing to say. */
export function churnSummary(job) {
  const restarts = job && Number.isFinite(job.restartsRecent) ? job.restartsRecent : 0;
  const minutes = job && Number.isFinite(job.windowMinutes) ? job.windowMinutes : 0;
  if (restarts <= 0 || minutes <= 0) return '';
  return t(restarts === 1 ? 'churn.one' : 'churn.many', { n: restarts, m: minutes });
}
