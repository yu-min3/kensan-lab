# Plan: <title>

> Spec: [spec.md](./spec.md)

## Constitution check

- [ ] `CLAUDE.md` — <how this plan complies>
- [ ] `.claude/rules/gitops-workflow.md` — <how>
- [ ] `.claude/rules/collaboration.md` — <how>
- [ ] <other applicable rule> — <how>

## Layout

- Category: <apps | argocd | auth | backstage | kube-system | namespaces | network | observability | policy | secrets | storage>
- Pattern: <A / B / ApplicationSet>
- Component path: `<path>`
- Application path: `<path / N/A>`

## Resources

| Change | kind / name | Namespace | Prune behavior |
|---|---|---|---|
| Add / change / delete | <resource> | <namespace> | <default / Prune=false + reason> |

## Chart and version

- Repository / chart: <value / N/A>
- `targetRevision`: <pinned version / N/A>
- Existing layout exception: <observability config.json / vault custom chart / none>

## Policy and namespace

- Namespace labels / PSS: <value / N/A>
- Kyverno / PolicyException impact: <value / none>
- Argo CD project / sync options / wave: <value>

## Storage

- PVC / storageClass / capacity: <value / none>
- Backup / restore: <value / N/A>
- `Prune=false`: <required + location / not required>

## Network and authentication

- Gateway / host / HTTPRoute / AuthPolicy: <value / none>
- Keycloak redirect URI: <required / not required>
- NetworkPolicy: <change / none>

## Secrets

- Method / path / Reloader: <value / none>

## Affected paths

- `<path>`

## Merge behavior and rollback

- Merge behavior: <what auto-sync changes>
- Risk: <risk>
- Rollback: <revertability and state handling>
