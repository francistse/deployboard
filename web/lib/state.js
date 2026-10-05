import { signal, computed, effect } from '@preact/signals';
import { classifyJob } from './classify.js';
import { stormLevel } from './format.js';

const SHOW_NOISE_KEY = 'deployboard:show-noise';
const LEGACY_SHOW_NOISE_KEY = 'launch-pilot:show-noise';

function readShowNoise() {
	try {
		let value = localStorage.getItem(SHOW_NOISE_KEY);
		if (value === null) {
			const legacy = localStorage.getItem(LEGACY_SHOW_NOISE_KEY);
			if (legacy !== null) {
				localStorage.setItem(SHOW_NOISE_KEY, legacy);
				value = legacy;
			}
		}
		return value === 'true';
	} catch {
		return false;
	}
}

/** @type {import('@preact/signals').Signal<Array>} Full job list from SSE */
export const jobs = signal([]);

/** @type {import('@preact/signals').Signal<string>} Current search/filter text */
export const searchQuery = signal('');

/** @type {import('@preact/signals').Signal<object|null>} Currently selected job */
export const selectedJob = signal(null);

/** @type {import('@preact/signals').Signal<Array<{id:number, message:string, ok:boolean}>>} Active toast notifications */
export const toasts = signal([]);

/** @type {import('@preact/signals').Signal<string|null>} Label of job whose detail panel is open */
export const expandedJob = signal(null);

/** @type {import('@preact/signals').Signal<'logs'|'diagnose'|null>} Which panel is active */
export const activePanel = signal(null);

/** @type {import('@preact/signals').Signal<'all'|'ours'|'noise'|'other'>} */
export const categoryFilter = signal('all');

/** @type {import('@preact/signals').Signal<'all'|'running'|'scheduled'|'completed'|'stopped'|'error'|'offline'|'disabled'>} */
export const statusFilter = signal('all');

/**
 * Write-mode state from GET /api/settings/access. `read_only` true means every
 * mutating action is refused server-side; `locked` means --read-only was passed
 * at start-up, so the toggler cannot enable write mode either.
 * @type {import('@preact/signals').Signal<object|null>}
 */
export const accessInfo = signal(null);

/** @type {import('@preact/signals').Signal<boolean>} Convenience view of accessInfo */
export const readOnly = computed(() => accessInfo.value?.read_only === true);

/** @type {import('@preact/signals').Signal<boolean>} Show system & vendor jobs — persisted to localStorage */
export const showNoise = signal(readShowNoise());

// Persist showNoise to localStorage
effect(() => {
	try { localStorage.setItem(SHOW_NOISE_KEY, String(showNoise.value)); }
	catch { /* private browsing */ }
});

// showNoise OFF → categoryFilter cannot be 'noise'
let _prevShowNoise = showNoise.peek();
effect(() => {
	const current = showNoise.value;
	if (!current && categoryFilter.value === 'noise') {
		categoryFilter.value = 'all';
	}
	_prevShowNoise = current;
});

/**
 * Jobs that are restarting repeatedly right now, worst first. Drives the banner
 * at the top of the table: the whole point is that you should not have to read
 * the table to notice a job is in a restart loop.
 */
export const churningJobs = computed(() => {
  const scored = jobs.value
    .map(job => ({ job, level: stormLevel(job), restarts: job.restartsRecent || 0 }))
    .filter(entry => entry.level !== null);
  scored.sort((a, b) => b.restarts - a.restarts);
  return scored;
});

/** Filtered jobs — pipeline: noise gate → category → status → search */
export const filteredJobs = computed(() => {
	let list = jobs.value;

	// Noise gate: when showNoise is off, drop all 'noise' jobs
	if (!showNoise.value) {
		list = list.filter(j => classifyJob(j) !== 'noise');
	}

	const cat = categoryFilter.value;
	if (cat !== 'all') {
		list = list.filter(j => classifyJob(j) === cat);
	}

	const st = statusFilter.value;
	if (st !== 'all') {
		list = list.filter(j => j.status === st);
	}

	const q = searchQuery.value.toLowerCase();
	if (q) {
		list = list.filter(j => j.label.toLowerCase().includes(q));
	}

	return list;
});

/** Category counts from full job list (not filtered). */
export const categoryCounts = computed(() => {
	const list = jobs.value;
	const counts = { all: list.length, ours: 0, noise: 0, other: 0 };
	for (const j of list) counts[classifyJob(j)]++;
	return counts;
});

/** Status counts from full job list (not filtered). */
export const statusCounts = computed(() => {
	const list = jobs.value;
	const counts = {
		all: list.length,
		running: 0,
		scheduled: 0,
		completed: 0,
		stopped: 0,
		error: 0,
		offline: 0,
		disabled: 0,
	};
	for (const j of list) {
		const s = j.status;
		if (s in counts) counts[s]++;
	}
	return counts;
});

let _toastId = 0;

/** Add a toast notification that auto-dismisses after 3 seconds. */
export function addToast(message, ok) {
	const id = ++_toastId;
	toasts.value = [...toasts.value, { id, message, ok }];
	setTimeout(() => {
		toasts.value = toasts.value.filter(t => t.id !== id);
	}, 3000);
}

/** Remove a specific toast by id. */
export function removeToast(id) {
	toasts.value = toasts.value.filter(t => t.id !== id);
}

/** @type {import('@preact/signals').Signal<{anchor: HTMLElement, label: string, enteredVia: 'focus'|'hover'} | null>} */
export const tooltipTarget = signal(null);