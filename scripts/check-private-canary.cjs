#!/usr/bin/env node
// Offline template/action rehearsal. It never calls a publisher or the cluster.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const deps = process.env.SENSE_TEMPLATE_DEPS || path.join(root, 'backstage');
const load = name => require(require.resolve(name, { paths: [deps] }));
const nunjucks = load('nunjucks');
const YAML = load('yaml');
const Ajv = load('ajv');
const source = path.join(root, 'backstage/templates/fastapi-template');
const template = YAML.parse(fs.readFileSync(path.join(source, 'template.yaml'), 'utf8'));
const output = path.resolve(process.argv[2] || '');
assert(process.argv[2] && !fs.existsSync(output), 'provide a new output directory');
fs.mkdirSync(output, { recursive: true });
const env = new nunjucks.Environment(null, { autoescape: false, tags: { variableStart: '${{', variableEnd: '}}' } });
env.addFilter('parseRepoUrl', value => Object.fromEntries(new URLSearchParams(value.split('?')[1])));
const render = (value, context) => {
  if (typeof value === 'string') return env.renderString(value, context);
  if (Array.isArray(value)) return value.map(item => render(item, context));
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, render(item, context)]));
  return value;
};
function fetchTree(from, to, context) {
  fs.mkdirSync(to, { recursive: true });
  for (const entry of fs.readdirSync(from, { withFileTypes: true })) {
    const input = path.join(from, entry.name), destination = path.join(to, entry.name);
    if (entry.isDirectory()) fetchTree(input, destination, context);
    else fs.writeFileSync(destination, render(fs.readFileSync(input, 'utf8'), context));
  }
}
const parameters = {
  name: 'canary', description: 'Private sense acceptance canary',
  owner: 'group:default/platform-engineering', repoUrl: 'github.com?repo=kensan-lab&owner=yu-min3',
  theme: 'day', message: 'Private canary', domain: 'platform', system: 'developer-portal',
  deploymentMode: 'private-canary',
};
const validate = new Ajv({ strict: false }).compile(template.spec.parameters[0]);
assert(validate(parameters), JSON.stringify(validate.errors));
assert(!validate({ ...parameters, name: 'konro' }), 'canary name must be bounded');
assert(!validate({ ...parameters, repoUrl: 'github.com?repo=other&owner=yu-min3' }), 'existing repository must be bounded');
for (const mode of ['private-canary', 'public', 'public-default']) {
  const directory = path.join(output, mode);
  fs.mkdirSync(directory);
  const context = { parameters: { ...parameters, deploymentMode: mode } };
  if (mode === 'public-default') delete context.parameters.deploymentMode;
  const remote = [];
  for (const step of template.spec.steps) {
    if (step.if && render(step.if, context) !== 'true') continue;
    const input = render(step.input, context);
    const safe = relative => {
      const result = path.resolve(directory, relative);
      assert(result.startsWith(directory + path.sep));
      return result;
    };
    if (step.action === 'fetch:template') {
      fetchTree(path.resolve(source, input.url), input.targetPath ? safe(input.targetPath) : directory, { values: input.values });
    } else if (step.action === 'fs:rename') {
      for (const file of input.files) {
        fs.mkdirSync(path.dirname(safe(file.to)), { recursive: true });
        fs.renameSync(safe(file.from), safe(file.to));
      }
    } else if (step.action === 'fs:delete') {
      for (const file of input.files) fs.rmSync(safe(file), { recursive: true });
    } else remote.push(step.action);
  }
  if (mode === 'private-canary') {
    assert.deepEqual(remote, []);
    const base = path.join(directory, 'canary-output');
    const values = YAML.parse(fs.readFileSync(path.join(base, 'kubernetes/apps/app-canary/values.yaml'), 'utf8'));
    assert.equal(values.image.repository, 'ghcr.io/yu-min3/kensan-lab/canary');
    assert.equal(values.httproute.enabled, false);
    assert.deepEqual(values.httproute.hostnames, []);
    assert.equal(values.auth.gatewayOAuth2.enabled, false);
    assert.equal(values.pvc.create, false);
    assert.equal(values.pvc.name, 'canary-data');
    const app = YAML.parse(fs.readFileSync(path.join(base, 'kubernetes/argocd/applications/apps/app-canary/app.yaml'), 'utf8'));
    assert.equal(app.spec.destination.namespace, 'app-canary');
    assert.equal(app.spec.sources[0].path, 'charts/app-base');
    assert.deepEqual(app.spec.sources[0].helm.valueFiles, ['$values/kubernetes/apps/app-canary/values.yaml']);
    assert.equal(app.spec.sources[2].path, 'kubernetes/apps/app-canary/resources');
    assert(app.spec.syncPolicy.automated.selfHeal);
    assert(app.spec.syncPolicy.automated.prune);
    const namespace = YAML.parse(fs.readFileSync(path.join(base, 'kubernetes/apps/app-canary/resources/namespace.yaml'), 'utf8'));
    assert.equal(namespace.metadata.name, 'app-canary');
    assert.equal(namespace.metadata.annotations['argocd.argoproj.io/sync-options'], 'Prune=false');
    const pvc = YAML.parse(fs.readFileSync(path.join(base, 'kubernetes/apps/app-canary/resources/pvc-data.yaml'), 'utf8'));
    assert.equal(pvc.metadata.annotations['argocd.argoproj.io/sync-options'], 'Prune=false');
    assert(!fs.existsSync(path.join(base, 'apps/canary/.github')));
    assert(!fs.existsSync(path.join(base, 'apps/canary/.gitea')));
    assert(!fs.existsSync(path.join(base, 'apps/canary/.devcontainer')));
    assert(!fs.existsSync(path.join(base, 'apps/canary/.backstage')));
    assert(fs.readFileSync(path.join(base, 'apps/canary/app/main.py'), 'utf8').includes('CANARY_RELEASE = "v1"'));
  } else {
    assert.deepEqual(remote, ['publish:github', 'publish:github:pull-request', 'catalog:register']);
    const values = YAML.parse(fs.readFileSync(path.join(directory, 'deploy/values.yaml'), 'utf8'));
    assert.equal(values.httproute.enabled, true);
    assert.equal(values.auth.gatewayOAuth2.enabled, true);
    assert(!values.pvc);
    assert(!fs.readFileSync(path.join(directory, 'app/main.py'), 'utf8').includes('CANARY_RELEASE'));
  }
  const text = render(template.spec.output.text[0].content, { ...context, steps: {} });
  if (mode === 'private-canary') {
    assert(text.includes('preview only'));
    assert(!text.includes('git push'));
  }
  console.log(`${mode}: offline render checks passed; remote actions executed=0`);
}
