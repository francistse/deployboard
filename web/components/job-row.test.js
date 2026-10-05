import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { renderToString } from 'preact-render-to-string';
import { html } from 'htm/preact';
import { classifyJob, CATEGORY_LABELS } from '../lib/classify.js';
import { buildStatusTooltip, buildStatusTooltipParts, placeTooltip } from './job-tooltip.js';
import { JobRow, ACTION_COPY, verifyAfterAction } from './job-row.js';

/**
 * S04 — Category Badge in Job Rows
 *
 * job-row.js renders badge class and text as:
 *   class = 'category-badge category-badge--' + classifyJob(job)
 *   text  = CATEGORY_LABELS[classifyJob(job)]
 *
 * These tests verify the contract between classify.js output
 * and the expected badge attributes for each job type.
 */

describe('JobRow category badge derivation', () => {
  const badgeClass = (job) => 'category-badge category-badge--' + classifyJob(job);
  const badgeText = (job) => CATEGORY_LABELS[classifyJob(job)];

  it('com.apple.* job → Noise badge with --noise class', () => {
    const job = { label: 'com.apple.spotlight', domain: 'user', category: 'noise' };
    assert.equal(badgeClass(job), 'category-badge category-badge--noise');
    assert.equal(badgeText(job), 'Noise');
  });

  it('com.apple.* job in global domain → still Noise badge', () => {
    const job = { label: 'com.apple.WindowServer', domain: 'global', category: 'noise' };
    assert.equal(badgeClass(job), 'category-badge category-badge--noise');
    assert.equal(badgeText(job), 'Noise');
  });

  it('a deployment of yours → Ours badge with --ours class', () => {
    const job = { label: 'com.example.myapp', domain: 'user', category: 'ours' };
    assert.equal(badgeClass(job), 'category-badge category-badge--ours');
    assert.equal(badgeText(job), 'Ours');
  });

  it('unclassified job → Other badge with --other class', () => {
    const job = { label: 'com.docker.vmnetd', domain: 'global', category: 'other' };
    assert.equal(badgeClass(job), 'category-badge category-badge--other');
    assert.equal(badgeText(job), 'Other');
  });

  it('the backend category wins over the label/domain fallback', () => {
    // A vendor label the path rule claimed as yours must read "Ours".
    const job = { label: 'com.docker.vmnetd', domain: 'global', category: 'ours' };
    assert.equal(badgeClass(job), 'category-badge category-badge--ours');
    assert.equal(badgeText(job), 'Ours');
  });

  it('every category key maps to a defined CATEGORY_LABELS entry', () => {
    // Ensure no undefined badge text for any classifyJob output
    const jobs = [
      { label: 'com.apple.x', domain: 'user' },
      { label: 'com.foo.bar', domain: 'user' },
      { label: 'com.foo.bar', domain: 'global' },
    ];
    for (const job of jobs) {
      const cat = classifyJob(job);
      assert.ok(CATEGORY_LABELS[cat] !== undefined,
        `CATEGORY_LABELS missing key "${cat}" for job ${job.label}`);
    }
  });
});

describe('buildStatusTooltip', () => {
  it('returns just status when no schedule fields are set', () => {
    const tip = buildStatusTooltip({ status: 'running' });
    assert.equal(tip, 'running');
  });

  it('includes Next run when nextRunAt is set', () => {
    const tip = buildStatusTooltip({
      status: 'scheduled',
      nextRunAt: '2026-04-17T09:00:00Z',
    });
    assert.ok(tip.startsWith('scheduled'));
    assert.ok(tip.includes('Next run:'));
  });

  it('includes Last run when lastRunAt is set', () => {
    const tip = buildStatusTooltip({
      status: 'completed',
      lastRunAt: '2026-04-17T08:55:00Z',
    });
    assert.ok(tip.includes('Last run:'));
  });

  it('shows unknown fallback for completed/scheduled without lastRunAt', () => {
    const tipCompleted = buildStatusTooltip({ status: 'completed' });
    assert.ok(tipCompleted.includes('Last run: unknown'));

    const tipScheduled = buildStatusTooltip({ status: 'scheduled' });
    assert.ok(tipScheduled.includes('Last run: unknown'));
  });

  it('does not show unknown fallback for stopped/running/error without lastRunAt', () => {
    assert.ok(!buildStatusTooltip({ status: 'stopped' }).includes('unknown'));
    assert.ok(!buildStatusTooltip({ status: 'running' }).includes('unknown'));
    assert.ok(!buildStatusTooltip({ status: 'error' }).includes('unknown'));
  });

  it('joins parts with em-dash-like separator', () => {
    const tip = buildStatusTooltip({
      status: 'scheduled',
      nextRunAt: '2026-04-17T09:00:00Z',
      lastRunAt: '2026-04-17T08:00:00Z',
    });
    // Three parts: status, Next run, Last run
    assert.equal(tip.split(' — ').length, 3);
  });
});

describe('buildStatusTooltipParts parity with buildStatusTooltip', () => {
  const cases = [
    { name: 'running no schedule',
      job: { status: 'running' } },
    { name: 'error with nextRunAt',
      job: { status: 'error', nextRunAt: '2026-04-17T09:00:00Z' } },
    { name: 'completed with lastRunAt',
      job: { status: 'completed', lastRunAt: '2026-04-17T08:00:00Z' } },
    { name: 'scheduled no lastRun with log path',
      job: { status: 'scheduled', nextRunAt: '2026-04-18T10:00:00Z',
             standardOutPath: '/tmp/a.log' } },
    { name: 'scheduled no lastRun no log path',
      job: { status: 'scheduled', nextRunAt: '2026-04-18T10:00:00Z' } },
    { name: 'completed no lastRun no log path',
      job: { status: 'completed' } },
    { name: 'stopped',
      job: { status: 'stopped' } },
    { name: 'offline with lastRunAt',
      job: { status: 'offline', lastRunAt: '2026-04-15T08:00:00Z' } },
  ];
  for (const { name, job } of cases) {
    it(`${name}: parts.join(' — ') === buildStatusTooltip`, () => {
      assert.equal(buildStatusTooltipParts(job).join(' — '), buildStatusTooltip(job));
    });
    it(`${name}: parts is a non-empty array of strings`, () => {
      const parts = buildStatusTooltipParts(job);
      assert.ok(Array.isArray(parts));
      assert.ok(parts.length >= 1);
      for (const p of parts) assert.equal(typeof p, 'string');
    });
  }
});

describe('StatusDot (via JobRow render)', () => {
  const renderRow = (job) => renderToString(html`<${JobRow} job=${job} />`);

  it('renders a <button> with status-dot + status-dot--<status> + status-dot-trigger classes', () => {
    const job = { label: 'com.example.a', status: 'running', domain: 'user', pid: 123, lastExitStatus: 0 };
    const out = renderRow(job);
    assert.match(out, /<button[^>]*class="[^"]*\bstatus-dot\b[^"]*\bstatus-dot--running\b[^"]*\bstatus-dot-trigger\b/);
    assert.match(out, /<button[^>]*type="button"/);
  });

  it('aria-label equals buildStatusTooltip(job) — running with no schedule', () => {
    const job = { label: 'com.example.r', status: 'running', domain: 'user', pid: 1, lastExitStatus: 0 };
    const expected = buildStatusTooltip(job);
    const out = renderRow(job);
    assert.ok(out.includes(`aria-label="${expected}"`), `render missing aria-label="${expected}":\n${out}`);
  });

  it('aria-label equals buildStatusTooltip(job) — scheduled with nextRunAt', () => {
    const job = {
      label: 'com.example.b', status: 'scheduled', domain: 'user',
      pid: 0, lastExitStatus: 0, nextRunAt: '2026-04-18T10:00:00Z',
    };
    const expected = buildStatusTooltip(job);
    const out = renderRow(job);
    const escaped = expected.replace(/&/g, '&amp;').replace(/"/g, '&quot;');
    assert.ok(out.includes(`aria-label="${escaped}"`), `render missing aria-label="${escaped}":\n${out}`);
  });

  it('aria-label equals buildStatusTooltip(job) — completed with lastRunAt', () => {
    const job = {
      label: 'com.example.c', status: 'completed', domain: 'user',
      pid: 0, lastExitStatus: 0, lastRunAt: '2026-04-18T09:00:00Z',
      standardOutPath: '/tmp/c.log',
    };
    const expected = buildStatusTooltip(job);
    const escaped = expected.replace(/&/g, '&amp;').replace(/"/g, '&quot;');
    const out = renderRow(job);
    assert.ok(out.includes(`aria-label="${escaped}"`), `render missing aria-label="${escaped}":\n${out}`);
  });

  it('carries data-label for E2E locators', () => {
    const job = { label: 'com.example.d', status: 'running', domain: 'user', pid: 1, lastExitStatus: 0 };
    const out = renderRow(job);
    assert.match(out, /<button[^>]*data-label="com\.example\.d"/);
  });

  it('does not set tabindex="-1" (button is naturally focusable)', () => {
    const job = { label: 'com.example.t', status: 'running', domain: 'user', pid: 1, lastExitStatus: 0 };
    const out = renderRow(job);
    // Look for the status-dot-trigger button and confirm no tabindex="-1" attribute on it.
    const m = out.match(/<button[^>]*status-dot-trigger[^>]*>/);
    assert.ok(m, `status-dot-trigger button not found in:\n${out}`);
    assert.ok(!/tabindex="-1"/i.test(m[0]), `unexpected tabindex="-1" on trigger: ${m[0]}`);
  });

  it('status-dot button comes before the action buttons in tab order', () => {
    const job = { label: 'com.example.e', status: 'running', domain: 'user', pid: 1, lastExitStatus: 0 };
    const out = renderRow(job);
    const dotIdx = out.indexOf('status-dot-trigger');
    const restartIdx = out.indexOf('>Restart<');
    const stopIdx = out.indexOf('>Stop<');
    assert.ok(dotIdx >= 0 && dotIdx < restartIdx, 'dot must come before the actions');
    assert.ok(restartIdx < stopIdx, 'the restart action comes before Stop');
  });

  // Stop is only offered when there is a process to stop. A scheduled job with
  // nothing running gets Start (and an explanation), never a Stop button that
  // would silently do nothing.
  it('offers Stop only while the job is running', () => {
    const running = renderRow({ label: 'com.example.r', status: 'running', domain: 'user', pid: 9 });
    assert.ok(running.includes('>Restart<'));
    assert.ok(running.includes('>Stop<'));
    assert.ok(!running.includes('>Start<'));

    for (const status of ['scheduled', 'stopped', 'completed', 'offline']) {
      const idle = renderRow({ label: 'com.example.i', status, domain: 'user', pid: 0 });
      assert.ok(idle.includes('>Start<'), `${status} should offer Start`);
      assert.ok(!idle.includes('>Stop<'), `${status} must not offer Stop with no process running`);
      assert.ok(!idle.includes('>Restart<'), `${status} has nothing to restart`);
    }
  });

  // A retired job is brought back with Enable, not Start/Stop.
  it('shows Enable instead of Disable on a retired job', () => {
    const retired = renderRow({ label: 'com.example.d', status: 'disabled', domain: 'user', pid: 0, disabled: true });
    assert.ok(retired.includes('>Enable<'), 'a retired job needs an Enable button');
    assert.ok(!retired.includes('>Disable<'), 'it is already disabled');
  });

  // The row only claims "retired <when>" when the dashboard recorded the
  // decision — a job that was disabled before the log existed gets no invented
  // date.
  it('dates a retirement only when the log knows about it', () => {
    const recorded = renderRow({
      label: 'com.example.d', status: 'disabled', domain: 'user', pid: 0, disabled: true,
      retiredAt: '2026-04-01T12:00:00Z',
    });
    assert.ok(/retired \d+ month/.test(recorded), `expected a retirement age, got: ${recorded.slice(0, 200)}`);

    const unknown = renderRow({ label: 'com.example.d', status: 'disabled', domain: 'user', pid: 0, disabled: true });
    assert.ok(!/retired /.test(unknown), 'no retirement date should be invented');
  });

  // A KeepAlive job caught between restarts has pid 0 but is still being
  // restarted by launchd; Stop is exactly what is needed there.
  it('offers Stop for a restart loop even when no pid is listed', () => {
    const flapping = renderRow({
      label: 'com.example.flap', status: 'error', domain: 'user', pid: 0, keepAlive: true, runs: 412,
    });
    assert.ok(flapping.includes('>Stop<'), 'a restart loop must be stoppable');
    // The confirmation copy is asserted on the table, since the dialog itself is
    // not in the DOM until it opens.
    assert.equal(ACTION_COPY.stop.title({ keepAlive: true, pid: 0 }), 'Stop this restart loop?');
    assert.equal(ACTION_COPY.stop.title({ keepAlive: true, pid: 5 }), 'Stop job? (it will be unloaded)');

    const idle = renderRow({ label: 'com.example.idle', status: 'scheduled', domain: 'user', pid: 0, keepAlive: false });
    assert.ok(!idle.includes('>Stop<'), 'nothing is running and nothing brings it back');
  });

  // Churn is called out in the row itself, not only in the banner.
  it('flags churn and shows a restart sparkline', () => {
    const churning = renderRow({
      label: 'com.example.c', status: 'running', domain: 'user', pid: 7, runs: 1207, keepAlive: true,
      restartsRecent: 14, windowMinutes: 30, runsSeries: [0, 1, 3, 10],
    });
    assert.ok(churning.includes('restart loop'), 'a restart loop must be visible in the row');
    assert.ok(churning.includes('spark'), 'and drawn as a sparkline');
    assert.ok(churning.includes('keepalive-mark'), 'KeepAlive explains why it can do that');

    const calm = renderRow({
      label: 'com.example.q', status: 'running', domain: 'user', pid: 8, runs: 3,
      restartsRecent: 0, windowMinutes: 30, runsSeries: [0, 0],
    });
    assert.ok(!calm.includes('restart loop'));
    assert.ok(!calm.includes('churn-badge'));
  });

  it('shows uptime next to the pid when the process age is known', () => {
    const out = renderRow({ label: 'com.example.u', status: 'running', domain: 'user', pid: 42, uptimeSeconds: 7325 });
    assert.ok(out.includes('2h 2m'), `expected a human uptime, got: ${out.slice(0, 200)}`);

    const noUptime = renderRow({ label: 'com.example.u', status: 'running', domain: 'user', pid: 42 });
    assert.ok(!noUptime.includes('job-row__uptime'));
  });

  it('marks the dashboard row itself', () => {
    const out = renderRow({
      label: 'com.deployboard.launch-pilot', status: 'running', domain: 'user', pid: 7, self: true, category: 'ours',
    });
    assert.ok(out.includes('self-badge'), 'the self row needs a marker next to the other badges');
    assert.ok(out.includes('this is the Deployboard dashboard itself'));

    const other = renderRow({ label: 'com.example.u', status: 'running', domain: 'user', pid: 8 });
    assert.ok(!other.includes('self-badge'));
  });
});

describe('verifyAfterAction (self restart tolerance)', () => {
  const running = { label: 'com.example.self', status: 'running', pid: 99 };

  it('retries when the read lands while the job is still respawning', async () => {
    let calls = 0;
    const flaky = async () => {
      calls += 1;
      if (calls < 3) throw new Error('fetch failed');
      return running;
    };
    const check = await verifyAfterAction('reload', running.label, { self: true, delayMs: 1, fetchJob: flaky });
    assert.equal(check.ok, true, 'a restart that worked must not report failure');
    assert.equal(calls, 3);
  });

  it('gives up with the honest message when the dashboard never comes back', async () => {
    let calls = 0;
    const dead = async () => { calls += 1; throw new Error('ECONNREFUSED'); };
    const check = await verifyAfterAction('reload', running.label, { self: true, delayMs: 1, fetchJob: dead });
    assert.equal(check.ok, false);
    assert.match(check.msg, /ECONNREFUSED/);
    assert.equal(calls, 4);
  });

  it('does not retry for an ordinary job', async () => {
    let calls = 0;
    const dead = async () => { calls += 1; throw new Error('ECONNREFUSED'); };
    const check = await verifyAfterAction('reload', running.label, { delayMs: 1, fetchJob: dead });
    assert.equal(check.ok, false);
    assert.equal(calls, 1);
  });
});

describe('placeTooltip', () => {
  const vp = { width: 1024, height: 768 };
  it('above anchor when top-space available', () => {
    const anchor = { top: 500, left: 500, width: 8, height: 8, bottom: 508, right: 508 };
    const tip = { width: 200, height: 40 };
    const pos = placeTooltip(anchor, tip, vp);
    assert.ok(pos.top < 500); // above
    assert.equal(pos.left, 500 + 4 - 100); // anchor center - tip width/2
  });
  it('flips below when top-space insufficient', () => {
    const anchor = { top: 4, left: 500, width: 8, height: 8, bottom: 12, right: 508 };
    const tip = { width: 200, height: 40 };
    const pos = placeTooltip(anchor, tip, vp);
    assert.ok(pos.top > 12); // below
  });
  it('clamps left edge to 4', () => {
    const anchor = { top: 500, left: 0, width: 8, height: 8, bottom: 508, right: 8 };
    const tip = { width: 200, height: 40 };
    const pos = placeTooltip(anchor, tip, vp);
    assert.equal(pos.left, 4);
  });
  it('clamps right edge to viewportW - tipW - 4', () => {
    const anchor = { top: 500, left: 1020, width: 8, height: 8, bottom: 508, right: 1028 };
    const tip = { width: 200, height: 40 };
    const pos = placeTooltip(anchor, tip, vp);
    assert.equal(pos.left, 1024 - 200 - 4);
  });
  it('centers horizontally over small anchor', () => {
    const anchor = { top: 500, left: 500, width: 8, height: 8, bottom: 508, right: 508 };
    const tip = { width: 100, height: 30 };
    const pos = placeTooltip(anchor, tip, vp);
    assert.equal(pos.left, 504 - 50);
  });
});

// The buttons must say what they will actually do: "Stop" on a KeepAlive job is
// an unload, not a signal, and that distinction is the whole point.
describe('JobRow action copy', () => {
  it('explains that stopping a KeepAlive job unloads it', () => {
    const note = ACTION_COPY.stop.note({ label: 'ai.fengshui.web', keepAlive: true });
    assert.match(note, /KeepAlive/);
    assert.match(note, /bootout/);
    assert.match(note, /Start brings it back/);
  });

  it('says a plain job is signalled and stays loaded', () => {
    const note = ACTION_COPY.stop.note({ label: 'com.example.oneshot', keepAlive: false });
    assert.match(note, /SIGTERM/);
    assert.ok(!/bootout/.test(note));
  });

  it('warns that Disable survives a reboot, and that Enable does not start it', () => {
    assert.match(ACTION_COPY.disable.note({}), /will not start it again/);
    assert.match(ACTION_COPY.disable.note({}), /Enable undoes this/);
    assert.match(ACTION_COPY.enable.note({}), /does not start/);
  });

  // Stop and Disable of the dashboard itself are allowed, but the page dies
  // with the process and KeepAlive will not bring it back after a bootout.
  it('tells you that stopping or retiring the dashboard leaves it down', () => {
    const stop = ACTION_COPY.stop.note({ self: true, keepAlive: true, pid: 7 });
    const disable = ACTION_COPY.disable.note({ self: true });
    for (const note of [stop, disable]) {
      assert.match(note, /this is the dashboard/);
      assert.match(note, /launchctl bootstrap/);
      assert.match(note, /prefer Restart/);
    }
    assert.equal(ACTION_COPY.stop.title({ self: true, keepAlive: true, pid: 7 }), 'Stop the dashboard?');
    assert.equal(ACTION_COPY.disable.title({ self: true }), 'Retire the dashboard?');
    assert.match(ACTION_COPY.stop.note({ keepAlive: true }), /Start brings it back/);
    assert.match(ACTION_COPY.disable.note({}), /Enable undoes this/);
  });

  // Restarting the dashboard is the one action that does NOT bootout, so the
  // generic "bootout + bootstrap" note would describe the wrong mechanism.
  it('describes a self restart as exit + respawn, not bootout', () => {
    const selfNote = ACTION_COPY.reload.note({ self: true });
    assert.match(selfNote, /exits and launchd respawns it/);
    assert.ok(!/bootout/.test(selfNote.replace(/bootout would delete[^.]*\./, '')), 'self copy must not promise a bootout');
    assert.equal(ACTION_COPY.reload.title({ self: true }), 'Restart the dashboard?');
    assert.equal(ACTION_COPY.reload.title({}), 'Reload job?');
    assert.match(ACTION_COPY.reload.note({}), /bootout \+ bootstrap/);
  });

  it('tells start on a retired job that the disable flag is cleared', () => {
    assert.match(ACTION_COPY.start.note({ disabled: true }), /retired/);
    assert.ok(!/retired/.test(ACTION_COPY.start.note({ disabled: false })));
  });

  it('only the destructive actions use the danger button', () => {
    assert.notEqual(ACTION_COPY.stop.danger, true);
    assert.notEqual(ACTION_COPY.start.danger, true);
    assert.notEqual(ACTION_COPY.enable.danger, true);
    assert.equal(ACTION_COPY.disable.danger, undefined);
  });
});
