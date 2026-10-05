import { html, render } from 'htm/preact';
import { useEffect, useState, useCallback } from 'preact/hooks';
import { connectSSE } from './lib/sse.js';
import { fetchAlerts, fetchInventory, fetchAccess } from './lib/api.js';
import { resolveInitialLocale, setLocale, t } from './lib/i18n.js';
import { SearchBar } from './components/search-bar.js';
import { FilterBar } from './components/filter-bar.js';
import { JobTable } from './components/job-table.js';
import { ToastContainer } from './components/toast.js';
import { StatusTooltip } from './components/job-tooltip.js';
import { ThemeToggle } from './components/theme-toggle.js';
import { LangSwitch } from './components/lang-switch.js';
import { SettingsPanel } from './components/settings-panel.js';
import { accessInfo, churningJobs, addToast, readOnly } from './lib/state.js';
import { postGroupAction } from './lib/api.js';
import { churnSummary } from './lib/format.js';
import { ConfirmDialog } from './components/confirm-dialog.js';

setLocale(resolveInitialLocale(), false);

/**
 * Restart-storm banner. A loop is the one failure mode a status column cannot
 * show — the job is "running" the whole time — so it gets called out above the
 * table with the two actions that end it.
 */
function StormBanner() {
  const churning = churningJobs.value;
  const [pending, setPending] = useState(null);
  if (churning.length === 0) return null;

  const worst = churning[0];
  const storm = churning.some(entry => entry.level === 'storm');
  const displayGroup = worst.job.group || t('table.ungrouped');

  const run = async (action) => {
    setPending(null);
    const group = worst.job.group || 'Ungrouped';
    try {
      const res = await postGroupAction(group, action);
      addToast(t('storm.toast', {
        action,
        group,
        ok: res.succeeded || 0,
        failed: res.failed || 0,
      }), !res.failed);
    } catch (err) {
      addToast(t('storm.toastFail', { action, group, message: err.message }), false);
    }
  };

  return html`
    <div class=${`storm-banner ${storm ? 'storm-banner--storm' : 'storm-banner--churn'}`} role="status">
      <span class="storm-banner__icon" aria-hidden>${storm ? '⚠' : '↻'}</span>
      <div class="storm-banner__body">
        <strong>
          ${t(churning.length === 1 ? 'storm.job' : 'storm.jobs', { n: churning.length })}
          ${t(storm ? 'storm.loop' : 'storm.repeating')}
        </strong>
        <span class="storm-banner__list">
          ${churning.slice(0, 4).map(({ job }) => html`
            <code key=${job.label}>${job.label}</code>
          `)}
          ${churning.length > 4 ? html`<span>${t('storm.more', { n: churning.length - 4 })}</span>` : null}
        </span>
        <span class="storm-banner__detail">
          ${t('storm.detail', { summary: churnSummary(worst.job) })}
        </span>
      </div>
      <span class="storm-banner__actions">
        <button class="btn btn--sm" disabled=${readOnly.value} onClick=${() => setPending('stop')}>${t('storm.stopThem')}</button>
        <button class="btn btn--sm btn--outline" disabled=${readOnly.value} onClick=${() => setPending('disable')}>${t('storm.retireThem')}</button>
      </span>
      <${ConfirmDialog}
        open=${pending !== null}
        title=${pending === 'disable' ? t('storm.retireTitle') : t('storm.stopTitle')}
        label=${t('storm.group', { group: displayGroup })}
        note=${pending === 'disable' ? t('storm.retireNote') : t('storm.stopNote')}
        action=${pending === 'disable' ? 'retire' : 'stop'}
        confirmClass=${pending === 'disable' ? 'btn--danger' : 'btn--active'}
        onConfirm=${() => pending && run(pending)}
        onCancel=${() => setPending(null)}
      />
    </div>
  `;
}

function App() {
  const [showAlerts, setShowAlerts] = useState(false);
  const [alertsSummary, setAlertsSummary] = useState({ available: false, enabled: 0, disabled: 0 });
  const [inventorySummary, setInventorySummary] = useState({ ours: 0, noise: 0, other: 0 });
  const [settingsOpen, setSettingsOpen] = useState(false);

  const loadAlerts = useCallback(async () => {
    try {
      const data = await fetchAlerts();
      setShowAlerts(data.available);
      setAlertsSummary(data);
    } catch (e) {
      setShowAlerts(false);
      setAlertsSummary({ available: false, enabled: 0, disabled: 0 });
    }
  }, []);

  // Write mode is server state, not a UI preference: fetch it on boot and again
  // whenever the settings panel closes, so the badge and the row buttons always
  // match what the server will actually allow.
  const loadAccess = useCallback(async () => {
    try {
      accessInfo.value = await fetchAccess();
    } catch (e) {
      accessInfo.value = null;
    }
  }, []);

  const loadInventory = useCallback(async () => {
    try {
      const data = await fetchInventory();
      setInventorySummary({ ours: data.ours, noise: data.noise, other: data.other });
    } catch (e) {
      setInventorySummary({ ours: 0, noise: 0, other: 0 });
    }
  }, []);

  useEffect(() => {
    const es = connectSSE();
    loadAlerts();
    loadInventory();
    loadAccess();
    return () => es.close();
  }, [loadAlerts, loadInventory, loadAccess]);

  return html`
    <header>
      <div class="header-top">
        <div>
          <h1>Deployboard</h1>
          <p>${t('header.subtitle')}</p>
        </div>
        <div class="header-actions">
          <${LangSwitch} className="lang-switch--header" />
          <${ThemeToggle} />
          <button class="btn btn--sm" onClick=${() => setSettingsOpen(true)} title=${t('settings.title')}>⚙ ${t('settings.title')}</button>
        </div>
      </div>
      <div class="header-badges">
        ${showAlerts && html`
          <span class="badge badge--alerts" title=${t('header.telegramTitle')}>
            🔔 ${t('header.alertsCount', { on: alertsSummary.enabled, off: alertsSummary.disabled })}
          </span>
        `}
        <span class="badge badge--inventory" title=${t('header.inventoryTitle')}>
          📦 ${t('header.inventoryCount', {
            ours: inventorySummary.ours,
            other: inventorySummary.other,
            noise: inventorySummary.noise,
          })}
        </span>
        ${accessInfo.value && (accessInfo.value.read_only
          ? html`<span
              class=${`badge ${accessInfo.value.locked ? 'badge--locked' : 'badge--readonly'}`}
              title=${accessInfo.value.locked
                ? accessInfo.value.lock_reason
                : t('header.readOnlyTitle')}
            >${accessInfo.value.locked
              ? `🔒 ${t('header.readOnlyLocked')}`
              : `👁 ${t('header.readOnly')}`}</span>`
          : html`<span class="badge badge--write" title=${t('header.writeTitle')}>✎ ${t('header.writeMode')}</span>`)}
        <span class="badge badge--url">http://127.0.0.1:9410</span>
      </div>
    </header>
    <main>
      <${StormBanner} />
      <${SearchBar} />
      <${FilterBar} />
      <${JobTable} showAlerts=${showAlerts} />
    </main>
    <${SettingsPanel}
      open=${settingsOpen}
      onClose=${() => { setSettingsOpen(false); loadAlerts(); loadAccess(); }}
      onAccessChange=${loadAccess}
    />
    <${ToastContainer} />
    <${StatusTooltip} />
  `;
}

render(html`<${App} />`, document.getElementById('app'));