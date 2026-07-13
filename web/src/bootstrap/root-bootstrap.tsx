import type { ReactNode } from "react";

import "@/app/globals.css";
import {
  applyDesktopRuntimeDocumentFlags,
  markDesktopPerformance,
  notifyDesktopWebFatal,
} from "@/config/desktop-runtime";
import { hydrateRuntimeOptions } from "@/app/runtime-options-resource";
import { isStrictModeEnabled } from "@/config/conversation-policy";
import { applyTheme, detectInitialTheme } from "@/shared/theme/theme-context";

import {
  installGlobalErrorHandlers,
  shouldRecoverAfterDesktopRuntimeAuthError,
} from "./recovery/chunk-error-recovery";
import { startAppRenderWatchdog } from "./recovery/render-watchdog";
import {
  renderApplication,
  renderBootstrapError,
  renderRecoveryScreen,
} from "./root-renderer";

markDesktopPerformance("bootstrap.module_loaded");

export function bootstrapReactApp(render: () => ReactNode): void {
  void bootstrap(render);
}

async function bootstrap(render: () => ReactNode): Promise<void> {
  markDesktopPerformance("bootstrap.start");
  installGlobalErrorHandlers();
  applyDesktopRuntimeDocumentFlags();
  applyTheme(detectInitialTheme());

  try {
    markDesktopPerformance("runtimeOptions.hydrateBegin");
    await hydrateRuntimeOptions();
    markDesktopPerformance("runtimeOptions.hydrateEnd");
    const strictMode = isStrictModeEnabled();
    renderApplication(render, strictMode);
    startAppRenderWatchdog((reason) => renderRecoveryScreen(reason, strictMode));
  } catch (error) {
    notifyDesktopWebFatal("bootstrap", error);
    if (shouldRecoverAfterDesktopRuntimeAuthError(error)) {
      markDesktopPerformance("runtimeOptions.authReload");
      return;
    }

    // 启动失败必须进入可见错误面，避免生产环境停留在无法诊断的空白根节点。
    const message = error instanceof Error ? error.message : "加载运行时配置失败";
    console.error("Bootstrap failed:", error);
    markDesktopPerformance("bootstrap.error");
    renderBootstrapError(message, isStrictModeEnabled());
  }
}
