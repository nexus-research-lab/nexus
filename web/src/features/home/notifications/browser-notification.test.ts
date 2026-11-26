// INPUT: 桌面宿主可用性、窗口可见性与原生通知点击事件。
// OUTPUT: 桌面优先走原生通知并携带路由；点击事件只接受应用内路由。
// POS: 系统通知边界回归。
import { afterEach, expect, it, vi } from "vitest";

import { showSystemNotification, subscribeDesktopNotificationOpen } from "./browser-notification";

afterEach(() => {
  delete window.__NEXUS_DESKTOP_BRIDGE__;
  vi.restoreAllMocks();
});

function backgroundWindow() {
  vi.spyOn(document, "hasFocus").mockReturnValue(false);
}

it("routes notifications through the desktop host when it is present", () => {
  backgroundWindow();
  const invoke = vi.fn().mockResolvedValue({ shown: true });
  window.__NEXUS_DESKTOP_BRIDGE__ = { invoke } as never;
  showSystemNotification({ body: "完成", route: "/rooms/r1", tag: "m1", title: "Nexus" }, vi.fn());
  expect(invoke).toHaveBeenCalledWith(expect.objectContaining({
    kind: "app.show_notification",
    payload: { body: "完成", route: "/rooms/r1", tag: "m1", title: "Nexus" },
  }));
});

it("skips notifications while the window is active", () => {
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  const invoke = vi.fn().mockResolvedValue({ shown: true });
  window.__NEXUS_DESKTOP_BRIDGE__ = { invoke } as never;
  showSystemNotification({ body: "完成", route: null, tag: "m1", title: "Nexus" }, vi.fn());
  expect(invoke).not.toHaveBeenCalled();
});

it("opens only in-app routes from a desktop notification click", () => {
  const open = vi.fn();
  const unsubscribe = subscribeDesktopNotificationOpen(open);
  window.dispatchEvent(new CustomEvent("nexus:notification-open", { detail: { route: "https://evil.example" } }));
  window.dispatchEvent(new CustomEvent("nexus:notification-open", { detail: { route: "/rooms/r1" } }));
  unsubscribe();
  window.dispatchEvent(new CustomEvent("nexus:notification-open", { detail: { route: "/rooms/r2" } }));
  expect(open.mock.calls).toEqual([["/rooms/r1"]]);
});
