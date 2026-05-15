export type DesktopRuntimeConfig = {
  api_base_url?: string;
  ws_url?: string;
  auth_token?: string;
  app_mode?: string;
  app_version?: string;
  build_number?: string;
  platform?: string;
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

type DesktopLifecycleMessage = {
  kind: "web.ready";
  location: string;
  source: string;
  performance: DesktopWebReadyPerformance;
};

export const DESKTOP_SESSION_TOKEN_HEADER = "X-Nexus-Desktop-Token";
const DESKTOP_SESSION_TOKEN_PROTOCOL_PREFIX = "nexus.desktop.token.";
const CONNECTOR_OAUTH_CALLBACK_PATH = "/capability/connectors/oauth/callback";
const DESKTOP_CONNECTOR_OAUTH_REDIRECT_URI = "nexus://connectors/oauth/callback";

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
  const lifecycle_handler = window.webkit?.messageHandlers?.nexusDesktopLifecycle;
  if (!lifecycle_handler) {
    return;
  }
  mark_desktop_performance("web.ready");
  lifecycle_handler.postMessage({
    kind: "web.ready",
    location: window.location.pathname || "/",
    source,
    performance: get_desktop_ready_performance(),
  });
}

export function get_connector_oauth_redirect_uri(): string {
  const runtime_config = get_desktop_runtime_config();
  if (runtime_config?.app_mode === "desktop") {
    return DESKTOP_CONNECTOR_OAUTH_REDIRECT_URI;
  }
  return `${window.location.origin}${CONNECTOR_OAUTH_CALLBACK_PATH}`;
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
