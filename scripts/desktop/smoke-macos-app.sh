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
MAIN_URL_TIMEOUT_SECONDS="${NEXUS_DESKTOP_SMOKE_MAIN_URL_TIMEOUT_SECONDS:-3}"
LAUNCHER_URL_TIMEOUT_SECONDS="${NEXUS_DESKTOP_SMOKE_LAUNCHER_URL_TIMEOUT_SECONDS:-3}"
EXPECTED_CREDENTIALS_STORAGE="${NEXUS_DESKTOP_SMOKE_EXPECTED_CREDENTIALS_STORAGE:-file}"
EXPECT_NXS_RUNTIME="${NEXUS_DESKTOP_SMOKE_EXPECT_NXS_RUNTIME:-0}"
ALLOW_FALLBACK="${NEXUS_DESKTOP_SMOKE_ALLOW_FALLBACK:-0}"
MAIN_READY_ROUTE_PATTERN="event=web\\.ready.*location_path=/launcher .*surface=main"
LAUNCHER_ROUTE_PATTERN="/launcher($|[[:space:]])"

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

wait_for_log_match() {
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
      return 1
    fi
    sleep 0.2
  done
}

wait_for_log() {
  local pattern="$1"
  local timeout_seconds="$2"
  if ! wait_for_log_match "${pattern}" "${timeout_seconds}"; then
    fail "timed out waiting for log: ${pattern}"
  fi
}

register_bundle_url_scheme() {
  local register_tool="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
  if [[ -x "${register_tool}" ]]; then
    "${register_tool}" -f "${APP_BUNDLE}" >/dev/null 2>&1 || true
  fi
}

post_launcher_notification() {
  swift -e 'import Foundation; DistributedNotificationCenter.default().postNotificationName(Notification.Name("com.leemysw.nexus.showLauncher"), object: nil, userInfo: nil, deliverImmediately: true); RunLoop.current.run(until: Date(timeIntervalSinceNow: 0.2))' >/dev/null 2>&1
}

post_main_window_notification() {
  swift -e 'import Foundation; DistributedNotificationCenter.default().postNotificationName(Notification.Name("com.leemysw.nexus.showMainWindow"), object: nil, userInfo: nil, deliverImmediately: true); RunLoop.current.run(until: Date(timeIntervalSinceNow: 0.2))' >/dev/null 2>&1
}

if [[ ! -x "${APP_EXECUTABLE}" ]]; then
  "${ROOT_DIR}/scripts/desktop/build-macos-app.sh"
fi

NEXUSCTL_EXECUTABLE="${APP_BUNDLE}/Contents/Resources/bin/nexusctl"
if [[ ! -x "${NEXUSCTL_EXECUTABLE}" ]]; then
  fail "missing bundled nexusctl executable: ${NEXUSCTL_EXECUTABLE}"
fi

if ! "${NEXUSCTL_EXECUTABLE}" --help >/dev/null 2>&1; then
  fail "bundled nexusctl --help failed"
fi

NXS_EXECUTABLE="${APP_BUNDLE}/Contents/Resources/bin/nxs"
if [[ "${EXPECT_NXS_RUNTIME}" == "1" ]]; then
  if [[ ! -x "${NXS_EXECUTABLE}" ]]; then
    fail "missing bundled nxs executable: ${NXS_EXECUTABLE}"
  fi
  if ! "${NXS_EXECUTABLE}" --version >/dev/null 2>&1; then
    fail "bundled nxs --version failed"
  fi
fi

if pgrep -x "${EXECUTABLE_NAME}" >/dev/null 2>&1; then
  fail "${EXECUTABLE_NAME} is already running; quit it before smoke testing"
fi

register_bundle_url_scheme

rm -f "${LOG_FILE}"
: > "${LOG_FILE}"

NEXUS_DESKTOP_DISABLE_UPDATE_CHECK=1 "${APP_EXECUTABLE}" >"${LOG_FILE}" 2>&1 &
APP_PID="$!"

wait_for_log "event=sidecar\\.credentials_key_ready" "${MAIN_TIMEOUT_SECONDS}"
if [[ -n "${EXPECTED_CREDENTIALS_STORAGE}" ]]; then
  wait_for_log "event=sidecar\\.credentials_key_ready.*storage=${EXPECTED_CREDENTIALS_STORAGE}" "${MAIN_TIMEOUT_SECONDS}"
fi

wait_for_log "event=main_window\\.created.*material=windowBackground" "${MAIN_TIMEOUT_SECONDS}"
wait_for_log "${MAIN_READY_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
if [[ "${ALLOW_FALLBACK}" == "1" ]]; then
  wait_for_log "event=main_window\\.revealed.*source=(web\\.ready|fallback_timeout)" "${MAIN_TIMEOUT_SECONDS}"
else
  wait_for_log "event=main_window\\.revealed.*source=web\\.ready" "${MAIN_TIMEOUT_SECONDS}"
fi

if open "nexus://open" >/dev/null 2>&1 &&
  wait_for_log_match "event=app\\.url_route.*host=open .*route_path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_URL_TIMEOUT_SECONDS}"; then
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
else
  post_main_window_notification || fail "failed to request launcher route through nexus://open"
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
fi
wait_for_log "${MAIN_READY_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"

if open "nexus://launcher" >/dev/null 2>&1 &&
  wait_for_log_match "event=app\\.url_route.*host=launcher .*route_path=${LAUNCHER_ROUTE_PATTERN}" "${LAUNCHER_URL_TIMEOUT_SECONDS}"; then
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
else
  post_launcher_notification || fail "failed to request launcher route"
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
fi
wait_for_log "${MAIN_READY_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"

unexpected_pattern="webview\\.content_process_terminated|startup\\.failed"
if [[ "${ALLOW_FALLBACK}" != "1" ]]; then
  unexpected_pattern="source=fallback_timeout|${unexpected_pattern}"
fi

if grep -Eq "${unexpected_pattern}" "${LOG_FILE}"; then
  fail "unexpected WebContent termination, startup failure, or disallowed fallback reveal"
fi

cleanup
trap - EXIT

sleep 0.5
if pgrep -fl "${APP_BUNDLE}/Contents/MacOS/nexus-server" >/dev/null 2>&1; then
  fail "sidecar process still running after app shutdown"
fi

echo "smoke passed: ${LOG_FILE}"
