#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP_NAME="${NEXUS_DESKTOP_APP_NAME:-Nexus}"
EXECUTABLE_NAME="${NEXUS_DESKTOP_EXECUTABLE_NAME:-Nexus}"
APP_BUILD_DIR="${NEXUS_DESKTOP_APP_BUILD_DIR:-${ROOT_DIR}/desktop/macos/.build/app}"
APP_BUNDLE="${APP_BUILD_DIR}/${APP_NAME}.app"
APP_EXECUTABLE="${APP_BUNDLE}/Contents/MacOS/${EXECUTABLE_NAME}"
LOG_FILE="${NEXUS_DESKTOP_SMOKE_LOG:-${TMPDIR:-/tmp}/nexus-desktop-smoke.log}"
MAIN_TIMEOUT_SECONDS="${NEXUS_DESKTOP_SMOKE_MAIN_TIMEOUT_SECONDS:-15}"
LAUNCHER_TIMEOUT_SECONDS="${NEXUS_DESKTOP_SMOKE_LAUNCHER_TIMEOUT_SECONDS:-10}"
EXPECTED_CREDENTIALS_STORAGE="${NEXUS_DESKTOP_SMOKE_EXPECTED_CREDENTIALS_STORAGE:-file}"

APP_PID=""

fail() {
  echo "smoke failed: $*" >&2
  if [[ -f "${LOG_FILE}" ]]; then
    echo "--- ${LOG_FILE} tail ---" >&2
    tail -120 "${LOG_FILE}" >&2 || true
  fi
  exit 1
}

cleanup() {
  if [[ -n "${APP_PID}" ]] && kill -0 "${APP_PID}" >/dev/null 2>&1; then
    kill "${APP_PID}" >/dev/null 2>&1 || true
    wait "${APP_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_for_log() {
  local pattern="$1"
  local timeout_seconds="$2"
  local started_at
  started_at="$(date +%s)"

  while true; do
    if grep -Eq "${pattern}" "${LOG_FILE}"; then
      return 0
    fi
    if [[ -n "${APP_PID}" ]] && ! kill -0 "${APP_PID}" >/dev/null 2>&1; then
      fail "app exited before log matched: ${pattern}"
    fi
    if (( "$(date +%s)" - started_at >= timeout_seconds )); then
      fail "timed out waiting for log: ${pattern}"
    fi
    sleep 0.2
  done
}

if [[ ! -x "${APP_EXECUTABLE}" ]]; then
  "${ROOT_DIR}/scripts/desktop/build-macos-app.sh"
fi

if pgrep -x "${EXECUTABLE_NAME}" >/dev/null 2>&1; then
  fail "${EXECUTABLE_NAME} is already running; quit it before smoke testing"
fi

rm -f "${LOG_FILE}"
: > "${LOG_FILE}"

"${APP_EXECUTABLE}" >"${LOG_FILE}" 2>&1 &
APP_PID="$!"

wait_for_log "event=sidecar\\.credentials_key_ready" "${MAIN_TIMEOUT_SECONDS}"
if [[ -n "${EXPECTED_CREDENTIALS_STORAGE}" ]]; then
  wait_for_log "event=sidecar\\.credentials_key_ready.*storage=${EXPECTED_CREDENTIALS_STORAGE}" "${MAIN_TIMEOUT_SECONDS}"
fi
wait_for_log "event=main_window\\.created.*material=windowBackground" "${MAIN_TIMEOUT_SECONDS}"
wait_for_log "event=web\\.ready.*surface=main" "${MAIN_TIMEOUT_SECONDS}"
wait_for_log "event=main_window\\.revealed.*source=web\\.ready" "${MAIN_TIMEOUT_SECONDS}"

open "nexus://launcher"
wait_for_log "event=launcher_window\\.created.*material=popover" "${LAUNCHER_TIMEOUT_SECONDS}"
wait_for_log "event=web\\.ready.*surface=launcher" "${LAUNCHER_TIMEOUT_SECONDS}"
wait_for_log "event=launcher_window\\.revealed.*source=web\\.ready" "${LAUNCHER_TIMEOUT_SECONDS}"

if grep -Eq "source=fallback_timeout|webview\\.content_process_terminated|startup\\.failed" "${LOG_FILE}"; then
  fail "unexpected fallback, WebContent termination, or startup failure"
fi

cleanup
trap - EXIT

sleep 0.5
if pgrep -fl "${APP_BUNDLE}/Contents/MacOS/nexus-server" >/dev/null 2>&1; then
  fail "sidecar process still running after app shutdown"
fi

echo "smoke passed: ${LOG_FILE}"
