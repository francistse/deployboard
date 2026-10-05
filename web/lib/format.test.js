import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { formatUptime, formatAgo, sparkBars, stormLevel, churnSummary } from './format.js';

describe('formatUptime', () => {
  it('picks the largest useful unit', () => {
    assert.equal(formatUptime(42), '42s');
    assert.equal(formatUptime(600), '10m');
    assert.equal(formatUptime(3660), '1h 1m');
    assert.equal(formatUptime(3 * 86400 + 4 * 3600), '3d 4h');
  });

  it('returns nothing for unknown or zero durations', () => {
    // Callers use '' to skip the element rather than render "0s" for a job that
    // is not running.
    assert.equal(formatUptime(0), '');
    assert.equal(formatUptime(undefined), '');
    assert.equal(formatUptime(-5), '');
    assert.equal(formatUptime(NaN), '');
  });
});

describe('formatAgo', () => {
  const now = Date.parse('2026-05-01T12:00:00Z');

  it('reads naturally at every scale', () => {
    assert.equal(formatAgo('2026-05-01T11:59:30Z', now), 'just now');
    assert.equal(formatAgo('2026-05-01T11:30:00Z', now), '30 min ago');
    assert.equal(formatAgo('2026-05-01T09:00:00Z', now), '3 hours ago');
    assert.equal(formatAgo('2026-04-28T12:00:00Z', now), '3 days ago');
    assert.equal(formatAgo('2026-03-01T12:00:00Z', now), '2 months ago');
    assert.equal(formatAgo('2024-05-01T12:00:00Z', now), '2 years ago');
  });

  it('handles a single unit without a plural', () => {
    assert.equal(formatAgo('2026-04-30T12:00:00Z', now), '1 day ago');
    assert.equal(formatAgo('2026-04-01T12:00:00Z', now), '1 month ago');
  });

  it('accepts epoch milliseconds and tolerates rubbish', () => {
    assert.equal(formatAgo(Date.parse('2026-04-28T12:00:00Z'), now), '3 days ago');
    assert.equal(formatAgo('not a date', now), '');
    assert.equal(formatAgo(null, now), '');
  });
});

describe('sparkBars', () => {
  it('always returns a fixed-width series so the shape does not jump', () => {
    assert.equal(sparkBars([], 24).length, 24);
    assert.equal(sparkBars([1, 2, 3], 24).length, 24);
    assert.equal(sparkBars([1, 2, 3], 4).length, 4);
  });

  it('scales the tallest restart to a full bar and keeps zeros flat', () => {
    const bars = sparkBars([0, 5, 10], 3);
    assert.deepEqual(bars, [0, 5, 10]);
  });

  it('keeps a single restart visible instead of rounding it away', () => {
    // Alone in the window it is the tallest bar...
    assert.deepEqual(sparkBars([0, 0, 1], 3), [0, 0, 10]);
    // ...and next to a huge count it must still be at least 1px tall, not rounded
    // down to nothing.
    const bars = sparkBars([0, 0, 1, 400], 4);
    assert.deepEqual(bars, [0, 0, 1, 10]);
  });

  it('survives a non-array series from an older server', () => {
    assert.deepEqual(sparkBars(undefined, 3), [0, 0, 0]);
    assert.deepEqual(sparkBars(null, 2), [0, 0]);
  });
});

describe('stormLevel / churnSummary', () => {
  it('calls out real churn over the observed window', () => {
    assert.equal(stormLevel({ restartsRecent: 12, windowMinutes: 30 }), 'storm');
    assert.equal(stormLevel({ restartsRecent: 5, windowMinutes: 30 }), 'churn');
    assert.equal(stormLevel({ restartsRecent: 1, windowMinutes: 30 }), null);
  });

  it('ignores the cumulative counter when there is no window yet', () => {
    // A job with 1207 lifetime restarts must not raise a permanent banner: on a
    // fresh dashboard the honest answer is "no recent churn observed yet".
    assert.equal(stormLevel({ restartWarn: true }), null);
    assert.equal(stormLevel({ restartsRecent: 0, windowMinutes: 0 }), null);
    assert.equal(stormLevel({ restartWarn: true, restartsRecent: 6, windowMinutes: 30 }), 'churn');
  });

  it('says nothing when there is nothing to report', () => {
    assert.equal(stormLevel({}), null);
    assert.equal(stormLevel(null), null);
    assert.equal(churnSummary({ restartsRecent: 0, windowMinutes: 30 }), '');
    assert.equal(churnSummary({}), '');
  });

  it('summarises the window in words', () => {
    assert.equal(churnSummary({ restartsRecent: 7, windowMinutes: 12 }), '7 restarts in the last 12m');
    assert.equal(churnSummary({ restartsRecent: 1, windowMinutes: 5 }), '1 restart in the last 5m');
  });
});
