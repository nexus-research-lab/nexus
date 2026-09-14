// INPUT: Application or failure content and the committed root readiness lifecycle.
// OUTPUT: Root rendering and desktop readiness after route/auth loading resolves.
// POS: React root owner; readiness observation and failure views have separate owners.
import { StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";

import {
  markDesktopPerformance,
  notifyDesktopRenderHealth,
  notifyDesktopWebReady,
} from "@/config/desktop-runtime";

import { RootErrorBoundary, RootFailureScreen } from "./root-failure-view";

import { getRootFailureCopy } from "./root-failure-copy";
import { observeRootReadiness } from "./root-readiness";

const rootContainer = document.getElementById("root");
if (!rootContainer) {
  throw new Error("Root container #root not found.");
}

const container: HTMLElement = rootContainer;
const root = createRoot(container);
let cancelReadiness: (() => void) | undefined;

function renderRoot(children: ReactNode, strictMode: boolean): void {
  const content = (
    <RootErrorBoundary>
      {children}
    </RootErrorBoundary>
  );
  root.render(strictMode ? <StrictMode>{content}</StrictMode> : content);
}

function notifyReadyAfterPaint(): void {
  cancelReadiness?.();
  cancelReadiness = observeRootReadiness(container, (source) => {
    markDesktopPerformance(`react.ready.${source}`);
    notifyDesktopWebReady(source);
    notifyDesktopRenderHealth(source, "ready");
  });
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
