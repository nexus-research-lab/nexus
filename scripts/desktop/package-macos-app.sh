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
OUTPUT_DIR="${NEXUS_DESKTOP_PACKAGE_OUTPUT_DIR:-${MACOS_DIR}/.build/package}"
DIST_NAME="${NEXUS_DESKTOP_PACKAGE_NAME:-${APP_NAME}-macos-${APP_VERSION}-${BUILD_NUMBER}}"
STAGING_ROOT="${OUTPUT_DIR}/staging"
STAGING_DIR="${STAGING_ROOT}/${DIST_NAME}"
DMG_DIR="${STAGING_ROOT}/${DIST_NAME}-dmg"
ARTIFACT_FORMAT="${NEXUS_DESKTOP_PACKAGE_FORMAT:-zip}"
ARTIFACT_PATH="${OUTPUT_DIR}/${DIST_NAME}.${ARTIFACT_FORMAT}"
SHA256_PATH="${ARTIFACT_PATH}.sha256"
METADATA_PATH="${STAGING_DIR}/PACKAGE-METADATA.json"
if [[ "${ARTIFACT_FORMAT}" == "dmg" && -z "${NEXUS_DESKTOP_PACKAGE_METADATA_PATH:-}" ]]; then
  METADATA_EXPORT_PATH="${OUTPUT_DIR}/${DIST_NAME}.dmg.metadata.json"
else
  METADATA_EXPORT_PATH="${NEXUS_DESKTOP_PACKAGE_METADATA_PATH:-${OUTPUT_DIR}/${DIST_NAME}.metadata.json}"
fi
CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
COMMIT_SHA="$(git -C "${ROOT_DIR}" rev-parse HEAD 2>/dev/null || echo unknown)"
COMMIT_SHORT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo unknown)"
SOURCE_DIRTY=false

case "${ARTIFACT_FORMAT}" in
  zip | dmg)
    ;;
  *)
    echo "unsupported macOS artifact format: ${ARTIFACT_FORMAT}" >&2
    echo "supported formats: zip, dmg" >&2
    exit 1
    ;;
esac

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

rm -rf "${STAGING_DIR}" "${DMG_DIR}" "${ARTIFACT_PATH}" "${SHA256_PATH}" "${METADATA_EXPORT_PATH}"
mkdir -p "${STAGING_DIR}" "${OUTPUT_DIR}"

rsync -a --delete --exclude ".DS_Store" "${APP_BUNDLE}/" "${STAGING_DIR}/${APP_NAME}.app/"

{
  printf 'Nexus macOS app package\n\n'
  printf 'Version: %s\n' "${APP_VERSION}"
  printf 'Build: %s\n' "${BUILD_NUMBER}"
  printf 'Commit: %s\n' "${COMMIT_SHORT}"
  printf 'Created: %s\n\n' "${CREATED_AT}"
  printf 'This package is ad-hoc signed and not notarized.\n'
  printf 'After verifying the sha256 file, drag %s.app to /Applications.\n' "${APP_NAME}"
  printf 'If macOS blocks the app because it is not notarized, use Finder right-click Open for trusted builds.\n'
  printf 'For local test machines only, quarantine can also be removed with:\n'
  printf '  xattr -dr com.apple.quarantine /Applications/%s.app\n\n' "${APP_NAME}"
  printf 'Data directory: ~/.nexus\n'
  printf 'Log directory: ~/.nexus/logs\n'
  printf 'To reset app data, quit Nexus first, then remove ~/.nexus.\n'
} > "${STAGING_DIR}/README.txt"

PACKAGE_APP_NAME="${APP_NAME}" \
PACKAGE_EXECUTABLE_NAME="${EXECUTABLE_NAME}" \
PACKAGE_BUNDLE_IDENTIFIER="${BUNDLE_IDENTIFIER}" \
PACKAGE_APP_VERSION="${APP_VERSION}" \
PACKAGE_BUILD_NUMBER="${BUILD_NUMBER}" \
PACKAGE_CREATED_AT="${CREATED_AT}" \
PACKAGE_COMMIT_SHA="${COMMIT_SHA}" \
PACKAGE_COMMIT_SHORT="${COMMIT_SHORT}" \
PACKAGE_SOURCE_DIRTY="${SOURCE_DIRTY}" \
PACKAGE_DIST_NAME="${DIST_NAME}" \
PACKAGE_ARTIFACT_FORMAT="${ARTIFACT_FORMAT}" \
node - "${METADATA_PATH}" <<'NODE'
const fs = require("fs");

const outputPath = process.argv[2];
const env = process.env;
const metadata = {
  app_name: env.PACKAGE_APP_NAME,
  executable_name: env.PACKAGE_EXECUTABLE_NAME,
  bundle_identifier: env.PACKAGE_BUNDLE_IDENTIFIER,
  platform: "macos",
  version: env.PACKAGE_APP_VERSION,
  build_number: env.PACKAGE_BUILD_NUMBER,
  created_at: env.PACKAGE_CREATED_AT,
  source: {
    commit: env.PACKAGE_COMMIT_SHA,
    short_commit: env.PACKAGE_COMMIT_SHORT,
    dirty: env.PACKAGE_SOURCE_DIRTY === "true",
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
    name: env.PACKAGE_DIST_NAME,
    format: env.PACKAGE_ARTIFACT_FORMAT,
  },
  validation: {
    build_script: "scripts/desktop/build-macos-app.sh",
    smoke_script: "scripts/desktop/smoke-macos-app.sh",
    expected_credentials_storage: "file",
  },
};

fs.writeFileSync(outputPath, `${JSON.stringify(metadata, null, 2)}\n`);
NODE

case "${ARTIFACT_FORMAT}" in
  zip)
    if command -v zip >/dev/null 2>&1; then
      (cd "${STAGING_ROOT}" && COPYFILE_DISABLE=1 zip -qry "${ARTIFACT_PATH}" "$(basename "${STAGING_DIR}")" -x "*.DS_Store" "*/._*")
    else
      COPYFILE_DISABLE=1 ditto -c -k --keepParent "${STAGING_DIR}" "${ARTIFACT_PATH}"
    fi
    ;;
  dmg)
    if ! command -v hdiutil >/dev/null 2>&1; then
      echo "hdiutil is required to build macOS dmg artifacts" >&2
      exit 1
    fi
    rm -rf "${DMG_DIR}"
    mkdir -p "${DMG_DIR}"
    rsync -a --delete --exclude ".DS_Store" "${STAGING_DIR}/${APP_NAME}.app/" "${DMG_DIR}/${APP_NAME}.app/"
    ln -s /Applications "${DMG_DIR}/Applications"
    COPYFILE_DISABLE=1 hdiutil create \
      -volname "${APP_NAME}" \
      -srcfolder "${DMG_DIR}" \
      -ov \
      -format UDZO \
      "${ARTIFACT_PATH}" >/dev/null
    ;;
esac

ARTIFACT_SHA256="$(shasum -a 256 "${ARTIFACT_PATH}" | awk '{print $1}')"
printf '%s  %s\n' "${ARTIFACT_SHA256}" "$(basename "${ARTIFACT_PATH}")" > "${SHA256_PATH}"
cp "${METADATA_PATH}" "${METADATA_EXPORT_PATH}"

echo "macOS ${ARTIFACT_FORMAT}: ${ARTIFACT_PATH}"
echo "sha256: ${SHA256_PATH}"
echo "metadata: ${METADATA_EXPORT_PATH}"
