import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { targetsFor } from './job-table.js';

/**
 * Which jobs a group's bulk buttons would act on. Getting this wrong is how a
 * "Stop all" either misses the job you were trying to stop (a restart loop with
 * no pid at that instant) or touches one you did not mean to.
 */
describe('targetsFor', () => {
  const jobs = [
    { label: 'a.running', pid: 100, status: 'running' },
    { label: 'b.flapping', pid: 0, status: 'error', keepAlive: true },
    { label: 'c.scheduled', pid: 0, status: 'scheduled' },
    { label: 'd.offline', pid: 0, status: 'offline' },
    { label: 'e.disabled', pid: 0, status: 'disabled', disabled: true },
    { label: 'f.running.ka', pid: 200, status: 'running', keepAlive: true },
  ];

  it('stop covers everything launchd is supervising, including a pid-less restart loop', () => {
    assert.deepEqual(targetsFor(jobs, 'stop'), ['a.running', 'b.flapping', 'f.running.ka']);
  });

  it('start covers what is not running, but leaves retired jobs alone', () => {
    assert.deepEqual(targetsFor(jobs, 'start'), ['b.flapping', 'c.scheduled', 'd.offline']);
  });

  it('reload covers only what currently has a process', () => {
    assert.deepEqual(targetsFor(jobs, 'reload'), ['a.running', 'f.running.ka']);
  });

  it('disable covers everything not already retired', () => {
    assert.deepEqual(targetsFor(jobs, 'disable'), [
      'a.running', 'b.flapping', 'c.scheduled', 'd.offline', 'f.running.ka',
    ]);
  });

  it('unknown actions touch nothing', () => {
    assert.deepEqual(targetsFor(jobs, ''), []);
  });
});
