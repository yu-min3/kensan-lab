#!/bin/sh
# Offline Helm lint/render and structural assertions. No kubectl, cluster access,
# registry push, raw Secret, or committed rendered manifests.
set -eu
cd "$(dirname "$0")"
helm lint --strict chart -f tests/values.json
python3 tests/check.py
