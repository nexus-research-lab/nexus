#!/usr/bin/env sh
set -eu

raw_release="${1:-${NEXUS_NXS_RUNTIME_RELEASE:-nxs-stable}}"
case "${raw_release}" in
  "" | "stable" | "nxs-stable")
    release="nxs-stable"
    ;;
  nxs-*)
    printf '%s\n' "${raw_release}"
    exit 0
    ;;
  v*)
    printf 'nxs-%s\n' "${raw_release}"
    exit 0
    ;;
  *)
    printf 'nxs-v%s\n' "${raw_release}"
    exit 0
    ;;
esac

repo="${NEXUS_NXS_RUNTIME_RELEASE_REPO:-${NEXUS_DESKTOP_NXS_RELEASE_REPO:-nexus-research-lab/nexus-agent-sdk-bridge}}"
manifest_url="${NEXUS_NXS_RUNTIME_MANIFEST_URL:-${NEXUS_DESKTOP_NXS_MANIFEST_URL:-https://github.com/${repo}/releases/download/${release}/nxs-manifest.json}}"
manifest="$(curl -fsSL --connect-timeout 10 --max-time 30 "${manifest_url}")"
resolved="$(printf '%s\n' "${manifest}" | sed -n 's/.*"release_tag"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"

if [ -z "${resolved}" ]; then
  echo "failed to resolve nxs runtime release from ${manifest_url}" >&2
  exit 1
fi

printf '%s\n' "${resolved}"
