#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MACOS_DIR="${ROOT_DIR}/desktop/macos"
load_desktop_env_from_dotenv() {
  local dotenv_path="${ROOT_DIR}/.env"
  [[ -f "${dotenv_path}" ]] || return 0
  local key value
  for key in NEXUS_DESKTOP_GITHUB_CLIENT_ID NEXUS_DESKTOP_GITHUB_CLIENT_SECRET; do
    if [[ -n "${!key:-}" ]]; then
      continue
    fi
    value="$(
      awk -F= -v key="${key}" '
        $0 ~ "^[[:space:]]*(export[[:space:]]+)?" key "[[:space:]]*=" {
          sub(/^[[:space:]]*export[[:space:]]+/, "", $0)
          sub("^[[:space:]]*" key "[[:space:]]*=", "", $0)
          sub(/[[:space:]]+#.*$/, "", $0)
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
          if (($0 ~ /^".*"$/) || ($0 ~ /^\047.*\047$/)) {
            $0 = substr($0, 2, length($0) - 2)
          }
          value = $0
        }
        END { print value }
      ' "${dotenv_path}"
    )"
    if [[ -n "${value}" ]]; then
      export "${key}=${value}"
    fi
  done
}

load_desktop_env_from_dotenv
APP_NAME="${NEXUS_DESKTOP_APP_NAME:-Nexus}"
EXECUTABLE_NAME="${NEXUS_DESKTOP_EXECUTABLE_NAME:-Nexus}"
BUNDLE_IDENTIFIER="${NEXUS_DESKTOP_BUNDLE_IDENTIFIER:-com.leemysw.nexus}"
APP_VERSION="${NEXUS_DESKTOP_VERSION:-$(cd "${ROOT_DIR}/web" && node -p "require('./package.json').version")}"
BUILD_NUMBER="${NEXUS_DESKTOP_BUILD_NUMBER:-$(git -C "${ROOT_DIR}" rev-list --count HEAD 2>/dev/null || date +%Y%m%d%H%M%S)}"
APP_BUILD_DIR="${NEXUS_DESKTOP_APP_BUILD_DIR:-${MACOS_DIR}/.build/app}"
APP_BUNDLE="${APP_BUILD_DIR}/${APP_NAME}.app"
CONTENTS_DIR="${APP_BUNDLE}/Contents"
MACOS_CONTENTS_DIR="${CONTENTS_DIR}/MacOS"
RESOURCES_DIR="${CONTENTS_DIR}/Resources"
SIDECAR_BUILD_DIR="${APP_BUILD_DIR}/.intermediates"
SIDECAR_BUILD_PATH="${SIDECAR_BUILD_DIR}/nexus-server"
SWIFT_PRODUCT="NexusDesktop"

echo "==> Building web/dist"
cd "${ROOT_DIR}/web"
pnpm install --frozen-lockfile
NEXUS_DESKTOP_BUILD=1 pnpm build

echo "==> Building Go sidecar"
mkdir -p "${SIDECAR_BUILD_DIR}"
cd "${ROOT_DIR}"
CGO_ENABLED="${CGO_ENABLED:-1}" go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "${SIDECAR_BUILD_PATH}" \
  ./cmd/nexus-server

echo "==> Building Swift shell"
swift build --package-path "${MACOS_DIR}" -c release
SWIFT_BIN_PATH="$(swift build --package-path "${MACOS_DIR}" -c release --show-bin-path)"

echo "==> Assembling ${APP_BUNDLE}"
rm -rf "${APP_BUNDLE}"
rm -f "${APP_BUILD_DIR}/nexus-server" "${APP_BUILD_DIR}/.DS_Store"
mkdir -p "${MACOS_CONTENTS_DIR}" "${RESOURCES_DIR}"

cp "${SWIFT_BIN_PATH}/${SWIFT_PRODUCT}" "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}"
cp "${SIDECAR_BUILD_PATH}" "${MACOS_CONTENTS_DIR}/nexus-server"
cp "${MACOS_DIR}/Resources/AppIcon.icns" "${RESOURCES_DIR}/AppIcon.icns"
chmod 0755 "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}" \
  "${MACOS_CONTENTS_DIR}/nexus-server"

rsync -a --delete --exclude '.DS_Store' "${ROOT_DIR}/web/dist/" "${RESOURCES_DIR}/Web/"
rsync -a --delete --exclude '.DS_Store' "${ROOT_DIR}/db/migrations/" "${RESOURCES_DIR}/db/migrations/"
rsync -a --delete --exclude '.DS_Store' "${ROOT_DIR}/skills/" "${RESOURCES_DIR}/skills/"

DESKTOP_ENV_PATH="${RESOURCES_DIR}/desktop.env"
rm -f "${DESKTOP_ENV_PATH}"
if [[ -n "${NEXUS_DESKTOP_GITHUB_CLIENT_ID:-}" || -n "${NEXUS_DESKTOP_GITHUB_CLIENT_SECRET:-}" ]]; then
  {
    if [[ -n "${NEXUS_DESKTOP_GITHUB_CLIENT_ID:-}" ]]; then
      printf 'CONNECTOR_GITHUB_CLIENT_ID=%s\n' "${NEXUS_DESKTOP_GITHUB_CLIENT_ID}"
    fi
    if [[ -n "${NEXUS_DESKTOP_GITHUB_CLIENT_SECRET:-}" ]]; then
      printf 'CONNECTOR_GITHUB_CLIENT_SECRET=%s\n' "${NEXUS_DESKTOP_GITHUB_CLIENT_SECRET}"
    fi
  } > "${DESKTOP_ENV_PATH}"
  chmod 0600 "${DESKTOP_ENV_PATH}"
fi

sed \
  -e "s/__APP_NAME__/${APP_NAME}/g" \
  -e "s/__EXECUTABLE_NAME__/${EXECUTABLE_NAME}/g" \
  -e "s/__BUNDLE_IDENTIFIER__/${BUNDLE_IDENTIFIER}/g" \
  -e "s/__APP_VERSION__/${APP_VERSION}/g" \
  -e "s/__BUILD_NUMBER__/${BUILD_NUMBER}/g" \
  "${MACOS_DIR}/Resources/Info.plist" > "${CONTENTS_DIR}/Info.plist"

printf 'APPL????' > "${CONTENTS_DIR}/PkgInfo"

if [[ "${NEXUS_DESKTOP_SKIP_CODESIGN:-0}" != "1" ]] && command -v codesign >/dev/null 2>&1; then
  echo "==> Applying ad-hoc signature"
  codesign --force --sign - "${MACOS_CONTENTS_DIR}/nexus-server" >/dev/null
  codesign --force --sign - "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}" >/dev/null
  codesign --force --deep --sign - "${APP_BUNDLE}" >/dev/null
fi

rm -rf "${SIDECAR_BUILD_DIR}"
rm -f "${APP_BUILD_DIR}/.DS_Store"

echo "==> Built ${APP_BUNDLE}"
