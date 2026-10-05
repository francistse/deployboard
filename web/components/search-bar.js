import { html } from 'htm/preact';
import { searchQuery } from '../lib/state.js';
import { t } from '../lib/i18n.js';

export function SearchBar() {
  const onInput = (e) => {
    searchQuery.value = e.target.value;
  };

  return html`
    <div class="search-bar">
      <input
        type="text"
        class="search-bar__input"
        placeholder=${t('search.placeholder')}
        aria-label=${t('search.placeholder')}
        value=${searchQuery}
        onInput=${onInput}
      />
    </div>
  `;
}
