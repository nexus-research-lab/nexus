import { Component, ErrorInfo, ReactNode, StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "@/app/globals.css";
import {
  getDesktopRenderSnapshot,
  getDesktopSessionToken,
  markDesktopPerformance,
  notifyDesktopRenderHealth,
  notifyDesktopWebFatal,
  notifyDesktopWebReady,
  recoverDesktopSessionTokenError,
} from "@/config/desktop-runtime";
import type { DesktopRenderHealthStatus, DesktopRenderSnapshot } from "@/config/desktop-runtime";
import { getAgentApiBaseUrl, hydrateRuntimeOptions, isStrictModeEnabled } from "@/config/options";
import { applyTheme, detectInitialTheme } from "@/shared/theme/theme-context";

markDesktopPerformance("bootstrap.module_loaded");

const APP_BLANK_RENDER_RELOAD_KEY_PREFIX = "nexus:app-blank-render-reload:";
const APP_CHUNK_ERROR_RELOAD_KEY_PREFIX = "nexus:app-chunk-error-reload:";
const APP_RENDER_WATCHDOG_INTERVAL_MS = 10_000;
const APP_RENDER_WATCHDOG_UNHEALTHY_THRESHOLD = 2;
const CHUNK_ERROR_PATTERNS = [
  /ChunkLoadError/i,
  /Loading chunk [\w-]+ failed/i,
  /Failed to fetch dynamically imported module/i,
  /Importing a module script failed/i,
  /error loading dynamically imported module/i,
  /Unable to preload CSS/i,
];

const rootContainer = document.getElementById("root");
if (!rootContainer) {
  throw new Error("Root container #root not found.");
}
const container: HTMLElement = rootContainer;
const root = createRoot(container);
let didInstallGlobalErrorHandlers = false;
let didStartAppRenderWatchdog = false;

interface RootErrorBoundaryProps {
  children: ReactNode;
}

interface RootErrorBoundaryState {
  hasError: boolean;
}

class RootErrorBoundary extends Component<RootErrorBoundaryProps, RootErrorBoundaryState> {
  public state: RootErrorBoundaryState = {
    hasError: false,
  };

  public static getDerivedStateFromError(): RootErrorBoundaryState {
    return { hasError: true };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error("[RootErrorBoundary] 应用渲染失败", error, errorInfo);
    notifyDesktopWebFatal("react.render", error, {
      componentStack: errorInfo.componentStack ?? undefined,
    });
    recoverFromChunkLoadError("react.render", error);
  }

  public render(): ReactNode {
    if (this.state.hasError) {
      return (
        <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
          <section className="surface-panel surface-radius-xl w-full max-w-[520px] border px-8 py-9 text-center">
            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
              N
            </div>
            <h1 className="text-[24px] font-bold text-(--text-strong)">
              界面渲染失败
            </h1>
            <p className="mt-2 text-[14px] leading-6 text-(--text-muted)">
              当前页面触发了渲染异常，请刷新页面恢复。若刚刚发布了新版本，刷新会重新拉取最新资源。
            </p>
            <button
              className="mt-5 inline-flex h-10 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground transition hover:opacity-90"
              onClick={() => window.location.reload()}
              type="button"
            >
              刷新页面
            </button>
          </section>
        </main>
      );
    }

    return this.props.children;
  }
}

export function bootstrapReactApp(render: () => ReactNode) {
  void bootstrap(render);
}

function withOptionalStrictMode(children: ReactNode) {
  if (!isStrictModeEnabled()) {
    return children;
  }
  return (
    <StrictMode>
      {children}
    </StrictMode>
  );
}

function renderApplication(render: () => ReactNode) {
  markDesktopPerformance("react.render_begin");
  root.render(withOptionalStrictMode(
    <RootErrorBoundary>
      {render()}
    </RootErrorBoundary>,
  ));
  markDesktopPerformance("react.render_scheduled");
  notifyReadyAfterPaint();
  startAppRenderWatchdog();
}

function renderBootstrapError(message: string) {
  markDesktopPerformance("react.error_render_begin");
  root.render(withOptionalStrictMode(
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className="surface-panel surface-radius-xl w-full max-w-[480px] border px-8 py-9 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
          N
        </div>
        <h1 className="text-[24px] font-bold text-(--text-strong)">
          运行时配置加载失败
        </h1>
        <p className="mt-2 text-[14px] leading-6 text-(--text-muted)">{message}</p>
        <button
          className="mt-5 inline-flex h-10 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground transition hover:opacity-90"
          onClick={() => window.location.reload()}
          type="button"
        >
          刷新页面
        </button>
      </section>
    </main>,
  ));
  markDesktopPerformance("react.error_render_scheduled");
  notifyReadyAfterPaint();
}

function notifyReadyAfterPaint() {
  let didNotify = false;
  const notifyOnce = (source: string) => {
    if (didNotify) {
      return;
    }
    didNotify = true;
    markDesktopPerformance(`react.ready.${source}`);
    notifyDesktopWebReady(source);
    notifyDesktopRenderHealth(source, "ready");
  };

  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      notifyOnce("afterPaint");
    });
  });
  window.setTimeout(() => {
    notifyOnce("timerFallback");
  }, 250);
}

function renderRecoveryScreen(reason: string) {
  reconnectRootContainer();
  markDesktopPerformance("react.recovery_render_begin");
  root.render(withOptionalStrictMode(
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className="surface-panel surface-radius-xl w-full max-w-[520px] border px-8 py-9 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
          N
        </div>
        <h1 className="text-[24px] font-bold text-(--text-strong)">
          界面暂时无法显示
        </h1>
        <p className="mt-2 text-[14px] leading-6 text-(--text-muted)">
          页面连续检测到空白状态：{reason}。请刷新页面恢复。
        </p>
        <button
          className="mt-5 inline-flex h-10 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground transition hover:opacity-90"
          onClick={() => window.location.reload()}
          type="button"
        >
          刷新页面
        </button>
      </section>
    </main>,
  ));
  markDesktopPerformance("react.recovery_render_scheduled");
  notifyDesktopRenderHealth("recoveryScreen", "ready");
}

function reconnectRootContainer() {
  if (container.isConnected) {
    return;
  }

  document.body.appendChild(container);
}

function installGlobalErrorHandlers() {
  if (didInstallGlobalErrorHandlers) {
    return;
  }

  didInstallGlobalErrorHandlers = true;
  window.addEventListener("error", (event) => {
    const error = event.error ?? event.message;
    notifyDesktopWebFatal("window.error", error);
    recoverFromChunkLoadError("window.error", error);
  });
  window.addEventListener("unhandledrejection", (event) => {
    notifyDesktopWebFatal("window.unhandledrejection", event.reason);
    recoverFromChunkLoadError("window.unhandledrejection", event.reason);
  });
}

function startAppRenderWatchdog() {
  if (didStartAppRenderWatchdog) {
    return;
  }

  didStartAppRenderWatchdog = true;
  let consecutiveUnhealthyCount = 0;
  const checkRenderHealth = (source: string) => {
    const snapshot = getDesktopRenderSnapshot();
    const unhealthyStatus = getRenderUnhealthyStatus(snapshot);
    if (!unhealthyStatus) {
      if (consecutiveUnhealthyCount > 0) {
        notifyDesktopRenderHealth(source, "ready");
      }
      consecutiveUnhealthyCount = 0;
      return;
    }

    consecutiveUnhealthyCount += 1;
    notifyDesktopRenderHealth(source, unhealthyStatus);
    if (consecutiveUnhealthyCount < APP_RENDER_WATCHDOG_UNHEALTHY_THRESHOLD) {
      return;
    }
    if (shouldReloadOnce(APP_BLANK_RENDER_RELOAD_KEY_PREFIX, unhealthyStatus)) {
      markDesktopPerformance(`web.health.${unhealthyStatus}.reload`);
      window.location.reload();
      return;
    }

    renderRecoveryScreen(unhealthyStatus === "empty_root" ? "根节点为空" : "根节点没有可见内容");
  };

  window.setInterval(() => {
    checkRenderHealth("watchdog");
  }, APP_RENDER_WATCHDOG_INTERVAL_MS);
  window.addEventListener("focus", () => {
    window.setTimeout(() => checkRenderHealth("focus"), 300);
  });
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      window.setTimeout(() => checkRenderHealth("visibility"), 300);
    }
  });
}

function getRenderUnhealthyStatus(snapshot: DesktopRenderSnapshot): DesktopRenderHealthStatus | null {
  if (snapshot.readyState === "loading") {
    return null;
  }
  if (!snapshot.hasRoot || snapshot.rootChildren <= 0) {
    return "empty_root";
  }
  if (snapshot.rootTextLength <= 0 && snapshot.bodyTextLength <= 0) {
    return "blank_root";
  }
  return null;
}

function shouldRecoverAfterDesktopRuntimeAuthError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }

  return recoverDesktopSessionTokenError(error.message, `${getAgentApiBaseUrl()}/runtime/options`);
}

function shouldReloadOnce(prefix: string, reason: string): boolean {
  const runtimeKey = getDesktopSessionToken() || window.location.origin || "web";
  const reloadKey = `${prefix}${runtimeKey}:${reason}:${window.location.pathname}`;
  try {
    if (window.sessionStorage.getItem(reloadKey) === "1") {
      return false;
    }
    window.sessionStorage.setItem(reloadKey, "1");
    return true;
  } catch {
    // 没有可靠的重载哨兵时直接显示错误页，避免陷入刷新循环。
    return false;
  }
}

function recoverFromChunkLoadError(source: string, error: unknown): boolean {
  if (!isChunkLoadError(error)) {
    return false;
  }

  notifyDesktopWebFatal(`${source}.chunkLoad`, error);
  if (!shouldReloadOnce(APP_CHUNK_ERROR_RELOAD_KEY_PREFIX, source)) {
    return false;
  }

  markDesktopPerformance(`web.chunkErrorReload.${source}`);
  window.setTimeout(() => {
    window.location.reload();
  }, 0);
  return true;
}

function isChunkLoadError(error: unknown): boolean {
  const diagnostic = diagnosticText(error);
  return CHUNK_ERROR_PATTERNS.some((pattern) => pattern.test(diagnostic));
}

function diagnosticText(error: unknown): string {
  if (error instanceof Error) {
    return `${error.name}\n${error.message}\n${error.stack ?? ""}`;
  }
  if (typeof error === "string") {
    return error;
  }
  try {
    return JSON.stringify(error);
  } catch {
    return String(error);
  }
}

async function bootstrap(render: () => ReactNode) {
  markDesktopPerformance("bootstrap.start");
  installGlobalErrorHandlers();
  applyTheme(detectInitialTheme());
  try {
    markDesktopPerformance("runtimeOptions.hydrateBegin");
    await hydrateRuntimeOptions();
    markDesktopPerformance("runtimeOptions.hydrateEnd");
    renderApplication(render);
  } catch (error) {
    notifyDesktopWebFatal("bootstrap", error);
    if (shouldRecoverAfterDesktopRuntimeAuthError(error)) {
      markDesktopPerformance("runtimeOptions.authReload");
      return;
    }
    // 启动期失败时必须把真实错误渲染出来，否则生产环境只会看到空白页或 failed。
    const message = error instanceof Error ? error.message : "加载运行时配置失败";
    console.error("Bootstrap failed:", error);
    markDesktopPerformance("bootstrap.error");
    renderBootstrapError(message);
  }
}
