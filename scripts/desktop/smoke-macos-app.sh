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
MAIN_NAVIGATION_FINISHED_PATTERN="event=webview\\.navigation_finished.*surface=main"
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

request_app_exit() {
  if [[ -z "${APP_PID}" ]] || ! kill -0 "${APP_PID}" >/dev/null 2>&1; then
    return 0
  fi

  local exit_command_status=0
  "${APP_EXECUTABLE}" --nexus-desktop-exit >/dev/null 2>&1 || exit_command_status=$?
  local started_at
  started_at="$(date +%s)"
  while kill -0 "${APP_PID}" >/dev/null 2>&1; do
    if (( "$(date +%s)" - started_at >= 5 )); then
      echo "smoke: --nexus-desktop-exit did not stop app (status=${exit_command_status}); sending SIGTERM" >&2
      kill -TERM "${APP_PID}" >/dev/null 2>&1 || true
      break
    fi
    sleep 0.2
  done

  started_at="$(date +%s)"
  while kill -0 "${APP_PID}" >/dev/null 2>&1; do
    if (( "$(date +%s)" - started_at >= 15 )); then
      return 1
    fi
    sleep 0.2
  done
  return 0
}

cleanup() {
  if [[ -n "${APP_PID}" ]] && kill -0 "${APP_PID}" >/dev/null 2>&1; then
    request_app_exit || kill "${APP_PID}" >/dev/null 2>&1 || true
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

log_match_count() {
  local pattern="$1"
  grep -Ec "${pattern}" "${LOG_FILE}" 2>/dev/null || true
}

wait_for_new_log_match() {
  local pattern="$1"
  local previous_count="$2"
  local timeout_seconds="$3"
  local started_at
  started_at="$(date +%s)"

  while true; do
    local current_count
    current_count="$(log_match_count "${pattern}")"
    if (( current_count > previous_count )); then
      return 0
    fi
    if [[ -n "${APP_PID}" ]] && ! kill -0 "${APP_PID}" >/dev/null 2>&1; then
      fail "app exited before a new log matched: ${pattern}"
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

NEXUSCFG_EXECUTABLE="${APP_BUNDLE}/Contents/Resources/bin/nexuscfg"
if [[ ! -x "${NEXUSCFG_EXECUTABLE}" ]]; then
  fail "missing bundled nexuscfg executable: ${NEXUSCFG_EXECUTABLE}"
fi

if ! "${NEXUSCFG_EXECUTABLE}" --help >/dev/null 2>&1; then
  fail "bundled nexuscfg --help failed"
fi

NEXUS_EXECUTABLE="${APP_BUNDLE}/Contents/Resources/bin/nexus"
if [[ ! -x "${NEXUS_EXECUTABLE}" ]]; then
  fail "missing bundled nexus executable: ${NEXUS_EXECUTABLE}"
fi

if ! "${NEXUS_EXECUTABLE}" --help >/dev/null 2>&1; then
  fail "bundled nexus --help failed"
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
wait_for_log "${MAIN_NAVIGATION_FINISHED_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"

main_ready_count="$(log_match_count "${MAIN_READY_ROUTE_PATTERN}")"
main_navigation_count="$(log_match_count "${MAIN_NAVIGATION_FINISHED_PATTERN}")"
if open "nexus://open" >/dev/null 2>&1 &&
  wait_for_log_match "event=app\\.url_route.*host=open .*route_path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_URL_TIMEOUT_SECONDS}"; then
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
else
  post_main_window_notification || fail "failed to request launcher route through nexus://open"
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
fi
if ! wait_for_new_log_match "${MAIN_NAVIGATION_FINISHED_PATTERN}" "${main_navigation_count}" "${MAIN_TIMEOUT_SECONDS}"; then
  fail "timed out waiting for the nexus://open navigation to finish"
fi
if ! wait_for_new_log_match "${MAIN_READY_ROUTE_PATTERN}" "${main_ready_count}" "${MAIN_TIMEOUT_SECONDS}"; then
  fail "timed out waiting for the nexus://open route to become ready"
fi

main_ready_count="$(log_match_count "${MAIN_READY_ROUTE_PATTERN}")"
main_navigation_count="$(log_match_count "${MAIN_NAVIGATION_FINISHED_PATTERN}")"
if open "nexus://launcher" >/dev/null 2>&1 &&
  wait_for_log_match "event=app\\.url_route.*host=launcher .*route_path=${LAUNCHER_ROUTE_PATTERN}" "${LAUNCHER_URL_TIMEOUT_SECONDS}"; then
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
else
  post_launcher_notification || fail "failed to request launcher route"
  wait_for_log "event=main_window\\.route_load.*path=${LAUNCHER_ROUTE_PATTERN}" "${MAIN_TIMEOUT_SECONDS}"
fi
if ! wait_for_new_log_match "${MAIN_NAVIGATION_FINISHED_PATTERN}" "${main_navigation_count}" "${MAIN_TIMEOUT_SECONDS}"; then
  fail "timed out waiting for the nexus://launcher navigation to finish"
fi
if ! wait_for_new_log_match "${MAIN_READY_ROUTE_PATTERN}" "${main_ready_count}" "${MAIN_TIMEOUT_SECONDS}"; then
  fail "timed out waiting for the nexus://launcher route to become ready"
fi

unexpected_pattern="webview\\.content_process_terminated|startup\\.failed"
if [[ "${ALLOW_FALLBACK}" != "1" ]]; then
  unexpected_pattern="source=fallback_timeout|${unexpected_pattern}"
fi

if grep -Eq "${unexpected_pattern}" "${LOG_FILE}"; then
  fail "unexpected WebContent termination, startup failure, or disallowed fallback reveal"
fi

if ! request_app_exit; then
  fail "app did not exit after --nexus-desktop-exit"
fi
wait "${APP_PID}" >/dev/null 2>&1 || true
APP_PID=""
trap - EXIT

sleep 0.5
if pgrep -fl "${APP_BUNDLE}/Contents/MacOS/nexus-server" >/dev/null 2>&1; then
  fail "sidecar process still running after app shutdown"
fi

echo "smoke passed: ${LOG_FILE}"
