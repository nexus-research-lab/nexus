"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowDown } from "lucide-react";

import { useAgentSession } from "@/hooks/agent";
import { MessageItem } from "@/components/message";
import { useExtractTodos } from "@/hooks/use-extract-todos";
import { useSessionLoader } from "@/hooks/use-session-loader";

import ChatHeader from "./chat-header";
import ChatInput from "./chat-input";
import { EmptyState } from "./empty-state";
import { Message } from "@/types/message";
import { TodoItem } from "@/components/workspace/agent-task-widget";


interface ChatInterfaceProps {
  agentId: string | null;
  sessionKey: string | null;
  onNewSession: () => void;
  layout?: "desktop" | "mobile";
  onOpenWorkspaceFile?: (path: string) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onLoadingChange?: (isLoading: boolean) => void;
  onSessionSnapshotChange?: (snapshot: {
    sessionKey: string;
    messageCount: number;
    lastActivityAt: number;
    sessionId: string | null;
  }) => void;
}

const BOTTOM_THRESHOLD_PX = 80;

/**
 * 按 roundId 对消息进行分组
 * 每组包含一个用户消息和对应的所有 assistant/result 消息
 */
function groupMessagesByRound(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();

  for (const msg of messages) {
    const roundId = msg.round_id || msg.message_id; // 回退到 messageId
    if (!groups.has(roundId)) {
      groups.set(roundId, []);
    }
    groups.get(roundId)!.push(msg);
  }

  return groups;
}

export function ChatInterface({
  agentId,
  sessionKey: externalSessionKey,
  onNewSession: onNewSession,
  layout = "desktop",
  onOpenWorkspaceFile,
  onTodosChange,
  onLoadingChange,
  onSessionSnapshotChange,
}: ChatInterfaceProps) {
  const isMobileLayout = layout === "mobile";
  const scrollRef = useRef<HTMLDivElement>(null);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  const shouldFollowLatestRef = useRef(true);
  const lastScrollTopRef = useRef(0);
  const pendingScrollFrameRef = useRef<number | null>(null);
  const pendingScrollInnerFrameRef = useRef<number | null>(null);
  const touchStartYRef = useRef<number | null>(null);
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);

  const {
    error,
    messages,
    sessionKey,
    isLoading,
    pendingPermission,
    sendMessage,
    stopGeneration,
    loadSession,
    sendPermissionResponse,
    deleteRound,
    regenerate,
  } = useAgentSession({
    agentId,
    onError: (err) => {
      console.error("Session error:", err);
    },
  });

  // Extract todos using custom hook
  const todos = useExtractTodos(messages, externalSessionKey);

  useEffect(() => {
    onTodosChange?.(todos);
  }, [onTodosChange, todos]);

  useEffect(() => {
    onLoadingChange?.(isLoading);
  }, [isLoading, onLoadingChange]);

  useEffect(() => {
    if (!externalSessionKey) {
      return;
    }

    const lastMessage = messages[messages.length - 1];
    onSessionSnapshotChange?.({
      sessionKey: externalSessionKey,
      messageCount: messages.length,
      lastActivityAt: lastMessage?.timestamp ?? Date.now(),
      sessionId: lastMessage?.session_id ?? null,
    });
  }, [externalSessionKey, messages, onSessionSnapshotChange]);

  // 响应式会话加载 - 统一处理外部 agentId 变化
  useSessionLoader(externalSessionKey, loadSession, "ChatInterface");

  // 按 roundId 分组消息
  const messageGroups = useMemo(() => {
    return groupMessagesByRound(messages);
  }, [messages]);

  const updateFollowState = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }

    const distanceToBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
    const isNearBottom = distanceToBottom <= BOTTOM_THRESHOLD_PX;
    shouldFollowLatestRef.current = isNearBottom;
    setShowScrollToBottom(!isNearBottom);
  }, []);

  const cancelPendingScroll = useCallback(() => {
    if (pendingScrollFrameRef.current !== null) {
      cancelAnimationFrame(pendingScrollFrameRef.current);
      pendingScrollFrameRef.current = null;
    }
    if (pendingScrollInnerFrameRef.current !== null) {
      cancelAnimationFrame(pendingScrollInnerFrameRef.current);
      pendingScrollInnerFrameRef.current = null;
    }
  }, []);

  const scheduleScrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    cancelPendingScroll();

    // 使用底部锚点滚动，避免 scrollHeight 在布局抖动时不准确。
    pendingScrollFrameRef.current = requestAnimationFrame(() => {
      pendingScrollInnerFrameRef.current = requestAnimationFrame(() => {
        bottomAnchorRef.current?.scrollIntoView({
          block: 'end',
          behavior,
        });
      });
    });
  }, [cancelPendingScroll]);

  // 滚动到底部的函数
  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    shouldFollowLatestRef.current = true;
    setShowScrollToBottom(false);
    scheduleScrollToBottom(behavior);
  }, [scheduleScrollToBottom]);

  // 消息变化时仅在“跟随最新输出”模式下自动滚动。
  useEffect(() => {
    if (!shouldFollowLatestRef.current) {
      updateFollowState();
      return;
    }

    scheduleScrollToBottom(isLoading ? 'auto' : 'smooth');
  }, [isLoading, messages, scheduleScrollToBottom, updateFollowState]);

  useEffect(() => {
    updateFollowState();
    lastScrollTopRef.current = scrollRef.current?.scrollTop || 0;
  }, [updateFollowState, externalSessionKey]);

  useEffect(() => {
    return () => {
      cancelPendingScroll();
    };
  }, [cancelPendingScroll]);

  const handleScroll = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }

    const currentScrollTop = container.scrollTop;
    const isScrollingUp = currentScrollTop < lastScrollTopRef.current;
    lastScrollTopRef.current = currentScrollTop;

    if (isScrollingUp) {
      cancelPendingScroll();
    }

    updateFollowState();
  }, [cancelPendingScroll, updateFollowState]);

  const handleWheel = useCallback((event: React.WheelEvent<HTMLDivElement>) => {
    if (event.deltaY < 0) {
      cancelPendingScroll();
    }
  }, [cancelPendingScroll]);

  const handleTouchStart = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    touchStartYRef.current = event.touches[0]?.clientY ?? null;
  }, []);

  const handleTouchMove = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    const currentY = event.touches[0]?.clientY;
    if (currentY === undefined || touchStartYRef.current === null) {
      return;
    }
    if (currentY > touchStartYRef.current) {
      cancelPendingScroll();
    }
  }, [cancelPendingScroll]);

  const handleTouchEnd = useCallback(() => {
    touchStartYRef.current = null;
  }, []);

  const handleJumpToBottom = useCallback(() => {
    scrollToBottom('smooth');
  }, [scrollToBottom]);

  const handleSendMessage = async (content: string) => {
    if (!content.trim() || isLoading) return;
    shouldFollowLatestRef.current = true;
    setShowScrollToBottom(false);
    await sendMessage(content);
    scrollToBottom('auto');
  };

  const handleStop = () => {
    stopGeneration();
  };

  // 获取所有 roundId 的有序列表
  const roundIds = Array.from(messageGroups.keys());

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
      {!isMobileLayout && (
        <>
          <div className="pointer-events-none absolute left-8 top-10 h-24 w-24 rounded-full glow-lilac opacity-35" />
          <div className="pointer-events-none absolute bottom-10 right-10 h-28 w-28 rounded-full glow-green opacity-30" />
        </>
      )}
      {/* WebSocket连接错误提示 */}
      {error && error.includes('服务器') && (
        <div className="absolute top-4 left-1/2 -translate-x-1/2 z-50 max-w-md">
          <div className="rounded-2xl border border-destructive/20 bg-destructive/8 p-3 backdrop-blur-sm">
            <div className="flex items-start gap-3">
              <svg className="w-5 h-5 text-destructive flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24"
                stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <div className="flex-1">
                <p className="text-sm font-medium text-destructive">无法连接到后端服务</p>
                <p className="text-xs text-muted-foreground mt-1">
                  请确保后端服务正在运行 (端口 8010)
                </p>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 如果没有agentId,显示空状态 */}
      {!externalSessionKey ? (
        <EmptyState onNewSession={onNewSession} />
      ) : (
        <>
          {!isMobileLayout && (
            <ChatHeader sessionKey={sessionKey} isLoading={isLoading} />
          )}

          {/* Messages Area */}
          <div
            ref={scrollRef}
            onScroll={handleScroll}
            onWheel={handleWheel}
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}
            className={
              isMobileLayout
                ? "soft-scrollbar relative z-0 min-w-0 flex-1 space-y-4 overflow-x-hidden overflow-y-auto px-1 py-2"
                : "soft-scrollbar relative z-0 min-w-0 flex-1 space-y-6 overflow-x-hidden overflow-y-auto px-2 py-3 sm:px-4 sm:py-5 xl:space-y-8 xl:px-6 xl:py-7"
            }
          >
            {roundIds.map((roundId, idx) => {
              const roundMessages = messageGroups.get(roundId) || [];
              const isLastRound = idx === roundIds.length - 1;

              return (
                <MessageItem
                  key={roundId}
                  roundId={roundId}
                  messages={roundMessages}
                  isLastRound={isLastRound}
                  isLoading={isLoading}
                  pendingPermission={isLastRound ? pendingPermission : null}
                  onPermissionResponse={sendPermissionResponse}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  onDelete={deleteRound}
                  onRegenerate={isLastRound ? regenerate : undefined}
                />
              );
            })}
            <div ref={bottomAnchorRef} className="h-px w-full" />
          </div>

          {showScrollToBottom && (
            <button
              type="button"
              onClick={handleJumpToBottom}
              className={
                isMobileLayout
                  ? "neo-pill absolute bottom-24 right-2 z-20 inline-flex items-center gap-2 rounded-full px-3 py-2 text-sm font-semibold text-foreground transition hover:-translate-y-0.5 hover:text-primary"
                  : "neo-pill absolute bottom-24 right-3 z-20 inline-flex items-center gap-2 rounded-full px-3 py-2 text-sm font-semibold text-foreground transition hover:-translate-y-0.5 hover:text-primary sm:bottom-30 sm:right-8 sm:px-4 sm:py-2.5"
              }
            >
              <ArrowDown className={isLoading ? "h-4 w-4 animate-bounce" : "h-4 w-4"} />
              {!isMobileLayout && <span>回到底部</span>}
            </button>
          )}

          {/* Input Area */}
          <ChatInput
            compact={isMobileLayout}
            isLoading={isLoading}
            onSendMessage={handleSendMessage}
            onStop={handleStop}
          />
        </>
      )}
    </div>
  );
}
