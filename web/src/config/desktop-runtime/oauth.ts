/** 连接器 OAuth 在桌面回环服务与浏览器环境之间的地址协议。 */

import { getDesktopRuntimeConfig } from "./runtime-config";

const CONNECTOR_OAUTH_CALLBACK_PATH = "/capability/connectors/oauth/callback";
const DESKTOP_LOOPBACK_OAUTH_PORT = "34343";
const DESKTOP_CONNECTORS_RETURN_URI = "nexus://capability/connectors";
const LOOPBACK_HOSTS = new Set(["127.0.0.1", "localhost", "::1", "[::1]"]);

export function getConnectorOauthRedirectUri(): string {
  const runtimeConfig = getDesktopRuntimeConfig();
  if (runtimeConfig?.appMode !== "desktop") return browserOauthRedirectUri();
  const configuredUri = runtimeConfig.oauthRedirectUri?.trim();
  if (configuredUri) return configuredUri;
  const apiBaseUrl = runtimeConfig.apiBaseUrl?.trim();
  if (!apiBaseUrl) return browserOauthRedirectUri();
  try {
    return `${new URL(apiBaseUrl).origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
  } catch {
    return browserOauthRedirectUri();
  }
}

export function isDesktopLoopbackOauthCallback(): boolean {
  if (typeof window === "undefined") return false;
  const host = window.location.hostname.trim().toLowerCase();
  return window.location.protocol === "http:" &&
    window.location.port === DESKTOP_LOOPBACK_OAUTH_PORT &&
    LOOPBACK_HOSTS.has(host) &&
    window.location.pathname === CONNECTOR_OAUTH_CALLBACK_PATH;
}

export function getDesktopConnectorsReturnUri(): string {
  return DESKTOP_CONNECTORS_RETURN_URI;
}

function browserOauthRedirectUri(): string {
  return `${window.location.origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
}
