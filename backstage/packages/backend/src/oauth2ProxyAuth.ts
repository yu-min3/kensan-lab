import { createBackendModule } from '@backstage/backend-plugin-api';
import {
  OAuth2ProxyResult,
  oauth2ProxyAuthenticator,
} from '@backstage/plugin-auth-backend-module-oauth2-proxy-provider';
import {
  authProvidersExtensionPoint,
  commonSignInResolvers,
  createProxyAuthProviderFactory,
} from '@backstage/plugin-auth-node';

/**
 * Adapts oauth2-proxy's auth_request response headers to Backstage's profile.
 *
 * oauth2-proxy is used as an Istio ext_authz endpoint in kensan-lab. In this
 * mode it returns X-Auth-Request-* response headers, while Backstage's default
 * oauth2Proxy profile transform expects X-Forwarded-* request headers.
 */
export async function oauth2ProxyProfileTransform(result: OAuth2ProxyResult) {
  const email = result.getHeader('x-auth-request-email');
  if (!email) {
    throw new Error(
      'Missing X-Auth-Request-Email from the authentication proxy',
    );
  }

  return {
    profile: {
      email,
      displayName: result.getHeader('x-auth-request-user') ?? email,
    },
  };
}

export default createBackendModule({
  pluginId: 'auth',
  moduleId: 'oauth2-proxy-auth-request-provider',
  register(reg) {
    reg.registerInit({
      deps: { providers: authProvidersExtensionPoint },
      async init({ providers }) {
        providers.registerProvider({
          providerId: 'oauth2Proxy',
          factory: createProxyAuthProviderFactory({
            authenticator: oauth2ProxyAuthenticator,
            profileTransform: oauth2ProxyProfileTransform,
            signInResolverFactories: commonSignInResolvers,
          }),
        });
      },
    });
  },
});
