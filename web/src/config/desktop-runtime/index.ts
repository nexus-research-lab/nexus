export {
  applyDesktopRuntimeDocumentFlags,
  getDesktopRuntimeConfig,
  isDesktopRuntime,
  type DesktopRuntimeConfig,
} from "./runtime-config";
export {
  applyDesktopRequestHeaders,
  getDesktopSessionToken,
  getDesktopWebsocketProtocols,
  recoverDesktopSessionTokenError,
} from "./session-auth";
export {
  getDesktopRenderSnapshot,
  markDesktopPerformance,
  notifyDesktopRenderHealth,
  notifyDesktopWebFatal,
  notifyDesktopWebReady,
  type DesktopRenderHealthStatus,
  type DesktopRenderSnapshot,
} from "./lifecycle";
export {
  getConnectorOauthRedirectUri,
  getDesktopConnectorsReturnUri,
  isDesktopLoopbackOauthCallback,
} from "./oauth";
