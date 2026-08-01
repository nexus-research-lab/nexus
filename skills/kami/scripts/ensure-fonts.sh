#!/usr/bin/env bash
set -euo pipefail

# Portable across bash 3.2+ (macOS stock /bin/bash) and bash 4+ (Linux, Homebrew).
# Avoids `declare -A` so the script runs on a fresh macOS without `brew install bash`.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_FONT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)/assets/fonts"

# Download target lives OUTSIDE the skill directory on purpose.
#
# Claude Desktop skill ZIPs exclude the large bundled fonts (TsangerJinKai TTFs,
# Source Han Serif K OTFs). The old code downloaded them back into the skill's
# own assets/fonts, which pushed the installed skill past Claude Desktop's size
# limit ("upload/execution too big"). We instead drop them in the XDG user font
# dir, which fontconfig scans by default on both macOS (Homebrew) and Linux, yet
# does NOT show up in macOS Font Book. WeasyPrint then resolves "TsangerJinKai02"
# / "Source Han Serif K" from here when the template's relative @font-face path
# is absent; online renders still fall back to the jsDelivr URL baked alongside
# each @font-face declaration.
FONT_DIR="${KAMI_FONT_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/fonts/kami}"

# Partial downloads from an interrupted run must not linger in FONT_DIR, and
# two concurrent runs must not fight over one temp path: temp files carry this
# run's PID and are swept on exit.
TMP_SUFFIX="tmp.$$"
cleanup_tmp() { rm -f "$FONT_DIR"/*."$TMP_SUFFIX" 2>/dev/null || true; }
trap cleanup_tmp EXIT

MIN_SIZE_CN=10000000  # 10MB for TsangerJinKai (large CJK glyph set)
MIN_SIZE_KO=6500000   # 6.5MB for Source Han Serif K (Adobe full subset)

# TsangerJinKai (CN): index N pairs CN_NAMES[N] with CN_LOCAL_NAMES[N].
CN_NAMES=("仓耳今楷02-W04.ttf" "仓耳今楷02-W05.ttf")
CN_LOCAL_NAMES=("TsangerJinKai02-W04.ttf" "TsangerJinKai02-W05.ttf")

# Source Han Serif K (KO): mirror filenames match the repo filenames, so there
# is no rename step (unlike Tsanger's Chinese-named official downloads).
KO_NAMES=("SourceHanSerifKR-Regular.otf" "SourceHanSerifKR-Medium.otf")

# Mirror order is intentionally jsdmirror-first here, opposite of the
# templates' @font-face fallback (which lists jsdelivr first). Reasoning:
# this script runs interactively when fonts are missing locally, often from
# China where jsdmirror is reachable and faster than jsdelivr; templates run
# anywhere and prioritize jsdelivr's broader global coverage.
MIRROR_SOURCES=(
  "https://cdn.jsdmirror.com/gh/tw93/Kami@main/assets/fonts"
  "https://cdn.jsdelivr.net/gh/tw93/Kami@main/assets/fonts"
)

check_size() {
  local file="$1"
  local min_size="$2"
  [[ -f "$file" ]] || return 1
  local size
  size=$(wc -c < "$file" | tr -d ' ')
  [[ "$size" -ge "$min_size" ]]
}

cn_present_in() {
  local dir="$1" name
  for name in "${CN_LOCAL_NAMES[@]}"; do
    check_size "$dir/$name" "$MIN_SIZE_CN" || return 1
  done
  return 0
}

ko_present_in() {
  local dir="$1" name
  for name in "${KO_NAMES[@]}"; do
    check_size "$dir/$name" "$MIN_SIZE_KO" || return 1
  done
  return 0
}

refresh_fontconfig() {
  # The XDG font dir is already on fontconfig's default scan path, so a cache
  # refresh is all that is needed for WeasyPrint to pick the fonts up. Optional:
  # absence of fc-cache (e.g. minimal sandbox) is non-fatal, fontconfig rescans
  # the directory lazily on next use.
  if command -v fc-cache >/dev/null 2>&1; then
    fc-cache -f "$FONT_DIR" >/dev/null 2>&1 || true
  fi
}

download_tsanger() {
  local cn_name="$1"
  local local_name="$2"
  local target="$FONT_DIR/$local_name"

  # Source 1: official tsanger.cn
  local official_url="https://tsanger.cn/download/${cn_name}"
  echo "  Trying: tsanger.cn (official)"
  if curl --retry 2 --connect-timeout 15 --max-time 300 -fSL "$official_url" -o "$target.$TMP_SUFFIX" 2>/dev/null; then
    if check_size "$target.$TMP_SUFFIX" "$MIN_SIZE_CN"; then
      mv "$target.$TMP_SUFFIX" "$target"
      echo "  OK: $local_name downloaded ($(du -h "$target" | cut -f1))"
      return 0
    else
      rm -f "$target.$TMP_SUFFIX"
    fi
  else
    rm -f "$target.$TMP_SUFFIX"
  fi

  # Source 2+: CDN mirrors (already named TsangerJinKai02-W0x.ttf)
  for src in "${MIRROR_SOURCES[@]}"; do
    local url="$src/$local_name"
    echo "  Trying: $url"
    if curl --retry 2 --connect-timeout 15 --max-time 300 -fSL "$url" -o "$target.$TMP_SUFFIX" 2>/dev/null; then
      if check_size "$target.$TMP_SUFFIX" "$MIN_SIZE_CN"; then
        mv "$target.$TMP_SUFFIX" "$target"
        echo "  OK: $local_name downloaded ($(du -h "$target" | cut -f1))"
        return 0
      else
        rm -f "$target.$TMP_SUFFIX"
      fi
    else
      rm -f "$target.$TMP_SUFFIX"
    fi
  done

  echo "  ERROR: all sources failed for $local_name"
  return 1
}

download_ko_serif() {
  local local_name="$1"
  local target="$FONT_DIR/$local_name"

  # CDN mirrors only: Source Han Serif K has no single official direct-download
  # URL, so we serve the committed OTFs from the same jsDelivr/jsdmirror gh path.
  for src in "${MIRROR_SOURCES[@]}"; do
    local url="$src/$local_name"
    echo "  Trying: $url"
    if curl --retry 2 --connect-timeout 15 --max-time 300 -fSL "$url" -o "$target.$TMP_SUFFIX" 2>/dev/null; then
      if check_size "$target.$TMP_SUFFIX" "$MIN_SIZE_KO"; then
        mv "$target.$TMP_SUFFIX" "$target"
        echo "  OK: $local_name downloaded ($(du -h "$target" | cut -f1))"
        return 0
      else
        rm -f "$target.$TMP_SUFFIX"
      fi
    else
      rm -f "$target.$TMP_SUFFIX"
    fi
  done

  echo "  ERROR: all sources failed for $local_name"
  return 1
}

# A repo checkout ships the committed font files. Templates resolve their
# relative `../fonts/*` @font-face path against them directly, so there is
# nothing to download or register. These branches are skipped inside a Claude
# Desktop skill, whose assets/fonts has the large fonts stripped out.

cn_failed=0
if cn_present_in "$REPO_FONT_DIR"; then
  echo "OK: TsangerJinKai fonts present in repo checkout ($REPO_FONT_DIR)"
else
  mkdir -p "$FONT_DIR"
  if cn_present_in "$FONT_DIR"; then
    echo "OK: TsangerJinKai fonts present ($FONT_DIR)"
  else
    echo "Downloading TsangerJinKai fonts to $FONT_DIR ..."
    for i in "${!CN_NAMES[@]}"; do
      cn_name="${CN_NAMES[$i]}"
      local_name="${CN_LOCAL_NAMES[$i]}"
      if check_size "$FONT_DIR/$local_name" "$MIN_SIZE_CN"; then
        echo "  OK: $local_name already present"
        continue
      fi
      if ! download_tsanger "$cn_name" "$local_name"; then
        cn_failed=$((cn_failed + 1))
      fi
    done
    if [[ "$cn_failed" -gt 0 ]]; then
      echo ""
      echo "Some TsangerJinKai files could not be downloaded. Alternatives:"
      echo "  1. Install Source Han Serif SC: brew install --cask font-source-han-serif-sc"
      echo "  2. Copy TsangerJinKai02-W04.ttf and W05.ttf manually into $FONT_DIR"
      # Don't exit yet, try the KO recovery too so a Korean-only user still gets KO fonts.
    fi
  fi
fi

ko_failed=0
if ko_present_in "$REPO_FONT_DIR"; then
  echo "OK: Source Han Serif K fonts present in repo checkout ($REPO_FONT_DIR)"
else
  mkdir -p "$FONT_DIR"
  if ko_present_in "$FONT_DIR"; then
    echo "OK: Source Han Serif K fonts present ($FONT_DIR)"
  else
    echo "Downloading Source Han Serif K fonts to $FONT_DIR ..."
    for local_name in "${KO_NAMES[@]}"; do
      if check_size "$FONT_DIR/$local_name" "$MIN_SIZE_KO"; then
        echo "  OK: $local_name already present"
        continue
      fi
      if ! download_ko_serif "$local_name"; then
        ko_failed=$((ko_failed + 1))
      fi
    done
    if [[ "$ko_failed" -gt 0 ]]; then
      echo ""
      echo "Some Source Han Serif K files could not be downloaded. Alternatives:"
      echo "  1. Download from https://github.com/adobe-fonts/source-han-serif/releases"
      echo "  2. Copy SourceHanSerifKR-Regular.otf and -Medium.otf manually into $FONT_DIR"
    fi
  fi
fi

if [[ "$cn_failed" -gt 0 || "$ko_failed" -gt 0 ]]; then
  exit 1
fi

refresh_fontconfig
echo "OK: all fonts ready"
