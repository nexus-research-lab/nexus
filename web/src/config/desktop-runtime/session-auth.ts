/** 桌面会话 token 的 HTTP、WebSocket 与失效恢复协议。 */

import {
  currentDesktopLocationPath,
  desktopRequestPath,
  isDesktopApiRequest,
} from "./desktop-location";
import { markDesktopPerformance, notifyDesktopWebFatal } from "./lifecycle";
import { getDesktopRuntimeConfig, isDesktopRuntime } from "./runtime-config";

const DESKTOP_SESSION_TOKEN_HEADER = "X-Nexus-Desktop-Token";
const DESKTOP_SESSION_TOKEN_INVALID_DETAIL = "桌面会话 token 无效";
const DESKTOP_SESSION_TOKEN_PROTOCOL_PREFIX = "nexus.desktop.token.";
const DESKTOP_SESSION_TOKEN_RELOAD_KEY_PREFIX = "nexus:desktop-session-token-reload";

export function getDesktopSessionToken(): string {
  return getDesktopRuntimeConfig()?.authToken?.trim() || "";
}

export function getDesktopWebsocketProtocols(): string[] {
  const token = getDesktopSessionToken();
  return token
    ? ["nexus.desktop.v1", `${DESKTOP_SESSION_TOKEN_PROTOCOL_PREFIX}${token}`]
    : [];
}

export function applyDesktopRequestHeaders(input: string, headers: Headers): Headers {
  const token = getDesktopSessionToken();
  if (!token || !shouldAttachDesktopSessionToken(input)) return headers;
  if (!headers.has(DESKTOP_SESSION_TOKEN_HEADER)) {
    headers.set(DESKTOP_SESSION_TOKEN_HEADER, token);
  }
  return headers;
}

export function recoverDesktopSessionTokenError(message: string, input: string): boolean {
  if (!isDesktopSessionTokenError(message)) return false;
  const requestPath = desktopRequestPath(input);
  notifyDesktopWebFatal(
    "desktop.session_token_invalid",
    new Error(`${DESKTOP_SESSION_TOKEN_INVALID_DETAIL}: ${requestPath}`),
  );
  markDesktopPerformance("desktop.session_token_invalid");
  if (!shouldReloadForDesktopSessionToken(input)) return false;
  window.setTimeout(() => window.location.reload(), 0);
  return true;
}

function isDesktopSessionTokenError(message: string): boolean {
  return isDesktopRuntime() && message.includes(DESKTOP_SESSION_TOKEN_INVALID_DETAIL);
}

function shouldAttachDesktopSessionToken(input: string): boolean {
  if (typeof window === "undefined") return false;
  const apiBaseUrl = getDesktopRuntimeConfig()?.apiBaseUrl?.trim();
  return Boolean(apiBaseUrl && isDesktopApiRequest(input, apiBaseUrl));
}

function shouldReloadForDesktopSessionToken(input: string): boolean {
  const apiBaseUrl = getDesktopRuntimeConfig()?.apiBaseUrl?.trim() || "missing-api";
  const key = [
    DESKTOP_SESSION_TOKEN_RELOAD_KEY_PREFIX,
    apiBaseUrl,
    desktopRequestPath(input),
    currentDesktopLocationPath(),
  ].join(":");
  try {
    if (window.sessionStorage.getItem(key) === "1") return false;
    window.sessionStorage.setItem(key, "1");
    return true;
  } catch {
    // 存储不可用时不能证明刷新已受限，因此保持当前页面，避免无限重载。
    return false;
  }
}
