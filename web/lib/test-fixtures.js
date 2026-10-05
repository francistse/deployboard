import {
  jobs,
  searchQuery,
  categoryFilter,
  statusFilter,
  showNoise,
} from './state.js';

/**
 * Job fixtures in the fork's shape: the Go backend sets `category`
 * (ours | noise | other) and `status` (the seven launchd states). The
 * classification tests deliberately omit `category` to exercise the fallback.
 */
export const FIXTURES = [
  { label: 'com.apple.spotlight',      domain: 'user',   category: 'noise', status: 'running' },
  { label: 'com.apple.WindowServer',   domain: 'global', category: 'noise', status: 'running' },
  { label: 'com.docker.vmnetd',        domain: 'global', category: 'noise', status: 'error' },
  { label: 'com.microsoft.autoupdate', domain: 'global', category: 'noise', status: 'stopped' },
  { label: 'com.example.myapp',        domain: 'user',   category: 'ours',  status: 'stopped' },
  { label: 'org.homebrew.mxcl.redis',  domain: 'user',   category: 'ours',  status: 'running' },
  { label: 'com.myco.agent',           domain: 'user',   category: 'ours',  status: 'error' },
  { label: 'com.example.legacy',       domain: 'global', category: 'other', status: 'disabled' },
];

/** Counts the suites assert on: ours 3, noise 4, other 1, all 8. */
export const COUNTS = { all: 8, ours: 3, noise: 4, other: 1 };

/** One job per status, for the status-count and badge coverage. */
export const WITH_ALL_STATUSES = [
  { label: 'com.example.a', category: 'ours', status: 'running' },
  { label: 'com.example.b', category: 'ours', status: 'scheduled' },
  { label: 'com.example.c', category: 'ours', status: 'completed' },
  { label: 'com.example.d', category: 'ours', status: 'stopped' },
  { label: 'com.example.e', category: 'ours', status: 'error' },
  { label: 'com.example.f', category: 'ours', status: 'offline' },
  { label: 'com.example.g', category: 'ours', status: 'disabled' },
];

export function resetSignals() {
  jobs.value = [];
  searchQuery.value = '';
  categoryFilter.value = 'all';
  statusFilter.value = 'all';
  // Deterministic: the noise gate is persisted to localStorage, which the Node
  // test runner does not have.
  showNoise.value = false;
}
