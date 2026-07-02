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
  const runtime_config = window.__NEXUS_DESKTOP_RUNTIME__;
  if (!runtime_config || typeof runtime_config !== "object") {
    return null;
  }
  return runtime_config;
}

export function is_desktop_runtime(): boolean {
  return get_desktop_runtime_config()?.app_mode === "desktop";
}

export function apply_desktop_runtime_document_flags(): void {
  const runtime_config = get_desktop_runtime_config();
  if (runtime_config?.app_mode !== "desktop") {
    return;
  }
  document.documentElement.dataset.desktopRuntime = "true";
  if (runtime_config.platform) {
    document.documentElement.dataset.desktopPlatform = runtime_config.platform;
  }
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
  if (!token || !should_attach_desktop_session_token(input)) {
    return headers;
  }
  if (!headers.has(DESKTOP_SESSION_TOKEN_HEADER)) {
    headers.set(DESKTOP_SESSION_TOKEN_HEADER, token);
  }
  return headers;
}

export function recover_desktop_session_token_error(message: string, input: string): boolean {
  if (!is_desktop_session_token_error(message)) {
    return false;
  }

  const request_path = desktop_request_path(input);
  notify_desktop_web_fatal(
    "desktop.session_token_invalid",
    new Error(`${DESKTOP_SESSION_TOKEN_INVALID_DETAIL}: ${request_path}`),
  );
  mark_desktop_performance("desktop.session_token_invalid");
  if (!should_reload_for_desktop_session_token(input)) {
    return false;
  }
  window.setTimeout(() => {
    window.location.reload();
  }, 0);
  return true;
}

function is_desktop_session_token_error(message: string): boolean {
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
  post_desktop_lifecycle_message({
    kind: "web.ready",
    location: window.location.pathname || "/",
    reduced_motion: prefers_reduced_motion(),
    source,
    performance: get_desktop_ready_performance(),
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
  post_desktop_lifecycle_message({
    kind: "web.fatal",
    location: current_location_path(),
    source,
    message: diagnostic_message(error),
    name: diagnostic_name(error),
    stack: diagnostic_stack(error),
    component_stack: trim_diagnostic_text(details.component_stack),
    snapshot: get_desktop_render_snapshot(),
    performance: get_desktop_ready_performance(),
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
  post_desktop_lifecycle_message({
    kind: "web.health",
    location: current_location_path(),
    source,
    status,
    snapshot: get_desktop_render_snapshot(),
    performance: get_desktop_ready_performance(),
  });
}

export function get_desktop_render_snapshot(): DesktopRenderSnapshot {
  const root = document.getElementById("root");
  const body = document.body;
  return {
    href: window.location.href,
    path: current_location_path(),
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
  const runtime_config = get_desktop_runtime_config();
  if (runtime_config?.app_mode === "desktop") {
    const configured_uri = runtime_config.oauth_redirect_uri?.trim();
    if (configured_uri) {
      return configured_uri;
    }
    const api_base_url = runtime_config.api_base_url?.trim();
    if (api_base_url) {
      try {
        return `${new URL(api_base_url).origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
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
  const is_loopback = host === "127.0.0.1" || host === "localhost" || host === "::1" || host === "[::1]";
  return window.location.protocol === "http:" &&
    window.location.port === DESKTOP_LOOPBACK_OAUTH_PORT &&
    is_loopback &&
    window.location.pathname === CONNECTOR_OAUTH_CALLBACK_PATH;
}

export function get_desktop_connectors_return_uri(): string {
  return DESKTOP_CONNECTORS_RETURN_URI;
}

function post_desktop_lifecycle_message(message: DesktopLifecycleMessage): void {
  const lifecycle_handler = window.webkit?.messageHandlers?.nexusDesktopLifecycle;
  if (!lifecycle_handler) {
    return;
  }
  lifecycle_handler.postMessage(message);
}

function should_attach_desktop_session_token(input: string): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  const runtime_config = get_desktop_runtime_config();
  const api_base_url = runtime_config?.api_base_url?.trim();
  if (!api_base_url) {
    return false;
  }
  try {
    const request_url = new URL(input, window.location.href);
    const api_url = new URL(api_base_url, window.location.href);
    const api_path = api_url.pathname.replace(/\/+$/, "");
    return request_url.origin === api_url.origin
      && (request_url.pathname === api_path || request_url.pathname.startsWith(`${api_path}/`));
  } catch {
    return false;
  }
}

function should_reload_for_desktop_session_token(input: string): boolean {
  const runtime_config = get_desktop_runtime_config();
  const api_base_url = runtime_config?.api_base_url?.trim() || "missing-api";
  const key = `${DESKTOP_SESSION_TOKEN_RELOAD_KEY_PREFIX}${api_base_url}:${desktop_request_path(input)}:${current_location_path()}`;
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

function desktop_request_path(input: string): string {
  try {
    const request_url = new URL(input, window.location.href);
    return `${request_url.pathname}${request_url.search}${request_url.hash}`;
  } catch {
    return input.trim() || "unknown";
  }
}

function current_location_path(): string {
  return `${window.location.pathname || "/"}${window.location.search}${window.location.hash}`;
}

function diagnostic_message(error: unknown): string {
  if (error instanceof Error) {
    return trim_diagnostic_text(error.message) || error.name;
  }
  if (typeof error === "string") {
    return trim_diagnostic_text(error) || "Unknown error";
  }
  return trim_diagnostic_text(String(error)) || "Unknown error";
}

function diagnostic_name(error: unknown): string | undefined {
  if (error instanceof Error) {
    return trim_diagnostic_text(error.name);
  }
  return undefined;
}

function diagnostic_stack(error: unknown): string | undefined {
  if (error instanceof Error) {
    return trim_diagnostic_text(error.stack);
  }
  return undefined;
}

function trim_diagnostic_text(value?: string): string | undefined {
  const normalized = value?.trim();
  if (!normalized) {
    return undefined;
  }
  if (normalized.length <= DESKTOP_DIAGNOSTIC_TEXT_LIMIT) {
    return normalized;
  }
  return `${normalized.slice(0, DESKTOP_DIAGNOSTIC_TEXT_LIMIT)}...`;
}

function get_desktop_ready_performance(): DesktopWebReadyPerformance {
  const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming | undefined;
  const paint_entries = performance.getEntriesByType("paint");
  const first_contentful_paint = paint_entries.find((entry) => entry.name === "first-contentful-paint");
  const payload: DesktopWebReadyPerformance = {
    ready_ms: rounded_milliseconds(performance.now()),
    marks: performance.getEntriesByType("mark")
      .filter((entry) => entry.name.startsWith("nexus."))
      .map((entry) => ({
        name: entry.name,
        start_time_ms: rounded_milliseconds(entry.startTime),
      })),
  };

  if (navigation) {
    payload.response_end_ms = rounded_milliseconds(navigation.responseEnd);
    payload.dom_content_loaded_ms = rounded_milliseconds(navigation.domContentLoadedEventEnd);
    payload.load_event_end_ms = rounded_milliseconds(navigation.loadEventEnd);
  }
  if (first_contentful_paint) {
    payload.first_contentful_paint_ms = rounded_milliseconds(first_contentful_paint.startTime);
  }
  return payload;
}

function rounded_milliseconds(value: number): number {
  return Math.round(value * 10) / 10;
}

function prefers_reduced_motion(): boolean {
  if (typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
