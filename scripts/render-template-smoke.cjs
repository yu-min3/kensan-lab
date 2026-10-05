#!/usr/bin/env node
// Match fetch:template content rendering, including conditions and raw blocks.
// Paths are copied as-is; only an explicit fs:rename action can rename them.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const nunjucks = require('./template-ci/node_modules/nunjucks');
const values = {
  "name": "widget-service",
  "description": "Sample FastAPI service",
  "ownerSlug": "platform-engineering",
  "repoOwner": "yu-min3",
  "repoName": "app-widget-service",
  "owner": "platform-engineering",
  "system": "developer-portal",
  "domain": "platform",
  "theme": "night",
  "message": "hello from CI",
  "hostname": "widget-service.app.yu-min3.com",
  "imageRepository": "ghcr.io/yu-min3/app-widget-service",
  "imageTag": "v0.1.0",
  "applicationUrl": "https://widget-service.app.yu-min3.com/docs",
  "argocdUrl": "https://argocd.platform.yu-min3.com/applications/app-widget-service",
  "loadButtonEnabled": "false",
  "ghcrPullSecretEnabled": "true",
  "otelEnabled": "true",
  "gatewayName": "gateway-prod",
  "gatewayNamespace": "istio-system",
  "platformRepoUrl": "https://github.com/yu-min3/kensan-lab",
  "applicationRepoUrl": "https://github.com/yu-min3/app-widget-service"
};
const source = path.resolve(process.argv[3] || path.join(__dirname, '../backstage/templates/fastapi-template/skeleton'));
const output = path.resolve(process.argv[2] || '');
assert(process.argv[2] && !fs.existsSync(output), 'provide a new output directory');
const env = new nunjucks.Environment(null, {
  autoescape: false, throwOnUndefined: true,
  tags: { variableStart: '${{', variableEnd: '}}' },
});
function renderTree(input, destination) {
  fs.mkdirSync(destination, { recursive: true });
  for (const entry of fs.readdirSync(input, { withFileTypes: true })) {
    assert(!/\$\{\{/.test(entry.name), 'fetch:template does not substitute paths: ' + entry.name);
    const from = path.join(input, entry.name), to = path.join(destination, entry.name);
    if (entry.isDirectory()) renderTree(from, to);
    else {
      assert(entry.isFile(), 'unsupported skeleton entry: ' + from);
      const bytes = fs.readFileSync(from);
      let text;
      try { text = new TextDecoder('utf-8', { fatal: true }).decode(bytes); }
      catch { fs.writeFileSync(to, bytes); continue; }
      const rendered = env.renderString(text, { values });
      assert(!/\{%|\$\{\{\s*(?:values\.|not\s+values\.)/.test(rendered), 'unrendered template content: ' + from);
      fs.writeFileSync(to, rendered);
    }
  }
}
renderTree(source, output);
console.log('public scaffold: Nunjucks content rendered; paths unchanged');
