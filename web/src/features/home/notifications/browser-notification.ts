/**
 * INPUT: 窗口可见性、浏览器通知权限与待发的系统通知。
 * OUTPUT: 首次交互时请求浏览器权限；窗口不在前台时经桌面宿主或浏览器发出系统通知。
 * POS: 聊天通知的唯一系统通知边界，站内未读不依赖它成功。
 */
import { isDesktopBridgeAvailable, showDesktopNotification } from "@/lib/desktop-bridge/desktop-bridge";

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

/** 桌面宿主在用户点击原生通知后派发该事件；页面内路由跳转，避免宿主整页重载。 */
const DESKTOP_NOTIFICATION_OPEN_EVENT = "nexus:notification-open";

export function subscribeDesktopNotificationOpen(openRoute: (route: string) => void): () => void {
  if (typeof window === "undefined") {
    return () => undefined;
  }
  const handleOpen = (event: Event) => {
    const route = event instanceof CustomEvent ? (event.detail as { route?: unknown } | null)?.route : null;
    if (typeof route === "string" && route.startsWith("/")) {
      openRoute(route);
    }
  };
  window.addEventListener(DESKTOP_NOTIFICATION_OPEN_EVENT, handleOpen);
  return () => window.removeEventListener(DESKTOP_NOTIFICATION_OPEN_EVENT, handleOpen);
}

export interface SystemNotification {
  body: string;
  /** 点击后打开的应用内路由；为空时只唤回窗口。 */
  route: string | null;
  tag: string;
  title: string;
}

/**
 * 窗口不在前台时发一条系统通知：桌面宿主走原生通知（点击由宿主唤回窗口并打开路由），
 * 普通浏览器退回 Notification API。权限缺失或宿主失败只丢弃本条通知，不影响站内未读。
 */
export function showSystemNotification(
  notification: SystemNotification,
  openRoute: (route: string) => void,
): void {
  if (isWindowActive()) {
    return;
  }
  if (isDesktopBridgeAvailable()) {
    void showDesktopNotification(notification)
      .catch((error: unknown) => console.warn("Desktop notification failed", error));
    return;
  }
  if (!supportsBrowserNotification() || Notification.permission !== "granted") {
    return;
  }
  const browserNotification = new Notification(notification.title, {
    body: notification.body,
    tag: notification.tag,
  });
  browserNotification.onclick = () => {
    window.focus();
    browserNotification.close();
    if (notification.route) {
      openRoute(notification.route);
    }
  };
}

function supportsBrowserNotification(): boolean {
  return typeof window !== "undefined" && "Notification" in window;
}
