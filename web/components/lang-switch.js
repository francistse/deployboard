import { html } from 'htm/preact';
import { t, locale, setLocale, LOCALES } from '../lib/i18n.js';

/**
 * Compact language radiogroup. Same control in the header and Settings so a
 * locale change is one click away and stays consistent wherever it is shown.
 */
export function LangSwitch({ className = '' } = {}) {
  const current = locale.value;
  return html`
    <div class=${`lang-switch ${className}`.trim()} role="radiogroup" aria-label=${t('settings.language')}>
      ${LOCALES.map((code) => html`
        <button
          key=${code}
          type="button"
          role="radio"
          class=${`lang-switch__btn ${current === code ? 'lang-switch__btn--active' : ''}`}
          aria-checked=${current === code}
          title=${t('lang.' + code)}
          onClick=${() => setLocale(code)}
        >${t('lang.' + code)}</button>
      `)}
    </div>
  `;
}
