#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP_NAME="${NEXUS_DESKTOP_APP_NAME:-Nexus}"
APP_BUILD_DIR="${NEXUS_DESKTOP_APP_BUILD_DIR:-${ROOT_DIR}/desktop/macos/.build/app}"
APP_BUNDLE="${APP_BUILD_DIR}/${APP_NAME}.app"

"${ROOT_DIR}/scripts/desktop/build-macos-app.sh"
"${APP_BUNDLE}/Contents/MacOS/${NEXUS_DESKTOP_EXECUTABLE_NAME:-Nexus}"
