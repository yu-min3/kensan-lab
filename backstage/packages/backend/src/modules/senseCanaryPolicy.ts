import { createHash } from 'node:crypto';

export const bundleSHA256 =
  'd90fbe17df37c7d1aecf28386c440b73bf0f83d5077cd92e9394e74d33db136e';
export type File = {
  path: string;
  base64Content: string;
  executable?: boolean;
};
export function hashBundle(files: File[]): string {
  const hash = createHash('sha256');
  for (const f of [...files].sort((a, b) =>
    a.path < b.path ? -1 : a.path > b.path ? 1 : 0,
  )) {
    hash.update(
      `${f.path}\0${createHash('sha256')
        .update(Buffer.from(f.base64Content, 'base64'))
        .digest('hex')}\n`,
    );
  }
  return hash.digest('hex');
}
export function safeValues(input: unknown) {
  if (!input || typeof input !== 'object' || Array.isArray(input))
    throw new Error('invalid generation input');
  const v = input as Record<string, unknown>;
  if (
    Object.keys(v).some(
      k => !['description', 'theme', 'message'].includes(k),
    ) ||
    !['day', 'night'].includes(v.theme as string)
  )
    throw new Error('invalid generation input');
  for (const [key, max] of [
    ['description', 200],
    ['message', 60],
  ] as const) {
    const s = v[key];
    if (
      typeof s !== 'string' ||
      s.length < 1 ||
      s.length > max ||
      !/^[\p{L}\p{N} _.,!?()。、「」-]+$/u.test(s) ||
      /[\x00-\x1f\x7f]|\$\{|\{\{|\{%/.test(s)
    )
      throw new Error('invalid generation input');
  }
  return {
    ...v,
    deploymentMode: 'private-canary',
    name: 'canary',
    repoUrl: 'github.com?repo=kensan-lab&owner=yu-min3',
    owner: 'group:default/platform-engineering',
    domain: 'platform',
    system: 'developer-portal',
  };
}
export function safeFiles(result: unknown): File[] {
  const files = (result as { directoryContents?: unknown })?.directoryContents;
  if (!Array.isArray(files) || !files.length || files.length > 100)
    throw new Error('invalid generation output');
  const seen = new Set<string>();
  let total = 0;
  const out: File[] = [];
  for (const f of files) {
    if (
      !f ||
      typeof f.path !== 'string' ||
      typeof f.base64Content !== 'string' ||
      !f.path.startsWith('canary-output/') ||
      !/^[A-Za-z0-9_./-]+$/.test(f.path)
    )
      throw new Error('invalid generation output');
    const path = f.path.slice('canary-output/'.length);
    const parts = path.split('/');
    if (
      parts.some(
        (p: string) =>
          !p ||
          p === '.' ||
          p === '..' ||
          p === '.git' ||
          p === '.github' ||
          p === '.gitea' ||
          p === '.backstage' ||
          p.startsWith('.env'),
      ) ||
      !(
        path.startsWith('apps/canary/') ||
        path.startsWith('kubernetes/apps/app-canary/') ||
        path.startsWith('kubernetes/argocd/applications/apps/app-canary/')
      ) ||
      seen.has(path) ||
      !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(
        f.base64Content,
      ) ||
      (f.executable !== undefined && typeof f.executable !== 'boolean')
    )
      throw new Error('invalid generation output');
    seen.add(path);
    total += Buffer.from(f.base64Content, 'base64').length;
    if (total > 4 << 20) throw new Error('generation output exceeds bound');
    out.push({
      path,
      base64Content: f.base64Content,
      executable: f.executable === true,
    });
  }
  if (
    !seen.has('apps/canary/app/main.py') ||
    !seen.has('kubernetes/apps/app-canary/values.yaml')
  )
    throw new Error('incomplete generation output');
  return out.sort((a, b) => a.path.localeCompare(b.path));
}
