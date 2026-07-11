import { getDesktopRuntimeConfig } from "@/config/desktop-runtime";

const DEFAULT_API_PATH = "/nexus/v1";
const DEFAULT_WS_PATH = "/nexus/v1/chat/ws";

function buildBrowserUrl(
  pathname: string,
  useWebsocketProtocol: boolean,
): string {
  if (typeof window === "undefined") {
    return pathname;
  }

  const normalizedPath = pathname.startsWith("/") ? pathname : `/${pathname}`;
  const origin = useWebsocketProtocol
    ? `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`
    : window.location.origin;
  return `${origin}${normalizedPath}`;
}

function resolveRuntimeUrl(
  rawUrl: string | undefined,
  fallbackPath: string,
  useWebsocketProtocol: boolean,
): string {
  const normalizedRawUrl = rawUrl?.trim();
  if (!normalizedRawUrl) {
    return buildBrowserUrl(fallbackPath, useWebsocketProtocol);
  }
  return normalizedRawUrl.startsWith("/")
    ? buildBrowserUrl(normalizedRawUrl, useWebsocketProtocol)
    : normalizedRawUrl;
}

export function getAgentApiBaseUrl(): string {
  const desktopUrl = getDesktopRuntimeConfig()?.apiBaseUrl?.trim();
  return desktopUrl
    || resolveRuntimeUrl(import.meta.env.VITE_API_URL, DEFAULT_API_PATH, false);
}

export function getAgentWsUrl(): string {
  const desktopUrl = getDesktopRuntimeConfig()?.wsUrl?.trim();
  return desktopUrl
    || resolveRuntimeUrl(import.meta.env.VITE_WS_URL, DEFAULT_WS_PATH, true);
}
