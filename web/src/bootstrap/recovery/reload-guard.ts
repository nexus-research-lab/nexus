import { getDesktopSessionToken } from "@/config/desktop-runtime";

export function shouldReloadOnce(prefix: string, reason: string): boolean {
  const runtimeKey = getDesktopSessionToken() || window.location.origin || "web";
  const reloadKey = `${prefix}${runtimeKey}:${reason}:${window.location.pathname}`;
  try {
    if (window.sessionStorage.getItem(reloadKey) === "1") {
      return false;
    }
    window.sessionStorage.setItem(reloadKey, "1");
    return true;
  } catch {
    // 哨兵不可用时拒绝自动刷新，避免恢复逻辑形成无限循环。
    return false;
  }
}
