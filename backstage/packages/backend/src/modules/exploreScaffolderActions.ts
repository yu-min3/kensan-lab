/*
 * The two actions the explore cluster's golden path needs and Backstage does
 * not ship.
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
          createGiteaPullRequestAction({ config }),
          createKeycloakRedirectUriAction({ config }),
        );
      },
    });
  },
});
