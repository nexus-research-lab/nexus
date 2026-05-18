#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RAW_VERSION="${1:-${NEXUS_RELEASE_VERSION:-}}"

if [[ -z "${RAW_VERSION}" ]]; then
  RAW_VERSION="$(cd "${ROOT_DIR}/web" && node -p "require('./package.json').version")"
fi

VERSION="${RAW_VERSION#v}"
TAG="v${VERSION}"

if [[ -n "${NEXUS_RELEASE_TARGET:-}" ]]; then
  TARGET="${NEXUS_RELEASE_TARGET}"
  TARGET_GOOS="${TARGET%%-*}"
  TARGET_GOARCH="${TARGET#*-}"
else
  TARGET_GOOS="${GOOS:-$(go env GOOS)}"
  TARGET_GOARCH="${GOARCH:-$(go env GOARCH)}"
  TARGET="${TARGET_GOOS}-${TARGET_GOARCH}"
fi

if [[ "${TARGET_GOOS}" == "${TARGET_GOARCH}" || -z "${TARGET_GOOS}" || -z "${TARGET_GOARCH}" ]]; then
  echo "invalid release target: ${TARGET}" >&2
  exit 1
fi

EXE_SUFFIX=""
if [[ "${TARGET_GOOS}" == "windows" ]]; then
  EXE_SUFFIX=".exe"
fi

OUTPUT_DIR="${NEXUS_RELEASE_OUTPUT_DIR:-${ROOT_DIR}/dist/release}"
WORK_DIR="${NEXUS_RELEASE_WORK_DIR:-${ROOT_DIR}/dist/release-work}"
DIST_NAME="nexus-${TAG}-${TARGET}"
STAGE_DIR="${WORK_DIR}/${DIST_NAME}"
ARCHIVE_EXT="tar.gz"
if [[ "${TARGET_GOOS}" == "windows" ]]; then
  ARCHIVE_EXT="zip"
fi
ARCHIVE_PATH="${OUTPUT_DIR}/${DIST_NAME}.${ARCHIVE_EXT}"
SHA256_PATH="${ARCHIVE_PATH}.sha256"
BUILD_CGO_ENABLED="${CGO_ENABLED:-0}"

echo "==> Checking Go dependencies"
(cd "${ROOT_DIR}" && GIT_TERMINAL_PROMPT=0 go mod download)

if [[ "${NEXUS_RELEASE_SKIP_PNPM_INSTALL:-0}" != "1" ]]; then
  echo "==> Installing frontend dependencies"
  (cd "${ROOT_DIR}/web" && pnpm install --frozen-lockfile)
fi

if [[ "${NEXUS_RELEASE_SKIP_WEB_BUILD:-0}" != "1" ]]; then
  echo "==> Building web/dist"
  (cd "${ROOT_DIR}/web" && pnpm run build)
fi

rm -rf "${STAGE_DIR}" "${ARCHIVE_PATH}" "${SHA256_PATH}"
mkdir -p "${STAGE_DIR}/bin" "${STAGE_DIR}/web" "${OUTPUT_DIR}"

build_binary() {
  local name="$1"
  local package_path="$2"
  local output_path="${STAGE_DIR}/bin/${name}${EXE_SUFFIX}"

  echo "==> Building ${name} (${TARGET}, CGO_ENABLED=${BUILD_CGO_ENABLED})"
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED="${BUILD_CGO_ENABLED}" GOOS="${TARGET_GOOS}" GOARCH="${TARGET_GOARCH}" \
      go build -trimpath -ldflags="-s -w" -o "${output_path}" "${package_path}"
  )
}

build_binary "nexus-server" "./cmd/nexus-server"
build_binary "nexus-migrate" "./cmd/nexus-migrate"
build_binary "nexusctl" "./cmd/nexusctl"

echo "==> Staging runtime files"
rsync -a --delete --exclude ".DS_Store" "${ROOT_DIR}/db/" "${STAGE_DIR}/db/"
rsync -a --delete --exclude ".DS_Store" "${ROOT_DIR}/skills/" "${STAGE_DIR}/skills/"
rsync -a --delete --exclude ".DS_Store" "${ROOT_DIR}/web/dist/" "${STAGE_DIR}/web/dist/"
if [[ -d "${ROOT_DIR}/docs/image" ]]; then
  mkdir -p "${STAGE_DIR}/docs/image"
  rsync -a --delete --exclude ".DS_Store" "${ROOT_DIR}/docs/image/" "${STAGE_DIR}/docs/image/"
fi
cp "${ROOT_DIR}/README.md" "${STAGE_DIR}/README.md"
if [[ -f "${ROOT_DIR}/README_zh.md" ]]; then
  cp "${ROOT_DIR}/README_zh.md" "${STAGE_DIR}/README_zh.md"
fi
cp "${ROOT_DIR}/CHANGELOG.md" "${STAGE_DIR}/CHANGELOG.md"

SERVER_BINARY="nexus-server${EXE_SUFFIX}"
cat > "${STAGE_DIR}/run-nexus" <<RUN_SCRIPT
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
PORT="\${PORT:-8010}"
DEFAULT_ORIGIN="http://localhost:\${PORT}"

export NEXUS_APP_ROOT="\${NEXUS_APP_ROOT:-\${ROOT_DIR}}"
export WEB_DIST_DIR="\${WEB_DIST_DIR:-\${ROOT_DIR}/web/dist}"
export CONNECTOR_OAUTH_REDIRECT_URI="\${CONNECTOR_OAUTH_REDIRECT_URI:-\${DEFAULT_ORIGIN}/capability/connectors/oauth/callback}"
export CONNECTOR_OAUTH_ALLOWED_ORIGINS="\${CONNECTOR_OAUTH_ALLOWED_ORIGINS:-\${DEFAULT_ORIGIN}}"

exec "\${ROOT_DIR}/bin/${SERVER_BINARY}" "\$@"
RUN_SCRIPT
chmod +x "${STAGE_DIR}/run-nexus"

if [[ "${TARGET_GOOS}" == "windows" ]]; then
  cat > "${STAGE_DIR}/run-nexus.cmd" <<'RUN_SCRIPT'
@echo off
setlocal

set "ROOT_DIR=%~dp0"
if "%PORT%"=="" set "PORT=8010"
if "%NEXUS_APP_ROOT%"=="" set "NEXUS_APP_ROOT=%ROOT_DIR:~0,-1%"
if "%WEB_DIST_DIR%"=="" set "WEB_DIST_DIR=%ROOT_DIR%web\dist"
if "%CONNECTOR_OAUTH_REDIRECT_URI%"=="" set "CONNECTOR_OAUTH_REDIRECT_URI=http://localhost:%PORT%/capability/connectors/oauth/callback"
if "%CONNECTOR_OAUTH_ALLOWED_ORIGINS%"=="" set "CONNECTOR_OAUTH_ALLOWED_ORIGINS=http://localhost:%PORT%"

"%ROOT_DIR%bin\nexus-server.exe" %*
RUN_SCRIPT
fi

START_COMMAND="./run-nexus"
if [[ "${TARGET_GOOS}" == "windows" ]]; then
  START_COMMAND="run-nexus.cmd"
fi

cat > "${STAGE_DIR}/RELEASE.md" <<RELEASE_DOC
# Nexus ${TAG}

启动：

\`\`\`bash
${START_COMMAND}
\`\`\`

默认 Web UI: http://localhost:8010
默认 API 前缀: /nexus/v1

这个包包含服务端、nexusctl、数据库迁移、内置技能与前端资源，不包含 macOS 桌面 app。
RELEASE_DOC

echo "==> Creating archive"
if [[ "${ARCHIVE_EXT}" == "zip" ]]; then
  (cd "${WORK_DIR}" && zip -qr "${ARCHIVE_PATH}" "${DIST_NAME}" -x "*.DS_Store" "*/._*")
else
  tar -czf "${ARCHIVE_PATH}" -C "${WORK_DIR}" "${DIST_NAME}"
fi

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "${ARCHIVE_PATH}" > "${SHA256_PATH}"
else
  shasum -a 256 "${ARCHIVE_PATH}" > "${SHA256_PATH}"
fi

echo "release package: ${ARCHIVE_PATH}"
echo "sha256: ${SHA256_PATH}"
