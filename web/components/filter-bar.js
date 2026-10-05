import { html } from 'htm/preact';
import {
	categoryFilter,
	statusFilter,
	showNoise,
	categoryCounts,
	statusCounts,
} from '../lib/state.js';
import { CATEGORY_KEYS, STATUS_KEYS } from '../lib/classify.js';
import { t } from '../lib/i18n.js';

/**
 * English status labels. The bar itself renders t('status.*') so a locale
 * change updates the tabs; these constants stay the en wording tests lock.
 */
export const STATUS_DISPLAY = {
	all: 'All',
	running: 'Running',
	scheduled: 'Scheduled',
	completed: 'Completed',
	stopped: 'Stopped',
	error: 'Error',
	offline: 'Offline',
	disabled: 'Disabled',
};

export function FilterBar() {
	const cat = categoryFilter.value;
	const st = statusFilter.value;
	const noise = showNoise.value;
	const catCounts = categoryCounts.value;
	const stCounts = statusCounts.value;

	return html`
		<div class="filter-bar">
			<div class="filter-bar__row filter-bar__row--space-between">
				<div class="filter-bar__chips">
					${CATEGORY_KEYS.map(key => {
						const isActive = cat === key;
						const isNoiseLocked = !noise && key === 'noise';
						let cls = 'filter-chip';
						if (isActive) cls += ' filter-chip--active';
						if (isNoiseLocked) cls += ' filter-chip--locked';
						return html`
							<button
								class=${cls}
								title=${isNoiseLocked ? t('filter.noiseLockedTitle') : ''}
								onClick=${() => {
									// Clicking Noise is a request to see them: flip the
									// toggle instead of blocking the click.
									if (isNoiseLocked) showNoise.value = true;
									categoryFilter.value = key;
								}}
							>
								${t('category.' + key)}
								<span class="filter-chip__count">${catCounts[key]}</span>
							</button>
						`;
					})}
				</div>
				<label class="noise-toggle">
					${t('filter.showNoise')}
					<input
						type="checkbox"
						class="noise-toggle__input"
						checked=${noise}
						onChange=${() => { showNoise.value = !showNoise.value; }}
					/>
					<span class="noise-toggle__slider"></span>
				</label>
			</div>
			<div class="filter-bar__row">
				${STATUS_KEYS.map(key => {
					const isActive = st === key;
					let cls = 'status-tab status-tab--' + key;
					if (isActive) cls += ' status-tab--active';
					return html`
						<button
							class=${cls}
							onClick=${() => { statusFilter.value = key; }}
						>
							${t('status.' + key)} (${stCounts[key]})
						</button>
					`;
				})}
			</div>
		</div>
	`;
}