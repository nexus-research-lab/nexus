#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

run_pnpm() {
  if [[ -n "${PNPM:-}" ]] && command -v "${PNPM%% *}" >/dev/null 2>&1; then
    ${PNPM} "$@"
  elif command -v pnpm >/dev/null 2>&1; then
    pnpm "$@"
  elif command -v corepack >/dev/null 2>&1; then
    corepack pnpm@9.15.2 "$@"
  else
    echo "Neither 'pnpm' nor 'corepack' found in PATH. Please install pnpm." >&2
    exit 1
  fi
}

cd "${ROOT_DIR}/web"
run_pnpm install --frozen-lockfile
NEXUS_DESKTOP_BUILD=1 run_pnpm build

"${ROOT_DIR}/scripts/desktop/build-macos-sidecar-dev.sh"

cd "${ROOT_DIR}/desktop/macos"
swift build
