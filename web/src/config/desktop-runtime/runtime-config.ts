/** 桌面宿主注入配置的解析与文档级标记。 */

interface DesktopRuntimeConfig {
  apiBaseUrl?: string;
  appMode?: string;
  appVersion?: string;
  authToken?: string;
  buildNumber?: string;
  desktopWindowControlsInset?: number;
  oauthRedirectUri?: string;
  platform?: string;
  wsUrl?: string;
}

const DESKTOP_WINDOW_CONTROLS_INSET_PROPERTY =
  "--desktop-window-controls-inset";

declare global {
  interface Window {
    __NEXUS_DESKTOP_RUNTIME__?: Record<string, unknown>;
  }
}

const RUNTIME_STRING_FIELDS = {
  api_base_url: "apiBaseUrl",
  app_mode: "appMode",
  app_version: "appVersion",
  auth_token: "authToken",
  build_number: "buildNumber",
  oauth_redirect_uri: "oauthRedirectUri",
  platform: "platform",
  ws_url: "wsUrl",
} as const satisfies Record<string, keyof DesktopRuntimeConfig>;

function normalizeDesktopRuntimeConfig(
  runtimeConfig: Record<string, unknown>,
): DesktopRuntimeConfig {
  const normalized: DesktopRuntimeConfig = {};
  Object.entries(RUNTIME_STRING_FIELDS).forEach(([sourceKey, targetKey]) => {
    const value = runtimeConfig[sourceKey];
    if (typeof value === "string") normalized[targetKey] = value;
  });
  const controlsInset = runtimeConfig.desktop_window_controls_inset;
  if (typeof controlsInset === "number" && Number.isFinite(controlsInset)) {
    normalized.desktopWindowControlsInset = controlsInset;
  }
  return normalized;
}

export function getDesktopRuntimeConfig(): DesktopRuntimeConfig | null {
  if (typeof window === "undefined") return null;
  const runtimeConfig = window.__NEXUS_DESKTOP_RUNTIME__;
  if (!runtimeConfig || typeof runtimeConfig !== "object") return null;
  return normalizeDesktopRuntimeConfig(runtimeConfig);
}

export function isDesktopRuntime(): boolean {
  return getDesktopRuntimeConfig()?.appMode === "desktop";
}

export function applyDesktopRuntimeDocumentFlags(): void {
  const runtimeConfig = getDesktopRuntimeConfig();
  if (runtimeConfig?.appMode !== "desktop") return;
  document.documentElement.dataset.desktopRuntime = "true";
  if (runtimeConfig.platform) {
    document.documentElement.dataset.desktopPlatform = runtimeConfig.platform;
  }
  const controlsInset = runtimeConfig.desktopWindowControlsInset;
  if (typeof controlsInset === "number" && controlsInset >= 0) {
    document.documentElement.style.setProperty(
      DESKTOP_WINDOW_CONTROLS_INSET_PROPERTY,
      `${controlsInset}px`,
    );
  }
}
