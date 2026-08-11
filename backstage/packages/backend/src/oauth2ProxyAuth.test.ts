import { OAuth2ProxyResult } from '@backstage/plugin-auth-backend-module-oauth2-proxy-provider';
import { oauth2ProxyProfileTransform } from './oauth2ProxyAuth';

function resultWithHeaders(headers: Record<string, string>): OAuth2ProxyResult {
  return {
    fullProfile: {},
    accessToken: '',
    headers,
    getHeader: name => headers[name.toLowerCase()],
  };
}

describe('oauth2ProxyProfileTransform', () => {
  it('maps Istio auth_request headers to a Backstage profile', async () => {
    await expect(
      oauth2ProxyProfileTransform(
        resultWithHeaders({
          'x-auth-request-email': 'ymisaki00@gmail.com',
          'x-auth-request-user': 'yu',
        }),
      ),
    ).resolves.toEqual({
      profile: {
        email: 'ymisaki00@gmail.com',
        displayName: 'yu',
      },
    });
  });

  it('fails closed when the trusted email header is missing', async () => {
    await expect(
      oauth2ProxyProfileTransform(resultWithHeaders({})),
    ).rejects.toThrow('Missing X-Auth-Request-Email');
  });
});
