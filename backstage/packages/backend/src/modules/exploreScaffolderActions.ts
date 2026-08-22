/*
 * The three actions the explore cluster's golden path needs from a backend that
 * ships none of them in a usable form.
 *
 * publish:gitea does exist upstream, but that package bundles a newer copy of
 * backend-plugin-api and registers as 'module-v1.1', which this app cannot
 * read — the backend then serves 404 on every route with initialization
 * aborted. Reimplemented here against the copies this backend already uses.
 *
 * Both are registered unconditionally and both refuse to run without the
 * configuration only the explore cluster has — an integrations.gitea entry, and
 * a kensanLab.keycloakAdmin block. On bare metal they are present, reachable
 * and inert, which is what lets one template describe both environments.
 */
import {
  coreServices,
  createBackendModule,
} from '@backstage/backend-plugin-api';
import { scaffolderActionsExtensionPoint } from '@backstage/plugin-scaffolder-node';
import { createGiteaPublishAction } from './giteaPublish';
import { createGiteaPullRequestAction } from './giteaPullRequest';
import { createKeycloakRedirectUriAction } from './keycloakRedirectUri';

export default createBackendModule({
  pluginId: 'scaffolder',
  moduleId: 'explore-actions',
  register(env) {
    env.registerInit({
      deps: {
        scaffolder: scaffolderActionsExtensionPoint,
        config: coreServices.rootConfig,
      },
      async init({ scaffolder, config }) {
        scaffolder.addActions(
          createGiteaPublishAction({ config }),
          createGiteaPullRequestAction({ config }),
          createKeycloakRedirectUriAction({ config }),
        );
      },
    });
  },
});
