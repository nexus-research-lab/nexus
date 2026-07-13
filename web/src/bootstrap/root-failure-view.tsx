import { Component, type ErrorInfo, type ReactNode } from "react";

import { notifyDesktopWebFatal } from "@/config/desktop-runtime";

import { recoverFromChunkLoadError } from "./recovery/chunk-error-recovery";

interface RootFailureScreenProps {
  title: string;
  description: ReactNode;
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
  size = "wide",
}: RootFailureScreenProps) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className={`surface-panel surface-radius-xl w-full ${ROOT_FAILURE_WIDTH_CLASSES[size]} border px-8 py-9 text-center`}>
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
          N
        </div>
        <h1 className="text-[24px] font-bold text-(--text-strong)">{title}</h1>
        <p className="mt-2 text-[14px] leading-6 text-(--text-muted)">{description}</p>
        <button
          className="mt-5 inline-flex h-10 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground transition hover:opacity-90"
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
          description="当前页面触发了渲染异常，请刷新页面恢复。若刚刚发布了新版本，刷新会重新拉取最新资源。"
          title="界面渲染失败"
        />
      );
    }
    return this.props.children;
  }
}
