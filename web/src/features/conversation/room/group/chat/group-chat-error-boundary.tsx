"use client";

import { Component, ErrorInfo, ReactNode } from "react";

interface GroupChatErrorBoundaryProps {
  children: ReactNode;
}

interface GroupChatErrorBoundaryState {
  has_error: boolean;
}

export class GroupChatErrorBoundary extends Component<
  GroupChatErrorBoundaryProps,
  GroupChatErrorBoundaryState
> {
  public state: GroupChatErrorBoundaryState = {
    has_error: false,
  };

  public static getDerivedStateFromError(): GroupChatErrorBoundaryState {
    return { has_error: true };
  }

  public componentDidCatch(error: Error, error_info: ErrorInfo): void {
    console.error("[GroupChatErrorBoundary] 聊天面板渲染失败", error, error_info);
  }

  public render(): ReactNode {
    if (this.state.has_error) {
      return (
        <div className="flex h-full min-h-80 items-center justify-center px-6 py-10">
          <div className="max-w-md rounded-3xl border border-[color:color-mix(in_srgb,var(--destructive)_24%,var(--divider-subtle-color))] bg-(--material-card-background) p-6 text-center shadow-[0_24px_80px_var(--material-panel-shadow)]">
            <p className="text-sm font-semibold text-(--text-strong)">聊天面板渲染失败</p>
            <p className="mt-2 text-sm leading-6 text-(--text-default)">
              当前会话在渲染阶段触发异常，错误详情已经输出到控制台。
            </p>
            <button
              className="mt-4 inline-flex items-center justify-center rounded-full bg-(--button-primary-background) px-4 py-2 text-sm font-semibold text-(--button-primary-color) transition hover:bg-(--button-primary-hover-background)"
              onClick={() => window.location.reload()}
              type="button"
            >
              刷新页面
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
