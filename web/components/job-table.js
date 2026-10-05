import { html } from 'htm/preact';
import { useState } from 'preact/hooks';
import { filteredJobs, addToast, readOnly } from '../lib/state.js';
import { postGroupAction } from '../lib/api.js';
import { t } from '../lib/i18n.js';
import { JobRow } from './job-row.js';
import { ConfirmDialog } from './confirm-dialog.js';

/** Empty groups are sent to the API as "Ungrouped"; the label is display-only. */
function groupKey(job) {
  return job.group || 'Ungrouped';
}

function groupLabel(name) {
  return name === 'Ungrouped' ? t('table.ungrouped') : name;
}

/** Build a displayable group list with per-group health summary. */
function groupJobs(list) {
  const groups = new Map(); // groupName -> { jobs: [], counts: {} }
  for (const job of list) {
    const group = groupKey(job);
    if (!groups.has(group)) {
      groups.set(group, { jobs: [], counts: { running: 0, error: 0, disabled: 0, total: 0 } });
    }
    const g = groups.get(group);
    g.jobs.push(job);
    g.counts.total++;
    if (job.status === 'running') g.counts.running++;
    else if (job.status === 'error') g.counts.error++;
    else if (job.status === 'disabled') g.counts.disabled++;
  }
  // Sort groups: our groups first (those with jobs.category === 'ours'), then others
  const sorted = Array.from(groups.entries()).sort((a, b) => {
    const aOurs = a[1].jobs.some(j => j.category === 'ours');
    const bOurs = b[1].jobs.some(j => j.category === 'ours');
    if (aOurs !== bOurs) return bOurs - aOurs;
    return a[0].localeCompare(b[0]);
  });
  return sorted;
}

/**
 * Bulk actions for a whole group: repeating the per-row buttons five times is the
 * wrong shape for "bring the stack back up". Each goes through the same
 * confirmation as a single row, listing exactly which labels will be touched.
 */
const GROUP_ACTIONS = [
  {
    action: 'reload',
    verb: 'restart',
    labelKey: 'group.restartAll',
    titleKey: 'group.title.reload',
    noteKey: 'group.note.reload',
    confirmClass: 'btn--danger',
  },
  {
    action: 'start',
    verb: 'start',
    labelKey: 'group.startAll',
    titleKey: 'group.title.start',
    noteKey: 'group.note.start',
    confirmClass: 'btn--active',
  },
  {
    action: 'stop',
    verb: 'stop',
    labelKey: 'group.stopAll',
    titleKey: 'group.title.stop',
    noteKey: 'group.note.stop',
    confirmClass: 'btn--danger',
  },
  {
    action: 'disable',
    verb: 'retire',
    labelKey: 'group.retireAll',
    titleKey: 'group.title.disable',
    noteKey: 'group.note.disable',
    confirmClass: 'btn--danger',
  },
];

/** Labels in a group that a given bulk action would actually act on. */
export function targetsFor(jobs, action) {
  switch (action) {
    case 'reload':
      return jobs.filter(j => (j.pid || 0) > 0).map(j => j.label);
    case 'start':
      // A retired job is brought back on purpose (Enable in its row); a blanket
      // "Start all" must not quietly un-retire things.
      return jobs.filter(j => (j.pid || 0) === 0 && !j.disabled).map(j => j.label);
    case 'stop':
      // Includes KeepAlive jobs with no pid: they are between restarts, and
      // unloading them is the only way to stop the loop.
      return jobs
        .filter(j => j.status !== 'offline' && j.status !== 'disabled')
        .filter(j => (j.pid || 0) > 0 || j.keepAlive === true)
        .map(j => j.label);
    case 'disable':
      return jobs.filter(j => !j.disabled).map(j => j.label);
    default:
      // An action nobody defined must not fall through to "everything".
      return [];
  }
}

export function GroupActions({ group, jobs }) {
  const [pending, setPending] = useState(null); // { spec, targets }
  const [busy, setBusy] = useState(false);

  const run = async (action) => {
    setPending(null);
    setBusy(true);
    try {
      const res = await postGroupAction(group, action);
      const failed = (res.results || []).filter(r => r.error);
      const ok = res.succeeded || 0;
      const skipped = res.skipped || 0;
      addToast(
        t('group.toast', { action, group, ok })
          + (skipped ? t('group.toastSkipped', { n: skipped }) : '')
          + (failed.length ? t('group.toastFailed', { n: failed.length }) : ''),
        failed.length === 0,
      );
      // Failures are named individually: a bulk action that half-worked is worse
      // than one that failed outright, because it is easy to miss.
      for (const f of failed.slice(0, 4)) {
        addToast(`${f.label}: ${f.error}`, false);
      }
    } catch (err) {
      addToast(t('group.toastFail', { action, group, message: err.message }), false);
    } finally {
      setBusy(false);
    }
  };

  const buttons = GROUP_ACTIONS.map((spec) => {
    const targets = targetsFor(jobs, spec.action);
    return html`
      <button
        key=${spec.action}
        class="btn btn--sm btn--outline group-action"
        disabled=${busy || readOnly.value || targets.length === 0}
        title=${readOnly.value
          ? t('access.readOnlyHint')
          : targets.length === 0
            ? t('group.nothing.' + spec.action)
            : `${t(spec.labelKey)} (${targets.length})`}
        onClick=${() => setPending({ spec, targets })}
      >${t(spec.labelKey)}${targets.length ? ` (${targets.length})` : ''}</button>
    `;
  });

  return html`
    <span class="group-actions">${buttons}</span>
    <${ConfirmDialog}
      open=${pending !== null}
      title=${pending ? t(pending.spec.titleKey, { group: groupLabel(group) }) : ''}
      label=${pending ? t('group.jobCount', { n: pending.targets.length }) : ''}
      note=${pending ? t(pending.spec.noteKey, { labels: pending.targets.join(', ') }) : ''}
      action=${pending ? pending.spec.verb : ''}
      confirmClass=${pending ? pending.spec.confirmClass : 'btn--danger'}
      onConfirm=${() => pending && run(pending.spec.action)}
      onCancel=${() => setPending(null)}
    />
  `;
}

export function JobTable({ showAlerts }) {
  const list = filteredJobs.value;

  if (list.length === 0) {
    return html`<p class="job-table__empty">${t('table.empty')}</p>`;
  }

  const grouped = groupJobs(list);

  return html`
    <table class="job-table">
      <thead>
        <tr>
          <th class="job-table__th--status"></th>
          <th>${t('table.label')}</th>
          <th>${t('table.pid')}</th>
          <th>${t('table.exit')}</th>
          <th>${t('table.runs')}</th>
          <th>${t('table.actions')}</th>
        </tr>
      </thead>
      <tbody>
        ${grouped.map(([groupName, { jobs, counts }]) => html`
          <tr class="job-table__group-header">
            <td colspan="6">
              <div class="group-header">
              <span class="group-name">${groupLabel(groupName)}</span>
              <span class="group-health">
                <span class="health-dot health-dot--running" title=${t('status.running')}></span>${counts.running}
                <span class="health-dot health-dot--error" title=${t('status.error')}></span>${counts.error}
                <span class="health-dot health-dot--disabled" title=${t('status.disabled')}></span>${counts.disabled}
                <span class="group-count">${t('table.jobCount', { n: counts.total })}</span>
              </span>
              <${GroupActions} group=${groupName} jobs=${jobs} />
              </div>
            </td>
          </tr>
          ${jobs.map(job => html`<${JobRow} key=${job.label} job=${job} showAlerts=${showAlerts} />`)}
        `)}
      </tbody>
    </table>
  `;
}