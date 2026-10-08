#!/usr/bin/env bash
# INPUT: Actual assembled App and evidence output path.
# OUTPUT: A real sidecar/nxs compatibility report; no user state or model calls.
# POS: Mandatory bundled-runtime package gate, including skip-build packaging.
set -euo pipefail

app_bundle="${1:?App path is required}"
report_path="${2:?Report path is required}"
sidecar="${app_bundle}/Contents/MacOS/nexus-server"
runtime="${app_bundle}/Contents/Resources/bin/nxs"
[[ -x "${sidecar}" && -x "${runtime}" ]] || { echo "missing bundled sidecar or nxs" >&2; exit 1; }

check_root="$(mktemp -d "${TMPDIR:-/tmp}/nexus-package-runtime.XXXXXX")"
trap 'rm -rf "${check_root}"' EXIT
mkdir -p "${check_root}/home" "$(dirname "${report_path}")"
echo "==> Checking the bundled App/runtime compatibility"
(
  cd "${check_root}"
  env -i PATH=/usr/bin:/bin:/usr/sbin:/sbin HOME="${check_root}/home" TMPDIR="${check_root}" \
    "${sidecar}" check-desktop-runtime --nxs "${runtime}" > "${check_root}/report.json"
)
mv "${check_root}/report.json" "${report_path}"
