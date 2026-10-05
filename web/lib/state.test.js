import { describe, it, beforeEach } from 'node:test';
import { strict as assert } from 'node:assert';
import {
  filteredJobs,
  categoryFilter,
  statusFilter,
  showNoise,
  categoryCounts,
  statusCounts,
  accessInfo,
  readOnly,
} from './state.js';
import { jobs, searchQuery } from './state.js';
import { FIXTURES, COUNTS, WITH_ALL_STATUSES, resetSignals } from './test-fixtures.js';

describe('filteredJobs', () => {
  beforeEach(resetSignals);

  it('returns every visible job when no filter is active (noise hidden)', () => {
    jobs.value = FIXTURES;
    assert.equal(filteredJobs.value.length, COUNTS.ours + COUNTS.other);
  });

  it('categoryFilter="ours" → only the deployments you classified as yours', () => {
    jobs.value = FIXTURES;
    categoryFilter.value = 'ours';
    const labels = filteredJobs.value.map(j => j.label);
    assert.deepEqual(labels, [
      'com.example.myapp',
      'org.homebrew.mxcl.redis',
      'com.myco.agent',
    ]);
  });

  it('categoryFilter="other" → only unclassified jobs', () => {
    jobs.value = FIXTURES;
    categoryFilter.value = 'other';
    assert.deepEqual(filteredJobs.value.map(j => j.label), ['com.example.legacy']);
  });

  it('categoryFilter="noise" → the vendor/OS jobs, once they are shown', () => {
    jobs.value = FIXTURES;
    showNoise.value = true;
    categoryFilter.value = 'noise';
    const labels = filteredJobs.value.map(j => j.label);
    assert.deepEqual(labels, [
      'com.apple.spotlight',
      'com.apple.WindowServer',
      'com.docker.vmnetd',
      'com.microsoft.autoupdate',
    ]);
  });

  it('statusFilter="running" → only running jobs (noise stays hidden)', () => {
    jobs.value = FIXTURES;
    statusFilter.value = 'running';
    assert.deepEqual(filteredJobs.value.map(j => j.label), ['org.homebrew.mxcl.redis']);
  });

  it('statusFilter="running" + showNoise → every running job', () => {
    jobs.value = FIXTURES;
    showNoise.value = true;
    statusFilter.value = 'running';
    const labels = filteredJobs.value.map(j => j.label);
    assert.deepEqual(labels, [
      'com.apple.spotlight',
      'com.apple.WindowServer',
      'org.homebrew.mxcl.redis',
    ]);
  });

  it('statusFilter="disabled" → the job retired on purpose', () => {
    jobs.value = FIXTURES;
    statusFilter.value = 'disabled';
    assert.deepEqual(filteredJobs.value.map(j => j.label), ['com.example.legacy']);
  });

  it('statusFilter="error" → only error jobs (noise stays hidden)', () => {
    jobs.value = FIXTURES;
    statusFilter.value = 'error';
    assert.deepEqual(filteredJobs.value.map(j => j.label), ['com.myco.agent']);
  });

  it('combined: categoryFilter + searchQuery → intersection of both', () => {
    jobs.value = FIXTURES;
    categoryFilter.value = 'ours';
    searchQuery.value = 'redis';
    assert.deepEqual(filteredJobs.value.map(j => j.label), ['org.homebrew.mxcl.redis']);
  });

  it('search query is preserved across filter changes', () => {
    jobs.value = FIXTURES;
    searchQuery.value = 'com.';
    assert.ok(filteredJobs.value.length > 0);

    categoryFilter.value = 'ours';
    for (const j of filteredJobs.value) {
      assert.ok(j.label.toLowerCase().includes('com.'));
    }

    statusFilter.value = 'stopped';
    for (const j of filteredJobs.value) {
      assert.ok(j.label.toLowerCase().includes('com.'));
      assert.equal(j.status, 'stopped');
    }
  });
});

describe('categoryCounts', () => {
  beforeEach(resetSignals);

  it('computed from the full jobs list, not filteredJobs', () => {
    jobs.value = FIXTURES;
    statusFilter.value = 'error';
    const counts = categoryCounts.value;
    assert.equal(counts.all, COUNTS.all);
    assert.equal(counts.ours, COUNTS.ours);
    assert.equal(counts.noise, COUNTS.noise);
    assert.equal(counts.other, COUNTS.other);
  });
});

describe('statusCounts', () => {
  beforeEach(resetSignals);

  it('computed from the full jobs list, not filteredJobs', () => {
    jobs.value = FIXTURES;
    categoryFilter.value = 'ours';
    const counts = statusCounts.value;
    assert.equal(counts.all, COUNTS.all);
    assert.equal(counts.running, 3);
    assert.equal(counts.stopped, 2);
    assert.equal(counts.error, 2);
    assert.equal(counts.disabled, 1);
  });

  it('has a key for every launchd status, defaulting to 0', () => {
    jobs.value = [];
    const counts = statusCounts.value;
    for (const key of ['running', 'scheduled', 'completed', 'stopped', 'error', 'offline', 'disabled']) {
      assert.equal(counts[key], 0, `${key} should default to 0`);
    }
  });

  it('counts all seven statuses when present', () => {
    jobs.value = WITH_ALL_STATUSES;
    const counts = statusCounts.value;
    assert.equal(counts.all, 7);
    for (const key of ['running', 'scheduled', 'completed', 'stopped', 'error', 'offline', 'disabled']) {
      assert.equal(counts[key], 1, `${key} should count 1`);
    }
  });
});

// The write-mode switch is server state, but the UI reads it through these
// signals, so the failure mode worth testing is "unknown → treat as read-only?"
// (it is not: the buttons stay enabled until the server says otherwise, because
// the server refuses the action anyway and the 403 carries the reason).
describe('access signals', () => {
  beforeEach(() => { accessInfo.value = null; });

  it('readOnly is false until the server reports the state', () => {
    assert.equal(readOnly.value, false);
  });

  it('readOnly follows read_only from GET /api/settings/access', () => {
    accessInfo.value = { read_only: true, locked: false, source: 'config' };
    assert.equal(readOnly.value, true);

    accessInfo.value = { read_only: false, locked: false, source: 'config' };
    assert.equal(readOnly.value, false);
  });

  it('a locked server is still read-only', () => {
    accessInfo.value = { read_only: true, locked: true, lock_reason: 'started with --read-only' };
    assert.equal(readOnly.value, true);
  });
});
