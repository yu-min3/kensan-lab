/*
 * `publish:gitea` — create a repository on a Gitea server and push the
 * scaffolded workspace into it.
 *
 * Backstage ships this action in @backstage/plugin-scaffolder-backend-module-gitea,
 * and using that package is what this file exists to avoid. The module carries
 * its own copy of @backstage/backend-plugin-api — 1.10.0 against this app's
 * 1.4.4 — and a BackendFeature built by the newer copy registers as
 * 'module-v1.1', which this app's backend-app-api cannot read. The backend then
 * listens on its port with initialization aborted, so every probe gets a 404
 * and the pod restarts forever while looking, from the outside, like a health
 * check problem.
 *
 * Written here instead, against the same copies everything else in this backend
 * uses. It is a thin wrapper over the API call and initRepoAndPush, which is
 * what the upstream module does too.
 */
import { Config } from '@backstage/config';
import { InputError } from '@backstage/errors';
import { ScmIntegrations, getGiteaRequestOptions } from '@backstage/integration';
import { createTemplateAction, initRepoAndPush } from '@backstage/plugin-scaffolder-node';

export function createGiteaPublishAction(options: { config: Config }) {
  const { config } = options;

  return createTemplateAction({
    id: 'publish:gitea',
    description: 'Creates a repository on a Gitea server and pushes the workspace to it',
    schema: {
      input: {
        repoUrl: z => z.string().describe('host?owner=<owner>&repo=<repo>'),
        description: z => z.string().optional().describe('Repository description'),
        defaultBranch: z => z.string().optional().describe('Default branch (default: main)'),
        repoVisibility: z =>
          z.enum(['public', 'private']).optional().describe('Default: public'),
        gitCommitMessage: z => z.string().optional().describe('Initial commit message'),
        sourcePath: z =>
          z.string().optional().describe('Subdirectory of the workspace to push'),
      },
      output: {
        remoteUrl: z => z.string().describe('Browser URL of the repository'),
        repoContentsUrl: z => z.string().describe('URL the catalog reads catalog-info.yaml from'),
      },
    },

    async handler(ctx) {
      const { repoUrl, description, gitCommitMessage, sourcePath } = ctx.input;
      const defaultBranch = ctx.input.defaultBranch ?? 'main';
      const repoVisibility = ctx.input.repoVisibility ?? 'public';

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
      const { username, password, baseUrl } = integration.config;
      if (!username || !password) {
        throw new InputError(`The integrations.gitea entry for "${host}" has no credentials.`);
      }

      const api = `${baseUrl ?? `https://${host}`}/api/v1`;
      const authHeaders = getGiteaRequestOptions(integration.config).headers ?? {};

      ctx.logger.info(`Creating ${owner}/${repo} on ${host}`);
      const res = await fetch(`${api}/user/repos`, {
        method: 'POST',
        headers: { ...authHeaders, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: repo,
          description: description ?? '',
          private: repoVisibility === 'private',
          auto_init: false,
          default_branch: defaultBranch,
        }),
      });
      if (!res.ok) {
        throw new Error(`Gitea refused to create the repository: ${res.status} ${await res.text()}`);
      }
      const created = (await res.json()) as { clone_url: string; html_url: string };

      await initRepoAndPush({
        dir: sourcePath ? `${ctx.workspacePath}/${sourcePath}` : ctx.workspacePath,
        remoteUrl: created.clone_url,
        auth: { username, password },
        logger: ctx.logger,
        defaultBranch,
        commitMessage: gitCommitMessage ?? 'Initial commit from Backstage template',
      });

      ctx.output('remoteUrl', created.html_url);
      // The catalog fetches catalog-info.yaml relative to this.
      ctx.output('repoContentsUrl', `${created.html_url}/src/branch/${defaultBranch}`);
      ctx.logger.info(`Pushed to ${created.html_url}`);
    },
  });
}
