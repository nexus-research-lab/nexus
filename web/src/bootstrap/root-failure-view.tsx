/**
 * INPUT: 启动或根渲染已经确认的失败标题与单句恢复说明。
 * OUTPUT: 不泄露底层原因、只提供刷新动作的全屏失败面。
 * POS: React 根边界的最后用户可见恢复入口；诊断只写宿主日志。
 */
import { Component, type ErrorInfo, type ReactNode } from "react";

import { notifyDesktopWebFatal } from "@/config/desktop-runtime";

import { recoverFromChunkLoadError } from "./recovery/chunk-error-recovery";

interface RootFailureScreenProps {
  title: string;
  message: ReactNode;
  size?: "compact" | "wide";
}

interface RootErrorBoundaryProps {
  children: ReactNode;
}

interface RootErrorBoundaryState {
  hasError: boolean;
}

const ROOT_FAILURE_WIDTH_CLASSES = {
  compact: "max-w-[480px]",
  wide: "max-w-[520px]",
} as const;

export function RootFailureScreen({
  title,
  message,
  size = "wide",
}: RootFailureScreenProps) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className={`surface-panel surface-radius-xl w-full ${ROOT_FAILURE_WIDTH_CLASSES[size]} border px-8 py-9 text-center`}>
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-semibold">
          N
        </div>
        <h1 className="text-lg font-semibold text-(--text-strong)">{title}</h1>
        <p className="mx-auto mt-3 max-w-[420px] text-sm leading-6 text-(--text-muted)">
          {message}
        </p>
        <button
          className="mt-5 inline-flex h-10 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground transition hover:opacity-90 motion-reduce:transition-none"
          onClick={() => window.location.reload()}
          type="button"
        >
          重试
        </button>
      </section>
    </main>
  );
}

export class RootErrorBoundary extends Component<RootErrorBoundaryProps, RootErrorBoundaryState> {
  public state: RootErrorBoundaryState = {hasError: false};

  public static getDerivedStateFromError(): RootErrorBoundaryState {
    return {hasError: true};
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error("[RootErrorBoundary] 应用渲染失败", error, errorInfo);
    notifyDesktopWebFatal("react.render", error, {
      componentStack: errorInfo.componentStack ?? undefined,
    });
    recoverFromChunkLoadError("react.render", error);
  }

  public render(): ReactNode {
    if (this.state.hasError) {
      return (
        <RootFailureScreen
          message="请稍后重试。"
          title="页面暂时无法显示"
        />
      );
    }
    return this.props.children;
  }
}
