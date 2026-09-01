import type { ReactNode } from "react";

import "@/app/globals.css";
import {
  applyDesktopRuntimeDocumentFlags,
  isDesktopRuntime,
  markDesktopPerformance,
  notifyDesktopWebFatal,
} from "@/config/desktop-runtime";
import { hydrateRuntimeOptions } from "@/app/runtime-options-resource";
import { isStrictModeEnabled } from "@/config/conversation-policy";
import { getErrorMessage } from "@/lib/error-message";
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
  void bootstrap(render, true);
}

// OAuth 回调运行在没有 Web 登录态和桌面 token 的系统浏览器中，必须先渲染公开回调页，
// 不能在 token 交换前预取受保护的 runtime/options。
export function bootstrapPublicReactApp(render: () => ReactNode): void {
  void bootstrap(render, false);
}

async function bootstrap(
  render: () => ReactNode,
  shouldHydrateRuntimeOptions: boolean,
): Promise<void> {
  markDesktopPerformance("bootstrap.start");
  installGlobalErrorHandlers();
  applyDesktopRuntimeDocumentFlags();
  applyTheme(detectInitialTheme());

  try {
    if (shouldHydrateRuntimeOptions) {
      markDesktopPerformance("runtimeOptions.hydrateBegin");
      await hydrateRuntimeOptions();
      markDesktopPerformance("runtimeOptions.hydrateEnd");
    }
    const strictMode = isStrictModeEnabled();
    renderApplication(render, strictMode);
    if (isDesktopRuntime()) {
      startAppRenderWatchdog((reason) => renderRecoveryScreen(reason, strictMode));
    }
  } catch (error) {
    notifyDesktopWebFatal("bootstrap", error);
    if (shouldRecoverAfterDesktopRuntimeAuthError(error)) {
      markDesktopPerformance("runtimeOptions.authReload");
      return;
    }

    // 启动失败必须进入可见错误面，避免生产环境停留在无法诊断的空白根节点。
    const message = getErrorMessage(error, "暂时无法加载运行时配置");
    console.error("Bootstrap failed:", error);
    markDesktopPerformance("bootstrap.error");
    renderBootstrapError(message, isStrictModeEnabled());
  }
}
