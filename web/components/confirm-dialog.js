import { html } from 'htm/preact';
import { useEffect, useRef } from 'preact/hooks';
import { t } from '../lib/i18n.js';

const VERB_KEYS = {
  reload: 'verb.reload',
  start: 'verb.start',
  stop: 'verb.stop',
  disable: 'verb.disable',
  enable: 'verb.enable',
  retire: 'verb.retire',
  restart: 'verb.restart',
};

function confirmVerb(action) {
  if (action && VERB_KEYS[action]) return t(VERB_KEYS[action]);
  if (!action) return '';
  const raw = String(action);
  return raw.charAt(0).toUpperCase() + raw.slice(1);
}

/**
 * Confirmation dialog for destructive actions (reload/start/stop, enabling
 * write mode). Uses <dialog> element for native modal behavior and keyboard
 * handling.
 *
 * @param {{ open: boolean, label: string, action: string, title?: string,
 *           note?: string, confirmClass?: string, onConfirm: () => void,
 *           onCancel: () => void }} props
 */
export function ConfirmDialog({ open, label, action, title, note, confirmClass, onConfirm, onCancel }) {
  const ref = useRef(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) {
      el.showModal();
    } else if (!open && el.open) {
      el.close();
    }
  }, [open]);

  if (!open) return null;

  const actionLabel = confirmVerb(action);

  return html`
    <dialog ref=${ref} class="confirm-dialog" onCancel=${onCancel} aria-label=${title || t('confirm.fallback', { action: actionLabel })}>
      <div class="confirm-dialog__body">
        <p class="confirm-dialog__title">${title || t('confirm.fallback', { action: actionLabel })}</p>
        <p class="confirm-dialog__label"><code>${label}</code></p>
        ${note && html`<p class="confirm-dialog__note">${note}</p>`}
        <div class="confirm-dialog__actions">
          <button class="btn btn--secondary" onClick=${onCancel}>${t('verb.cancel')}</button>
          <button class=${`btn ${confirmClass || 'btn--danger'}`} onClick=${onConfirm}>${actionLabel}</button>
        </div>
      </div>
    </dialog>
  `;
}
