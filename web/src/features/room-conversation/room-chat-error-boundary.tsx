"use client";

import { Component, ErrorInfo, ReactNode } from "react";

interface RoomChatErrorBoundaryProps {
  children: ReactNode;
}

interface RoomChatErrorBoundaryState {
  has_error: boolean;
}

export class RoomChatErrorBoundary extends Component<
  RoomChatErrorBoundaryProps,
  RoomChatErrorBoundaryState
> {
  public state: RoomChatErrorBoundaryState = {
    has_error: false,
  };

  public static getDerivedStateFromError(): RoomChatErrorBoundaryState {
    return { has_error: true };
  }

  public componentDidCatch(error: Error, error_info: ErrorInfo): void {
    console.error("[RoomChatErrorBoundary] 聊天面板渲染失败", error, error_info);
  }

  public render(): ReactNode {
    if (this.state.has_error) {
      return (
        <div className="flex h-full min-h-80 items-center justify-center px-6 py-10">
          <div className="max-w-md rounded-3xl border border-rose-200/70 bg-white/88 p-6 text-center shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-md">
            <p className="text-sm font-semibold text-slate-900">聊天面板渲染失败</p>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              当前会话在渲染阶段触发异常，错误详情已经输出到控制台。
            </p>
            <button
              className="mt-4 inline-flex items-center justify-center rounded-full bg-slate-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-slate-800"
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
