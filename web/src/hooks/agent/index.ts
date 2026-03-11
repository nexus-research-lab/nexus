/**
 * useAgentSession Hook - 主入口
 *
 * 管理Agent会话的WebSocket连接、消息处理和会话状态
 *
 * [INPUT]: 依赖 react, @/lib/websocket, @/store/session, @/types, ./types, ./session-operations, ./message-reducers
 * [OUTPUT]: 对外提供 useAgentSession hook
 * [POS]: hooks/agent 模块主入口，被 ChatInterface 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { useWebSocket } from '@/lib/websocket';
import { useSessionStore } from '@/store/session';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { Message, StreamEvent, ToolCall, UserMessage } from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/permission';
import { UseAgentSessionOptions, UseAgentSessionReturn } from './types';
import {
  createClearSession,
  createLoadHistoryMessages,
  createLoadSession,
  createResetSession,
  createStartSession,
} from './session-operations';
import { deleteRound as deleteRoundApi } from '@/lib/agent-api';
import {
  reduceIncomingMessage,
  extractToolCallsFromMessage,
  mergeToolCalls,
} from './message-reducers';

// ==================== Hook实现 ====================

interface ConversationEventPayload {
  event_id: string;
  seq: number;
  turn_id: string;
  kind: 'message_upsert' | 'message_delta';
  message?: Message;
  delta?: StreamEvent;
}

interface WorkspaceEventPayload {
  type: 'file_write_start' | 'file_write_delta' | 'file_write_end';
  agent_id: string;
  path: string;
  version: number;
  source: 'agent' | 'api' | 'system' | 'unknown';
  session_key?: string | null;
  tool_use_id?: string | null;
  content_snapshot?: string | null;
  appended_text?: string | null;
  diff_stats?: {
    additions: number;
    deletions: number;
    changed_lines: number;
  } | null;
  timestamp: string;
}

export function useAgentSession(options: UseAgentSessionOptions = {}): UseAgentSessionReturn {
  const wsUrl = options.wsUrl || process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8010/agent/v1/chat/ws';

  // 状态
  const [messages, setMessages] = useState<Message[]>([]);
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // sessionKey 初始为 null，只在创建或加载 session 时设置
  const [sessionKey, setSessionKey] = useState<string | null>(null);
  // 权限请求状态
  const [pendingPermission, setPendingPermission] = useState<PendingPermission | null>(null);

  // Store
  const { updateSession } = useSessionStore();
  const applyWorkspaceEvent = useWorkspaceLiveStore((state) => state.applyEvent);

  // Refs
  const abortControllerRef = useRef<AbortController | null>(null);

  /**
   * 处理WebSocket消息
   */
  const handleWebSocketMessage = useCallback((backendMsg: any) => {
    // 处理错误
    if (backendMsg.error_type) {
      console.error('[useAgentSession] Error:', backendMsg);
      setError(backendMsg.message || 'Unknown error');
      setIsLoading(false);
      return;
    }

    // 处理事件
    if (backendMsg.event_type) {
      // 处理权限请求事件
      if (backendMsg.event_type === 'permission_request') {
        const data = backendMsg.data || {};
        console.debug('[useAgentSession] Permission request:', data);
        setPendingPermission({
          request_id: data.request_id,
          tool_name: data.tool_name,
          tool_input: data.tool_input || {},
          risk_level: data.risk_level,
          risk_label: data.risk_label,
          summary: data.summary,
          suggestions: data.suggestions || [],
          expires_at: data.expires_at,
        });
        return;
      }

      if (backendMsg.event_type === 'conversation_event') {
        const payload = backendMsg.data as ConversationEventPayload;
        const messageSessionKey = backendMsg.agent_id || sessionKey;
        if (!payload || !messageSessionKey) {
          return;
        }

        if (payload.kind === 'message_delta' && payload.delta) {
          setMessages(prev => reduceIncomingMessage(prev, payload.delta!, messageSessionKey, payload.turn_id || ''));
          setIsLoading(true);
          return;
        }

        if (payload.kind === 'message_upsert' && payload.message) {
          setMessages(prev => reduceIncomingMessage(prev, payload.message!, messageSessionKey, payload.turn_id || ''));

          const toolCallsFromMessage = extractToolCallsFromMessage(payload.message);
          if (toolCallsFromMessage.length > 0) {
            setToolCalls(prev => mergeToolCalls(prev, toolCallsFromMessage));
          }

          if (payload.message.role === 'result') {
            setIsLoading(false);
          } else if (payload.message.role === 'assistant') {
            setIsLoading(true);
          }
          return;
        }
      }

      if (backendMsg.event_type === 'workspace_event') {
        const payload = backendMsg.data as WorkspaceEventPayload;
        if (payload?.agent_id && payload?.path) {
          applyWorkspaceEvent(payload);
        }
        return;
      }
    }
  }, [applyWorkspaceEvent, sessionKey]);

  // WebSocket
  const { state: wsState, send: wsSend } = useWebSocket({
    url: wsUrl,
    autoConnect: true,  // 启用自动连接
    reconnect: true,
    heartbeatInterval: 0,
    onMessage: handleWebSocketMessage,
    onError: (event) => {
      const errorMsg = 'WebSocket error occurred';
      console.error('[useAgentSession] WebSocket error:', event);
      setError(errorMsg);
      if (options.onError) {
        options.onError(new Error(errorMsg));
      }
    },
  });

  useEffect(() => {
    const agentId = options.agentId;
    if (!agentId || wsState !== 'connected') {
      return;
    }

    wsSend({
      type: 'subscribe_workspace',
      agent_id: agentId,
    });

    return () => {
      wsSend({
        type: 'unsubscribe_workspace',
        agent_id: agentId,
      });
    };
  }, [options.agentId, wsSend, wsState]);
  /**
   * 发送消息
   */
  const sendMessage = useCallback(async (content: string) => {
    if (!content.trim()) return;

    if (!sessionKey) {
      const errorMsg = '请先选择或创建会话';
      console.error('[sendMessage] No sessionKey available');
      setError(errorMsg);
      return;
    }

    if (wsState !== 'connected') {
      const errorMsg = 'WebSocket未连接,请稍候重试';
      console.error('[sendMessage] WebSocket not connected, state:', wsState);
      setError(errorMsg);
      return;
    }

    console.debug('[sendMessage] 发送消息, sessionKey:', sessionKey);

    try {
      // 创建用户消息
      const message_id = crypto.randomUUID();
      const userMessage: Message = {
        message_id: message_id,
        round_id: message_id,
        agent_id: sessionKey,
        role: 'user',
        content,
        timestamp: Date.now(),
      };

      // 先添加到UI
      setMessages(prev => [...prev, userMessage]);
      setIsLoading(true);
      setError(null);

      // 发送到后端，带上 round_id 保证前后端一致
      wsSend({
        type: 'chat',
        content,
        session_key: sessionKey,
        agent_id: sessionKey,
        round_id: message_id,
      });

      console.debug('[sendMessage] 消息发送成功');
    } catch (err) {
      console.error('[sendMessage] 发送消息失败:', err);
      setError(err instanceof Error ? err.message : 'Failed to send message');
      setIsLoading(false);
    }
  }, [wsState, sessionKey, wsSend]);

  /**
   * 停止生成
   */
  const stopGeneration = useCallback(() => {
    const latestUserRoundId = [...messages]
      .reverse()
      .find(message => message.role === 'user')?.round_id;

    console.debug('[useAgentSession] 停止生成被调用:', {
      sessionKey,
      roundId: latestUserRoundId,
      wsState,
      hasAbortController: !!abortControllerRef.current,
      hasWsSend: !!wsSend
    });

    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }

    // 发送到后端
    if (sessionKey && wsSend) {
      const interruptMsg: { type: 'interrupt'; session_key: string; agent_id: string; round_id?: string } = {
        type: 'interrupt',
        session_key: sessionKey,
        agent_id: sessionKey,
      };
      if (latestUserRoundId) {
        interruptMsg.round_id = latestUserRoundId;
      }
      console.debug('[useAgentSession] 发送停止消息:', interruptMsg);
      console.debug('[useAgentSession] WebSocket 状态:', wsState);

      try {
        wsSend(interruptMsg);
        console.debug('[useAgentSession] 停止消息已发送');
      } catch (error) {
        console.error('[useAgentSession] 发送停止消息失败:', error);
      }
    } else {
      console.warn('[useAgentSession] 无法发送停止消息:', {
        sessionKey: !!sessionKey,
        wsSend: !!wsSend,
        wsState
      });
    }

    setIsLoading(false);
    setToolCalls([]);

  }, [sessionKey, messages, wsSend, wsState]);
  /**
   * 发送权限响应（也用于 AskUserQuestion）
   */
  const sendPermissionResponse = useCallback((payload: PermissionDecisionPayload) => {
    if (!pendingPermission) return;
    if (wsState !== 'connected') {
      setError('WebSocket未连接，无法提交权限决策');
      return;
    }

    const response: Record<string, any> = {
      type: 'permission_response',
      request_id: pendingPermission.request_id,
      session_key: sessionKey,
      agent_id: sessionKey,
      decision: payload.decision,
      message: payload.message || (payload.decision === 'deny' ? 'User denied permission' : ''),
      interrupt: payload.interrupt ?? false,
    };

    // 如果是 AskUserQuestion，附带用户答案
    if (payload.userAnswers && payload.userAnswers.length > 0) {
      response.user_answers = payload.userAnswers;
    }

    if (payload.updatedPermissions && payload.updatedPermissions.length > 0) {
      response.updated_permissions = payload.updatedPermissions;
    }

    console.debug('[useAgentSession] Sending permission response:', response);
    wsSend(response as any);
    setPendingPermission(null);
  }, [pendingPermission, sessionKey, wsSend, wsState]);

  /**
   * 删除一轮对话
   */
  const deleteRound = useCallback(async (roundId: string) => {
    if (!sessionKey) {
      console.error('[deleteRound] No sessionKey available');
      return;
    }

    try {
      await deleteRoundApi(sessionKey, roundId);
      // 从本地消息中移除
      setMessages(prev => prev.filter(m => m.round_id !== roundId));
      console.debug('[deleteRound] 删除成功:', roundId);
    } catch (err) {
      console.error('[deleteRound] 删除失败:', err);
      setError(err instanceof Error ? err.message : 'Failed to delete round');
    }
  }, [sessionKey]);

  /**
   * 重新生成最后一轮回答
   * 保留用户问题，只删除回答后重新生成
   */
  const regenerate = useCallback(async (roundId: string) => {
    // 使用 ref 获取最新的 messages
    if (!sessionKey) {
      console.error('[regenerate] No sessionKey or messages');
      return;
    }

    // 找到最后一轮的用户消息
    const lastUserMessage = messages.findLast(m => m.role === 'user' && m.message_id === roundId);
    console.debug('[regenerate] 找到最后一轮用户消息:', lastUserMessage);

    if (!lastUserMessage) {
      console.error('[regenerate] No user message found');
      return;
    }
    const lastContent = (lastUserMessage as UserMessage).content ?? '';

    try {
      // 1. 删除后端的整轮数据
      await deleteRound(roundId);

      // 2. 发送消息
      await sendMessage(lastContent);

      console.debug('[regenerate] 重新生成成功，保留用户问题');
    } catch (err) {
      console.error('[regenerate] 重新生成失败:', err);
      setError(err instanceof Error ? err.message : 'Failed to regenerate');
      setIsLoading(false);
    }
  }, [sessionKey, messages, wsSend]);

  // 创建操作函数
  const loadHistoryMessages = useCallback(
    createLoadHistoryMessages(setMessages, updateSession),
    [updateSession]
  );

  const startSession = useCallback(
    createStartSession(setSessionKey, setMessages, setToolCalls, setError, setIsLoading),
    []
  );

  const loadSession = useCallback(
    createLoadSession(setSessionKey, setMessages, setError),
    []
  );

  const clearSession = useCallback(
    createClearSession(setMessages, setToolCalls, setError, setIsLoading, setSessionKey, abortControllerRef),
    []
  );

  const resetSession = useCallback(
    createResetSession(startSession),
    [startSession]
  );

  // 清理
  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  return {
    error,
    messages,
    toolCalls,
    sessionKey,
    isLoading,
    pendingPermission,
    sendMessage,
    startSession,
    loadSession,
    clearSession,
    resetSession,
    loadHistoryMessages,
    stopGeneration,
    deleteRound,
    regenerate,
    sendPermissionResponse,
  };
}

// 导出类型
export type { UseAgentSessionOptions, UseAgentSessionReturn } from './types';
