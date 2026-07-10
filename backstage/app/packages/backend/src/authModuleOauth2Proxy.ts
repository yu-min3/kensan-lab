/*
 * oauth2Proxy auth provider with a custom sign-in resolver.
 *
 * Why not the stock @backstage/plugin-auth-backend-module-oauth2-proxy-provider
 * registration + built-in resolvers:
 *
 *  - The built-in resolvers read `x-forwarded-user` / `profile.email`, but this
 *    platform fronts Backstage with Istio ext_authz + oauth2-proxy, which forwards
 *    `x-auth-request-*` headers instead (kubernetes/network/istio/istiod/values.yaml,
 *    headersToUpstreamOnAllow).
 *  - The Keycloak JWT in the Authorization header is deliberately stripped at the
 *    workload before it reaches this backend
 *    (kubernetes/backstage/requestauthentication-strip-jwt.yaml), so the
 *    authenticator's `fullProfile` (decoded from that JWT) is always empty here.
 *
 * So we reuse the stock authenticator (which tolerates the missing JWT) and resolve
 * sign-in from the `x-auth-request-email` header, matching it against the catalog
 * User entity's spec.profile.email.
 */
import { createBackendModule } from '@backstage/backend-plugin-api';
import {
  authProvidersExtensionPoint,
  createProxyAuthProviderFactory,
} from '@backstage/plugin-auth-node';
import { oauth2ProxyAuthenticator } from '@backstage/plugin-auth-backend-module-oauth2-proxy-provider';

export default createBackendModule({
  pluginId: 'auth',
  moduleId: 'oauth2-proxy-x-auth-request',
  register(reg) {
    reg.registerInit({
      deps: { providers: authProvidersExtensionPoint },
      async init({ providers }) {
        providers.registerProvider({
          providerId: 'oauth2Proxy',
          factory: createProxyAuthProviderFactory({
            authenticator: oauth2ProxyAuthenticator,
            async signInResolver(info, ctx) {
              const email = info.result.getHeader('x-auth-request-email');
              if (!email) {
                throw new Error(
                  'Missing x-auth-request-email header — Backstage must be reached ' +
                    'through the platform gateway (oauth2-proxy ext_authz)',
                );
              }
              return ctx.signInWithCatalogUser({
                filter: {
                  kind: 'User',
                  'spec.profile.email': email,
                },
              });
            },
          }),
        });
      },
    });
  },
});
