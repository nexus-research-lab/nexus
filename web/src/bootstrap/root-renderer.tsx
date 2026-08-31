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
      impact="应用还没有完成启动；这个加载失败没有修改已有数据。"
      nextStep="刷新页面重新加载运行时配置。"
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
      description={<>页面连续检测到空白状态：{reason}。</>}
      impact="当前界面无法继续显示；已经确认保存的内容不会因此被撤销。"
      nextStep="刷新页面重新加载当前状态。"
      title="界面暂时无法显示"
    />,
    strictMode,
  );
  markDesktopPerformance("react.recovery_render_scheduled");
  notifyDesktopRenderHealth("recoveryScreen", "ready");
}
