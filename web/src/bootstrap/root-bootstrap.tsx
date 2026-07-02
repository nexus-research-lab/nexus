import { Component, ErrorInfo, ReactNode, StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "@/app/globals.css";
import {
  apply_desktop_runtime_document_flags,
  get_desktop_render_snapshot,
  get_desktop_session_token,
  mark_desktop_performance,
  notify_desktop_render_health,
  notify_desktop_web_fatal,
  notify_desktop_web_ready,
  recover_desktop_session_token_error,
} from "@/config/desktop-runtime";
import type { DesktopRenderHealthStatus, DesktopRenderSnapshot } from "@/config/desktop-runtime";
import { get_agent_api_base_url, hydrate_runtime_options, is_strict_mode_enabled } from "@/config/options";
import { apply_theme, detect_initial_theme } from "@/shared/theme/theme-context";

mark_desktop_performance("bootstrap.module_loaded");

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

const root_container = document.getElementById("root");
if (!root_container) {
  throw new Error("Root container #root not found.");
}
const container: HTMLElement = root_container;
const root = createRoot(container);
let did_install_global_error_handlers = false;
let did_start_app_render_watchdog = false;

interface RootErrorBoundaryProps {
  children: ReactNode;
}

interface RootErrorBoundaryState {
  has_error: boolean;
}

class RootErrorBoundary extends Component<RootErrorBoundaryProps, RootErrorBoundaryState> {
  public state: RootErrorBoundaryState = {
    has_error: false,
  };

  public static getDerivedStateFromError(): RootErrorBoundaryState {
    return { has_error: true };
  }

  public componentDidCatch(error: Error, error_info: ErrorInfo): void {
    console.error("[RootErrorBoundary] 应用渲染失败", error, error_info);
    notify_desktop_web_fatal("react.render", error, {
      component_stack: error_info.componentStack ?? undefined,
    });
    recover_from_chunk_load_error("react.render", error);
  }

  public render(): ReactNode {
    if (this.state.has_error) {
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

export function bootstrap_react_app(render: () => ReactNode) {
  void bootstrap(render);
}

function with_optional_strict_mode(children: ReactNode) {
  if (!is_strict_mode_enabled()) {
    return children;
  }
  return (
    <StrictMode>
      {children}
    </StrictMode>
  );
}

function render_application(render: () => ReactNode) {
  mark_desktop_performance("react.render_begin");
  root.render(with_optional_strict_mode(
    <RootErrorBoundary>
      {render()}
    </RootErrorBoundary>,
  ));
  mark_desktop_performance("react.render_scheduled");
  notify_ready_after_paint();
  start_app_render_watchdog();
}

function render_bootstrap_error(message: string) {
  mark_desktop_performance("react.error_render_begin");
  root.render(with_optional_strict_mode(
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
  mark_desktop_performance("react.error_render_scheduled");
  notify_ready_after_paint();
}

function notify_ready_after_paint() {
  let did_notify = false;
  const notify_once = (source: string) => {
    if (did_notify) {
      return;
    }
    did_notify = true;
    mark_desktop_performance(`react.ready.${source}`);
    notify_desktop_web_ready(source);
    notify_desktop_render_health(source, "ready");
  };

  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      notify_once("after_paint");
    });
  });
  window.setTimeout(() => {
    notify_once("timer_fallback");
  }, 250);
}

function render_recovery_screen(reason: string) {
  reconnect_root_container();
  mark_desktop_performance("react.recovery_render_begin");
  root.render(with_optional_strict_mode(
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
  mark_desktop_performance("react.recovery_render_scheduled");
  notify_desktop_render_health("recovery_screen", "ready");
}

function reconnect_root_container() {
  if (container.isConnected) {
    return;
  }

  document.body.appendChild(container);
}

function install_global_error_handlers() {
  if (did_install_global_error_handlers) {
    return;
  }

  did_install_global_error_handlers = true;
  window.addEventListener("error", (event) => {
    const error = event.error ?? event.message;
    notify_desktop_web_fatal("window.error", error);
    recover_from_chunk_load_error("window.error", error);
  });
  window.addEventListener("unhandledrejection", (event) => {
    notify_desktop_web_fatal("window.unhandledrejection", event.reason);
    recover_from_chunk_load_error("window.unhandledrejection", event.reason);
  });
}

function start_app_render_watchdog() {
  if (did_start_app_render_watchdog) {
    return;
  }

  did_start_app_render_watchdog = true;
  let consecutive_unhealthy_count = 0;
  const check_render_health = (source: string) => {
    const snapshot = get_desktop_render_snapshot();
    const unhealthy_status = get_render_unhealthy_status(snapshot);
    if (!unhealthy_status) {
      if (consecutive_unhealthy_count > 0) {
        notify_desktop_render_health(source, "ready");
      }
      consecutive_unhealthy_count = 0;
      return;
    }

    consecutive_unhealthy_count += 1;
    notify_desktop_render_health(source, unhealthy_status);
    if (consecutive_unhealthy_count < APP_RENDER_WATCHDOG_UNHEALTHY_THRESHOLD) {
      return;
    }
    if (should_reload_once(APP_BLANK_RENDER_RELOAD_KEY_PREFIX, unhealthy_status)) {
      mark_desktop_performance(`web.health.${unhealthy_status}_reload`);
      window.location.reload();
      return;
    }

    render_recovery_screen(unhealthy_status === "empty_root" ? "根节点为空" : "根节点没有可见内容");
  };

  window.setInterval(() => {
    check_render_health("watchdog");
  }, APP_RENDER_WATCHDOG_INTERVAL_MS);
  window.addEventListener("focus", () => {
    window.setTimeout(() => check_render_health("focus"), 300);
  });
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      window.setTimeout(() => check_render_health("visibility"), 300);
    }
  });
}

function get_render_unhealthy_status(snapshot: DesktopRenderSnapshot): DesktopRenderHealthStatus | null {
  if (snapshot.ready_state === "loading") {
    return null;
  }
  if (!snapshot.has_root || snapshot.root_children <= 0) {
    return "empty_root";
  }
  if (snapshot.root_text_length <= 0 && snapshot.body_text_length <= 0) {
    return "blank_root";
  }
  return null;
}

function should_recover_after_desktop_runtime_auth_error(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }

  return recover_desktop_session_token_error(error.message, `${get_agent_api_base_url()}/runtime/options`);
}

function should_reload_once(prefix: string, reason: string): boolean {
  const runtime_key = get_desktop_session_token() || window.location.origin || "web";
  const reload_key = `${prefix}${runtime_key}:${reason}:${window.location.pathname}`;
  try {
    if (window.sessionStorage.getItem(reload_key) === "1") {
      return false;
    }
    window.sessionStorage.setItem(reload_key, "1");
    return true;
  } catch {
    // 没有可靠的重载哨兵时直接显示错误页，避免陷入刷新循环。
    return false;
  }
}

function recover_from_chunk_load_error(source: string, error: unknown): boolean {
  if (!is_chunk_load_error(error)) {
    return false;
  }

  notify_desktop_web_fatal(`${source}.chunk_load`, error);
  if (!should_reload_once(APP_CHUNK_ERROR_RELOAD_KEY_PREFIX, source)) {
    return false;
  }

  mark_desktop_performance(`web.chunk_error_reload.${source}`);
  window.setTimeout(() => {
    window.location.reload();
  }, 0);
  return true;
}

function is_chunk_load_error(error: unknown): boolean {
  const diagnostic = diagnostic_text(error);
  return CHUNK_ERROR_PATTERNS.some((pattern) => pattern.test(diagnostic));
}

function diagnostic_text(error: unknown): string {
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
  mark_desktop_performance("bootstrap.start");
  install_global_error_handlers();
  apply_desktop_runtime_document_flags();
  apply_theme(detect_initial_theme());
  try {
    mark_desktop_performance("runtime_options.hydrate_begin");
    await hydrate_runtime_options();
    mark_desktop_performance("runtime_options.hydrate_end");
    render_application(render);
  } catch (error) {
    notify_desktop_web_fatal("bootstrap", error);
    if (should_recover_after_desktop_runtime_auth_error(error)) {
      mark_desktop_performance("runtime_options.auth_reload");
      return;
    }
    // 启动期失败时必须把真实错误渲染出来，否则生产环境只会看到空白页或 failed。
    const message = error instanceof Error ? error.message : "加载运行时配置失败";
    console.error("Bootstrap failed:", error);
    mark_desktop_performance("bootstrap.error");
    render_bootstrap_error(message);
  }
}
