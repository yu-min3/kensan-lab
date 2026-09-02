# Generated applications

Backstage writes here. Every app scaffolded inside the explore cluster opens a
pull request against this repository adding one directory:

```
app-<name>/
  app.yaml            Argo CD Application, project app-project
  referencegrant.yaml lets the app's /oauth2 route reach oauth2-proxy
```

`explore-root-app.yaml` reads this path as a second source with `recurse: true`,
so merging the pull request is the whole deployment step.

Nothing here is committed by hand, and nothing bare metal reads is touched: the
production template writes the same two files to
`kubernetes/argocd/applications/apps/` and
`kubernetes/auth/oauth2-proxy/resources/` instead.

This file exists so the directory survives in Git while it is empty — Argo CD
needs the path to resolve on a cluster where nothing has been scaffolded yet.
