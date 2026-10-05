import {
  coreServices,
  createBackendPlugin,
  HttpRouterService,
  HttpAuthService,
  AuthService,
} from '@backstage/backend-plugin-api';
import express from 'express';
import { Config } from '@backstage/config';
import { load } from 'js-yaml';
import { lstat, readdir, readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import {
  bundleSHA256,
  hashBundle,
  safeFiles,
  safeValues,
  File,
} from './senseCanaryPolicy';

// No external caller can select files, steps, URLs, secrets or publication mode.
export default createBackendPlugin({
  pluginId: 'sense-canary',
  register(env) {
    env.registerInit({
      deps: {
        config: coreServices.rootConfig,
        router: coreServices.httpRouter,
        httpAuth: coreServices.httpAuth,
        auth: coreServices.auth,
      },
      init: installSenseCanary,
    });
  },
});

export async function installSenseCanary({
  config,
  router,
  httpAuth,
  auth,
}: {
  config: Config;
  router: HttpRouterService;
  httpAuth: HttpAuthService;
  auth: AuthService;
}) {
  const settings = config.getOptionalConfig('senseCanary');
  if (!settings?.getOptionalBoolean('enabled')) return;
  // Missing machine credentials leave the new API inert without changing existing services.
  const subject = settings.getString('subject');
  const access =
    config.getOptionalConfigArray('backend.auth.externalAccess') ?? [];
  const configured = access.some(entry => {
    if (
      entry.getString('type') !== 'static' ||
      entry.getOptionalString('options.subject') !== subject
    )
      return false;
    const token = entry.getOptionalString('options.token');
    const restrictions = entry.getOptionalConfigArray('accessRestrictions');
    return (
      !!token &&
      token.length >= 32 &&
      !/\s/.test(token) &&
      restrictions?.length === 1 &&
      restrictions[0].getString('plugin') === 'sense-canary'
    );
  });
  if (!configured) return;
  if (config.getOptionalNumber('backend.listen.port') !== 7007)
    throw new Error('canary internal backend port must be 7007');
  if (settings.getString('templateSHA256') !== bundleSHA256)
    throw new Error('canary template pin mismatch');
  const root = resolve('templates/fastapi-template');
  const files: File[] = [];
  let total = 0;
  async function collect(path: string) {
    const full = resolve(root, path);
    const stat = await lstat(full);
    if (stat.isSymbolicLink())
      throw new Error('canary template symlink forbidden');
    if (stat.isDirectory()) {
      for (const entry of await readdir(full))
        await collect(path ? `${path}/${entry}` : entry);
    } else {
      if (!stat.isFile() || stat.size > 1 << 20 || files.length >= 100)
        throw new Error('canary template outside bound');
      const data = await readFile(full);
      total += data.length;
      if (total > 4 << 20) throw new Error('canary template outside bound');
      files.push({ path, base64Content: data.toString('base64') });
    }
  }
  if (
    !(await lstat(root)).isDirectory() ||
    (await lstat(root)).isSymbolicLink()
  )
    throw new Error('invalid canary template root');
  for (const path of ['template.yaml', 'skeleton', 'canary-overlay'])
    await collect(path);
  if (hashBundle(files) !== bundleSHA256)
    throw new Error('canary template content mismatch');
  const template = load(
    Buffer.from(
      files.find(f => f.path === 'template.yaml')!.base64Content,
      'base64',
    ).toString(),
  ) as any;
  const ids = [
    'fetch-canary',
    'arrange-canary',
    'remove-canary-publication',
    'canary-storage',
  ];
  template.spec.steps = ids.map(id =>
    template.spec.steps.find((s: any) => s.id === id),
  );
  if (
    template.spec.steps.some(
      (s: any) =>
        !s || !['fetch:template', 'fs:rename', 'fs:delete'].includes(s.action),
    )
  )
    throw new Error('invalid canary generation actions');
  template.spec.output = {};
  const api = express.Router();
  api.use(express.json({ limit: '4kb' }));
  api.post('/v1/generate', async (req, res) => {
    try {
      const credentials = await httpAuth.credentials(req, {
        allow: ['service'],
      });
      if (
        !auth.isPrincipal(credentials, 'service') ||
        credentials.principal.subject !== subject
      ) {
        res
          .status(403)
          .json({ error: 'generation service principal rejected' });
        return;
      }
      let values;
      try {
        values = safeValues(req.body);
      } catch {
        res.status(400).json({ error: 'invalid generation input' });
        return;
      }
      // Both plugins run in this backend. Never send the plugin token through a public route.
      const base = 'http://127.0.0.1:7007/api/scaffolder';
      const { token } = await auth.getPluginRequestToken({
        onBehalfOf: await auth.getOwnServiceCredentials(),
        targetPluginId: 'scaffolder',
      });
      const response = await fetch(`${base}/v2/dry-run`, {
        method: 'POST',
        redirect: 'error',
        signal: AbortSignal.timeout(30_000),
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          template,
          values,
          directoryContents: files.filter(f => f.path !== 'template.yaml'),
        }),
      });
      if (!response.ok || !response.body)
        throw new Error('generation unavailable');
      const chunks: Uint8Array[] = [];
      let size = 0;
      for await (const chunk of response.body as any) {
        size += chunk.length;
        if (size > 6 << 20)
          throw new Error('generation response exceeds bound');
        chunks.push(chunk);
      }
      const output = safeFiles(JSON.parse(Buffer.concat(chunks).toString()));
      res.json({
        templateSHA256: bundleSHA256,
        filesSHA256: hashBundle(output),
        files: output,
      });
    } catch {
      res.status(503).json({ error: 'fixed canary generation unavailable' });
    }
  });
  router.use(api);
}
