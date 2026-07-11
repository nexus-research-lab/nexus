/** 鉴权失效事件是 HTTP 与 WebSocket 共用的浏览器边界。 */

export const AUTH_REQUIRED_EVENT = "nexus:auth-required";

export function notifyAuthRequired(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AUTH_REQUIRED_EVENT));
}
