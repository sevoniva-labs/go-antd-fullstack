#!/usr/bin/env bash
set -euo pipefail

platform=$(uname -s)
architecture=$(uname -m)
if [[ "$platform" != "Linux" || ( "$architecture" != "aarch64" && "$architecture" != "arm64" ) ]]; then
  echo "No validated public domestic Chromium artifact for ${platform}/${architecture}." >&2
  echo "Use an internal checksum-pinned artifact or PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH." >&2
  exit 1
fi

readonly PLAYWRIGHT_DOWNLOAD_HOST='https://npmmirror.com/mirrors/playwright'
export PLAYWRIGHT_DOWNLOAD_HOST
exec pnpm exec playwright install chromium
