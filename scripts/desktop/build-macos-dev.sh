#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cd "${ROOT_DIR}/web"
corepack pnpm@9.15.2 install --frozen-lockfile
NEXUS_DESKTOP_BUILD=1 corepack pnpm@9.15.2 build

cd "${ROOT_DIR}/desktop/macos"
swift build
