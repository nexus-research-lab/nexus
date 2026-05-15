import { ReactNode, StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "@/app/globals.css";
import { mark_desktop_performance, notify_desktop_web_ready } from "@/config/desktop-runtime";
import { hydrate_runtime_options, is_strict_mode_enabled } from "@/config/options";
import { apply_theme, detect_initial_theme } from "@/shared/theme/theme-context";

mark_desktop_performance("bootstrap.module_loaded");

const container = document.getElementById("root");
if (!container) {
  throw new Error("Root container #root not found.");
}
const root = createRoot(container);

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
  root.render(with_optional_strict_mode(render()));
  mark_desktop_performance("react.render_scheduled");
  notify_ready_after_paint();
}

function render_bootstrap_error(message: string) {
  mark_desktop_performance("react.error_render_begin");
  root.render(with_optional_strict_mode(
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className="surface-panel radius-shell-xl w-full max-w-[480px] border px-8 py-9 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
          N
        </div>
        <h1 className="text-[24px] font-bold tracking-[-0.04em] text-(--text-strong)">
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

async function bootstrap(render: () => ReactNode) {
  mark_desktop_performance("bootstrap.start");
  apply_theme(detect_initial_theme());
  try {
    mark_desktop_performance("runtime_options.hydrate_begin");
    await hydrate_runtime_options();
    mark_desktop_performance("runtime_options.hydrate_end");
    render_application(render);
  } catch (error) {
    // 启动期失败时必须把真实错误渲染出来，否则生产环境只会看到空白页或 failed。
    const message = error instanceof Error ? error.message : "加载运行时配置失败";
    console.error("Bootstrap failed:", error);
    mark_desktop_performance("bootstrap.error");
    render_bootstrap_error(message);
  }
}
