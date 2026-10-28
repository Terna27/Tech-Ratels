import test from 'node:test';
import assert from 'node:assert/strict';
import {getCsrfToken, setCsrfToken, clearCsrfToken} from '../src/lib/api.ts';

test('CSRF cookie rotation in another tab supersedes cached login token', () => {
  const original = globalThis.document;
  try {
    setCsrfToken('old-login');
    globalThis.document = {cookie: 'ratels_csrf=new-login'};
    assert.equal(getCsrfToken(), 'new-login');
    globalThis.document.cookie = 'ratels_csrf=third-login';
    assert.equal(getCsrfToken(), 'third-login');
  } finally {
    clearCsrfToken();
    if (original === undefined) delete globalThis.document;
    else globalThis.document = original;
  }
});
