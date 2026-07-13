export function isWindowActive(): boolean {
  if (typeof document === "undefined") {
    return true;
  }
  return document.visibilityState === "visible" && document.hasFocus();
}

export function subscribeBrowserNotificationPermission(): () => void {
  if (!supportsBrowserNotification() || Notification.permission !== "default") {
    return () => undefined;
  }
  const removeListeners = () => {
    window.removeEventListener("pointerdown", requestPermission, true);
    window.removeEventListener("keydown", requestPermission, true);
  };
  const requestPermission = () => {
    removeListeners();
    // 浏览器可能因用户策略拒绝权限，请求失败不影响站内未读状态。
    void Notification.requestPermission().catch(() => undefined);
  };
  window.addEventListener("pointerdown", requestPermission, { capture: true, once: true });
  window.addEventListener("keydown", requestPermission, { capture: true, once: true });
  return removeListeners;
}

export function showBrowserNotification(title: string, body: string, tag: string): void {
  if (
    !supportsBrowserNotification()
    || Notification.permission !== "granted"
    || isWindowActive()
  ) {
    return;
  }
  const notification = new Notification(title, { body, tag });
  notification.onclick = () => {
    window.focus();
    notification.close();
  };
}

function supportsBrowserNotification(): boolean {
  return typeof window !== "undefined" && "Notification" in window;
}
