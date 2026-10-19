/** 桌面宿主注入配置的解析与文档级标记。 */

interface DesktopRuntimeConfig {
  apiBaseUrl?: string;
  appMode?: string;
  appVersion?: string;
  authToken?: string;
  buildNumber?: string;
  desktopWindowCloseButtonCenterX?: number;
  desktopWindowCloseButtonCenterY?: number;
  desktopWindowControlsInset?: number;
  oauthRedirectUri?: string;
  platform?: string;
  wsUrl?: string;
}

const DESKTOP_WINDOW_CONTROLS_INSET_PROPERTY =
  "--desktop-window-controls-inset";
const DESKTOP_WINDOW_CLOSE_BUTTON_CENTER_X_PROPERTY =
  "--desktop-window-close-button-center-x";
const DESKTOP_WINDOW_CLOSE_BUTTON_CENTER_Y_PROPERTY =
  "--desktop-window-close-button-center-y";

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
  const closeButtonCenterX =
    runtimeConfig.desktop_window_close_button_center_x;
  if (
    typeof closeButtonCenterX === "number"
    && Number.isFinite(closeButtonCenterX)
  ) {
    normalized.desktopWindowCloseButtonCenterX = closeButtonCenterX;
  }
  const closeButtonCenterY =
    runtimeConfig.desktop_window_close_button_center_y;
  if (
    typeof closeButtonCenterY === "number"
    && Number.isFinite(closeButtonCenterY)
  ) {
    normalized.desktopWindowCloseButtonCenterY = closeButtonCenterY;
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

/** 宿主注入的平台标识（macOS 为 "macos"，Windows 为 "windows"）；非桌面宿主为 null。 */
export function getDesktopPlatform(): string | null {
  const platform = getDesktopRuntimeConfig()?.platform;
  return platform ? platform.toLowerCase() : null;
}

/** Computer Use 目前只交付 macOS；其余平台（含旧宿主未注入 platform）一律隐藏入口。 */
export function isMacDesktopRuntime(): boolean {
  return isDesktopRuntime() && getDesktopPlatform() === "macos";
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
  const closeButtonCenterX = runtimeConfig.desktopWindowCloseButtonCenterX;
  if (typeof closeButtonCenterX === "number" && closeButtonCenterX >= 0) {
    document.documentElement.style.setProperty(
      DESKTOP_WINDOW_CLOSE_BUTTON_CENTER_X_PROPERTY,
      `${closeButtonCenterX}px`,
    );
  }
  const closeButtonCenterY = runtimeConfig.desktopWindowCloseButtonCenterY;
  if (typeof closeButtonCenterY === "number" && closeButtonCenterY >= 0) {
    document.documentElement.style.setProperty(
      DESKTOP_WINDOW_CLOSE_BUTTON_CENTER_Y_PROPERTY,
      `${closeButtonCenterY}px`,
    );
  }
}
