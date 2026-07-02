export type DesktopRuntimeConfig = {
  api_base_url?: string;
  ws_url?: string;
  auth_token?: string;
  app_mode?: string;
  app_version?: string;
  build_number?: string;
  platform?: string;
  oauth_redirect_uri?: string;
};

type DesktopPerformanceMark = {
  name: string;
  start_time_ms: number;
};

type DesktopWebReadyPerformance = {
  ready_ms: number;
  response_end_ms?: number;
  dom_content_loaded_ms?: number;
  load_event_end_ms?: number;
  first_contentful_paint_ms?: number;
  marks: DesktopPerformanceMark[];
};

export type DesktopRenderSnapshot = {
  href: string;
  path: string;
  ready_state: DocumentReadyState;
  title: string;
  has_root: boolean;
  root_children: number;
  root_text_length: number;
  body_children: number;
  body_text_length: number;
};

export type DesktopRenderHealthStatus = "ready" | "empty_root" | "blank_root";

type DesktopLifecycleMessage = DesktopWebReadyMessage | DesktopWebFatalMessage | DesktopWebHealthMessage;

type DesktopWebReadyMessage = {
  kind: "web.ready";
  location: string;
  reduced_motion: boolean;
  source: string;
  performance: DesktopWebReadyPerformance;
};

type DesktopWebFatalMessage = {
  kind: "web.fatal";
  location: string;
  source: string;
  message: string;
  name?: string;
  stack?: string;
  component_stack?: string;
  snapshot: DesktopRenderSnapshot;
  performance: DesktopWebReadyPerformance;
};

type DesktopWebHealthMessage = {
  kind: "web.health";
  location: string;
  source: string;
  status: DesktopRenderHealthStatus;
  snapshot: DesktopRenderSnapshot;
  performance: DesktopWebReadyPerformance;
};

const DESKTOP_SESSION_TOKEN_HEADER = "X-Nexus-Desktop-Token";
const DESKTOP_SESSION_TOKEN_INVALID_DETAIL = "桌面会话 token 无效";
const DESKTOP_SESSION_TOKEN_PROTOCOL_PREFIX = "nexus.desktop.token.";
const CONNECTOR_OAUTH_CALLBACK_PATH = "/capability/connectors/oauth/callback";
const DESKTOP_LOOPBACK_OAUTH_PORT = "34343";
const DESKTOP_CONNECTORS_RETURN_URI = "nexus://capability/connectors";
const DESKTOP_DIAGNOSTIC_TEXT_LIMIT = 4_096;
const DESKTOP_SESSION_TOKEN_RELOAD_KEY_PREFIX = "nexus:desktop-session-token-reload:";

declare global {
  interface Window {
    __NEXUS_DESKTOP_RUNTIME__?: DesktopRuntimeConfig;
    webkit?: {
      messageHandlers?: {
        nexusDesktopLifecycle?: {
          postMessage: (message: DesktopLifecycleMessage) => void;
        };
      };
    };
  }
}

export function get_desktop_runtime_config(): DesktopRuntimeConfig | null {
  if (typeof window === "undefined") {
    return null;
  }
  const runtimeConfig = window.__NEXUS_DESKTOP_RUNTIME__;
  if (!runtimeConfig || typeof runtimeConfig !== "object") {
    return null;
  }
  return runtimeConfig;
}

export function is_desktop_runtime(): boolean {
  return get_desktop_runtime_config()?.app_mode === "desktop";
}

export function get_desktop_session_token(): string {
  return get_desktop_runtime_config()?.auth_token?.trim() || "";
}

export function get_desktop_websocket_protocols(): string[] {
  const token = get_desktop_session_token();
  if (!token) {
    return [];
  }
  return ["nexus.desktop.v1", `${DESKTOP_SESSION_TOKEN_PROTOCOL_PREFIX}${token}`];
}

export function apply_desktop_request_headers(input: string, headers: Headers): Headers {
  const token = get_desktop_session_token();
  if (!token || !shouldAttachDesktopSessionToken(input)) {
    return headers;
  }
  if (!headers.has(DESKTOP_SESSION_TOKEN_HEADER)) {
    headers.set(DESKTOP_SESSION_TOKEN_HEADER, token);
  }
  return headers;
}

export function recover_desktop_session_token_error(message: string, input: string): boolean {
  if (!isDesktopSessionTokenError(message)) {
    return false;
  }

  const requestPath = desktopRequestPath(input);
  notify_desktop_web_fatal(
    "desktop.session_token_invalid",
    new Error(`${DESKTOP_SESSION_TOKEN_INVALID_DETAIL}: ${requestPath}`),
  );
  mark_desktop_performance("desktop.session_token_invalid");
  if (!shouldReloadForDesktopSessionToken(input)) {
    return false;
  }
  window.setTimeout(() => {
    window.location.reload();
  }, 0);
  return true;
}

function isDesktopSessionTokenError(message: string): boolean {
  return is_desktop_runtime() && message.includes(DESKTOP_SESSION_TOKEN_INVALID_DETAIL);
}

export function mark_desktop_performance(name: string): void {
  if (!get_desktop_runtime_config()) {
    return;
  }
  try {
    performance.mark(`nexus.${name}`);
  } catch {
    // 性能标记只用于诊断，启动流程不能依赖它们。
  }
}

export function notify_desktop_web_ready(source = "unknown"): void {
  mark_desktop_performance("web.ready");
  postDesktopLifecycleMessage({
    kind: "web.ready",
    location: window.location.pathname || "/",
    reduced_motion: prefersReducedMotion(),
    source,
    performance: getDesktopReadyPerformance(),
  });
}

export function notify_desktop_web_fatal(
  source: string,
  error: unknown,
  details: { component_stack?: string } = {},
): void {
  if (!is_desktop_runtime()) {
    return;
  }

  mark_desktop_performance(`web.fatal.${source}`);
  postDesktopLifecycleMessage({
    kind: "web.fatal",
    location: currentLocationPath(),
    source,
    message: diagnosticMessage(error),
    name: diagnosticName(error),
    stack: diagnosticStack(error),
    component_stack: trimDiagnosticText(details.component_stack),
    snapshot: get_desktop_render_snapshot(),
    performance: getDesktopReadyPerformance(),
  });
}

export function notify_desktop_render_health(
  source: string,
  status: DesktopRenderHealthStatus,
): void {
  if (!is_desktop_runtime()) {
    return;
  }

  mark_desktop_performance(`web.health.${status}`);
  postDesktopLifecycleMessage({
    kind: "web.health",
    location: currentLocationPath(),
    source,
    status,
    snapshot: get_desktop_render_snapshot(),
    performance: getDesktopReadyPerformance(),
  });
}

export function get_desktop_render_snapshot(): DesktopRenderSnapshot {
  const root = document.getElementById("root");
  const body = document.body;
  return {
    href: window.location.href,
    path: currentLocationPath(),
    ready_state: document.readyState,
    title: document.title,
    has_root: Boolean(root),
    root_children: root?.childElementCount ?? -1,
    root_text_length: root?.innerText?.trim().length ?? -1,
    body_children: body?.childElementCount ?? -1,
    body_text_length: body?.innerText?.length ?? -1,
  };
}

export function get_connector_oauth_redirect_uri(): string {
  const runtimeConfig = get_desktop_runtime_config();
  if (runtimeConfig?.app_mode === "desktop") {
    const configuredUri = runtimeConfig.oauth_redirect_uri?.trim();
    if (configuredUri) {
      return configuredUri;
    }
    const apiBaseUrl = runtimeConfig.api_base_url?.trim();
    if (apiBaseUrl) {
      try {
        return `${new URL(apiBaseUrl).origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
      } catch {
        return `${window.location.origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
      }
    }
  }
  return `${window.location.origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
}

export function is_desktop_loopback_oauth_callback(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  const host = window.location.hostname.trim().toLowerCase();
  const isLoopback = host === "127.0.0.1" || host === "localhost" || host === "::1" || host === "[::1]";
  return window.location.protocol === "http:" &&
    window.location.port === DESKTOP_LOOPBACK_OAUTH_PORT &&
    isLoopback &&
    window.location.pathname === CONNECTOR_OAUTH_CALLBACK_PATH;
}

export function get_desktop_connectors_return_uri(): string {
  return DESKTOP_CONNECTORS_RETURN_URI;
}

function postDesktopLifecycleMessage(message: DesktopLifecycleMessage): void {
  const lifecycleHandler = window.webkit?.messageHandlers?.nexusDesktopLifecycle;
  if (!lifecycleHandler) {
    return;
  }
  lifecycleHandler.postMessage(message);
}

function shouldAttachDesktopSessionToken(input: string): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  const runtimeConfig = get_desktop_runtime_config();
  const apiBaseUrl = runtimeConfig?.api_base_url?.trim();
  if (!apiBaseUrl) {
    return false;
  }
  try {
    const requestUrl = new URL(input, window.location.href);
    const apiUrl = new URL(apiBaseUrl, window.location.href);
    const apiPath = apiUrl.pathname.replace(/\/+$/, "");
    return requestUrl.origin === apiUrl.origin
      && (requestUrl.pathname === apiPath || requestUrl.pathname.startsWith(`${apiPath}/`));
  } catch {
    return false;
  }
}

function shouldReloadForDesktopSessionToken(input: string): boolean {
  const runtimeConfig = get_desktop_runtime_config();
  const apiBaseUrl = runtimeConfig?.api_base_url?.trim() || "missing-api";
  const key = `${DESKTOP_SESSION_TOKEN_RELOAD_KEY_PREFIX}${apiBaseUrl}:${desktopRequestPath(input)}:${currentLocationPath()}`;
  try {
    if (window.sessionStorage.getItem(key) === "1") {
      return false;
    }
    window.sessionStorage.setItem(key, "1");
    return true;
  } catch {
    // sessionStorage 不可用时不要盲目刷新，避免进入无上限重载循环。
    return false;
  }
}

function desktopRequestPath(input: string): string {
  try {
    const requestUrl = new URL(input, window.location.href);
    return `${requestUrl.pathname}${requestUrl.search}${requestUrl.hash}`;
  } catch {
    return input.trim() || "unknown";
  }
}

function currentLocationPath(): string {
  return `${window.location.pathname || "/"}${window.location.search}${window.location.hash}`;
}

function diagnosticMessage(error: unknown): string {
  if (error instanceof Error) {
    return trimDiagnosticText(error.message) || error.name;
  }
  if (typeof error === "string") {
    return trimDiagnosticText(error) || "Unknown error";
  }
  return trimDiagnosticText(String(error)) || "Unknown error";
}

function diagnosticName(error: unknown): string | undefined {
  if (error instanceof Error) {
    return trimDiagnosticText(error.name);
  }
  return undefined;
}

function diagnosticStack(error: unknown): string | undefined {
  if (error instanceof Error) {
    return trimDiagnosticText(error.stack);
  }
  return undefined;
}

function trimDiagnosticText(value?: string): string | undefined {
  const normalized = value?.trim();
  if (!normalized) {
    return undefined;
  }
  if (normalized.length <= DESKTOP_DIAGNOSTIC_TEXT_LIMIT) {
    return normalized;
  }
  return `${normalized.slice(0, DESKTOP_DIAGNOSTIC_TEXT_LIMIT)}...`;
}

function getDesktopReadyPerformance(): DesktopWebReadyPerformance {
  const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming | undefined;
  const paintEntries = performance.getEntriesByType("paint");
  const firstContentfulPaint = paintEntries.find((entry) => entry.name === "first-contentful-paint");
  const payload: DesktopWebReadyPerformance = {
    ready_ms: roundedMilliseconds(performance.now()),
    marks: performance.getEntriesByType("mark")
      .filter((entry) => entry.name.startsWith("nexus."))
      .map((entry) => ({
        name: entry.name,
        start_time_ms: roundedMilliseconds(entry.startTime),
      })),
  };

  if (navigation) {
    payload.response_end_ms = roundedMilliseconds(navigation.responseEnd);
    payload.dom_content_loaded_ms = roundedMilliseconds(navigation.domContentLoadedEventEnd);
    payload.load_event_end_ms = roundedMilliseconds(navigation.loadEventEnd);
  }
  if (firstContentfulPaint) {
    payload.first_contentful_paint_ms = roundedMilliseconds(firstContentfulPaint.startTime);
  }
  return payload;
}

function roundedMilliseconds(value: number): number {
  return Math.round(value * 10) / 10;
}

function prefersReducedMotion(): boolean {
  if (typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
