import {
  getDesktopRenderSnapshot,
  markDesktopPerformance,
  notifyDesktopRenderHealth,
  type DesktopRenderHealthStatus,
  type DesktopRenderSnapshot,
} from "@/config/desktop-runtime";

import { shouldReloadOnce } from "./reload-guard";

type UnhealthyRenderStatus = Exclude<DesktopRenderHealthStatus, "ready">;

const APP_BLANK_RENDER_RELOAD_KEY_PREFIX = "nexus:app-blank-render-reload:";
const APP_RENDER_WATCHDOG_INTERVAL_MS = 10_000;
const APP_RENDER_WATCHDOG_UNHEALTHY_THRESHOLD = 2;
const RECOVERY_REASONS: Record<UnhealthyRenderStatus, string> = {
  empty_root: "根节点为空",
  blank_root: "根节点没有可见内容",
};

let didStartAppRenderWatchdog = false;

function getRenderUnhealthyStatus(
  snapshot: DesktopRenderSnapshot,
): UnhealthyRenderStatus | null {
  if (snapshot.readyState === "loading") {
    return null;
  }
  if (!snapshot.hasRoot || snapshot.rootChildren <= 0) {
    return "empty_root";
  }
  return snapshot.rootTextLength <= 0 && snapshot.bodyTextLength <= 0
    ? "blank_root"
    : null;
}

export function startAppRenderWatchdog(
  renderRecoveryScreen: (reason: string) => void,
): void {
  if (didStartAppRenderWatchdog) {
    return;
  }
  didStartAppRenderWatchdog = true;

  let consecutiveUnhealthyCount = 0;
  const checkRenderHealth = (source: string) => {
    const unhealthyStatus = getRenderUnhealthyStatus(getDesktopRenderSnapshot());
    if (!unhealthyStatus) {
      if (consecutiveUnhealthyCount > 0) {
        notifyDesktopRenderHealth(source, "ready");
      }
      consecutiveUnhealthyCount = 0;
      return;
    }

    consecutiveUnhealthyCount += 1;
    notifyDesktopRenderHealth(source, unhealthyStatus);
    if (consecutiveUnhealthyCount < APP_RENDER_WATCHDOG_UNHEALTHY_THRESHOLD) {
      return;
    }
    if (shouldReloadOnce(APP_BLANK_RENDER_RELOAD_KEY_PREFIX, unhealthyStatus)) {
      markDesktopPerformance(`web.health.${unhealthyStatus}.reload`);
      window.location.reload();
      return;
    }
    renderRecoveryScreen(RECOVERY_REASONS[unhealthyStatus]);
  };

  window.setInterval(
    () => checkRenderHealth("watchdog"),
    APP_RENDER_WATCHDOG_INTERVAL_MS,
  );
  window.addEventListener("focus", () => {
    window.setTimeout(() => checkRenderHealth("focus"), 300);
  });
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      window.setTimeout(() => checkRenderHealth("visibility"), 300);
    }
  });
}
