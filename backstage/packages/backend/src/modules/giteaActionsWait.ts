/*
 * `gitea:actions:wait` — make a scaffolder task depend on the first build of
 * the repository it just published.
 *
 * Gitea Actions is asynchronous, as CI should be. The Golden Path is not:
 * declaring the template complete before an artifact exists leaves a platform
 * administrator with a pull request that cannot safely be merged. This action
 * keeps the work on the runner while making that dependency explicit in the
 * portal task.
 */
import { Config } from '@backstage/config';
import { InputError } from '@backstage/errors';
import { ScmIntegrations, getGiteaRequestOptions } from '@backstage/integration';
import { createTemplateAction } from '@backstage/plugin-scaffolder-node';

type WorkflowRun = {
  conclusion?: string;
  head_sha?: string;
  html_url?: string;
  status?: string;
};

type WorkflowRunsResponse = { workflow_runs?: WorkflowRun[] };

const pause = (milliseconds: number) =>
  new Promise(resolve => setTimeout(resolve, milliseconds));

export function createGiteaActionsWaitAction(options: { config: Config }) {
  const { config } = options;

  return createTemplateAction({
    id: 'gitea:actions:wait',
    description: 'Waits for the first Gitea Actions workflow in a repository to succeed',
    schema: {
      input: {
        repoUrl: z => z.string().describe('host?owner=<owner>&repo=<repo>'),
        timeoutSeconds: z =>
          z.number().int().positive().optional().describe('Maximum wait (default: 900 seconds)'),
      },
      output: {
        buildUrl: z => z.string().describe('URL of the successful workflow run'),
        imageTag: z => z.string().describe('Immutable source SHA written by the workflow'),
      },
    },

    async handler(ctx) {
      const [host, query] = ctx.input.repoUrl.split('?');
      const params = new URLSearchParams(query ?? '');
      const owner = params.get('owner');
      const repo = params.get('repo');
      if (!host || !owner || !repo) {
        throw new InputError(
          `repoUrl must look like "host?owner=<owner>&repo=<repo>", got "${ctx.input.repoUrl}"`,
        );
      }

      const integration = ScmIntegrations.fromConfig(config).gitea.byHost(host);
      if (!integration) {
        throw new InputError(`No integrations.gitea entry for host "${host}".`);
      }
      const authHeaders = getGiteaRequestOptions(integration.config).headers ?? {};
      if (!Object.keys(authHeaders).length) {
        throw new InputError(`The integrations.gitea entry for "${host}" carries no credentials.`);
      }

      const baseUrl = integration.config.baseUrl ?? `https://${host}`;
      const api = `${baseUrl}/api/v1`;
      const actionsUrl = `${baseUrl}/${owner}/${repo}/actions`;
      const deadline = Date.now() + (ctx.input.timeoutSeconds ?? 900) * 1000;
      let run: WorkflowRun | undefined;

      ctx.logger.info(`Waiting for ${owner}/${repo} to produce a deployable image`);
      while (Date.now() < deadline) {
        const response = await fetch(
          `${api}/repos/${owner}/${repo}/actions/runs?event=push&branch=main&limit=10`,
          { headers: authHeaders },
        );
        if (!response.ok) {
          throw new Error(
            `Gitea answered ${response.status} while reading Actions for ${owner}/${repo}: ` +
              (await response.text()),
          );
        }

        const runs = (await response.json()) as WorkflowRunsResponse;
        run = runs.workflow_runs?.at(-1);
        if (!run) {
          await pause(3000);
          continue;
        }
        if (run.status === 'completed') {
          if (run.conclusion !== 'success') {
            throw new Error(
              `The generated service build finished with ${run.conclusion ?? 'an unknown result'}. ` +
                `Inspect ${run.html_url ?? actionsUrl}`,
            );
          }
          break;
        }
        ctx.logger.info(`Build is ${run.status ?? 'starting'}; waiting`);
        await pause(3000);
      }

      if (run?.status !== 'completed' || run.conclusion !== 'success' || !run.head_sha) {
        throw new Error(
          `Timed out waiting for the generated service build. Inspect ${run?.html_url ?? actionsUrl}`,
        );
      }

      let recorded = false;
      while (Date.now() < deadline) {
        const values = await fetch(
          `${api}/repos/${owner}/${repo}/contents/deploy/values.yaml?ref=main`,
          { headers: authHeaders },
        );
        if (values.ok) {
          const item = (await values.json()) as { content?: string };
          const text = item.content
            ? Buffer.from(item.content.replace(/\s/g, ''), 'base64').toString('utf8')
            : '';
          recorded = new RegExp(`^  tag: ${run.head_sha}$`, 'm').test(text);
          if (recorded) break;
        }
        await pause(1000);
      }
      if (!recorded) {
        throw new Error(
          `Build succeeded, but deploy/values.yaml did not record ${run.head_sha}. ` +
            `Inspect ${run.html_url ?? actionsUrl}`,
        );
      }

      ctx.output('buildUrl', run.html_url ?? actionsUrl);
      ctx.output('imageTag', run.head_sha);
      ctx.logger.info(`Build succeeded with immutable image tag ${run.head_sha}`);
    },
  });
}
