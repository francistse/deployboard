import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';
import { t } from '../lib/i18n.js';

const THEME_KEY = 'deployboard:theme';
const LEGACY_THEME_KEY = 'launch-pilot:theme';

/** Allowed values: 'system' | 'light' | 'dark'. */
function readTheme() {
  try {
    const current = localStorage.getItem(THEME_KEY);
    if (current) return current;
    const legacy = localStorage.getItem(LEGACY_THEME_KEY);
    if (legacy) {
      localStorage.setItem(THEME_KEY, legacy);
      return legacy;
    }
    return 'system';
  } catch { return 'system'; }
}

function applyTheme(mode) {
  document.documentElement.setAttribute('data-theme', mode);
  try { localStorage.setItem(THEME_KEY, mode); } catch { /* private browsing */ }
}

const MODES = [
  { key: 'system', icon: '🖥', labelKey: 'theme.system' },
  { key: 'light', icon: '☀', labelKey: 'theme.light' },
  { key: 'dark', icon: '🌙', labelKey: 'theme.dark' },
];

/**
 * Three-way theme switch: System (follows macOS appearance) / Light / Dark.
 * Persisted in localStorage and applied via `data-theme` on <html>, which the
 * stylesheet keys off alongside the prefers-color-scheme media query.
 */
export function ThemeToggle() {
  const [mode, setMode] = useState(readTheme);

  useEffect(() => {
    applyTheme(mode);
    // While on "system", follow live appearance changes without a reload.
    if (mode !== 'system' || !window.matchMedia) return undefined;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => applyTheme('system');
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, [mode]);

  return html`
    <div class="theme-toggle" role="radiogroup" aria-label=${t('theme.label')}>
      ${MODES.map((m) => html`
        <button
          key=${m.key}
          type="button"
          role="radio"
          aria-checked=${mode === m.key}
          class=${`theme-toggle__btn ${mode === m.key ? 'theme-toggle__btn--active' : ''}`}
          title=${t(m.labelKey)}
          onClick=${() => setMode(m.key)}
        >${m.icon}<span class="theme-toggle__label">${t(m.labelKey)}</span></button>
      `)}
    </div>
  `;
}
