export type DesktopRuntimeConfig = {
  apiBaseUrl?: string;
  wsUrl?: string;
  authToken?: string;
  appMode?: string;
  appVersion?: string;
  buildNumber?: string;
  platform?: string;
  oauthRedirectUri?: string;
};

type DesktopPerformanceMark = {
  name: string;
  startTimeMs: number;
};

type DesktopWebReadyPerformance = {
  readyMs: number;
  responseEndMs?: number;
  domContentLoadedMs?: number;
  loadEventEndMs?: number;
  firstContentfulPaintMs?: number;
  marks: DesktopPerformanceMark[];
};

export type DesktopRenderSnapshot = {
  href: string;
  path: string;
  readyState: DocumentReadyState;
  title: string;
  hasRoot: boolean;
  rootChildren: number;
  rootTextLength: number;
  bodyChildren: number;
  bodyTextLength: number;
};

export type DesktopRenderHealthStatus = "ready" | "empty_root" | "blank_root";

type DesktopLifecycleMessage = DesktopWebReadyMessage | DesktopWebFatalMessage | DesktopWebHealthMessage;

type DesktopWebReadyMessage = {
  kind: "web.ready";
  location: string;
  reducedMotion: boolean;
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
  componentStack?: string;
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
    __NEXUS_DESKTOP_RUNTIME__?: Record<string, unknown>;
    webkit?: {
      messageHandlers?: {
        nexusDesktopLifecycle?: {
          postMessage: (message: Record<string, unknown>) => void;
        };
      };
    };
  }
}

function readRuntimeString(runtimeConfig: Record<string, unknown>, key: string): string | undefined {
  const value = runtimeConfig[key];
  return typeof value === "string" ? value : undefined;
}

function normalizeDesktopRuntimeConfig(runtimeConfig: Record<string, unknown>): DesktopRuntimeConfig {
  return {
    apiBaseUrl: readRuntimeString(runtimeConfig, "api_base_url"),
    wsUrl: readRuntimeString(runtimeConfig, "ws_url"),
    authToken: readRuntimeString(runtimeConfig, "auth_token"),
    appMode: readRuntimeString(runtimeConfig, "app_mode"),
    appVersion: readRuntimeString(runtimeConfig, "app_version"),
    buildNumber: readRuntimeString(runtimeConfig, "build_number"),
    platform: readRuntimeString(runtimeConfig, "platform"),
    oauthRedirectUri: readRuntimeString(runtimeConfig, "oauth_redirect_uri"),
  };
}

export function getDesktopRuntimeConfig(): DesktopRuntimeConfig | null {
  if (typeof window === "undefined") {
    return null;
  }
  const runtimeConfig = window.__NEXUS_DESKTOP_RUNTIME__;
  if (!runtimeConfig || typeof runtimeConfig !== "object") {
    return null;
  }
  return normalizeDesktopRuntimeConfig(runtimeConfig);
}

export function isDesktopRuntime(): boolean {
  return getDesktopRuntimeConfig()?.appMode === "desktop";
}

export function getDesktopSessionToken(): string {
  return getDesktopRuntimeConfig()?.authToken?.trim() || "";
}

export function getDesktopWebsocketProtocols(): string[] {
  const token = getDesktopSessionToken();
  if (!token) {
    return [];
  }
  return ["nexus.desktop.v1", `${DESKTOP_SESSION_TOKEN_PROTOCOL_PREFIX}${token}`];
}

export function applyDesktopRequestHeaders(input: string, headers: Headers): Headers {
  const token = getDesktopSessionToken();
  if (!token || !shouldAttachDesktopSessionToken(input)) {
    return headers;
  }
  if (!headers.has(DESKTOP_SESSION_TOKEN_HEADER)) {
    headers.set(DESKTOP_SESSION_TOKEN_HEADER, token);
  }
  return headers;
}

export function recoverDesktopSessionTokenError(message: string, input: string): boolean {
  if (!isDesktopSessionTokenError(message)) {
    return false;
  }

  const requestPath = desktopRequestPath(input);
  notifyDesktopWebFatal(
    "desktop.session_token_invalid",
    new Error(`${DESKTOP_SESSION_TOKEN_INVALID_DETAIL}: ${requestPath}`),
  );
  markDesktopPerformance("desktop.session_token_invalid");
  if (!shouldReloadForDesktopSessionToken(input)) {
    return false;
  }
  window.setTimeout(() => {
    window.location.reload();
  }, 0);
  return true;
}

function isDesktopSessionTokenError(message: string): boolean {
  return isDesktopRuntime() && message.includes(DESKTOP_SESSION_TOKEN_INVALID_DETAIL);
}

export function markDesktopPerformance(name: string): void {
  if (!getDesktopRuntimeConfig()) {
    return;
  }
  try {
    performance.mark(`nexus.${name}`);
  } catch {
    // 性能标记只用于诊断，启动流程不能依赖它们。
  }
}

export function notifyDesktopWebReady(source = "unknown"): void {
  markDesktopPerformance("web.ready");
  postDesktopLifecycleMessage({
    kind: "web.ready",
    location: window.location.pathname || "/",
    reducedMotion: prefersReducedMotion(),
    source,
    performance: getDesktopReadyPerformance(),
  });
}

export function notifyDesktopWebFatal(
  source: string,
  error: unknown,
  details: { componentStack?: string } = {},
): void {
  if (!isDesktopRuntime()) {
    return;
  }

  markDesktopPerformance(`web.fatal.${source}`);
  postDesktopLifecycleMessage({
    kind: "web.fatal",
    location: currentLocationPath(),
    source,
    message: diagnosticMessage(error),
    name: diagnosticName(error),
    stack: diagnosticStack(error),
    componentStack: trimDiagnosticText(details.componentStack),
    snapshot: getDesktopRenderSnapshot(),
    performance: getDesktopReadyPerformance(),
  });
}

export function notifyDesktopRenderHealth(
  source: string,
  status: DesktopRenderHealthStatus,
): void {
  if (!isDesktopRuntime()) {
    return;
  }

  markDesktopPerformance(`web.health.${status}`);
  postDesktopLifecycleMessage({
    kind: "web.health",
    location: currentLocationPath(),
    source,
    status,
    snapshot: getDesktopRenderSnapshot(),
    performance: getDesktopReadyPerformance(),
  });
}

export function getDesktopRenderSnapshot(): DesktopRenderSnapshot {
  const root = document.getElementById("root");
  const body = document.body;
  return {
    href: window.location.href,
    path: currentLocationPath(),
    readyState: document.readyState,
    title: document.title,
    hasRoot: Boolean(root),
    rootChildren: root?.childElementCount ?? -1,
    rootTextLength: root?.innerText?.trim().length ?? -1,
    bodyChildren: body?.childElementCount ?? -1,
    bodyTextLength: body?.innerText?.length ?? -1,
  };
}

export function getConnectorOauthRedirectUri(): string {
  const runtimeConfig = getDesktopRuntimeConfig();
  if (runtimeConfig?.appMode === "desktop") {
    const configuredUri = runtimeConfig.oauthRedirectUri?.trim();
    if (configuredUri) {
      return configuredUri;
    }
    const apiBaseUrl = runtimeConfig.apiBaseUrl?.trim();
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

export function isDesktopLoopbackOauthCallback(): boolean {
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

export function getDesktopConnectorsReturnUri(): string {
  return DESKTOP_CONNECTORS_RETURN_URI;
}

function postDesktopLifecycleMessage(message: DesktopLifecycleMessage): void {
  const lifecycleHandler = window.webkit?.messageHandlers?.nexusDesktopLifecycle;
  if (!lifecycleHandler) {
    return;
  }
  lifecycleHandler.postMessage(toDesktopLifecyclePayload(message));
}

function toDesktopLifecyclePayload(message: DesktopLifecycleMessage): Record<string, unknown> {
  const basePayload = {
    kind: message.kind,
    location: message.location,
    source: message.source,
    performance: toDesktopPerformancePayload(message.performance),
  };

  if (message.kind === "web.ready") {
    return {
      ...basePayload,
      reduced_motion: message.reducedMotion,
    };
  }
  if (message.kind === "web.fatal") {
    return {
      ...basePayload,
      message: message.message,
      name: message.name,
      stack: message.stack,
      component_stack: message.componentStack,
      snapshot: toDesktopRenderSnapshotPayload(message.snapshot),
    };
  }
  return {
    ...basePayload,
    status: message.status,
    snapshot: toDesktopRenderSnapshotPayload(message.snapshot),
  };
}

function toDesktopPerformancePayload(performancePayload: DesktopWebReadyPerformance): Record<string, unknown> {
  return {
    ready_ms: performancePayload.readyMs,
    response_end_ms: performancePayload.responseEndMs,
    dom_content_loaded_ms: performancePayload.domContentLoadedMs,
    load_event_end_ms: performancePayload.loadEventEndMs,
    first_contentful_paint_ms: performancePayload.firstContentfulPaintMs,
    marks: performancePayload.marks.map((mark) => ({
      name: mark.name,
      start_time_ms: mark.startTimeMs,
    })),
  };
}

function toDesktopRenderSnapshotPayload(snapshot: DesktopRenderSnapshot): Record<string, unknown> {
  return {
    href: snapshot.href,
    path: snapshot.path,
    ready_state: snapshot.readyState,
    title: snapshot.title,
    has_root: snapshot.hasRoot,
    root_children: snapshot.rootChildren,
    root_text_length: snapshot.rootTextLength,
    body_children: snapshot.bodyChildren,
    body_text_length: snapshot.bodyTextLength,
  };
}

function shouldAttachDesktopSessionToken(input: string): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  const runtimeConfig = getDesktopRuntimeConfig();
  const apiBaseUrl = runtimeConfig?.apiBaseUrl?.trim();
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
  const runtimeConfig = getDesktopRuntimeConfig();
  const apiBaseUrl = runtimeConfig?.apiBaseUrl?.trim() || "missing-api";
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
    readyMs: roundedMilliseconds(performance.now()),
    marks: performance.getEntriesByType("mark")
      .filter((entry) => entry.name.startsWith("nexus."))
      .map((entry) => ({
        name: entry.name,
        startTimeMs: roundedMilliseconds(entry.startTime),
      })),
  };

  if (navigation) {
    payload.responseEndMs = roundedMilliseconds(navigation.responseEnd);
    payload.domContentLoadedMs = roundedMilliseconds(navigation.domContentLoadedEventEnd);
    payload.loadEventEndMs = roundedMilliseconds(navigation.loadEventEnd);
  }
  if (firstContentfulPaint) {
    payload.firstContentfulPaintMs = roundedMilliseconds(firstContentfulPaint.startTime);
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
