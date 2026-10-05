import { html } from 'htm/preact';
import { toasts, removeToast } from '../lib/state.js';
import { t } from '../lib/i18n.js';

/**
 * Toast container — renders active toast notifications.
 * Green for success (ok=true), red for failure (ok=false).
 * Each toast auto-dismisses after 3 seconds (handled in state.js).
 */
export function ToastContainer() {
  const list = toasts.value;
  if (list.length === 0) return null;

  return html`
    <div class="toast-container">
      ${list.map(item => html`
        <div key=${item.id} class="toast toast--${item.ok ? 'success' : 'error'}">
          <span class="toast__message">${item.message}</span>
          <button class="toast__close" onClick=${() => removeToast(item.id)} aria-label=${t('toast.close')} title=${t('toast.close')}>\u00d7</button>
        </div>
      `)}
    </div>
  `;
}
