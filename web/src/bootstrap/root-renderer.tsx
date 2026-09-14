import { StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";

import {
  markDesktopPerformance,
  notifyDesktopRenderHealth,
  notifyDesktopWebReady,
} from "@/config/desktop-runtime";

import { RootErrorBoundary, RootFailureScreen } from "./root-failure-view";

import { getRootFailureCopy } from "./root-failure-copy";

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

export function renderBootstrapError(_message: string, strictMode: boolean): void {
  markDesktopPerformance("react.error_render_begin");
  renderRoot(
    <RootFailureScreen
      message={getRootFailureCopy().message}
      size="compact"
      title={getRootFailureCopy().startup}
    />,
    strictMode,
  );
  markDesktopPerformance("react.error_render_scheduled");
  notifyReadyAfterPaint();
}

export function renderRecoveryScreen(_reason: string, strictMode: boolean): void {
  if (!container.isConnected) {
    document.body.appendChild(container);
  }
  markDesktopPerformance("react.recovery_render_begin");
  renderRoot(
    <RootFailureScreen
      message={getRootFailureCopy().message}
      title={getRootFailureCopy().page}
    />,
    strictMode,
  );
  markDesktopPerformance("react.recovery_render_scheduled");
  notifyDesktopRenderHealth("recoveryScreen", "ready");
}
