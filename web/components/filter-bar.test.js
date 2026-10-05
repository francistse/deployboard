import { describe, it, beforeEach } from 'node:test';
import { strict as assert } from 'node:assert';
import {
  jobs,
  filteredJobs,
  categoryFilter,
  statusFilter,
  showNoise,
  categoryCounts,
  statusCounts,
} from '../lib/state.js';
import { searchQuery } from '../lib/state.js';
import { CATEGORY_LABELS, CATEGORY_KEYS, STATUS_KEYS } from '../lib/classify.js';
import { STATUS_DISPLAY } from './filter-bar.js';
import { FIXTURES, COUNTS, resetSignals } from '../lib/test-fixtures.js';

describe('FilterBar: category chips', () => {
  beforeEach(resetSignals);

  it('has 4 category options: All, Ours, Noise, Other', () => {
    assert.equal(CATEGORY_KEYS.length, 4);
    assert.deepEqual(CATEGORY_KEYS, ['all', 'ours', 'noise', 'other']);
    assert.equal(CATEGORY_LABELS.ours, 'Ours');
    assert.equal(CATEGORY_LABELS.noise, 'Noise');
    assert.equal(CATEGORY_LABELS.other, 'Other');
  });

  it('categoryCounts reflects the full job list, noise included', () => {
    jobs.value = FIXTURES;
    const counts = categoryCounts.value;
    assert.equal(counts.all, COUNTS.all);
    assert.equal(counts.ours, COUNTS.ours);
    assert.equal(counts.noise, COUNTS.noise);
    assert.equal(counts.other, COUNTS.other);
  });

  it('setting categoryFilter updates filteredJobs', () => {
    jobs.value = FIXTURES;
    showNoise.value = true; // the noise chip is only selectable with noise shown

    categoryFilter.value = 'ours';
    assert.equal(filteredJobs.value.length, COUNTS.ours);
    categoryFilter.value = 'noise';
    assert.equal(filteredJobs.value.length, COUNTS.noise);
    categoryFilter.value = 'other';
    assert.equal(filteredJobs.value.length, COUNTS.other);
    categoryFilter.value = 'all';
    assert.equal(filteredJobs.value.length, COUNTS.all);
  });
});

describe('FilterBar: status tabs', () => {
  beforeEach(resetSignals);

  it('has 8 status options including disabled', () => {
    assert.equal(STATUS_KEYS.length, 8);
    assert.deepEqual(STATUS_KEYS, [
      'all', 'running', 'scheduled', 'completed', 'stopped', 'error', 'offline', 'disabled',
    ]);
  });

  it('STATUS_DISPLAY exports a label for all 8 statuses', () => {
    assert.equal(STATUS_DISPLAY.all, 'All');
    assert.equal(STATUS_DISPLAY.running, 'Running');
    assert.equal(STATUS_DISPLAY.scheduled, 'Scheduled');
    assert.equal(STATUS_DISPLAY.completed, 'Completed');
    assert.equal(STATUS_DISPLAY.stopped, 'Stopped');
    assert.equal(STATUS_DISPLAY.error, 'Error');
    assert.equal(STATUS_DISPLAY.offline, 'Offline');
    assert.equal(STATUS_DISPLAY.disabled, 'Disabled');
  });

  it('statusCounts reflects correct counts per status', () => {
    jobs.value = FIXTURES;
    const counts = statusCounts.value;
    assert.equal(counts.all, COUNTS.all);
    assert.equal(counts.running, 3);
    assert.equal(counts.stopped, 2);
    assert.equal(counts.error, 2);
    assert.equal(counts.disabled, 1);
  });
});

describe('FilterBar: noise gate', () => {
  beforeEach(resetSignals);

  it('hides every noise job while showNoise is off', () => {
    jobs.value = FIXTURES;
    assert.equal(filteredJobs.value.length, COUNTS.ours + COUNTS.other);
    assert.ok(filteredJobs.value.every(j => j.category !== 'noise'));
  });

  it('showNoise=true brings the vendor/OS jobs back', () => {
    jobs.value = FIXTURES;
    showNoise.value = true;
    assert.equal(filteredJobs.value.length, COUNTS.all);
  });

  it('cannot stay on the noise chip once noise is hidden', () => {
    jobs.value = FIXTURES;
    showNoise.value = true;
    categoryFilter.value = 'noise';
    assert.equal(categoryFilter.value, 'noise');

    showNoise.value = false;
    assert.equal(categoryFilter.value, 'all', 'the noise chip must fall back to all');
  });
});

describe('FilterBar: filter composition', () => {
  beforeEach(resetSignals);

  it('category + status compose as AND (Ours + Error = intersection)', () => {
    jobs.value = FIXTURES;
    categoryFilter.value = 'ours';
    statusFilter.value = 'error';
    const labels = filteredJobs.value.map(j => j.label);
    assert.deepEqual(labels, ['com.myco.agent']);
  });

  it('category + status + search all compose as AND', () => {
    jobs.value = FIXTURES;
    showNoise.value = true;
    categoryFilter.value = 'noise';
    statusFilter.value = 'running';
    searchQuery.value = 'spotlight';
    const labels = filteredJobs.value.map(j => j.label);
    assert.deepEqual(labels, ['com.apple.spotlight']);
  });

  it('search is preserved across chip switches', () => {
    jobs.value = FIXTURES;
    searchQuery.value = 'com.';

    for (const cat of ['all', 'ours', 'other']) {
      categoryFilter.value = cat;
      for (const j of filteredJobs.value) {
        assert.ok(j.label.toLowerCase().includes('com.'));
      }
    }
  });

  it('status filter still applies while searching', () => {
    jobs.value = FIXTURES;
    searchQuery.value = 'com.';
    statusFilter.value = 'error';
    for (const j of filteredJobs.value) {
      assert.equal(j.status, 'error');
    }
  });
});
