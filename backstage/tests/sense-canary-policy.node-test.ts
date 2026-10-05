import test from 'node:test';
import assert from 'node:assert/strict';
import {
  safeValues,
  safeFiles,
  hashBundle,
  bundleSHA256,
} from '../packages/backend/src/modules/senseCanaryPolicy.ts';
import { readdirSync, readFileSync, lstatSync } from 'node:fs';
import { resolve } from 'node:path';
const request = {
  description: 'Private canary',
  theme: 'day',
  message: 'Hello',
};
const output = {
  directoryContents: [
    { path: 'canary-output/apps/canary/app/main.py', base64Content: 'YQ==' },
    {
      path: 'canary-output/kubernetes/apps/app-canary/values.yaml',
      base64Content: 'Yg==',
    },
  ],
};
test('caller cannot replace repository, source, secret or mode', () => {
  assert.equal(safeValues(request).name, 'canary');
  for (const key of [
    'template',
    'steps',
    'secrets',
    'directoryContents',
    'name',
    'repoUrl',
    'deploymentMode',
    'url',
    'owner',
  ]) {
    assert.throws(() => safeValues({ ...request, [key]: 'evil' }));
  }
  for (const value of [
    '',
    '${{ secrets.TOKEN }}',
    '{{x}}',
    'a\n',
    '"; evil()',
    'x\\y',
    '# yaml',
    'x'.repeat(61),
    5,
  ]) {
    assert.throws(() => safeValues({ ...request, message: value }));
  }
  assert.throws(() => safeValues({ ...request, theme: 'public' }));
});
test('publication files, encoded/traversal paths, duplicates and incomplete output fail closed', () => {
  assert.equal(safeFiles(output).length, 2);
  for (const path of [
    'apps/other/app.py',
    'apps/canary/../../x',
    'apps/canary/.github/workflow.yml',
    'apps/canary/.env',
    'apps/canary/%2fetc',
    '/etc/passwd',
    'kubernetes/infra/secret.yaml',
  ]) {
    assert.throws(() =>
      safeFiles({
        directoryContents: [
          ...output.directoryContents,
          { path: `canary-output/${path}`, base64Content: 'YQ==' },
        ],
      }),
    );
  }
  assert.throws(() =>
    safeFiles({
      directoryContents: [
        ...output.directoryContents,
        output.directoryContents[0],
      ],
    }),
  );
  assert.throws(() =>
    safeFiles({ directoryContents: output.directoryContents.slice(0, 1) }),
  );
  assert.throws(() =>
    safeFiles({
      directoryContents: output.directoryContents.map(f => ({
        ...f,
        base64Content: '!!!',
      })),
    }),
  );
});
test('source pin covers template plus every skeleton and overlay byte', () => {
  const root = resolve('backstage/templates/fastapi-template');
  const files: { path: string; base64Content: string }[] = [];
  function walk(path: string) {
    const p = resolve(root, path);
    assert.equal(lstatSync(p).isSymbolicLink(), false);
    if (lstatSync(p).isDirectory())
      for (const child of readdirSync(p)) walk(`${path}/${child}`);
    else
      files.push({ path, base64Content: readFileSync(p).toString('base64') });
  }
  for (const p of ['template.yaml', 'skeleton', 'canary-overlay']) walk(p);
  assert.equal(hashBundle(files), bundleSHA256);
  files[0].base64Content = 'YQ==';
  assert.notEqual(hashBundle(files), bundleSHA256);
});
