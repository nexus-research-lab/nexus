#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cd "${ROOT_DIR}/web"
pnpm install --frozen-lockfile
NEXUS_DESKTOP_BUILD=1 pnpm build

cd "${ROOT_DIR}/desktop/macos"
swift build
