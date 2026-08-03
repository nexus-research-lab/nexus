#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-"$ROOT/dist/kami.zip"}"
case "$OUT" in
  /*) ;;
  *) OUT="$ROOT/$OUT" ;;
esac
PACKAGE_ROOT_NAME="${KAMI_PACKAGE_ROOT_NAME:-kami}"
PACKAGE_MAX_BYTES="${KAMI_PACKAGE_MAX_BYTES:-6000000}"
PACKAGE_FORBIDDEN_RE='^(\.agents/|\.claude/|\.claude-plugin/|\.github/|plugins/|assets/(showcase|demos|examples|illustrations)/|assets/images/[123]\.png$|assets/fonts/TsangerJinKai02-W0[45]\.ttf$|assets/fonts/SourceHanSerifKR-(Regular|Medium)\.otf$|dist/|index(-[^/]+)?\.html$|styles\.css$|llms\.txt$|robots\.txt$|sitemap\.xml$|vercel\.json$|AGENTS\.md$|CLAUDE\.md$|README\.md$|\.gitignore$|scripts/(build_metadata|draft-release-notes|package-skill)\.py$|scripts/package-skill\.sh$|scripts/tests/)'
PACKAGE_REQUIRED_ENTRIES=(
  "SKILL.md"
  "CHEATSHEET.md"
  "VERSION"
  "LICENSE"
  "assets/images/logo.svg"
  "assets/fonts/JetBrainsMono.woff2"
  "assets/templates/resume.html"
  "assets/templates/landing-page.html"
  "assets/diagrams/sequence.html"
  "references/design.md"
  "scripts/build.py"
  "scripts/ensure-fonts.sh"
  "scripts/site_facts.py"
  "scripts/content.py"
  "scripts/visual.py"
  "scripts/mcp_server.py"
  "references/schemas/resume.json"
)

mkdir -p "$(dirname "$OUT")"
rm -f "$OUT"

cd "$ROOT"

MANIFEST="$(mktemp)"
FILTERED_MANIFEST="$(mktemp)"
ZIP_MANIFEST="$(mktemp)"
STAGING="$(mktemp -d)"
trap 'rm -f "$MANIFEST" "$FILTERED_MANIFEST" "$ZIP_MANIFEST"; rm -rf "$STAGING"' EXIT

git ls-files > "$MANIFEST"
awk '
  /(^|\/)__pycache__\// { next }
  /\.pyc$/ { next }
  /(^|\/)\.DS_Store$/ { next }
  /^(SKILL\.md|CHEATSHEET\.md|VERSION|LICENSE)$/ { print; next }
  /^assets\/templates\// { print; next }
  /^assets\/diagrams\// { print; next }
  /^assets\/images\/logo\.svg$/ { print; next }
  /^assets\/fonts\/JetBrainsMono\.woff2$/ { print; next }
  /^assets\/fonts\/LICENSE-SourceHanSerifK\.txt$/ { print; next }
  /^references\// { print; next }
  /^scripts\/(build|check-update|checks|content|ensure-fonts|highlight|lint|mcp_server|mermaid_normalize|optional_deps|render|shared|site_facts|tokens|verify|visual)\.(py|sh)$/ { print; next }
' "$MANIFEST" > "$FILTERED_MANIFEST"

# Coverage gate: every tracked scripts/ file must be either packaged by the
# allowlist above or named in the repo-only exclusion below. Without this, a
# new runtime module that build.py imports would silently miss the zip and
# the installed skill would ImportError while every local check stays green.
SCRIPTS_REPO_ONLY_RE='^scripts/(build_metadata\.py|draft-release-notes\.py|package-skill\.sh|tests/)'
unaccounted="$(grep '^scripts/' "$MANIFEST" \
  | grep -Ev "$SCRIPTS_REPO_ONLY_RE" \
  | grep -Fvx -f <(grep '^scripts/' "$FILTERED_MANIFEST" || true) || true)"
if [ -n "$unaccounted" ]; then
  echo "ERROR: tracked scripts neither packaged nor listed as repo-only:" >&2
  printf '%s\n' "$unaccounted" >&2
  echo "Add them to the packaging allowlist or to SCRIPTS_REPO_ONLY_RE." >&2
  exit 1
fi

while IFS= read -r entry; do
  dest="$STAGING/$PACKAGE_ROOT_NAME/$entry"
  mkdir -p "$(dirname "$dest")"
  cp -p "$entry" "$dest"
done < "$FILTERED_MANIFEST"

(
  cd "$STAGING"
  find "$PACKAGE_ROOT_NAME" -type f | sort > "$ZIP_MANIFEST"
  zip -X -q "$OUT" -@ < "$ZIP_MANIFEST"
)

entries="$(zipinfo -1 "$OUT")"
bad_root="$(printf '%s\n' "$entries" | awk -v prefix="${PACKAGE_ROOT_NAME}/" 'index($0, prefix) != 1 { print }')"
if [ -n "$bad_root" ]; then
  echo "ERROR: package entries must live under ${PACKAGE_ROOT_NAME}/:" >&2
  printf '%s\n' "$bad_root" >&2
  exit 1
fi

stripped_entries="$(printf '%s\n' "$entries" | sed "s#^${PACKAGE_ROOT_NAME}/##")"
if forbidden_entries="$(printf '%s\n' "$stripped_entries" | grep -E "$PACKAGE_FORBIDDEN_RE")"; then
  echo "ERROR: disallowed package entry found in $OUT:" >&2
  printf '%s\n' "$forbidden_entries" >&2
  exit 1
fi

for required in "${PACKAGE_REQUIRED_ENTRIES[@]}"; do
  if ! printf '%s\n' "$entries" | grep -Fxq "${PACKAGE_ROOT_NAME}/${required}"; then
    echo "ERROR: required package entry missing from $OUT: ${PACKAGE_ROOT_NAME}/${required}" >&2
    exit 1
  fi
done

size_bytes="$(wc -c < "$OUT" | tr -d '[:space:]')"
if (( size_bytes > PACKAGE_MAX_BYTES )); then
  echo "ERROR: package exceeds ${PACKAGE_MAX_BYTES} bytes: ${size_bytes} bytes" >&2
  exit 1
fi

echo "OK: package audit passed (${size_bytes} bytes, limit ${PACKAGE_MAX_BYTES})"
echo "OK: wrote $OUT"
