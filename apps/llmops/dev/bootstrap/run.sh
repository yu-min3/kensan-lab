#!/bin/sh
# Bootstrap only the local development stack's identity and access model.
# Generates private limited credentials, creates app clients/grants, then asserts actual denials.
set -eu
umask 077
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../.." && pwd)
cd "$root"
apps/llmops/dev/run.sh
python3 -m venv temp/llmops-probe/venv
temp/llmops-probe/venv/bin/pip install -r apps/llmops/dev/probes/requirements.txt
temp/llmops-probe/venv/bin/python apps/llmops/dev/bootstrap/clients.py
temp/llmops-probe/venv/bin/python apps/llmops/dev/bootstrap/storage.py
docker compose --env-file temp/llmops-dev/dev.env -f apps/llmops/dev/compose.yaml up -d polaris
python3 apps/llmops/dev/check.py
temp/llmops-probe/venv/bin/python apps/llmops/dev/bootstrap/catalog.py
