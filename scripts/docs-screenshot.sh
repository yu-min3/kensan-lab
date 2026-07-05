#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/docs-screenshot.sh [page] [viewport] [output]

Pages:
  home       http://127.0.0.1:8001/kensan-lab/
  network    http://127.0.0.1:8001/kensan-lab/architecture/network/
  design     http://127.0.0.1:8001/kensan-lab/design/DESIGN_SYSTEM/
  <url>      Any full URL

Viewports:
  desktop    1440x1200
  tall       1440x2200
  mobile     390x1200
  <WxH>      e.g. 1280x1600

Output:
  Defaults to /Users/yu/kensan-workspace/temp/docs-<page>-<viewport>.png
USAGE
}

page="${1:-network}"
viewport="${2:-desktop}"
output="${3:-}"

case "$page" in
  home)
    url="http://127.0.0.1:8001/kensan-lab/"
    page_name="home"
    ;;
  network)
    url="http://127.0.0.1:8001/kensan-lab/architecture/network/"
    page_name="network"
    ;;
  design)
    url="http://127.0.0.1:8001/kensan-lab/design/DESIGN_SYSTEM/"
    page_name="design"
    ;;
  http://*|https://*)
    url="$page"
    page_name="custom"
    ;;
  -h|--help)
    usage
    exit 0
    ;;
  *)
    echo "Unknown page: $page" >&2
    usage >&2
    exit 2
    ;;
esac

case "$viewport" in
  desktop)
    size="1440,1200"
    ;;
  tall)
    size="1440,2200"
    ;;
  mobile)
    size="390,1200"
    ;;
  *x*)
    size="${viewport/x/,}"
    ;;
  *)
    echo "Unknown viewport: $viewport" >&2
    usage >&2
    exit 2
    ;;
esac

if [[ -z "$output" ]]; then
  output="/Users/yu/kensan-workspace/temp/docs-${page_name}-${viewport}.png"
fi

mkdir -p "$(dirname "$output")"

chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
if [[ ! -x "$chrome" ]]; then
  echo "Google Chrome not found: $chrome" >&2
  exit 1
fi

"$chrome" \
  --headless \
  --disable-gpu \
  --hide-scrollbars \
  --window-size="$size" \
  --screenshot="$output" \
  "$url"

echo "$output"
