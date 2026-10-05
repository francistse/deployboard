import { describe, it, afterEach } from 'node:test';
import { strict as assert } from 'node:assert';
import {
  catalogs,
  t,
  setLocale,
  locale,
  getLocale,
  resolveInitialLocale,
} from './i18n.js';

const CODES = ['ja', 'zh-Hant', 'zh-Hans'];

function placeholders(str) {
  return [...String(str).matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');
}

function codeMarks(str) {
  return [...String(str).matchAll(/\[\[([^\]]+)\]\]/g)].map((m) => m[1]).sort().join('|');
}

describe('catalog parity', () => {
  const enKeys = Object.keys(catalogs.en).sort();

  for (const code of CODES) {
    it(`${code} has exactly the same keys as en`, () => {
      assert.deepEqual(Object.keys(catalogs[code]).sort(), enKeys);
    });

    it(`${code} placeholders and [[code]] marks match en`, () => {
      for (const key of enKeys) {
        assert.equal(placeholders(catalogs[code][key]), placeholders(catalogs.en[key]), `${code} ${key}`);
        assert.equal(codeMarks(catalogs[code][key]), codeMarks(catalogs.en[key]), `${code} ${key}`);
        assert.equal(typeof catalogs[code][key], 'string', `${code} ${key}`);
        assert.ok(catalogs[code][key] !== '', `${code} ${key} is blank`);
      }
    });
  }
});

describe('t()', () => {
  afterEach(() => setLocale('en', false));

  it('returns the English string by default', () => {
    assert.equal(locale.value, 'en');
    assert.equal(t('status.running'), 'Running');
    assert.equal(t('table.empty'), 'No jobs found.');
  });

  it('uses the active locale', () => {
    setLocale('ja', false);
    assert.equal(getLocale(), 'ja');
    assert.equal(t('status.running'), catalogs.ja['status.running']);
    assert.notEqual(t('status.running'), 'Running');
  });

  it('falls back to en when the key is missing from a non-English catalog', () => {
    const key = 'status.running';
    const saved = catalogs.ja[key];
    delete catalogs.ja[key];
    setLocale('ja', false);
    try {
      assert.equal(t(key), catalogs.en[key]);
      assert.equal(t(key), 'Running');
    } finally {
      catalogs.ja[key] = saved;
    }
  });

  it('returns the key itself when it is missing everywhere, without throwing', () => {
    setLocale('zh-Hant', false);
    assert.equal(t('no.such.key'), 'no.such.key');
    assert.doesNotThrow(() => t(null));
    assert.doesNotThrow(() => t(undefined));
    assert.doesNotThrow(() => t('', { n: 1 }));
    assert.ok(t(null), 'null key must not produce a blank string');
    assert.ok(t(undefined), 'undefined key must not produce a blank string');
    assert.ok(t(''), 'empty key must not produce a blank string');
  });

  it('interpolates {name} and ignores unknown vars', () => {
    assert.equal(t('ago.min', { n: 30 }), '30 min ago');
    assert.equal(t('storm.toast', { action: 'stop', group: 'App', ok: 2, failed: 1 }), 'stop on App: 2 done, 1 failed');
    assert.equal(t('ago.min', {}), ' min ago');
  });

  it('ignores an unknown locale without throwing', () => {
    setLocale('ja', false);
    assert.equal(setLocale('fr', false), 'ja');
    assert.equal(locale.value, 'ja');
    assert.doesNotThrow(() => setLocale(null, false));
  });

  it('covers every known status and the read-only lock tooltip key', () => {
    for (const status of ['running', 'scheduled', 'completed', 'stopped', 'error', 'offline', 'disabled', 'all']) {
      assert.ok(catalogs.en['status.' + status], `missing status.${status}`);
    }
    assert.ok(catalogs.en['row.readOnlyLocked']);
    assert.notEqual(t('row.readOnlyLocked'), 'row.readOnlyLocked');
    setLocale('zh-Hans', false);
    assert.notEqual(t('row.readOnlyLocked'), 'row.readOnlyLocked');
    assert.notEqual(t('row.readOnlyLocked'), catalogs.en['row.readOnlyLocked']);
  });
});

describe('resolveInitialLocale()', () => {
  const cases = [
    ['ja', 'en-US', 'ja'],
    ['zh-Hant', 'ja', 'zh-Hant'],
    ['zh-Hans', 'en', 'zh-Hans'],
    ['en', 'ja-JP', 'en'],
    ['fr', 'ja', 'ja'],
    ['', 'zh-TW', 'zh-Hant'],
    [null, 'ja', 'ja'],
    [null, 'ja-JP', 'ja'],
    [null, 'JA', 'ja'],
    [null, 'zh-TW', 'zh-Hant'],
    [null, 'zh-HK', 'zh-Hant'],
    [null, 'zh-Hant', 'zh-Hant'],
    [null, 'zh-MO', 'zh-Hant'],
    [null, 'zh-Hant-HK', 'zh-Hant'],
    [null, 'zh_TW', 'zh-Hant'],
    [null, 'zh-CN', 'zh-Hans'],
    [null, 'zh-SG', 'zh-Hans'],
    [null, 'zh-Hans', 'zh-Hans'],
    [null, 'zh', 'zh-Hans'],
    [null, 'zh-yue', 'zh-Hans'],
    [null, 'en-US', 'en'],
    [null, 'en', 'en'],
    [null, '', 'en'],
    [null, 'fr-FR', 'en'],
  ];

  for (const [stored, language, want] of cases) {
    it(`stored=${stored === null ? 'null' : JSON.stringify(stored)} language=${JSON.stringify(language)} → ${want}`, () => {
      assert.equal(resolveInitialLocale({ stored, language }), want);
    });
  }
});
