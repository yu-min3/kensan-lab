/*
 * `keycloak:redirect-uri:add` — register a hostname's callback against the
 * oauth2-proxy client, so a service that did not exist a minute ago can be
 * signed in to.
 *
 * The gate in front of a scaffolded service is inverted and needs no per-host
 * change: the AuthorizationPolicy protects `*.127-0-0-1.sslip.io` and names its
 * exceptions. Keycloak does not invert. Its redirect URIs are matched
 * literally — a wildcard in the host is accepted at registration and then
 * refused at authorization with "Invalid parameter: redirect_uri". Measured on
 * a live realm, not assumed.
 *
 * So the URI is registered as the hostname is created. Bare metal does the same
 * thing by hand, by adding the host to GATEWAY_PROTECTED_HOSTS and re-running
 * bootstrap/keycloak/setup.sh — a step that has been forgotten before and cost
 * an afternoon. Doing it here removes the chance to forget.
 *
 * The list is replaced rather than appended to, because that is the only shape
 * the admin API offers. Two scaffolds running at the same moment would race;
 * with one visitor and one cluster that cannot happen, and the read-modify-write
 * below is deliberately simple for that reason.
 */
import { Config } from '@backstage/config';
import { InputError } from '@backstage/errors';
import { createTemplateAction } from '@backstage/plugin-scaffolder-node';

export function createKeycloakRedirectUriAction(options: { config: Config }) {
  const { config } = options;

  return createTemplateAction({
    id: 'keycloak:redirect-uri:add',
    description:
      "Adds https://<hostname>/oauth2/callback to a Keycloak client's redirect URIs",
    schema: {
      input: {
        hostname: z => z.string().describe('The host the new service is served on'),
        clientId: z =>
          z.string().optional().describe('Client to update (default: oauth2-proxy)'),
      },
    },

    async handler(ctx) {
      const { hostname } = ctx.input;
      const clientId = ctx.input.clientId ?? 'oauth2-proxy';

      // Present only in the explore cluster. Absent on bare metal, which is
      // what keeps this step from doing anything there.
      const kc = config.getOptionalConfig('kensanLab.keycloakAdmin');
      if (!kc) {
        throw new InputError(
          'No kensanLab.keycloakAdmin configuration. This action only runs in the explore cluster.',
        );
      }
      const baseUrl = kc.getString('baseUrl');
      const realm = kc.getString('realm');
      const username = kc.getString('username');
      const password = kc.getString('password');

      const form = new URLSearchParams({
        grant_type: 'password',
        client_id: 'admin-cli',
        username,
        password,
      });
      const tokenRes = await fetch(
        `${baseUrl}/realms/master/protocol/openid-connect/token`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: form,
        },
      );
      if (!tokenRes.ok) {
        throw new Error(
          `Keycloak refused the admin login: ${tokenRes.status} ${await tokenRes.text()}`,
        );
      }
      const { access_token: token } = (await tokenRes.json()) as { access_token: string };
      const admin = async (method: string, url: string, body?: unknown) => {
        const res = await fetch(`${baseUrl}/admin/realms/${realm}${url}`, {
          method,
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
          body: body === undefined ? undefined : JSON.stringify(body),
        });
        if (!res.ok) {
          throw new Error(`Keycloak answered ${res.status} for ${method} ${url}: ${await res.text()}`);
        }
        return res.status === 204 ? undefined : await res.json();
      };

      const found = (await admin(
        'GET',
        `/clients?clientId=${encodeURIComponent(clientId)}`,
      )) as Array<{ id: string; redirectUris?: string[] }>;
      if (!found?.length) {
        throw new Error(`No client "${clientId}" in realm "${realm}".`);
      }
      const client = found[0];

      const uri = `https://${hostname}/oauth2/callback`;
      if (client.redirectUris?.includes(uri)) {
        ctx.logger.info(`${uri} is already registered`);
        return;
      }
      await admin('PUT', `/clients/${client.id}`, {
        ...client,
        redirectUris: [...(client.redirectUris ?? []), uri],
      });
      ctx.logger.info(`Registered ${uri} on ${clientId}`);
    },
  });
}
