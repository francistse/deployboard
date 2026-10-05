import { html } from 'htm/preact';
import { useState } from 'preact/hooks';
import { computed } from '@preact/signals';
import { expandedJob, activePanel, addToast, readOnly, accessInfo } from '../lib/state.js';
import { postAction, postAlertToggle, postAlertTest, postClassify, fetchJob } from '../lib/api.js';
import { formatUptime, formatAgo, sparkBars, stormLevel, churnSummary } from '../lib/format.js';
import { classifyJob } from '../lib/classify.js';
import { t } from '../lib/i18n.js';
import { ConfirmDialog } from './confirm-dialog.js';
import { LogViewer } from './log-viewer.js';
import { DiagnosePanel } from './diagnose-panel.js';
import { StatusDot } from './job-tooltip.js';

/**
 * Tooltip for the disabled write buttons. Distinguishes "monitoring mode, you
 * can turn this on" from "a start-up flag forbids it" — otherwise the button
 * just looks broken.
 */
const READONLY_TITLE = computed(() => {
  const info = accessInfo.value;
  if (info && info.locked) {
    return info.lock_reason || t('row.readOnlyLocked');
  }
  return t('access.readOnlyHint');
});

/** Human label for how a job was classified (mirrors the Go inventory.Source). */
function sourceLabel(src) {
  if (src === 'ours_pattern' || src === 'hidden_pattern') return t('source.listed');
  if (src === 'derived_path') return t('source.autoPath');
  if (src === 'default') return t('source.unclassified');
  return t('source.auto');
}

/** Map the current classification back to the value shown in the select. */
function classifyValue(job) {
  const src = job.categorySource;
  if (src === 'ours_pattern') return 'ours';
  if (src === 'hidden_pattern') return 'noise';
  return 'auto';
}

/**
 * Single row in the job table with action buttons, expandable panels,
 * and an alert toggle (bell) for fork deployments.
 * @param {{ job: object }} props
 */
/** How long to wait before re-reading launchd: long enough for a KeepAlive job
 * that is going to come straight back to have come back. */
const VERIFY_DELAY_MS = 2000;

/** What a successful action should look like when we look again. */
const VERIFY_EXPECTATION = {
  stop: (j) => (j.pid || 0) === 0,
  start: (j) => (j.pid || 0) > 0 || j.status === 'scheduled' || j.status === 'completed',
  reload: (j) => (j.pid || 0) > 0 || j.status === 'scheduled' || j.status === 'completed',
  disable: (j) => j.disabled === true && (j.pid || 0) === 0,
  enable: (j) => j.disabled !== true,
};

function describeJob(j) {
  const bits = [t('status.' + j.status)];
  if (j.pid > 0) bits.push(`pid ${j.pid}`);
  if (j.disabled) bits.push(t('status.disabled'));
  return bits.join(', ');
}

/**
 * Re-read one job after a pause and report whether the action actually stuck.
 * A 200 from launchctl means "the command ran", not "the job is in the state you
 * asked for" — this is the difference, and the reason the old Stop button looked
 * like it worked on a KeepAlive job.
 */
export async function verifyAfterAction(action, label, opts = {}) {
  const expect = VERIFY_EXPECTATION[action];
  if (!expect) return { ok: true, msg: '' };
  const get = opts.fetchJob || fetchJob;
  const delayMs = opts.delayMs || VERIFY_DELAY_MS;
  // A self reload ends with this process exiting, so the read can land while
  // launchd is still respawning the job: one retry is the difference between
  // "verified" and a red toast on a restart that actually worked.
  const attempts = opts.self ? 4 : 1;
  await new Promise((resolve) => setTimeout(resolve, delayMs));
  let lastErr = null;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      const job = await get(label);
      const ok = expect(job);
      if (ok) {
        return { ok: true, msg: t('toast.verified', { action, state: describeJob(job), seconds: delayMs / 1000 }) };
      }
      return { ok: false, msg: t('toast.notVerified', { action, state: describeJob(job) }) };
    } catch (err) {
      lastErr = err;
      if (attempt < attempts - 1) {
        await new Promise((resolve) => setTimeout(resolve, delayMs));
      }
    }
  }
  return { ok: false, msg: t('toast.couldNotVerify', { action, message: lastErr ? lastErr.message : '' }) };
}

/**
 * What each button will do, in the user's words, plus the launchctl call it
 * maps to. Shown in the confirmation dialog so nobody has to guess whether
 * "Stop" means kill, unload, or retire.
 */
export const ACTION_COPY = {
  reload: {
    title: (job) => t(job && job.self ? 'confirm.reload.titleSelf' : 'confirm.reload.title'),
    note: (job) => t(job && job.self ? 'confirm.self.reloadNote' : 'confirm.reload.note'),
  },
  start: {
    title: () => t('confirm.start.title'),
    note: (job) => t(job.disabled ? 'confirm.start.noteRetired' : 'confirm.start.note'),
  },
  stop: {
    title: (job) => t(job.self
      ? 'confirm.stop.titleSelf'
      : job.keepAlive && !job.pid ? 'confirm.stop.loop' : job.keepAlive ? 'confirm.stop.unload' : 'confirm.stop.title'),
    // Stop on the dashboard runs bootout against this process. Nothing
    // respawns it, so the row has to say that before the click, not after
    // the page is gone.
    note: (job) => t(job.self ? 'confirm.self.staysDown' : job.keepAlive ? 'confirm.stop.noteKeepAlive' : 'confirm.stop.note'),
    danger: false,
  },
  disable: {
    title: (job) => t(job.self ? 'confirm.disable.titleSelf' : 'confirm.disable.title'),
    note: (job) => t(job.self ? 'confirm.self.staysDown' : 'confirm.disable.note'),
  },
  enable: {
    title: () => t('confirm.enable.title'),
    note: () => t('confirm.enable.note'),
    danger: false,
  },
};

export function JobRow({ job, showAlerts }) {
  const [confirm, setConfirm] = useState(null); // { action: string } | null
  const [alertLoading, setAlertLoading] = useState(false);

  const category = classifyJob(job);
  const isNoise = category === 'noise';
  const isDisabled = job.disabled === true || job.status === 'disabled';
  const isExpanded = expandedJob.value === job.label;
  const panel = activePanel.value;
  const runs = job.runs || 0;
  const hasRuns = runs > 0;
  const alertEnabled = showAlerts && job.alertEnabled === true;
  const isRunning = (job.pid || 0) > 0;
  const isLoaded = job.status !== 'offline' && job.status !== 'disabled';
  // Stop applies whenever launchd is supervising a process — including a
  // KeepAlive job caught between restarts (pid 0, still restarting), which is
  // precisely the case a Stop button is most needed for.
  const canStop = isLoaded && (isRunning || job.keepAlive === true);
  const uptime = formatUptime(job.uptimeSeconds);
  const series = Array.isArray(job.runsSeries) ? job.runsSeries : [];
  const churn = stormLevel(job);
  const retiredAgo = job.retiredAt ? formatAgo(job.retiredAt) : '';
  const nextRun = job.nextRunAt ? new Date(job.nextRunAt).toLocaleString() : '';
  const startTitle = job.disabled
    ? t('row.startDisabled')
    : nextRun
      ? t('row.startScheduled', { when: nextRun })
      : t('row.startNow');
  const runsTitle = [
    job.keepAlive ? t('row.keepAliveRuns') : t('row.runsSince'),
    churnSummary(job),
  ].filter(Boolean).join(' · ');

  const noiseStyle = isNoise ? { opacity: '0.6' } : {};

  const togglePanel = (panelName) => {
    if (isExpanded && panel === panelName) {
      expandedJob.value = null;
      activePanel.value = null;
    } else {
      expandedJob.value = job.label;
      activePanel.value = panelName;
    }
  };

  const handleAction = async (action) => {
    setConfirm(null);
    try {
      const res = await postAction(job.label, action);
      const immediate = res && res.verified;
      // The server re-read the job before answering; if that already disagrees
      // with the intent, do not claim success and do not wait two seconds to say so.
      if (immediate && immediate.ok === false) {
        addToast(
          t('toast.notTake', {
            action,
            verdict: immediate.verdict || t('toast.unexpected'),
            state: describeJob(immediate),
          }),
          false,
        );
        return;
      }
      addToast(
        res && res.note
          ? t('toast.sentNote', { action, label: job.label, note: res.note })
          : t('toast.sent', { action, label: job.label }),
        true,
      );
      const check = await verifyAfterAction(action, job.label, { self: job.self === true });
      if (check.msg) addToast(check.msg, check.ok);
    } catch (err) {
      addToast(t('toast.failed', { action, message: err.message }), false);
    }
  };

  const handleAlertToggle = async () => {
    if (!showAlerts) return;
    setAlertLoading(true);
    try {
      const newState = !alertEnabled;
      await postAlertToggle(job.label, newState);
      addToast(t(newState ? 'toast.alertsEnabled' : 'toast.alertsDisabled', { label: job.label }), true);
    } catch (err) {
      addToast(t('toast.alertToggleFailed', { message: err.message }), false);
    } finally {
      setAlertLoading(false);
    }
  };

  const handleAlertTest = async () => {
    if (!showAlerts) return;
    setAlertLoading(true);
    try {
      await postAlertTest(job.label);
      addToast(t('toast.testSent', { label: job.label }), true);
    } catch (err) {
      addToast(t('toast.testFailed', { message: err.message }), false);
    } finally {
      setAlertLoading(false);
    }
  };

  const handleClassify = async (category) => {
    try {
      await postClassify(job.label, category);
      addToast(
        category === 'auto'
          ? t('toast.backAuto', { label: job.label })
          : t(category === 'ours' ? 'row.pinnedOurs' : 'row.pinnedHidden', { label: job.label }),
        true,
      );
      // The SSE snapshot refreshes the row within ~5s; the toast confirms now.
    } catch (err) {
      addToast(t('toast.classifyFailed', { message: err.message }), false);
    }
  };

  return html`
    <tr class="job-row${isNoise ? ' job-row--noise' : ''}" style=${noiseStyle}>
      <td class="job-row__status"><${StatusDot} job=${job} /></td>
      <td class="job-row__label"><code>${job.label}</code><span class=${'category-badge category-badge--' + category}>${t('category.' + category)}</span><span class="source-badge" title=${t('row.sourceHow')}>${sourceLabel(job.categorySource)}</span>${job.self ? html`<span class="self-badge" title=${t('row.selfTitle')}>${t('row.self')}</span>` : null}${retiredAgo ? html`<span class="retired-badge" title=${t('row.retiredTitle', { ago: retiredAgo })}>⏹ ${t('row.retired', { ago: retiredAgo })}</span>` : null}${churn ? html`<span class=${`churn-badge churn-badge--${churn}`} title=${churnSummary(job)}>${churn === 'storm' ? `⚠ ${t('row.restartLoop')}` : `↻ ${t('row.churn')}`}</span>` : null}</td>
      <td class="job-row__pid">
        ${job.pid > 0 ? job.pid : '\u2014'}
        ${uptime && html`<span class="job-row__uptime" title=${t('row.runningFor', { uptime })}>${uptime}</span>`}
      </td>
      <td class="job-row__exit">${isDisabled && !job.hasExit ? '\u2014' : (job.hasExit ? job.lastExitStatus : job.lastExitStatus)}</td>
      <td class="job-row__runs ${hasRuns && job.restartWarn ? 'job-row__runs--warn' : ''}">
        <span class="job-row__runs-value" title=${runsTitle}>
          ${hasRuns ? runs : '\u2014'}${job.keepAlive ? html`<span class="keepalive-mark" title=${t('row.keepAliveMark')}>↻</span>` : null}
        </span>
        ${series.length > 1 && html`<svg class="spark" width="40" height="12" title=${churnSummary(job) || t('row.restartsPerMinute')} aria-label=${t('row.restartsAria', { summary: churnSummary(job) || t('row.restartsNone') })}>
          ${sparkBars(series, 20).map((h, i) => h === 0 ? null : html`<rect key=${i} x=${i * 2} y=${12 - h} width="1.6" height=${h} rx="0.6" />`)}
        </svg>`}
      </td>
      <td class="job-row__actions">
        ${showAlerts && html`
          <button
            class=${`btn btn--sm btn--icon ${alertEnabled ? 'btn--active' : ''}`}
            onClick=${handleAlertToggle}
            disabled=${alertLoading}
            title=${alertEnabled ? t('row.alertsDisable') : t('row.alertsEnable')}
            aria-label=${alertEnabled ? t('row.alertsDisable') : t('row.alertsEnable')}
          >
            ${alertLoading ? '⏳' : (alertEnabled ? '🔔' : '🔕')}
          </button>
          <button
            class="btn btn--sm btn--icon"
            onClick=${handleAlertTest}
            disabled=${alertLoading}
            title=${t('row.sendTest')}
            aria-label=${t('row.sendTest')}
          >
            🧪
          </button>
        `}
        ${isRunning
          ? html`<button class="btn btn--sm" onClick=${() => setConfirm({ action: 'reload' })} disabled=${readOnly.value} title=${readOnly.value ? READONLY_TITLE.value : t('row.restartNow')}>${t('verb.restart')}</button>`
          : html`<button class="btn btn--sm" onClick=${() => setConfirm({ action: 'start' })} disabled=${readOnly.value} title=${readOnly.value ? READONLY_TITLE.value : startTitle}>${t('verb.start')}</button>`}
        ${canStop && html`<button class="btn btn--sm" onClick=${() => setConfirm({ action: 'stop' })} disabled=${readOnly.value} title=${readOnly.value ? READONLY_TITLE.value : (job.keepAlive ? t('row.stopUnload') : t('row.stopSignal'))}>${t('verb.stop')}</button>`}
        ${isDisabled
          ? html`<button class="btn btn--sm" onClick=${() => setConfirm({ action: 'enable' })} disabled=${readOnly.value} title=${readOnly.value ? READONLY_TITLE.value : t('row.enableTitle')}>${t('verb.enable')}</button>`
          : html`<button class="btn btn--sm btn--outline" onClick=${() => setConfirm({ action: 'disable' })} disabled=${readOnly.value} title=${readOnly.value ? READONLY_TITLE.value : t('row.disableTitle')}>${t('verb.disable')}</button>`}
        <button class=${`btn btn--sm btn--outline ${isExpanded && panel === 'logs' ? 'btn--active' : ''}`} onClick=${() => togglePanel('logs')}>${t('verb.logs')}</button>
        <button class=${`btn btn--sm btn--outline ${isExpanded && panel === 'diagnose' ? 'btn--active' : ''}`} onClick=${() => togglePanel('diagnose')}>${t('verb.diagnose')}</button>
      </td>
    </tr>
    ${isExpanded && html`
      <tr class="job-row__detail">
        <td colspan="6">
          <div class="classify-bar">
            <span class="classify-bar__label">${t('classify.label')}</span>
            <select
              class="classify-select"
              value=${classifyValue(job)}
              aria-label=${t('classify.label')}
              onChange=${(e) => handleClassify(e.currentTarget.value)}
            >
              <option value="auto">${t('classify.auto')}</option>
              <option value="ours">${t('classify.ours')}</option>
              <option value="noise">${t('classify.hide')}</option>
            </select>
            <span class="classify-bar__hint">
              ${sourceLabel(job.categorySource)}
              ${job.group ? ` · ${job.group}` : ''}
            </span>
          </div>
          ${panel === 'logs' && html`<${LogViewer} label=${job.label} />`}
          ${panel === 'diagnose' && html`<${DiagnosePanel} label=${job.label} />`}
        </td>
      </tr>
    `}
    <${ConfirmDialog}
      open=${confirm !== null}
      label=${job.label}
      action=${confirm ? confirm.action : ''}
      title=${confirm ? ACTION_COPY[confirm.action].title(job) : ''}
      note=${confirm ? ACTION_COPY[confirm.action].note(job) : ''}
      confirmClass=${confirm && ACTION_COPY[confirm.action].danger === false ? 'btn--active' : 'btn--danger'}
      onConfirm=${() => confirm && handleAction(confirm.action)}
      onCancel=${() => setConfirm(null)}
    />
  `;
}