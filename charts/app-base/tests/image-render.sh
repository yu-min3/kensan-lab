#!/usr/bin/env bash
set -euo pipefail

chart="$(cd "$(dirname "$0")/.." && pwd)"
repo='ghcr.io/example-team/widget'
digest="sha256:$(printf 'a%.0s' {1..64})"

tag_output="$(helm template image-test "$chart" --set "image.repository=$repo" --set 'image.tag=v1')"
[[ "$tag_output" == *"image: \"$repo:v1\""* ]]

digest_output="$(helm template image-test "$chart" --set "image.repository=$repo" --set "image.digest=$digest")"
[[ "$digest_output" == *"image: \"$repo@$digest\""* ]]

for values in 'image.digest=unknown' 'image.digest=sha256:123' 'image.tag=v1,image.digest=sha256:'"${digest#sha256:}"; do
  if helm template image-test "$chart" --set "image.repository=$repo" --set "$values" >/dev/null 2>&1; then
    echo "invalid image values rendered: $values" >&2
    exit 1
  fi
done

if helm template image-test "$chart" --set "image.repository=$repo" >/dev/null 2>&1; then
  echo 'missing tag and digest rendered' >&2
  exit 1
fi
