import { Component, type ErrorInfo, type ReactNode } from "react";

import { notifyDesktopWebFatal } from "@/config/desktop-runtime";

import { recoverFromChunkLoadError } from "./recovery/chunk-error-recovery";

interface RootFailureScreenProps {
  title: string;
  description: ReactNode;
  impact: ReactNode;
  nextStep: ReactNode;
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
  description,
  impact,
  nextStep,
  size = "wide",
}: RootFailureScreenProps) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className={`surface-panel surface-radius-xl w-full ${ROOT_FAILURE_WIDTH_CLASSES[size]} border px-8 py-9 text-center`}>
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-semibold">
          N
        </div>
        <h1 className="text-lg font-semibold text-(--text-strong)">{title}</h1>
        <p className="mt-2 text-base leading-6 text-(--text-muted)">{description}</p>
        <p className="mt-3 text-sm leading-6 text-(--text-muted)">{impact}</p>
        <p className="mt-1 text-sm font-medium leading-6 text-(--text-default)">
          {nextStep}
        </p>
        <button
          className="mt-5 inline-flex h-10 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground transition hover:opacity-90 motion-reduce:transition-none"
          onClick={() => window.location.reload()}
          type="button"
        >
          刷新页面
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
          description="当前页面在显示内容时发生异常。"
          impact="已经确认保存的内容不会因此被撤销；尚未确认结果的操作需要刷新后核对。"
          nextStep="刷新页面重新加载当前状态。若刚刚安装了新版本，刷新也会载入最新资源。"
          title="界面渲染失败"
        />
      );
    }
    return this.props.children;
  }
}
