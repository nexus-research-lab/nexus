#!/usr/bin/env bash
# INPUT: One notarized acceptance DMG and metadata in an isolated output directory.
# OUTPUT: Signature/Gatekeeper logs and installed bundled-runtime handshake evidence.
# POS: Fresh runner package copy; not user-data upgrade, rollback or interactive UI acceptance.
set -euo pipefail

output_dir="${1:?usage: check-macos-signed-install.sh /absolute/artifact-directory}"
[[ "${output_dir}" = /* && -d "${output_dir}" ]]
shopt -s nullglob
packages=("${output_dir}"/*.dmg)
[[ ${#packages[@]} -eq 1 ]]
package_path="${packages[0]}"
(cd "${output_dir}" && shasum -a 256 -c "$(basename "${package_path}").sha256")
node - "${package_path}.metadata.json" <<'NODE'
const fs = require('node:fs');
const metadata = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
if (metadata.source?.dirty !== false || metadata.signing?.developer_id !== true || metadata.signing?.notarized !== true) {
  throw new Error('Acceptance requires clean source, Developer ID and notarization metadata');
}
NODE

install_root="$(mktemp -d "${TMPDIR:-/tmp}/nexus-signed-install.XXXXXX")"
mount_path="${install_root}/volume"
mkdir -p "${mount_path}"
# Only the disk attached by this invocation is detached; retain its copied App for runner diagnostics.
mounted=0
cleanup() {
  local status=$?
  if [[ "${mounted}" == "1" ]] && ! hdiutil detach "${mount_path}" >> "${output_dir}/mount.log" 2>&1; then
    echo "Acceptance DMG cleanup failed: ${mount_path}" >&2
    exit 1
  fi
  exit "${status}"
}
trap cleanup EXIT
hdiutil attach -readonly -nobrowse -mountpoint "${mount_path}" "${package_path}" > "${output_dir}/mount.log"
mounted=1
[[ -d "${mount_path}/Nexus.app" ]]
ditto "${mount_path}/Nexus.app" "${install_root}/Nexus.app"
app_path="${install_root}/Nexus.app"
xattr -w com.apple.quarantine "0083;$(printf '%x' "$(date +%s)");NexusSandboxAcceptance;" "${app_path}"
xattr -p com.apple.quarantine "${app_path}" > "${output_dir}/quarantine.log"
codesign --verify --deep --strict --verbose=4 "${app_path}" > "${output_dir}/codesign-verify.log" 2>&1
xcrun stapler validate "${app_path}" > "${output_dir}/stapler-validate.log" 2>&1
spctl --assess --type execute --verbose=4 "${app_path}" > "${output_dir}/gatekeeper.log" 2>&1
"${app_path}/Contents/MacOS/nexus-server" check-desktop-runtime \
  --nxs "${app_path}/Contents/Resources/bin/nxs" > "${output_dir}/signed-install-check.json"
