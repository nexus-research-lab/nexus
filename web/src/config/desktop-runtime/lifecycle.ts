/** WebView 启动、崩溃与空白页诊断的宿主消息协议。 */

import { currentDesktopLocationPath } from "./desktop-location";
import { getDesktopRuntimeConfig, isDesktopRuntime } from "./runtime-config";

interface DesktopPerformanceMark {
  name: string;
  startTimeMs: number;
}

interface DesktopWebReadyPerformance {
  domContentLoadedMs?: number;
  firstContentfulPaintMs?: number;
  loadEventEndMs?: number;
  marks: DesktopPerformanceMark[];
  readyMs: number;
  responseEndMs?: number;
}

export interface DesktopRenderSnapshot {
  bodyChildren: number;
  bodyTextLength: number;
  hasRoot: boolean;
  href: string;
  path: string;
  readyState: DocumentReadyState;
  rootChildren: number;
  rootTextLength: number;
  title: string;
}

export type DesktopRenderHealthStatus = "blank_root" | "empty_root" | "ready";

interface DesktopWebReadyMessage {
  kind: "web.ready";
  location: string;
  performance: DesktopWebReadyPerformance;
  reducedMotion: boolean;
  source: string;
}

interface DesktopWebFatalMessage {
  componentStack?: string;
  kind: "web.fatal";
  location: string;
  message: string;
  name?: string;
  performance: DesktopWebReadyPerformance;
  snapshot: DesktopRenderSnapshot;
  source: string;
  stack?: string;
}

interface DesktopWebHealthMessage {
  kind: "web.health";
  location: string;
  performance: DesktopWebReadyPerformance;
  snapshot: DesktopRenderSnapshot;
  source: string;
  status: DesktopRenderHealthStatus;
}

type DesktopLifecycleMessage =
  | DesktopWebFatalMessage
  | DesktopWebHealthMessage
  | DesktopWebReadyMessage;

const DESKTOP_DIAGNOSTIC_TEXT_LIMIT = 4_096;

declare global {
  interface Window {
    webkit?: {
      messageHandlers?: {
        nexusDesktopLifecycle?: {
          postMessage: (message: Record<string, unknown>) => void;
        };
      };
    };
  }
}

export function markDesktopPerformance(name: string): void {
  if (!getDesktopRuntimeConfig()) return;
  try {
    performance.mark(`nexus.${name}`);
  } catch {
    // 性能标记只用于诊断，启动流程不能依赖浏览器是否支持该 API。
  }
}

export function notifyDesktopWebReady(source = "unknown"): void {
  markDesktopPerformance("web.ready");
  postDesktopLifecycleMessage({
    kind: "web.ready",
    location: window.location.pathname || "/",
    performance: getDesktopReadyPerformance(),
    reducedMotion: prefersReducedMotion(),
    source,
  });
}

export function notifyDesktopWebFatal(
  source: string,
  error: unknown,
  details: { componentStack?: string } = {},
): void {
  if (!isDesktopRuntime()) return;
  markDesktopPerformance(`web.fatal.${source}`);
  postDesktopLifecycleMessage({
    componentStack: trimDiagnosticText(details.componentStack),
    kind: "web.fatal",
    location: currentDesktopLocationPath(),
    message: diagnosticMessage(error),
    name: diagnosticName(error),
    performance: getDesktopReadyPerformance(),
    snapshot: getDesktopRenderSnapshot(),
    source,
    stack: diagnosticStack(error),
  });
}

export function notifyDesktopRenderHealth(
  source: string,
  status: DesktopRenderHealthStatus,
): void {
  if (!isDesktopRuntime()) return;
  markDesktopPerformance(`web.health.${status}`);
  postDesktopLifecycleMessage({
    kind: "web.health",
    location: currentDesktopLocationPath(),
    performance: getDesktopReadyPerformance(),
    snapshot: getDesktopRenderSnapshot(),
    source,
    status,
  });
}

export function getDesktopRenderSnapshot(): DesktopRenderSnapshot {
  const root = document.getElementById("root");
  const body = document.body;
  return {
    bodyChildren: body?.childElementCount ?? -1,
    bodyTextLength: body?.innerText?.length ?? -1,
    hasRoot: Boolean(root),
    href: window.location.href,
    path: currentDesktopLocationPath(),
    readyState: document.readyState,
    rootChildren: root?.childElementCount ?? -1,
    rootTextLength: root?.innerText?.trim().length ?? -1,
    title: document.title,
  };
}

function postDesktopLifecycleMessage(message: DesktopLifecycleMessage): void {
  window.webkit?.messageHandlers?.nexusDesktopLifecycle?.postMessage(
    toDesktopLifecyclePayload(message),
  );
}

function toDesktopLifecyclePayload(
  message: DesktopLifecycleMessage,
): Record<string, unknown> {
  const payload = {
    kind: message.kind,
    location: message.location,
    performance: toDesktopPerformancePayload(message.performance),
    source: message.source,
  };
  if (message.kind === "web.ready") {
    return { ...payload, reduced_motion: message.reducedMotion };
  }
  if (message.kind === "web.fatal") {
    return {
      ...payload,
      component_stack: message.componentStack,
      message: message.message,
      name: message.name,
      snapshot: toDesktopRenderSnapshotPayload(message.snapshot),
      stack: message.stack,
    };
  }
  return {
    ...payload,
    snapshot: toDesktopRenderSnapshotPayload(message.snapshot),
    status: message.status,
  };
}

function toDesktopPerformancePayload(
  performancePayload: DesktopWebReadyPerformance,
): Record<string, unknown> {
  return {
    dom_content_loaded_ms: performancePayload.domContentLoadedMs,
    first_contentful_paint_ms: performancePayload.firstContentfulPaintMs,
    load_event_end_ms: performancePayload.loadEventEndMs,
    marks: performancePayload.marks.map((mark) => ({
      name: mark.name,
      start_time_ms: mark.startTimeMs,
    })),
    ready_ms: performancePayload.readyMs,
    response_end_ms: performancePayload.responseEndMs,
  };
}

function toDesktopRenderSnapshotPayload(
  snapshot: DesktopRenderSnapshot,
): Record<string, unknown> {
  return {
    body_children: snapshot.bodyChildren,
    body_text_length: snapshot.bodyTextLength,
    has_root: snapshot.hasRoot,
    href: snapshot.href,
    path: snapshot.path,
    ready_state: snapshot.readyState,
    root_children: snapshot.rootChildren,
    root_text_length: snapshot.rootTextLength,
    title: snapshot.title,
  };
}

function diagnosticMessage(error: unknown): string {
  if (error instanceof Error) return trimDiagnosticText(error.message) || error.name;
  const message = typeof error === "string" ? error : String(error);
  return trimDiagnosticText(message) || "Unknown error";
}

function diagnosticName(error: unknown): string | undefined {
  return error instanceof Error ? trimDiagnosticText(error.name) : undefined;
}

function diagnosticStack(error: unknown): string | undefined {
  return error instanceof Error ? trimDiagnosticText(error.stack) : undefined;
}

function trimDiagnosticText(value?: string): string | undefined {
  const normalized = value?.trim();
  if (!normalized) return undefined;
  return normalized.length <= DESKTOP_DIAGNOSTIC_TEXT_LIMIT
    ? normalized
    : `${normalized.slice(0, DESKTOP_DIAGNOSTIC_TEXT_LIMIT)}...`;
}

function getDesktopReadyPerformance(): DesktopWebReadyPerformance {
  const navigation = performance.getEntriesByType("navigation")[0] as
    | PerformanceNavigationTiming
    | undefined;
  const firstContentfulPaint = performance
    .getEntriesByType("paint")
    .find((entry) => entry.name === "first-contentful-paint");
  const payload: DesktopWebReadyPerformance = {
    marks: performance
      .getEntriesByType("mark")
      .filter((entry) => entry.name.startsWith("nexus."))
      .map((entry) => ({
        name: entry.name,
        startTimeMs: roundedMilliseconds(entry.startTime),
      })),
    readyMs: roundedMilliseconds(performance.now()),
  };
  if (navigation) {
    payload.domContentLoadedMs = roundedMilliseconds(navigation.domContentLoadedEventEnd);
    payload.loadEventEndMs = roundedMilliseconds(navigation.loadEventEnd);
    payload.responseEndMs = roundedMilliseconds(navigation.responseEnd);
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
  return typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
