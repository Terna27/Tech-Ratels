import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdtempSync, mkdirSync, writeFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, dirname} from 'node:path';

// Resolve the CSS parser used by Next itself, including any nested installation.
const require = createRequire(import.meta.url);
const postcss = require(require.resolve('postcss', {
  paths: [dirname(require.resolve('next/package.json'))],
}));

test('CSS sourceMappingURL cannot load a file outside the CSS directory', () => {
  const root = mkdtempSync(join(tmpdir(), 'ratels-css-'));
  try {
    const cssDir = join(root, 'css');
    mkdirSync(cssDir);
    const secretMap = JSON.stringify({version: 3, sources: ['private.txt'],
      sourcesContent: ['PRIVATE_SENTINEL'], names: [], mappings: ''});
    writeFileSync(join(root, 'private.map'), secretMap);
    for (const [annotation, options] of [
      ['../private.map', {from: join(cssDir, 'input.css')}],
      [join(root, 'private.map'), {}],
    ]) {
      const parsed = postcss.parse(`a { color: red } /*# sourceMappingURL=${annotation} */`, options);
      assert.equal(parsed.source.input.map?.text, undefined,
        'untrusted source map loaded private file contents');
    }
    writeFileSync(join(cssDir, 'allowed.map'), secretMap);
    const allowed = postcss.parse('a {} /*# sourceMappingURL=allowed.map */',
      {from: join(cssDir, 'input.css')});
    assert.equal(allowed.source.input.map.text, secretMap,
      'legitimate adjacent source maps must still load');
  } finally {
    rmSync(root, {recursive: true, force: true});
  }
});
