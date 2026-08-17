#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MACOS_DIR="${ROOT_DIR}/desktop/macos"
load_desktop_env_from_dotenv() {
  local dotenv_path="${ROOT_DIR}/.env"
  [[ -f "${dotenv_path}" ]] || return 0
  local key value
  for key in NEXUS_DESKTOP_GITHUB_CLIENT_ID; do
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
NEXUSCTL_BUILD_PATH="${SIDECAR_BUILD_DIR}/nexusctl"
NEXUSCFG_BUILD_PATH="${SIDECAR_BUILD_DIR}/nexuscfg"
NEXUS_BUILD_PATH="${SIDECAR_BUILD_DIR}/nexus"
SWIFT_PRODUCT="NexusDesktop"
BUNDLE_NXS_RUNTIME="${NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME:-0}"
NXS_RUNTIME_PATH="${NEXUS_DESKTOP_NXS_RUNTIME_PATH:-}"
HOST_ARCH="$(uname -m)"

case "${NEXUS_DESKTOP_TARGET_ARCH:-${HOST_ARCH}}" in
  arm64 | aarch64)
    TARGET_ARCH="arm64"
    TARGET_GOARCH="arm64"
    ;;
  x86_64 | amd64 | intel)
    TARGET_ARCH="x86_64"
    TARGET_GOARCH="amd64"
    ;;
  *)
    echo "unsupported macOS target architecture: ${NEXUS_DESKTOP_TARGET_ARCH:-${HOST_ARCH}}" >&2
    echo "supported architectures: arm64, x86_64" >&2
    exit 1
    ;;
esac

if [[ "${TARGET_ARCH}" == "${HOST_ARCH}" ]]; then
  DEFAULT_CGO_ENABLED="$(go env CGO_ENABLED)"
else
  DEFAULT_CGO_ENABLED=0
fi
BUILD_CGO_ENABLED="${CGO_ENABLED:-${DEFAULT_CGO_ENABLED}}"

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

CODESIGN_IDENTITY="${NEXUS_DESKTOP_CODESIGN_IDENTITY:-${NEXUS_DESKTOP_DEVELOPER_ID_APPLICATION:-}}"
CODESIGN_SIGN_VALUE="${CODESIGN_IDENTITY:-"-"}"
CODESIGN_DEVELOPER_ID="${NEXUS_DESKTOP_CODESIGN_DEVELOPER_ID:-}"
if [[ -z "${CODESIGN_DEVELOPER_ID}" ]]; then
  if [[ "${CODESIGN_IDENTITY}" == Developer\ ID\ Application:* ]]; then
    CODESIGN_DEVELOPER_ID=1
  else
    CODESIGN_DEVELOPER_ID=0
  fi
fi
CODESIGN_OPTIONS="${NEXUS_DESKTOP_CODESIGN_OPTIONS:-}"
if [[ -z "${CODESIGN_OPTIONS}" ]] && is_enabled "${CODESIGN_DEVELOPER_ID}"; then
  CODESIGN_OPTIONS="runtime"
fi
CODESIGN_TIMESTAMP="${NEXUS_DESKTOP_CODESIGN_TIMESTAMP:-}"
if [[ -z "${CODESIGN_TIMESTAMP}" ]]; then
  if is_enabled "${CODESIGN_DEVELOPER_ID}"; then
    CODESIGN_TIMESTAMP=1
  else
    CODESIGN_TIMESTAMP=0
  fi
fi

codesign_target() {
  local target="$1"
  local args=(--force --sign "${CODESIGN_SIGN_VALUE}")
  if [[ "${CODESIGN_SIGN_VALUE}" != "-" ]] && is_enabled "${CODESIGN_TIMESTAMP}"; then
    args+=(--timestamp)
  fi
  if [[ -n "${CODESIGN_OPTIONS}" && "${CODESIGN_OPTIONS}" != "none" ]]; then
    args+=(--options "${CODESIGN_OPTIONS}")
  fi
  codesign "${args[@]}" "${target}" >/dev/null
}

require_macho_architecture() {
  local executable_path="$1"
  local executable_architectures
  executable_architectures="$(lipo -archs "${executable_path}" 2>/dev/null || true)"
  if [[ " ${executable_architectures} " != *" ${TARGET_ARCH} "* ]]; then
    echo "macOS executable architecture mismatch: ${executable_path}" >&2
    echo "expected ${TARGET_ARCH}, got ${executable_architectures:-unknown}" >&2
    exit 1
  fi
}

echo "==> Building web/dist"
cd "${ROOT_DIR}/web"
pnpm install --frozen-lockfile --prefer-offline
NEXUS_DESKTOP_BUILD=1 pnpm build

echo "==> Building Go sidecar"
mkdir -p "${SIDECAR_BUILD_DIR}"
cd "${ROOT_DIR}"
CGO_ENABLED="${BUILD_CGO_ENABLED}" GOOS=darwin GOARCH="${TARGET_GOARCH}" go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "${SIDECAR_BUILD_PATH}" \
  ./cmd/nexus-server

echo "==> Building nexusctl"
CGO_ENABLED="${BUILD_CGO_ENABLED}" GOOS=darwin GOARCH="${TARGET_GOARCH}" go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "${NEXUSCTL_BUILD_PATH}" \
  ./cmd/nexusctl

echo "==> Building nexuscfg"
CGO_ENABLED="${BUILD_CGO_ENABLED}" GOOS=darwin GOARCH="${TARGET_GOARCH}" go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "${NEXUSCFG_BUILD_PATH}" \
  ./cmd/nexuscfg

echo "==> Building nexus"
CGO_ENABLED="${BUILD_CGO_ENABLED}" GOOS=darwin GOARCH="${TARGET_GOARCH}" go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "${NEXUS_BUILD_PATH}" \
  ./cmd/nexus

echo "==> Building Swift shell (${TARGET_ARCH})"
swift build --package-path "${MACOS_DIR}" -c release --arch "${TARGET_ARCH}"
SWIFT_BIN_PATH="$(swift build --package-path "${MACOS_DIR}" -c release --arch "${TARGET_ARCH}" --show-bin-path)"

echo "==> Assembling ${APP_BUNDLE}"
rm -rf "${APP_BUNDLE}"
rm -f "${APP_BUILD_DIR}/nexus-server" "${APP_BUILD_DIR}/.DS_Store"
mkdir -p "${MACOS_CONTENTS_DIR}" "${RESOURCES_DIR}/bin"

cp "${SWIFT_BIN_PATH}/${SWIFT_PRODUCT}" "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}"
cp "${SIDECAR_BUILD_PATH}" "${MACOS_CONTENTS_DIR}/nexus-server"
cp "${NEXUSCTL_BUILD_PATH}" "${RESOURCES_DIR}/bin/nexusctl"
cp "${NEXUSCFG_BUILD_PATH}" "${RESOURCES_DIR}/bin/nexuscfg"
cp "${NEXUS_BUILD_PATH}" "${RESOURCES_DIR}/bin/nexus"
cp "${MACOS_DIR}/Resources/AppIcon.icns" "${RESOURCES_DIR}/AppIcon.icns"

if is_enabled "${BUNDLE_NXS_RUNTIME}"; then
  nxs_output_path="${RESOURCES_DIR}/bin/nxs"
  nxs_ripgrep_output_path="${RESOURCES_DIR}/bin/rg"
  NXS_GOOS="${NEXUS_DESKTOP_NXS_GOOS:-darwin}"
  NXS_GOARCH="${NEXUS_DESKTOP_NXS_GOARCH:-${TARGET_GOARCH}}"
  if [[ "${NXS_GOARCH}" != "${TARGET_GOARCH}" ]]; then
    echo "bundled nxs architecture ${NXS_GOARCH} does not match app target ${TARGET_GOARCH}" >&2
    exit 1
  fi
  if [[ -n "${NXS_RUNTIME_PATH}" ]]; then
    if [[ ! -x "${NXS_RUNTIME_PATH}" ]]; then
      echo "missing cached nxs runtime: ${NXS_RUNTIME_PATH}" >&2
      exit 1
    fi
    cached_ripgrep_path="$(dirname "${NXS_RUNTIME_PATH}")/rg"
    if [[ ! -x "${cached_ripgrep_path}" ]]; then
      echo "missing cached nxs ripgrep sidecar: ${cached_ripgrep_path}" >&2
      exit 1
    fi
    echo "==> Using cached bundled nxs runtime"
    cp "${NXS_RUNTIME_PATH}" "${nxs_output_path}"
    cp "${cached_ripgrep_path}" "${nxs_ripgrep_output_path}"
  else
    echo "==> Downloading bundled nxs runtime"
    node "${ROOT_DIR}/scripts/desktop/fetch-nxs-runtime.js" \
      --goos "${NXS_GOOS}" \
      --goarch "${NXS_GOARCH}" \
      --output "${nxs_output_path}" \
      --ripgrep-output "${nxs_ripgrep_output_path}"
  fi
fi

require_macho_architecture "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}"
require_macho_architecture "${MACOS_CONTENTS_DIR}/nexus-server"
require_macho_architecture "${RESOURCES_DIR}/bin/nexusctl"
require_macho_architecture "${RESOURCES_DIR}/bin/nexuscfg"
require_macho_architecture "${RESOURCES_DIR}/bin/nexus"
if [[ -x "${RESOURCES_DIR}/bin/nxs" ]]; then
  require_macho_architecture "${RESOURCES_DIR}/bin/nxs"
fi
if [[ -x "${RESOURCES_DIR}/bin/rg" ]]; then
  require_macho_architecture "${RESOURCES_DIR}/bin/rg"
fi

chmod 0755 "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}" \
  "${MACOS_CONTENTS_DIR}/nexus-server" \
  "${RESOURCES_DIR}/bin/nexusctl" \
  "${RESOURCES_DIR}/bin/nexuscfg" \
  "${RESOURCES_DIR}/bin/nexus"
if [[ -f "${RESOURCES_DIR}/bin/nxs" ]]; then
  chmod 0755 "${RESOURCES_DIR}/bin/nxs"
fi
if [[ -f "${RESOURCES_DIR}/bin/rg" ]]; then
  chmod 0755 "${RESOURCES_DIR}/bin/rg"
fi

rsync -a --delete --exclude '.DS_Store' "${ROOT_DIR}/web/dist/" "${RESOURCES_DIR}/Web/"
rsync -a --delete --exclude '.DS_Store' "${ROOT_DIR}/db/migrations/" "${RESOURCES_DIR}/db/migrations/"
rsync -a --delete --exclude '.DS_Store' "${ROOT_DIR}/skills/" "${RESOURCES_DIR}/skills/"

DESKTOP_ENV_PATH="${RESOURCES_DIR}/desktop.env"
rm -f "${DESKTOP_ENV_PATH}"
if [[ -n "${NEXUS_DESKTOP_GITHUB_CLIENT_ID:-}" ]]; then
  {
    printf 'CONNECTOR_GITHUB_CLIENT_ID=%s\n' "${NEXUS_DESKTOP_GITHUB_CLIENT_ID}"
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
  if [[ "${CODESIGN_SIGN_VALUE}" == "-" ]]; then
    echo "==> Applying ad-hoc signature"
  else
    echo "==> Applying code signature: ${CODESIGN_IDENTITY}"
  fi
  codesign_target "${MACOS_CONTENTS_DIR}/nexus-server"
  codesign_target "${RESOURCES_DIR}/bin/nexusctl"
  codesign_target "${RESOURCES_DIR}/bin/nexuscfg"
  codesign_target "${RESOURCES_DIR}/bin/nexus"
  if [[ -x "${RESOURCES_DIR}/bin/nxs" ]]; then
    codesign_target "${RESOURCES_DIR}/bin/nxs"
  fi
  if [[ -x "${RESOURCES_DIR}/bin/rg" ]]; then
    codesign_target "${RESOURCES_DIR}/bin/rg"
  fi
  codesign_target "${MACOS_CONTENTS_DIR}/${EXECUTABLE_NAME}"
  codesign_target "${APP_BUNDLE}"
fi

rm -rf "${SIDECAR_BUILD_DIR}"
rm -f "${APP_BUILD_DIR}/.DS_Store"

echo "==> Built ${APP_BUNDLE}"
