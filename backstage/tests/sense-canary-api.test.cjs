const test = require('node:test');
const assert = require('node:assert/strict');
const { resolve } = require('node:path');
const { installSenseCanary } = require(resolve(
  process.env.SENSE_CANARY_TEST_BUILD || 'temp/t081-build',
  'senseCanary.js',
));
const { bundleSHA256 } = require(resolve(
  process.env.SENSE_CANARY_TEST_BUILD || 'temp/t081-build',
  'senseCanaryPolicy.js',
));
const backendRequire = require('node:module').createRequire(
  resolve('backstage/packages/backend/package.json'),
);
const express = backendRequire('express');
const { ConfigReader } = backendRequire('@backstage/config');
const directoryContents = [
  { path: 'canary-output/apps/canary/app/main.py', base64Content: 'YQ==' },
  {
    path: 'canary-output/kubernetes/apps/app-canary/values.yaml',
    base64Content: 'Yg==',
  },
];
test('disabled plugin does not need token, template, or discovery', async () => {
  await installSenseCanary({ config: new ConfigReader({}) });
  await installSenseCanary({
    config: new ConfigReader({
      senseCanary: { enabled: true, subject: 'sense-canary-agent' },
    }),
  });
});
test('service-only fixed route performs internal authenticated dry-run and returns files, not logs/secrets', async () => {
  let calls = 0;
  let role = 'service';
  let subject = 'sense-canary-agent';
  const oldFetch = global.fetch;
  const cwd = process.cwd();
  const app = express();
  let server;
  try {
    global.fetch = async (url, options) => {
      calls++;
      assert.equal(url, 'http://127.0.0.1:7007/api/scaffolder/v2/dry-run');
      assert.equal(
        options.headers.Authorization,
        'Bearer internal-plugin-token',
      );
      assert.equal(options.redirect, 'error');
      const body = JSON.parse(options.body);
      assert.equal(body.values.name, 'canary');
      assert.equal(body.values.deploymentMode, 'private-canary');
      assert.equal(body.secrets, undefined);
      assert.deepEqual(
        body.template.spec.steps.map(s => s.id),
        [
          'fetch-canary',
          'arrange-canary',
          'remove-canary-publication',
          'canary-storage',
        ],
      );
      assert.deepEqual(body.template.spec.output, {});
      return new Response(
        JSON.stringify({
          directoryContents,
          log: ['SECRET'],
          steps: ['SECRET'],
        }),
      );
    };
    process.chdir(resolve('backstage'));
    await installSenseCanary({
      config: new ConfigReader({
        backend: {
          listen: { port: 7007 },
          auth: {
            externalAccess: [
              {
                type: 'static',
                options: {
                  subject: 'sense-canary-agent',
                  token: 'fixture-token-never-real-credential-12345',
                },
                accessRestrictions: [{ plugin: 'sense-canary' }],
              },
            ],
          },
        },
        senseCanary: {
          enabled: true,
          subject: 'sense-canary-agent',
          templateSHA256: bundleSHA256,
        },
      }),
      router: { use: r => app.use('/api/sense-canary', r) },
      httpAuth: {
        credentials: async (_, policy) => {
          assert.deepEqual(policy.allow, ['service']);
          return { principal: { type: role, subject } };
        },
      },
      auth: {
        isPrincipal: (c, type) => c.principal.type === type,
        getOwnServiceCredentials: async () => 'OWN',
        getPluginRequestToken: async o => {
          assert.deepEqual(o, {
            onBehalfOf: 'OWN',
            targetPluginId: 'scaffolder',
          });
          return { token: 'internal-plugin-token' };
        },
      },
      discovery: {
        getBaseUrl: async id => {
          assert.equal(id, 'scaffolder');
          return 'http://internal/api/scaffolder';
        },
      },
    });
    server = app.listen(0, '127.0.0.1');
    await new Promise(r => server.once('listening', r));
    const request = body =>
      oldFetch(
        `http://127.0.0.1:${
          server.address().port
        }/api/sense-canary/v1/generate`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        },
      );
    const input = { description: 'Canary', theme: 'day', message: 'Hello' };
    let response = await request(input);
    assert.equal(response.status, 200);
    const result = await response.json();
    assert.equal(result.files.length, 2);
    assert.equal(result.log, undefined);
    assert.equal(result.steps, undefined);
    response = await request({ ...input, secrets: { token: 'evil' } });
    assert.equal(response.status, 400);
    assert.equal(calls, 1);
    role = 'user';
    response = await request(input);
    assert.equal(response.status, 403);
    assert.equal(calls, 1);
    role = 'service';
    subject = 'different-service';
    response = await request(input);
    assert.equal(response.status, 403);
    assert.equal(calls, 1);
    subject = 'sense-canary-agent';
    global.fetch = async () =>
      new Response(
        JSON.stringify({
          directoryContents: [{ path: '../../evil', base64Content: 'YQ==' }],
        }),
      );
    response = await request(input);
    assert.equal(response.status, 503);
  } finally {
    process.chdir(cwd);
    global.fetch = oldFetch;
    if (server) await new Promise(r => server.close(r));
  }
});
