import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import "./app/globals.css";
import { hydrate_runtime_options } from "./config/options";
import { apply_theme, detect_initial_theme } from "./shared/theme/theme-context";

const container = document.getElementById("root");
if (!container) {
  throw new Error("Root container #root not found.");
}
const root_container = container;
const root = createRoot(root_container);

function render_application() {
  root.render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

function render_bootstrap_error(message: string) {
  root.render(
    <StrictMode>
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
      </main>
    </StrictMode>,
  );
}

async function bootstrap() {
  apply_theme(detect_initial_theme());
  try {
    await hydrate_runtime_options();
    render_application();
  } catch (error) {
    // 启动期失败时必须把真实错误渲染出来，否则生产环境只会看到空白页或 failed。
    const message = error instanceof Error ? error.message : "加载运行时配置失败";
    console.error("Bootstrap failed:", error);
    render_bootstrap_error(message);
  }
}

void bootstrap();
