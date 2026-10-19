export {
  applyDesktopRuntimeDocumentFlags,
  getDesktopPlatform,
  getDesktopRuntimeConfig,
  isDesktopRuntime,
  isMacDesktopRuntime,
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
  notifyDesktopDiagnostic,
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
