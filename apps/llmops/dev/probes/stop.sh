#!/bin/sh
# Stop only the isolated compatibility stack, preserving its probe volume and credentials.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../.." && pwd)
cd "$root"
docker compose --env-file temp/llmops-probe/probe.env -f apps/llmops/dev/probes/compose.yaml down
