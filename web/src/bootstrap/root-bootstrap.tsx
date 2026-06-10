import { Component, ErrorInfo, ReactNode, StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "@/app/globals.css";
import {
  get_desktop_render_snapshot,
  get_desktop_session_token,
  is_desktop_runtime,
  mark_desktop_performance,
  notify_desktop_render_health,
  notify_desktop_web_fatal,
  notify_desktop_web_ready,
  recover_desktop_session_token_error,
} from "@/config/desktop-runtime";
import { get_agent_api_base_url, hydrate_runtime_options, is_strict_mode_enabled } from "@/config/options";
import { apply_theme, detect_initial_theme } from "@/shared/theme/theme-context";

mark_desktop_performance("bootstrap.module_loaded");

const DESKTOP_EMPTY_RENDER_RELOAD_KEY_PREFIX = "nexus:desktop-empty-render-reload:";
const DESKTOP_RENDER_WATCHDOG_INTERVAL_MS = 10_000;
const DESKTOP_RENDER_WATCHDOG_EMPTY_THRESHOLD = 2;

const container = document.getElementById("root");
if (!container) {
  throw new Error("Root container #root not found.");
}
const root = createRoot(container);
let did_install_desktop_global_error_handlers = false;
let did_start_desktop_render_watchdog = false;

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
              当前页面触发了渲染异常，请刷新页面恢复。
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
  start_desktop_render_watchdog();
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

function install_desktop_global_error_handlers() {
  if (!is_desktop_runtime() || did_install_desktop_global_error_handlers) {
    return;
  }

  did_install_desktop_global_error_handlers = true;
  window.addEventListener("error", (event) => {
    notify_desktop_web_fatal("window.error", event.error ?? event.message);
  });
  window.addEventListener("unhandledrejection", (event) => {
    notify_desktop_web_fatal("window.unhandledrejection", event.reason);
  });
}

function start_desktop_render_watchdog() {
  if (!is_desktop_runtime() || did_start_desktop_render_watchdog) {
    return;
  }

  did_start_desktop_render_watchdog = true;
  let consecutive_empty_root_count = 0;
  const check_render_health = (source: string) => {
    const snapshot = get_desktop_render_snapshot();
    if (snapshot.has_root && snapshot.root_children > 0 && snapshot.ready_state !== "loading") {
      consecutive_empty_root_count = 0;
      return;
    }

    consecutive_empty_root_count += 1;
    notify_desktop_render_health(source, "empty_root");
    if (consecutive_empty_root_count < DESKTOP_RENDER_WATCHDOG_EMPTY_THRESHOLD) {
      return;
    }
    if (should_reload_once(DESKTOP_EMPTY_RENDER_RELOAD_KEY_PREFIX, "empty-render")) {
      mark_desktop_performance("web.health.empty_root_reload");
      window.location.reload();
    }
  };

  window.setInterval(() => {
    check_render_health("watchdog");
  }, DESKTOP_RENDER_WATCHDOG_INTERVAL_MS);
  window.addEventListener("focus", () => {
    window.setTimeout(() => check_render_health("focus"), 300);
  });
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      window.setTimeout(() => check_render_health("visibility"), 300);
    }
  });
}

function should_recover_after_desktop_runtime_auth_error(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }

  return recover_desktop_session_token_error(error.message, `${get_agent_api_base_url()}/runtime/options`);
}

function should_reload_once(prefix: string, reason: string): boolean {
  const token = get_desktop_session_token() || "missing";
  const reload_key = `${prefix}${token}:${reason}:${window.location.pathname}`;
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

async function bootstrap(render: () => ReactNode) {
  mark_desktop_performance("bootstrap.start");
  install_desktop_global_error_handlers();
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
