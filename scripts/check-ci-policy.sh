#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

if [[ -d .github/workflows ]] && find .github/workflows -type f -print -quit | grep -q .; then
  echo "GitHub Actions workflows are forbidden; use GitLab CI or Jenkins" >&2
  exit 1
fi

files=(Makefile .gitlab-ci.yml Jenkinsfile)
patterns=(
  '@latest'
  'ubuntu-latest'
  'actions/'
  'github-actions'
  'GOPROXY=.*direct'
  'registry\.npmjs\.org'
)
for pattern in "${patterns[@]}"; do
  if rg -n -- "$pattern" "${files[@]}"; then
    echo "forbidden CI dependency pattern: $pattern" >&2
    exit 1
  fi
done

required_versions=(
  'github.com/golangci/golangci-lint/v2 v2.12.2'
  'github.com/securego/gosec/v2 v2.28.0'
  'golang.org/x/vuln v1.7.0'
  'honnef.co/go/tools v0.7.0'
)
for version in "${required_versions[@]}"; do
  rg -Fq "$version" tools/go.mod || {
    echo "missing pinned tool version: $version" >&2
    exit 1
  }
done

echo "CI policy OK: domestic sources and pinned tools"
