"use client";

import { Component, type ErrorInfo, type ReactNode } from "react";

import { RoomChatErrorView } from "./room-chat-error-view";

interface RoomChatErrorBoundaryProps {
  children: ReactNode;
  resetKey: string;
}

interface RoomChatErrorBoundaryState {
  hasError: boolean;
}

export class RoomChatErrorBoundary extends Component<
  RoomChatErrorBoundaryProps,
  RoomChatErrorBoundaryState
> {
  public state: RoomChatErrorBoundaryState = { hasError: false };

  public static getDerivedStateFromError(): RoomChatErrorBoundaryState {
    return { hasError: true };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error("[RoomChatErrorBoundary] 聊天面板渲染失败", error, errorInfo);
  }

  public componentDidUpdate(previousProps: RoomChatErrorBoundaryProps): void {
    if (this.state.hasError && previousProps.resetKey !== this.props.resetKey) {
      // 错误只属于触发它的会话，切换身份后允许新会话重新渲染。
      this.setState({ hasError: false });
    }
  }

  public render(): ReactNode {
    return this.state.hasError
      ? <RoomChatErrorView />
      : this.props.children;
  }
}
