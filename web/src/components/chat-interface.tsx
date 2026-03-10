"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import { useAgentSession } from "@/hooks/agent";
import { MessageItem } from "@/components/message";
import { useExtractTodos } from "@/hooks/use-extract-todos";
import { useSessionLoader } from "@/hooks/use-session-loader";

import ChatHeader from "@/components/header/chat-header";
import ChatInput from "@/components/chat/chat-input";
import { EmptyState } from "@/components/empty-state";
import { Message } from "@/types/message";
import { TodoItem } from "@/components/todo/agent-task-widget";


interface ChatInterfaceProps {
  agentId: string | null;
  sessionKey: string | null;
  onNewSession: () => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onLoadingChange?: (isLoading: boolean) => void;
  onSessionSnapshotChange?: (snapshot: {
    messageCount: number;
    lastActivityAt: number;
    sessionId: string | null;
  }) => void;
}

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
  onTodosChange,
  onLoadingChange,
  onSessionSnapshotChange,
}: ChatInterfaceProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  const {
    error,
    messages,
    toolCalls,
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

  // 滚动到底部的函数
  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    // 使用 requestAnimationFrame + setTimeout 确保 DOM 已完全更新
    requestAnimationFrame(() => {
      setTimeout(() => {
        if (scrollRef.current) {
          scrollRef.current.scrollTo({
            top: scrollRef.current.scrollHeight,
            behavior
          });
        }
      }, 50);
    });
  }, []);

  // 消息变化时自动滚动到底部
  useEffect(() => {
    scrollToBottom('smooth');
  }, [messages, toolCalls, scrollToBottom]);

  const handleSendMessage = async (content: string) => {
    if (!content.trim() || isLoading) return;
    await sendMessage(content);
  };

  const handleStop = () => {
    stopGeneration();
  };

  // 获取所有 roundId 的有序列表
  const roundIds = Array.from(messageGroups.keys());

  return (
    <div className="flex-1 flex flex-col h-full relative overflow-hidden bg-transparent">
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
          {/* Header */}
          <ChatHeader sessionKey={sessionKey} isLoading={isLoading} />

          {/* Messages Area */}
          <div ref={scrollRef} className="soft-scrollbar flex-1 overflow-y-auto p-6 space-y-8 relative z-0 scroll-smooth">
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
                  pendingPermission={pendingPermission}
                  onPermissionResponse={sendPermissionResponse}
                  onDelete={deleteRound}
                  onRegenerate={isLastRound ? regenerate : undefined}
                />
              );
            })}
          </div>

          {/* Input Area */}
          <ChatInput
            isLoading={isLoading}
            onSendMessage={handleSendMessage}
            onStop={handleStop}
          />
        </>
      )}
    </div>
  );
}
