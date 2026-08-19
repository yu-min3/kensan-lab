/*
 * `gitea:pull-request` — open a pull request against a repository on a Gitea
 * server, from files the scaffolder produced.
 *
 * Why this exists: @backstage/plugin-scaffolder-backend-module-gitea ships a
 * single action, publish:gitea, which creates a repository. There is no Gitea
 * counterpart to publish:github:pull-request, and the golden path needs one —
 * the manifests that put a new service on the platform belong in the platform
 * repository, and they arrive as a change somebody merges rather than a commit
 * that appears. Dropping the review step would remove the part of GitOps this
 * whole path exists to show.
 *
 * Only the explore cluster reaches this. On bare metal the template branches to
 * the GitHub actions instead, and no integrations.gitea entry exists at all.
 */
import { resolveSafeChildPath } from '@backstage/backend-plugin-api';
import { Config } from '@backstage/config';
import { InputError } from '@backstage/errors';
import { ScmIntegrations, getGiteaRequestOptions } from '@backstage/integration';
import { createTemplateAction } from '@backstage/plugin-scaffolder-node';
import fs from 'fs/promises';
import path from 'path';

type FileToCommit = { path: string; content: string };

/** Every file under `dir`, relative to it, with contents. Empty dirs are skipped. */
async function collectFiles(dir: string, base = dir): Promise<FileToCommit[]> {
  const out: FileToCommit[] = [];
  for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...(await collectFiles(full, base)));
    } else if (entry.isFile()) {
      out.push({
        path: path.relative(base, full).split(path.sep).join('/'),
        content: (await fs.readFile(full)).toString('base64'),
      });
    }
  }
  return out;
}

export function createGiteaPullRequestAction(options: { config: Config }) {
  const { config } = options;

  return createTemplateAction({
    id: 'gitea:pull-request',
    description:
      'Creates a branch on a Gitea repository, commits a directory onto it, and opens a pull request',
    schema: {
      input: {
        repoUrl: z =>
          z
            .string()
            .describe('host?owner=<owner>&repo=<repo> — the repository to open the request against'),
        branchName: z => z.string().describe('Branch to create'),
        title: z => z.string().describe('Pull request title'),
        description: z => z.string().optional().describe('Pull request body'),
        sourcePath: z =>
          z.string().describe('Directory in the workspace whose contents are committed'),
        targetBranch: z =>
          z
            .string()
            .optional()
            .describe("Branch to merge into (default: the repository's default branch)"),
      },
      output: {
        remoteUrl: z => z.string().describe('URL of the pull request'),
        pullRequestNumber: z => z.number().describe('Number of the pull request'),
      },
    },

    async handler(ctx) {
      const { repoUrl, branchName, title, description, sourcePath, targetBranch } = ctx.input;

      // host?owner=x&repo=y — the same shape RepoUrlPicker hands the GitHub
      // actions, so the template can pass its parameter through unchanged.
      const [host, query] = repoUrl.split('?');
      const params = new URLSearchParams(query ?? '');
      const owner = params.get('owner');
      const repo = params.get('repo');
      if (!host || !owner || !repo) {
        throw new InputError(
          `repoUrl must look like "host?owner=<owner>&repo=<repo>", got "${repoUrl}"`,
        );
      }

      const integration = ScmIntegrations.fromConfig(config).gitea.byHost(host);
      if (!integration) {
        throw new InputError(
          `No integrations.gitea entry for host "${host}". ` +
            'This action only runs where a Gitea server is configured.',
        );
      }

      // The same helper publish:gitea uses, rather than assembling a Basic
      // header here — it is where the credential shape is allowed to change.
      const authHeaders = getGiteaRequestOptions(integration.config).headers ?? {};
      if (!Object.keys(authHeaders).length) {
        throw new InputError(
          `The integrations.gitea entry for "${host}" carries no credentials, so nothing can be written.`,
        );
      }

      const api = `${integration.config.baseUrl ?? `https://${host}`}/api/v1`;
      const call = async (method: string, url: string, body?: unknown) => {
        const res = await fetch(`${api}${url}`, {
          method,
          headers: { ...authHeaders, 'Content-Type': 'application/json' },
          body: body === undefined ? undefined : JSON.stringify(body),
        });
        if (!res.ok) {
          throw new Error(`Gitea answered ${res.status} for ${method} ${url}: ${await res.text()}`);
        }
        return res.status === 204 ? undefined : await res.json();
      };

      const repoInfo = (await call('GET', `/repos/${owner}/${repo}`)) as {
        default_branch?: string;
      };
      const base = targetBranch ?? repoInfo.default_branch ?? 'main';

      ctx.logger.info(`Creating branch ${branchName} on ${owner}/${repo} from ${base}`);
      await call('POST', `/repos/${owner}/${repo}/branches`, {
        new_branch_name: branchName,
        old_branch_name: base,
      });

      const dir = resolveSafeChildPath(ctx.workspacePath, sourcePath);
      const files = await collectFiles(dir);
      if (files.length === 0) {
        throw new InputError(`No files under "${sourcePath}" to commit.`);
      }

      // One request per file. Gitea has a batch endpoint, but the per-file form
      // reports which path failed, and a scaffolded change is a handful of
      // files rather than a tree.
      for (const file of files) {
        ctx.logger.info(`Adding ${file.path}`);
        await call('POST', `/repos/${owner}/${repo}/contents/${file.path}`, {
          content: file.content,
          branch: branchName,
          message: `${title} — ${file.path}`,
        });
      }

      const pr = (await call('POST', `/repos/${owner}/${repo}/pulls`, {
        head: branchName,
        base,
        title,
        body: description ?? '',
      })) as { number: number; html_url: string };

      ctx.output('pullRequestNumber', pr.number);
      ctx.output('remoteUrl', pr.html_url);
      ctx.logger.info(`Opened ${pr.html_url}`);
    },
  });
}
