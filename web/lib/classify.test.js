import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { classifyJob, CATEGORY_LABELS, CATEGORY_KEYS, STATUS_KEYS } from './classify.js';

describe('classifyJob', () => {
  it('prefers the category the backend decided (allowlist or path derivation)', () => {
    assert.equal(classifyJob({ label: 'com.example.worker.run', domain: 'user', category: 'ours' }), 'ours');
    assert.equal(classifyJob({ label: 'com.docker.vmnetd', domain: 'global', category: 'noise' }), 'noise');
    assert.equal(classifyJob({ label: 'com.unknown.thing', domain: 'global', category: 'other' }), 'other');
  });

  it('falls back to "noise" for com.apple.* labels regardless of domain', () => {
    assert.equal(classifyJob({ label: 'com.apple.spotlight', domain: 'user' }), 'noise');
    assert.equal(classifyJob({ label: 'com.apple.spotlight', domain: 'global' }), 'noise');
    assert.equal(classifyJob({ label: 'com.apple.cfprefsd', domain: '' }), 'noise');
  });

  it('falls back to "ours" for user-domain non-apple labels', () => {
    assert.equal(classifyJob({ label: 'com.example.myapp', domain: 'user' }), 'ours');
    assert.equal(classifyJob({ label: 'org.homebrew.mxcl.redis', domain: 'user' }), 'ours');
  });

  it('falls back to "other" for global-domain and plist-less jobs', () => {
    assert.equal(classifyJob({ label: 'com.docker.vmnetd', domain: 'global' }), 'other');
    assert.equal(classifyJob({ label: 'com.example.noplist', domain: '' }), 'other');
    assert.equal(classifyJob({ label: 'com.example.noplist', domain: undefined }), 'other');
  });

  it('survives a missing or null label', () => {
    assert.equal(classifyJob({ domain: 'user' }), 'other');
    assert.equal(classifyJob({ label: null, domain: 'user' }), 'other');
    assert.equal(classifyJob({ label: '', domain: 'user' }), 'other');
  });
});

describe('CATEGORY_LABELS', () => {
  it('exports the fork display strings', () => {
    assert.equal(CATEGORY_LABELS.ours, 'Ours');
    assert.equal(CATEGORY_LABELS.noise, 'Noise');
    assert.equal(CATEGORY_LABELS.other, 'Other');
  });
});

describe('CATEGORY_KEYS', () => {
  it('has 4 entries: all, ours, noise, other', () => {
    assert.deepEqual(CATEGORY_KEYS, ['all', 'ours', 'noise', 'other']);
  });
});

describe('STATUS_KEYS', () => {
  it('has 8 entries: the six upstream states plus disabled', () => {
    assert.deepEqual(STATUS_KEYS, [
      'all', 'running', 'scheduled', 'completed', 'stopped', 'error', 'offline', 'disabled',
    ]);
    assert.equal(STATUS_KEYS.length, 8);
  });

  it('includes the fork-only disabled status and the upstream three', () => {
    assert.ok(STATUS_KEYS.includes('disabled'));
    assert.ok(STATUS_KEYS.includes('scheduled'));
    assert.ok(STATUS_KEYS.includes('completed'));
    assert.ok(STATUS_KEYS.includes('offline'));
  });
});
