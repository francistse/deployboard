import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';
import {
  fetchTelegramSettings,
  saveTelegramSettings,
  forgetTelegram,
  testTelegram,
  fetchAccess,
  setAccess,
} from '../lib/api.js';
import { addToast, accessInfo } from '../lib/state.js';
import { t, codeSegments, locale, setLocale, LOCALES } from '../lib/i18n.js';
import { ConfirmDialog } from './confirm-dialog.js';

function rich(key, vars) {
  return codeSegments(key, vars).map((part, i) =>
    typeof part === 'string' ? part : html`<code key=${i}>${part.code}</code>`
  );
}

/**
 * Settings drawer: write access + the Telegram bot token.
 *
 * Security contract: the token travels browser → loopback → macOS Keychain and
 * is never read back, logged, or echoed by the API. This panel only ever shows
 * "stored / not stored" plus the non-secret chat id and Keychain item names.
 */
export function SettingsPanel({ open, onClose, onAccessChange }) {
  const [status, setStatus] = useState(null);
  const [access, setAccessState] = useState(null);
  const [token, setToken] = useState('');
  const [chatId, setChatId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const [confirmEnable, setConfirmEnable] = useState(false);

  const load = async () => {
    try {
      const [st, ac] = await Promise.all([fetchTelegramSettings(), fetchAccess()]);
      setStatus(st);
      setAccessState(ac);
      accessInfo.value = ac;
      setChatId(st.chat_id || '');
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => { if (open) load(); }, [open]);

  if (!open) return null;

  const run = async (fn, okMsg) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      addToast(okMsg, true);
      await load();
      if (onAccessChange) onAccessChange();
      return true;
    } catch (err) {
      setError(err.message);
      addToast(err.message, false);
      return false;
    } finally {
      setBusy(false);
    }
  };

  const handleSave = async (e) => {
    e.preventDefault();
    const ok = await run(
      () => saveTelegramSettings(token, chatId),
      token ? t('settings.toast.tokenStored') : t('settings.toast.chatUpdated'),
    );
    if (ok) setToken(''); // never keep the secret in component state longer than needed
  };

  const handleForget = () => run(() => forgetTelegram(), t('settings.toast.tokenRemoved'));
  const handleTest = () => run(() => testTelegram(), t('settings.toast.testSent'));

  // Write mode goes through a confirmation: it is the switch that lets this UI
  // stop a service, so it should never flip by a stray click.
  const applyWriteMode = (readOnly) =>
    run(
      () => setAccess(readOnly),
      readOnly ? t('settings.toast.writeOff') : t('settings.toast.writeOn'),
    );

  const toggleWriteMode = () => {
    if (!access) return;
    if (access.read_only) {
      setConfirmEnable(true);
    } else {
      applyWriteMode(true);
    }
  };

  const stored = status && status.configured;

  return html`
    <div class="settings-overlay" onClick=${(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <aside class="settings-panel" role="dialog" aria-label=${t('settings.aria')}>
        <header class="settings-panel__head">
          <h2>${t('settings.title')}</h2>
          <button class="btn btn--sm" onClick=${onClose} title=${t('verb.close')} aria-label=${t('verb.close')}>✕</button>
        </header>

        <section class="settings-panel__section">
          <h3>${t('settings.language')}</h3>
          <div class="lang-switch" role="radiogroup" aria-label=${t('settings.language')}>
            ${LOCALES.map((code) => html`
              <button
                key=${code}
                type="button"
                role="radio"
                class=${`lang-switch__btn ${locale.value === code ? 'lang-switch__btn--active' : ''}`}
                aria-checked=${locale.value === code}
                onClick=${() => setLocale(code)}
              >${t('lang.' + code)}</button>
            `)}
          </div>
        </section>

        <section class="settings-panel__section">
          <h3>${t('settings.access')}</h3>
          <p class="settings-panel__help">
            ${rich('settings.accessHelp')}
          </p>

          <div class="settings-panel__status">
            <span class=${`dot ${access && !access.read_only ? 'dot--ok' : 'dot--idle'}`}></span>
            ${access === null
              ? t('settings.checking')
              : access.read_only
                ? (access.locked
                    ? t('settings.readOnlyLocked')
                    : t('settings.readOnlyRefused'))
                : t('settings.writeAllowed')}
          </div>

          ${access && html`
            <label class=${`settings-switch ${access.locked ? 'settings-switch--disabled' : ''}`}>
              <input
                type="checkbox"
                checked=${!access.read_only}
                disabled=${busy || access.locked}
                onChange=${toggleWriteMode}
              />
              <span class="settings-switch__track" aria-hidden="true"></span>
              <span class="settings-switch__text">
                ${t('settings.allow')}
                <em>${access.locked
                  ? access.lock_reason
                  : t(access.source === 'flag' ? 'settings.sourceFlag' : 'settings.sourceConfig')}</em>
              </span>
            </label>
          `}

          ${access && access.locked && html`
            <p class="settings-panel__hint">
              ${rich('settings.unlock')}
            </p>
          `}
          ${error && html`<p class="settings-panel__error">${error}</p>`}
        </section>

        <section class="settings-panel__section">
          <h3>${t('settings.telegram')}</h3>
          <p class="settings-panel__help">
            ${rich('settings.telegramHelp')}
          </p>

          <div class="settings-panel__status">
            <span class=${`dot ${stored ? 'dot--ok' : 'dot--idle'}`}></span>
            ${status === null
              ? t('settings.checking')
              : stored
                ? t(status.alerts_enabled ? 'settings.tokenActive' : 'settings.tokenInactive')
                : t('settings.noToken')}
          </div>

          ${status && html`
            <dl class="settings-panel__meta">
              <dt>${t('settings.keychainService')}</dt><dd><code>${status.keychain_service}</code></dd>
              <dt>${t('settings.keychainAccount')}</dt><dd><code>${status.keychain_account}</code></dd>
            </dl>
          `}

          <form class="settings-form" onSubmit=${handleSave}>
            <label class="settings-field">
              <span>${stored ? t('settings.botTokenKeep') : t('settings.botToken')}</span>
              <input
                type="password"
                class="settings-input"
                value=${token}
                aria-label=${stored ? t('settings.botTokenKeep') : t('settings.botToken')}
                placeholder=${stored ? t('settings.placeholderStored') : '123456789:AA…'}
                autocomplete="new-password"
                spellcheck="false"
                onInput=${(e) => setToken(e.currentTarget.value)}
              />
            </label>
            <label class="settings-field">
              <span>${t('settings.chatId')}</span>
              <input
                type="text"
                class="settings-input"
                value=${chatId}
                aria-label=${t('settings.chatId')}
                placeholder="123456789"
                onInput=${(e) => setChatId(e.currentTarget.value)}
              />
            </label>
            <div class="settings-form__actions">
              <button class="btn btn--sm btn--active" type="submit" disabled=${busy}>
                ${busy ? t('verb.saving') : t('verb.save')}
              </button>
              <button class="btn btn--sm" type="button" disabled=${busy || !stored} onClick=${handleTest}>
                ${t('settings.sendTest')}
              </button>
              <button class="btn btn--sm btn--danger" type="button" disabled=${busy || !stored} onClick=${handleForget}>
                ${t('settings.forget')}
              </button>
            </div>
          </form>

          ${status && html`
            <p class="settings-panel__hint">
              ${t('settings.preferCli')} <code>${status.store_hint}</code>
            </p>
          `}
          ${error && html`<p class="settings-panel__error">${error}</p>`}
        </section>

        <section class="settings-panel__section">
          <h3>${t('settings.dataSources')}</h3>
          <ul class="settings-panel__list">
            <li>${rich('settings.src.list')}</li>
            <li>${rich('settings.src.print')}</li>
            <li>${rich('settings.src.disabled')}</li>
            <li>${rich('settings.src.plist')}</li>
          </ul>
          <p class="settings-panel__hint">
            ${rich('settings.neverEdits')}
          </p>
        </section>
        <${ConfirmDialog}
          open=${confirmEnable}
          title=${t('settings.enableWriteTitle')}
          label=${t('settings.enableWriteLabel')}
          note=${t('settings.enableWriteNote')}
          action="enable"
          confirmClass="btn--active"
          onConfirm=${() => { setConfirmEnable(false); applyWriteMode(false); }}
          onCancel=${() => setConfirmEnable(false)}
        />
      </aside>
    </div>
  `;
}
