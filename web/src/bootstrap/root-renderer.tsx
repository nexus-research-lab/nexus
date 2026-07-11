import { StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";

import {
  markDesktopPerformance,
  notifyDesktopRenderHealth,
  notifyDesktopWebReady,
} from "@/config/desktop-runtime";

import { RootErrorBoundary, RootFailureScreen } from "./root-failure-view";

const rootContainer = document.getElementById("root");
if (!rootContainer) {
  throw new Error("Root container #root not found.");
}

const container: HTMLElement = rootContainer;
const root = createRoot(container);

function renderRoot(children: ReactNode, strictMode: boolean): void {
  const content = (
    <RootErrorBoundary>
      {children}
    </RootErrorBoundary>
  );
  root.render(strictMode ? <StrictMode>{content}</StrictMode> : content);
}

function notifyReadyAfterPaint(): void {
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
    requestAnimationFrame(() => notifyOnce("afterPaint"));
  });
  window.setTimeout(() => notifyOnce("timerFallback"), 250);
}

export function renderApplication(
  render: () => ReactNode,
  strictMode: boolean,
): void {
  markDesktopPerformance("react.render_begin");
  renderRoot(render(), strictMode);
  markDesktopPerformance("react.render_scheduled");
  notifyReadyAfterPaint();
}

export function renderBootstrapError(message: string, strictMode: boolean): void {
  markDesktopPerformance("react.error_render_begin");
  renderRoot(
    <RootFailureScreen
      description={message}
      size="compact"
      title="运行时配置加载失败"
    />,
    strictMode,
  );
  markDesktopPerformance("react.error_render_scheduled");
  notifyReadyAfterPaint();
}

export function renderRecoveryScreen(reason: string, strictMode: boolean): void {
  if (!container.isConnected) {
    document.body.appendChild(container);
  }
  markDesktopPerformance("react.recovery_render_begin");
  renderRoot(
    <RootFailureScreen
      description={<>页面连续检测到空白状态：{reason}。请刷新页面恢复。</>}
      title="界面暂时无法显示"
    />,
    strictMode,
  );
  markDesktopPerformance("react.recovery_render_scheduled");
  notifyDesktopRenderHealth("recoveryScreen", "ready");
}
