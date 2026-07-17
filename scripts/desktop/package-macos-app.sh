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
PACKAGE_SIGNING_KIND="unsigned"
PACKAGE_SIGNING_DEVELOPER_ID=false
PACKAGE_SIGNING_NOTARIZED=false
PACKAGE_SIGNING_TEAM_ID=""
PACKAGE_KEYCHAIN_EXPECTED_STORAGE="file"
PACKAGE_KEYCHAIN_EXPECTED_REASON="ad_hoc_signature"

is_enabled() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1 | true | yes | on)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

detect_app_signature() {
  if ! command -v codesign >/dev/null 2>&1; then
    return 0
  fi

  local signature_details
  signature_details="$(codesign -dv --verbose=4 "${APP_BUNDLE}" 2>&1 || true)"
  if printf '%s\n' "${signature_details}" | grep -q '^Authority=Developer ID Application:'; then
    PACKAGE_SIGNING_KIND="developer-id"
    PACKAGE_SIGNING_DEVELOPER_ID=true
    PACKAGE_KEYCHAIN_EXPECTED_STORAGE="keychain"
    PACKAGE_KEYCHAIN_EXPECTED_REASON="signed_auto"
  elif printf '%s\n' "${signature_details}" | grep -q '^Signature=adhoc'; then
    PACKAGE_SIGNING_KIND="ad-hoc"
  elif printf '%s\n' "${signature_details}" | grep -q '^Authority='; then
    PACKAGE_SIGNING_KIND="custom"
  fi

  PACKAGE_SIGNING_TEAM_ID="$(
    printf '%s\n' "${signature_details}" |
      awk -F= '/^TeamIdentifier=/ { print $2; exit }'
  )"
  if [[ "${PACKAGE_SIGNING_TEAM_ID}" == "not set" ]]; then
    PACKAGE_SIGNING_TEAM_ID=""
  fi
}

submit_for_notarization() {
  local package_path="$1"
  local label="$2"
  local auth_args=()

  if [[ -n "${NEXUS_DESKTOP_NOTARY_PROFILE:-}" ]]; then
    auth_args+=(--keychain-profile "${NEXUS_DESKTOP_NOTARY_PROFILE}")
  else
    if [[ -z "${NEXUS_DESKTOP_NOTARY_APPLE_ID:-}" ||
      -z "${NEXUS_DESKTOP_NOTARY_TEAM_ID:-}" ||
      -z "${NEXUS_DESKTOP_NOTARY_PASSWORD:-}" ]]; then
      echo "missing notarization credentials" >&2
      echo "set NEXUS_DESKTOP_NOTARY_PROFILE, or set NEXUS_DESKTOP_NOTARY_APPLE_ID/NEXUS_DESKTOP_NOTARY_TEAM_ID/NEXUS_DESKTOP_NOTARY_PASSWORD" >&2
      exit 1
    fi
    auth_args+=(
      --apple-id "${NEXUS_DESKTOP_NOTARY_APPLE_ID}"
      --team-id "${NEXUS_DESKTOP_NOTARY_TEAM_ID}"
      --password "${NEXUS_DESKTOP_NOTARY_PASSWORD}"
    )
  fi

  echo "==> Notarizing ${label}"
  xcrun notarytool submit "${package_path}" "${auth_args[@]}" --wait
}

staple_notarization_ticket() {
  local package_path="$1"
  echo "==> Stapling notarization ticket: ${package_path}"
  xcrun stapler staple "${package_path}" >/dev/null
  xcrun stapler validate "${package_path}" >/dev/null
}

notarize_app_bundle() {
  if ! is_enabled "${NEXUS_DESKTOP_NOTARIZE:-0}"; then
    return 0
  fi
  if [[ "${PACKAGE_SIGNING_DEVELOPER_ID}" != "true" ]]; then
    echo "NEXUS_DESKTOP_NOTARIZE=1 requires a Developer ID Application signature" >&2
    echo "set NEXUS_DESKTOP_CODESIGN_IDENTITY='Developer ID Application: Your Name (TEAMID)'" >&2
    exit 1
  fi
  if ! command -v xcrun >/dev/null 2>&1; then
    echo "xcrun is required for notarization" >&2
    exit 1
  fi
  if ! command -v ditto >/dev/null 2>&1; then
    echo "ditto is required to create the notarization archive" >&2
    exit 1
  fi

  local notary_zip_path="${OUTPUT_DIR}/${DIST_NAME}-notary.zip"
  rm -f "${notary_zip_path}"
  echo "==> Creating notarization archive"
  (
    cd "$(dirname "${APP_BUNDLE}")"
    COPYFILE_DISABLE=1 ditto -c -k --keepParent "$(basename "${APP_BUNDLE}")" "${notary_zip_path}"
  )
  submit_for_notarization "${notary_zip_path}" "${APP_NAME}.app"
  staple_notarization_ticket "${APP_BUNDLE}"
  spctl --assess --type execute --verbose "${APP_BUNDLE}" >/dev/null
  PACKAGE_SIGNING_NOTARIZED=true
}

notarize_dmg_artifact() {
  if [[ "${ARTIFACT_FORMAT}" != "dmg" || "${PACKAGE_SIGNING_NOTARIZED}" != "true" ]]; then
    return 0
  fi
  if ! is_enabled "${NEXUS_DESKTOP_NOTARIZE_DMG:-1}"; then
    return 0
  fi
  submit_for_notarization "${ARTIFACT_PATH}" "${ARTIFACT_FORMAT} artifact"
  staple_notarization_ticket "${ARTIFACT_PATH}"
}

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
export NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME="${NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME:-1}"

if [[ "${NEXUS_DESKTOP_PACKAGE_SKIP_BUILD:-0}" != "1" ]]; then
  "${ROOT_DIR}/scripts/desktop/build-macos-app.sh"
fi

if [[ ! -d "${APP_BUNDLE}" ]]; then
  echo "missing app bundle: ${APP_BUNDLE}" >&2
  exit 1
fi

NXS_RUNTIME_PATH="${APP_BUNDLE}/Contents/Resources/bin/nxs"
if is_enabled "${NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME}" && [[ ! -x "${NXS_RUNTIME_PATH}" ]]; then
  echo "missing bundled nxs runtime: ${NXS_RUNTIME_PATH}" >&2
  exit 1
fi

plutil -lint "${APP_BUNDLE}/Contents/Info.plist" >/dev/null
if command -v codesign >/dev/null 2>&1; then
  codesign --verify --deep --strict "${APP_BUNDLE}" >/dev/null
fi

detect_app_signature

if [[ "${NEXUS_DESKTOP_PACKAGE_SKIP_SMOKE:-0}" != "1" ]]; then
  NEXUS_DESKTOP_SMOKE_EXPECTED_CREDENTIALS_STORAGE="${NEXUS_DESKTOP_SMOKE_EXPECTED_CREDENTIALS_STORAGE:-${PACKAGE_KEYCHAIN_EXPECTED_STORAGE}}" \
    NEXUS_DESKTOP_SMOKE_EXPECT_NXS_RUNTIME="${NEXUS_DESKTOP_SMOKE_EXPECT_NXS_RUNTIME:-${NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME}}" \
    "${ROOT_DIR}/scripts/desktop/smoke-macos-app.sh"
fi

mkdir -p "${OUTPUT_DIR}"
notarize_app_bundle

rm -rf "${STAGING_DIR}" "${DMG_DIR}" "${ARTIFACT_PATH}" "${SHA256_PATH}" "${METADATA_EXPORT_PATH}"
mkdir -p "${STAGING_DIR}" "${OUTPUT_DIR}"

rsync -a --delete --exclude ".DS_Store" "${APP_BUNDLE}/" "${STAGING_DIR}/${APP_NAME}.app/"
NXS_RUNTIME_BUNDLED=false
if [[ -x "${NXS_RUNTIME_PATH}" ]]; then
  NXS_RUNTIME_BUNDLED=true
fi

{
  printf 'Nexus macOS app package\n\n'
  printf 'Version: %s\n' "${APP_VERSION}"
  printf 'Build: %s\n' "${BUILD_NUMBER}"
  printf 'Commit: %s\n' "${COMMIT_SHORT}"
  printf 'Created: %s\n\n' "${CREATED_AT}"
  if [[ "${PACKAGE_SIGNING_DEVELOPER_ID}" == "true" && "${PACKAGE_SIGNING_NOTARIZED}" == "true" ]]; then
    printf 'This package is Developer ID signed and notarized.\n'
    if [[ -n "${PACKAGE_SIGNING_TEAM_ID}" ]]; then
      printf 'Developer Team: %s\n' "${PACKAGE_SIGNING_TEAM_ID}"
    fi
    printf 'After verifying the sha256 file, drag %s.app to /Applications.\n\n' "${APP_NAME}"
  else
    printf 'This package is %s signed and not notarized.\n' "${PACKAGE_SIGNING_KIND}"
    printf 'After verifying the sha256 file, drag %s.app to /Applications.\n' "${APP_NAME}"
    printf 'If macOS blocks the app because it is not notarized, use Finder right-click Open for trusted builds.\n'
    printf 'For local test machines only, quarantine can also be removed with:\n'
    printf '  xattr -dr com.apple.quarantine /Applications/%s.app\n\n' "${APP_NAME}"
  fi
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
PACKAGE_NXS_RUNTIME_BUNDLED="${NXS_RUNTIME_BUNDLED}" \
PACKAGE_NXS_RUNTIME_RELEASE="${NEXUS_DESKTOP_NXS_RELEASE:-${NEXUS_NXS_RUNTIME_RELEASE:-nxs-stable}}" \
PACKAGE_SIGNING_KIND="${PACKAGE_SIGNING_KIND}" \
PACKAGE_SIGNING_DEVELOPER_ID="${PACKAGE_SIGNING_DEVELOPER_ID}" \
PACKAGE_SIGNING_NOTARIZED="${PACKAGE_SIGNING_NOTARIZED}" \
PACKAGE_SIGNING_TEAM_ID="${PACKAGE_SIGNING_TEAM_ID}" \
PACKAGE_KEYCHAIN_EXPECTED_STORAGE="${PACKAGE_KEYCHAIN_EXPECTED_STORAGE}" \
PACKAGE_KEYCHAIN_EXPECTED_REASON="${PACKAGE_KEYCHAIN_EXPECTED_REASON}" \
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
    kind: env.PACKAGE_SIGNING_KIND,
    developer_id: env.PACKAGE_SIGNING_DEVELOPER_ID === "true",
    notarized: env.PACKAGE_SIGNING_NOTARIZED === "true",
    team_id: env.PACKAGE_SIGNING_TEAM_ID || null,
  },
  keychain: {
    expected_storage: env.PACKAGE_KEYCHAIN_EXPECTED_STORAGE,
    expected_reason: env.PACKAGE_KEYCHAIN_EXPECTED_REASON,
  },
  runtime: {
    nxs: {
      bundled: env.PACKAGE_NXS_RUNTIME_BUNDLED === "true",
      release: env.PACKAGE_NXS_RUNTIME_RELEASE,
      relative_path: "Contents/Resources/bin/nxs",
    },
  },
  artifact: {
    name: env.PACKAGE_DIST_NAME,
    format: env.PACKAGE_ARTIFACT_FORMAT,
  },
  validation: {
    build_script: "scripts/desktop/build-macos-app.sh",
    smoke_script: "scripts/desktop/smoke-macos-app.sh",
    expected_credentials_storage: env.PACKAGE_KEYCHAIN_EXPECTED_STORAGE,
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

notarize_dmg_artifact

ARTIFACT_SHA256="$(shasum -a 256 "${ARTIFACT_PATH}" | awk '{print $1}')"
printf '%s  %s\n' "${ARTIFACT_SHA256}" "$(basename "${ARTIFACT_PATH}")" > "${SHA256_PATH}"
cp "${METADATA_PATH}" "${METADATA_EXPORT_PATH}"

echo "macOS ${ARTIFACT_FORMAT}: ${ARTIFACT_PATH}"
echo "sha256: ${SHA256_PATH}"
echo "metadata: ${METADATA_EXPORT_PATH}"
echo "signing: ${PACKAGE_SIGNING_KIND}, notarized: ${PACKAGE_SIGNING_NOTARIZED}"
