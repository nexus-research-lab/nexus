#!/usr/bin/env bash
# Build the current checkout as a separately installed macOS test application.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-build}"
case "${MODE}" in
  build|run|package) ;;
  *) echo "usage: $0 [build|run|package]" >&2; exit 2 ;;
esac
if [[ "$(uname -s)" != Darwin ]]; then
  echo "Nexus Preview currently supports macOS only" >&2
  exit 1
fi

# Identity and output paths are fixed: release environment settings must not
# accidentally turn a preview into the production app. Expand ~ on the target
# machine, not the build machine, so distributed previews keep their own state.
export NEXUS_DESKTOP_APP_NAME='Nexus Preview'
export NEXUS_DESKTOP_EXECUTABLE_NAME=NexusPreview
export NEXUS_DESKTOP_BUNDLE_IDENTIFIER=dev.nexus.preview
export NEXUS_DESKTOP_DEFAULT_STATE_ROOT='~/.nexus-preview'
export NEXUS_DESKTOP_APP_BUILD_DIR="${ROOT_DIR}/desktop/macos/.build/preview"
export NEXUS_DESKTOP_PACKAGE_OUTPUT_DIR="${ROOT_DIR}/desktop/macos/.build/preview-package"
export NEXUS_DESKTOP_REGISTER_URL_SCHEME=0
export NEXUS_DESKTOP_DISABLE_UPDATE_CHECK=1
export NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME=1
export NEXUS_DESKTOP_PACKAGE_SKIP_BUILD=0
export NEXUS_DESKTOP_PACKAGE_FORMAT=zip
unset NEXUS_DESKTOP_PACKAGE_NAME NEXUS_DESKTOP_PACKAGE_METADATA_PATH
# Do not inherit a production or onboarding state override from the shell.
unset NEXUS_DESKTOP_STATE_ROOT NEXUS_DESKTOP_PREFERENCES_SUITE NEXUS_STATE_ROOT NEXUS_PREVIOUS_STATE_ROOT

# Always run the tools packaged with this build. Make exports .env values,
# which may still point to an old CLI or runtime in the production state root.
unset NEXUSCTL_COMMAND_PATH NEXUSCFG_COMMAND_PATH NEXUS_NXS_COMMAND_PATH

# Build local source on every invocation; Go's cache may skip unchanged work,
# but a stale dist/nxs binary is never used as the runtime source.
SDK_ROOT="${NEXUS_PREVIEW_SDK_ROOT:-${ROOT_DIR}/../nexus-agent-sdk-go}"
BRIDGE_ROOT="${NEXUS_PREVIEW_BRIDGE_ROOT:-${ROOT_DIR}/../nexus-agent-sdk-bridge}"
for source_root in "${SDK_ROOT}" "${BRIDGE_ROOT}"; do
  [[ -f "${source_root}/go.mod" ]] || { echo "missing local source: ${source_root}" >&2; exit 1; }
done
SDK_ROOT="$(cd "${SDK_ROOT}" && pwd)"
BRIDGE_ROOT="$(cd "${BRIDGE_ROOT}" && pwd)"
DEPS_DIR="${NEXUS_DESKTOP_APP_BUILD_DIR}/local-sources"
mkdir -p "${DEPS_DIR}/bin"
export NEXUS_DESKTOP_GO_WORK="${DEPS_DIR}/go.work"
export NEXUS_DESKTOP_SOURCE_MANIFEST="${DEPS_DIR}/preview-sources.json"
python3 - "${ROOT_DIR}" "${SDK_ROOT}" "${BRIDGE_ROOT}" "${DEPS_DIR}" <<'PYTHON'
import json, pathlib, subprocess, sys
host, sdk, bridge, output = map(pathlib.Path, sys.argv[1:])
def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()
records = {}
for name, root in [('nexus', host), ('sdk', sdk), ('bridge', bridge)]:
    records[name] = dict(path=str(root), commit=git(root, 'rev-parse', 'HEAD'),
                         branch=git(root, 'branch', '--show-current') or None,
                         dirty=bool(git(root, 'status', '--porcelain')))
    print(f"Preview source {name}: {records[name]['commit']} (dirty={records[name]['dirty']})", flush=True)
(output / 'preview-sources.json').write_text(json.dumps(records, indent=2) + '\n')
version = subprocess.check_output(['go', 'env', 'GOVERSION'], text=True).strip().removeprefix('go')
(output / 'go.work').write_text(f'go {version}\n\nuse (\n  {json.dumps(str(host))}\n  {json.dumps(str(bridge))}\n)\n')
PYTHON
case "${NEXUS_DESKTOP_TARGET_ARCH:-$(uname -m)}" in
  arm64|aarch64) SDK_ARCH=arm64 ;;
  x86_64|amd64|intel) SDK_ARCH=amd64 ;;
  *) echo "unsupported Preview architecture" >&2; exit 1 ;;
esac
RG_PATH="${NEXUS_PREVIEW_RIPGREP_PATH:-$(command -v rg || true)}"
[[ -x "${RG_PATH}" ]] || { echo "Preview needs a local rg executable" >&2; exit 1; }
(
  cd "${SDK_ROOT}"
  GOWORK=off CGO_ENABLED=0 GOOS=darwin GOARCH="${SDK_ARCH}" go build -trimpath -ldflags='-s -w' -o "${DEPS_DIR}/bin/nxs" ./cmd/nxs
)
rm -f "${DEPS_DIR}/bin/rg"
cp "${RG_PATH}" "${DEPS_DIR}/bin/rg"
chmod 0755 "${DEPS_DIR}/bin/rg"
export NEXUS_DESKTOP_NXS_RUNTIME_PATH="${DEPS_DIR}/bin/nxs"

case "${MODE}" in
  build) exec "${ROOT_DIR}/scripts/desktop/build-macos-app.sh" ;;
  run) exec "${ROOT_DIR}/scripts/desktop/run-macos-app.sh" ;;
  package) exec "${ROOT_DIR}/scripts/desktop/package-macos-app.sh" ;;
esac
