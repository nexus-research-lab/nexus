#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

"${ROOT_DIR}/scripts/desktop/build-macos-dev.sh"

cd "${ROOT_DIR}/desktop/macos"
swift run NexusDesktop
