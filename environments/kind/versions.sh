# Versions that explore-up.sh needs *before* Argo CD exists, so they cannot be
# read from an Application. Each one is checked against its bare-metal
# counterpart by scripts/validate_argocd_apps.py, which fails CI on drift —
# see environments/kind/README.md#anti-drift.
#
# Sourced by bash, so `.env` would be the natural extension — but .gitignore
# excludes `*.env` to keep credentials out of the repository, and this file has
# to be committed for a fresh clone to work. Do not rename it back.

# mirrors: kubernetes/argocd/applications/gitops/argocd/app.yaml
ARGOCD_CHART_VERSION=9.1.0
ARGOCD_CHART_REPO=https://argoproj.github.io/argo-helm
