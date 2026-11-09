#!/bin/sh
# Run isolated real-service compatibility checks. Requires Docker and Python 3.
# Creates private local credentials and probe data only; does not use cluster services.
set -eu
umask 077
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../.." && pwd)
cd "$root"
mkdir -p temp/llmops-probe
python3 - <<'PY'
from pathlib import Path
import secrets,os
p=Path('temp/llmops-probe/probe.env')
if not p.exists():
 p.write_text(''.join(k+'='+secrets.token_hex(24)+'\n' for k in ['PROBE_S3_KEY','PROBE_S3_SECRET','PROBE_CATALOG_SECRET','PROBE_DB_SECRET']))
 os.chmod(p,0o600)
PY
python3 -m venv temp/llmops-probe/venv
temp/llmops-probe/venv/bin/pip install -r apps/llmops/dev/probes/requirements.txt
docker compose --env-file temp/llmops-probe/probe.env -f apps/llmops/dev/probes/compose.yaml up -d
temp/llmops-probe/venv/bin/python apps/llmops/dev/probes/probe.py --env-file temp/llmops-probe/probe.env
