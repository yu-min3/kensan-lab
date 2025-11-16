#!/bin/bash
# This script requires manual input for secrets.
# Replace <strong-password> and <github-pat> with your actual values.

# PostgreSQL Secret
kubectl create secret generic postgresql-secret \
  --namespace=backstage \
  --from-literal=POSTGRES_USER=backstage \
  --from-literal=POSTGRES_PASSWORD=<strong-password> \
  --dry-run=client -o yaml > ../../temp/backstage-postgresql-secret-raw.yaml

kubeseal --format=yaml < ../../temp/backstage-postgresql-secret-raw.yaml \
  > ../../base-infra/backstage/postgresql-secret.yaml


# Backstage Secret
kubectl create secret generic backstage-secret \
  --namespace=backstage \
  --from-literal=POSTGRES_USER=backstage \
  --from-literal=POSTGRES_PASSWORD=<strong-password> \
  --from-literal=GITHUB_TOKEN=<github-pat> \
  --dry-run=client -o yaml > ../../temp/backstage-secret-raw.yaml

kubeseal --format=yaml < ../../temp/backstage-secret-raw.yaml \
  > ../../base-infra/backstage/backstage-secret.yaml
