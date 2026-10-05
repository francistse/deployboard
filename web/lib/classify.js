const APPLE_PREFIX = 'com.apple.';

/**
 * Display labels for each category in the fork.
 * "Ours" = your deployed apps; "Noise" = system/vendor background; "Other" = everything else.
 */
export const CATEGORY_LABELS = {
	ours: 'Ours',
	noise: 'Noise',
	other: 'Other',
};

/** All category filter keys including "all". */
export const CATEGORY_KEYS = ['all', 'ours', 'noise', 'other'];

/** All status filter keys including "all". Added "disabled". */
export const STATUS_KEYS = [
	'all',
	'running',
	'scheduled',
	'completed',
	'stopped',
	'error',
	'offline',
	'disabled',
];

/**
 * Classify a job using the fork's category field (set by Go backend).
 * Falls back to the old heuristic when category is absent.
 */
export function classifyJob(job) {
	if (job.category) return job.category;
	if (!job.label) return 'other';
	if (job.label.startsWith(APPLE_PREFIX)) return 'noise';
	if (job.domain === 'user') return 'ours';
	return 'other';
}