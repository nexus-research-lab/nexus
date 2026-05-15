#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MACOS_DIR="${ROOT_DIR}/desktop/macos"
APP_NAME="${NEXUS_DESKTOP_APP_NAME:-Nexus}"
EXECUTABLE_NAME="${NEXUS_DESKTOP_EXECUTABLE_NAME:-Nexus}"
BUNDLE_IDENTIFIER="${NEXUS_DESKTOP_BUNDLE_IDENTIFIER:-com.leemysw.nexus}"
APP_VERSION="${NEXUS_DESKTOP_VERSION:-$(cd "${ROOT_DIR}/web" && node -p "require('./package.json').version")}"
BUILD_NUMBER="${NEXUS_DESKTOP_BUILD_NUMBER:-$(git -C "${ROOT_DIR}" rev-list --count HEAD 2>/dev/null || date +%Y%m%d%H%M%S)}"
APP_BUILD_DIR="${NEXUS_DESKTOP_APP_BUILD_DIR:-${MACOS_DIR}/.build/app}"
APP_BUNDLE="${APP_BUILD_DIR}/${APP_NAME}.app"
OUTPUT_DIR="${NEXUS_DESKTOP_DOGFOOD_OUTPUT_DIR:-${MACOS_DIR}/.build/dogfood}"
DIST_NAME="${NEXUS_DESKTOP_DOGFOOD_NAME:-${APP_NAME}-macos-${APP_VERSION}-${BUILD_NUMBER}}"
STAGING_ROOT="${OUTPUT_DIR}/staging"
STAGING_DIR="${STAGING_ROOT}/${DIST_NAME}"
ZIP_PATH="${OUTPUT_DIR}/${DIST_NAME}.zip"
SHA256_PATH="${ZIP_PATH}.sha256"
METADATA_PATH="${STAGING_DIR}/DOGFOOD-METADATA.json"
METADATA_EXPORT_PATH="${OUTPUT_DIR}/${DIST_NAME}.metadata.json"
CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
COMMIT_SHA="$(git -C "${ROOT_DIR}" rev-parse HEAD 2>/dev/null || echo unknown)"
COMMIT_SHORT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo unknown)"
SOURCE_DIRTY=false

if ! git -C "${ROOT_DIR}" diff --quiet --ignore-submodules -- ||
  ! git -C "${ROOT_DIR}" diff --cached --quiet --ignore-submodules -- ||
  [[ -n "$(git -C "${ROOT_DIR}" ls-files --others --exclude-standard)" ]]; then
  SOURCE_DIRTY=true
fi

export NEXUS_DESKTOP_VERSION="${APP_VERSION}"
export NEXUS_DESKTOP_BUILD_NUMBER="${BUILD_NUMBER}"
export NEXUS_DESKTOP_APP_BUILD_DIR="${APP_BUILD_DIR}"

if [[ "${NEXUS_DESKTOP_PACKAGE_SKIP_BUILD:-0}" != "1" ]]; then
  "${ROOT_DIR}/scripts/desktop/build-macos-app.sh"
fi

if [[ ! -d "${APP_BUNDLE}" ]]; then
  echo "missing app bundle: ${APP_BUNDLE}" >&2
  exit 1
fi

plutil -lint "${APP_BUNDLE}/Contents/Info.plist" >/dev/null
if command -v codesign >/dev/null 2>&1; then
  codesign --verify --deep --strict "${APP_BUNDLE}" >/dev/null
fi

if [[ "${NEXUS_DESKTOP_PACKAGE_SKIP_SMOKE:-0}" != "1" ]]; then
  NEXUS_DESKTOP_SMOKE_EXPECTED_CREDENTIALS_STORAGE="${NEXUS_DESKTOP_SMOKE_EXPECTED_CREDENTIALS_STORAGE:-file}" \
    "${ROOT_DIR}/scripts/desktop/smoke-macos-app.sh"
fi

rm -rf "${STAGING_DIR}" "${ZIP_PATH}" "${SHA256_PATH}" "${METADATA_EXPORT_PATH}"
mkdir -p "${STAGING_DIR}" "${OUTPUT_DIR}"

rsync -a --delete --exclude ".DS_Store" "${APP_BUNDLE}/" "${STAGING_DIR}/${APP_NAME}.app/"

{
  printf 'Nexus macOS dogfood build\n\n'
  printf 'Version: %s\n' "${APP_VERSION}"
  printf 'Build: %s\n' "${BUILD_NUMBER}"
  printf 'Commit: %s\n' "${COMMIT_SHORT}"
  printf 'Created: %s\n\n' "${CREATED_AT}"
  printf 'This is an ad-hoc signed internal dogfood package. It is not notarized.\n'
  printf 'After verifying the sha256 file, drag %s.app to /Applications.\n' "${APP_NAME}"
  printf 'If macOS blocks the app because it is not notarized, use Finder right-click Open for trusted dogfood builds.\n'
  printf 'For local test machines only, quarantine can also be removed with:\n'
  printf '  xattr -dr com.apple.quarantine /Applications/%s.app\n\n' "${APP_NAME}"
  printf 'Data directory: ~/Library/Application Support/Nexus\n'
  printf 'Log directory: ~/Library/Logs/Nexus\n'
  printf 'To reset dogfood data, quit Nexus first, then remove those directories.\n'
} > "${STAGING_DIR}/README-DOGFOOD.txt"

DOGFOOD_APP_NAME="${APP_NAME}" \
DOGFOOD_EXECUTABLE_NAME="${EXECUTABLE_NAME}" \
DOGFOOD_BUNDLE_IDENTIFIER="${BUNDLE_IDENTIFIER}" \
DOGFOOD_APP_VERSION="${APP_VERSION}" \
DOGFOOD_BUILD_NUMBER="${BUILD_NUMBER}" \
DOGFOOD_CREATED_AT="${CREATED_AT}" \
DOGFOOD_COMMIT_SHA="${COMMIT_SHA}" \
DOGFOOD_COMMIT_SHORT="${COMMIT_SHORT}" \
DOGFOOD_SOURCE_DIRTY="${SOURCE_DIRTY}" \
DOGFOOD_DIST_NAME="${DIST_NAME}" \
node - "${METADATA_PATH}" <<'NODE'
const fs = require("fs");

const outputPath = process.argv[2];
const env = process.env;
const metadata = {
  app_name: env.DOGFOOD_APP_NAME,
  executable_name: env.DOGFOOD_EXECUTABLE_NAME,
  bundle_identifier: env.DOGFOOD_BUNDLE_IDENTIFIER,
  platform: "macos",
  version: env.DOGFOOD_APP_VERSION,
  build_number: env.DOGFOOD_BUILD_NUMBER,
  created_at: env.DOGFOOD_CREATED_AT,
  source: {
    commit: env.DOGFOOD_COMMIT_SHA,
    short_commit: env.DOGFOOD_COMMIT_SHORT,
    dirty: env.DOGFOOD_SOURCE_DIRTY === "true",
  },
  signing: {
    kind: "ad-hoc",
    developer_id: false,
    notarized: false,
  },
  keychain: {
    expected_storage: "file",
    expected_reason: "ad_hoc_signature",
  },
  artifact: {
    name: env.DOGFOOD_DIST_NAME,
    format: "zip",
  },
  validation: {
    build_script: "scripts/desktop/build-macos-app.sh",
    smoke_script: "scripts/desktop/smoke-macos-app.sh",
    expected_credentials_storage: "file",
  },
};

fs.writeFileSync(outputPath, `${JSON.stringify(metadata, null, 2)}\n`);
NODE

if command -v zip >/dev/null 2>&1; then
  (cd "${STAGING_ROOT}" && COPYFILE_DISABLE=1 zip -qry "${ZIP_PATH}" "$(basename "${STAGING_DIR}")" -x "*.DS_Store" "*/._*")
else
  COPYFILE_DISABLE=1 ditto -c -k --keepParent "${STAGING_DIR}" "${ZIP_PATH}"
fi

ARTIFACT_SHA256="$(shasum -a 256 "${ZIP_PATH}" | awk '{print $1}')"
printf '%s  %s\n' "${ARTIFACT_SHA256}" "$(basename "${ZIP_PATH}")" > "${SHA256_PATH}"
cp "${METADATA_PATH}" "${METADATA_EXPORT_PATH}"

echo "dogfood package: ${ZIP_PATH}"
echo "sha256: ${SHA256_PATH}"
echo "metadata: ${METADATA_EXPORT_PATH}"
