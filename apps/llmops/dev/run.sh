#!/bin/sh
# Start isolated development services and verify local HTTPS trust. Requires Docker/Python/OpenSSL.
# Credentials stay in ignored temp/llmops-dev; cluster services and production settings are not used.
set -eu
umask 077
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"
python3 apps/llmops/dev/prepare.py
docker compose --env-file temp/llmops-dev/dev.env -f apps/llmops/dev/compose.yaml up -d
python3 apps/llmops/dev/check.py
