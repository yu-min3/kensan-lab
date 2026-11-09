#!/bin/sh
# Stop only the development stack. Keep its database/object volumes and private local credentials.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"
docker compose --env-file temp/llmops-dev/dev.env -f apps/llmops/dev/compose.yaml down
