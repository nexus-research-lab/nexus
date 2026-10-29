#!/usr/bin/env bash
# INPUT: Current checkout and its pinned Bridge module, without go.work replacements.
# OUTPUT: A fresh development sidecar bundle and an atomically switched pointer.
# POS: Keep prior bundles intact while their processes may still be running.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"
BUILD_ROOT="${ROOT_DIR}/desktop/macos/.build"
mkdir -p "${BUILD_ROOT}"
STAGING="$(mktemp -d "${BUILD_ROOT}/sidecar-build.XXXXXX")"
APP_BUNDLE="${STAGING}/Nexus.app"
mkdir -p "${APP_BUNDLE}/Contents/MacOS" "${APP_BUNDLE}/Contents/Resources/bin"
GOWORK=off CGO_ENABLED=1 go build -o "${APP_BUNDLE}/Contents/MacOS/nexus-server" ./cmd/nexus-server
# Rename the symlink itself, never an existing bundle.
node --input-type=module - "${APP_BUNDLE}" "${BUILD_ROOT}/sidecar-current" <<'JS'
import { symlink, rename } from 'node:fs/promises'
const [bundle, pointer] = process.argv.slice(2)
const temporary = `${pointer}.${process.pid}`
await symlink(bundle, temporary)
await rename(temporary, pointer)
JS
