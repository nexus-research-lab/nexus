import { useCallback, useEffect, useRef, useState } from 'react';
import { getAgentWsUrl } from '@/lib/runtime-config';
import { generateUuid } from '@/lib/uuid';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { AssistantMessage, EventMessage, Message, StreamMessage, UserMessage } from '@/types';
import { deleteRound as deleteRoundApi, getSessionMessages } from '@/lib/agent-api';
import { PendingPermission, PermissionDecisionPayload } from '@/types/permission';
import { UseAgentSessionOptions, UseAgentSessionReturn } from './types';

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

function upsertMessage(messages: Message[], incoming: Message): Message[] {
  const existingIndex = messages.findIndex(
    (message) => message.message_id === incoming.message_id,
  );
  if (existingIndex === -1) {
    return [...messages, incoming];
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = incoming;
  return nextMessages;
}

function applyStreamMessage(messages: Message[], event: StreamMessage): Message[] {
  const existingIndex = messages.findIndex(
    (message) => message.role === 'assistant' && message.message_id === event.message_id,
  );

  if (event.type === 'message_start') {
    if (existingIndex !== -1) {
      return messages;
    }
    return [
      ...messages,
      {
        message_id: event.message_id,
        session_key: event.session_key,
        agent_id: event.agent_id,
        round_id: event.round_id,
        session_id: event.session_id,
        role: 'assistant',
        content: [],
        model: event.message?.model,
        timestamp: event.timestamp,
      },
    ];
  }

  if (existingIndex === -1) {
    return messages;
  }

  const assistantMessage = messages[existingIndex] as AssistantMessage;
  const nextMessage: AssistantMessage = {
    ...assistantMessage,
    model: event.message?.model || assistantMessage.model,
    stop_reason: event.message?.stop_reason || assistantMessage.stop_reason,
    usage: event.usage || assistantMessage.usage,
    content: [...assistantMessage.content],
  };

  if (
    (event.type === 'content_block_start' || event.type === 'content_block_delta') &&
    typeof event.index === 'number' &&
    event.content_block
  ) {
    while (nextMessage.content.length <= event.index) {
      nextMessage.content.push({ type: 'text', text: '' });
    }
    nextMessage.content[event.index] = event.content_block;
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = nextMessage;
  return nextMessages;
}

function sortMessages(messages: Message[]): Message[] {
  return [...messages].sort((left, right) => left.timestamp - right.timestamp);
}

export function useAgentSession(options: UseAgentSessionOptions = {}): UseAgentSessionReturn {
  const wsUrl = options.wsUrl || getAgentWsUrl();
  const applyWorkspaceEvent = useWorkspaceLiveStore((state) => state.applyEvent);

  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionKey, setSessionKey] = useState<string | null>(null);
  const [pendingPermission, setPendingPermission] = useState<PendingPermission | null>(null);

  const activeSessionKeyRef = useRef<string | null>(null);
  const loadRequestIdRef = useRef(0);

  const resetSessionView = useCallback((nextError: string | null = null) => {
    setMessages([]);
    setPendingPermission(null);
    setIsLoading(false);
    setError(nextError);
  }, []);

  const isCurrentSessionEvent = useCallback((incomingSessionKey?: string | null) => {
    if (!incomingSessionKey) {
      return false;
    }
    return activeSessionKeyRef.current === incomingSessionKey;
  }, []);

  const handleWebSocketMessage = useCallback((backendMessage: unknown) => {
    const event = backendMessage as EventMessage;
    const incomingSessionKey = event.session_key || null;

    if (event.event_type === 'error') {
      if (incomingSessionKey && !isCurrentSessionEvent(incomingSessionKey)) {
        return;
      }
      setError(event.data?.message || 'Unknown error');
      setIsLoading(false);
      return;
    }

    if (event.event_type === 'permission_request') {
      if (!isCurrentSessionEvent(incomingSessionKey)) {
        return;
      }
      const data = event.data || {};
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

    if (event.event_type === 'workspace_event') {
      const payload = event.data as WorkspaceEventPayload;
      if (payload?.agent_id && payload?.path) {
        applyWorkspaceEvent(payload);
      }
      return;
    }

    if (event.event_type !== 'message') {
      if (event.event_type !== 'stream') {
        return;
      }

      const payload = event.data as StreamMessage;
      const messageSessionKey = payload?.session_key || incomingSessionKey;
      if (!payload || !messageSessionKey || !isCurrentSessionEvent(messageSessionKey)) {
        return;
      }

      setMessages((prev) => applyStreamMessage(prev, payload));
      setIsLoading(true);
      return;
    }

    const payload = event.data as Message;
    const messageSessionKey = payload?.session_key || incomingSessionKey;
    if (!payload || !messageSessionKey || !isCurrentSessionEvent(messageSessionKey)) {
      return;
    }

    setMessages((prev) => upsertMessage(prev, payload));
    if (payload.role === 'result') {
      setPendingPermission(null);
      setIsLoading(false);
      return;
    }
    if (payload.role === 'assistant') {
      setIsLoading(true);
    }
  }, [applyWorkspaceEvent, isCurrentSessionEvent]);

  const { state: wsState, send: wsSend } = useWebSocket({
    url: wsUrl,
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleWebSocketMessage,
    onError: (event) => {
      const errorMessage = 'WebSocket error occurred';
      console.error('[useAgentSession] WebSocket error:', event);
      setError(errorMessage);
      options.onError?.(new Error(errorMessage));
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

  const sendMessage = useCallback(async (content: string) => {
    if (!content.trim()) {
      return;
    }
    if (!sessionKey) {
      setError('请先选择或创建会话');
      return;
    }
    if (wsState !== 'connected') {
      setError('WebSocket未连接,请稍候重试');
      return;
    }

    const roundId = generateUuid();
    activeSessionKeyRef.current = sessionKey;
    const userMessage: Message = {
      message_id: roundId,
      session_key: sessionKey,
      round_id: roundId,
      agent_id: options.agentId || 'main',
      role: 'user',
      content,
      timestamp: Date.now(),
    };

    setMessages((prev) => upsertMessage(prev, userMessage));
    setPendingPermission(null);
    setIsLoading(true);
    setError(null);

    wsSend({
      type: 'chat',
      content,
      session_key: sessionKey,
      agent_id: options.agentId || 'main',
      round_id: roundId,
    });
  }, [options.agentId, sessionKey, wsSend, wsState]);

  const stopGeneration = useCallback(() => {
    if (!sessionKey || wsState !== 'connected') {
      setIsLoading(false);
      return;
    }

    const latestUserRoundId = [...messages]
      .reverse()
      .find((message) => message.role === 'user')?.round_id;

    wsSend({
      type: 'interrupt',
      session_key: sessionKey,
      agent_id: options.agentId || 'main',
      round_id: latestUserRoundId,
    });

    setIsLoading(false);
    setPendingPermission(null);
  }, [messages, options.agentId, sessionKey, wsSend, wsState]);

  const sendPermissionResponse = useCallback((payload: PermissionDecisionPayload) => {
    if (!pendingPermission) {
      return;
    }
    if (!sessionKey || activeSessionKeyRef.current !== sessionKey) {
      setPendingPermission(null);
      return;
    }
    if (wsState !== 'connected') {
      setError('WebSocket未连接，无法提交权限决策');
      return;
    }

    const response: Record<string, unknown> = {
      type: 'permission_response',
      request_id: pendingPermission.request_id,
      session_key: sessionKey,
      agent_id: options.agentId || 'main',
      decision: payload.decision,
      message: payload.message || (payload.decision === 'deny' ? 'User denied permission' : ''),
      interrupt: payload.interrupt ?? false,
    };

    if (payload.userAnswers?.length) {
      response.user_answers = payload.userAnswers;
    }
    if (payload.updatedPermissions?.length) {
      response.updated_permissions = payload.updatedPermissions;
    }

    wsSend(response as never);
    setPendingPermission(null);
  }, [options.agentId, pendingPermission, sessionKey, wsSend, wsState]);

  const deleteRound = useCallback(async (roundId: string) => {
    if (!sessionKey) {
      return;
    }

    try {
      await deleteRoundApi(sessionKey, roundId);
      setMessages((prev) => prev.filter((message) => message.round_id !== roundId));
    } catch (err) {
      console.error('[deleteRound] 删除失败:', err);
      setError(err instanceof Error ? err.message : 'Failed to delete round');
    }
  }, [sessionKey]);

  const regenerate = useCallback(async (roundId: string) => {
    if (!sessionKey) {
      return;
    }

    const lastUserMessage = messages.findLast(
      (message) => message.role === 'user' && message.message_id === roundId,
    ) as UserMessage | undefined;
    if (!lastUserMessage?.content) {
      return;
    }

    try {
      await deleteRound(roundId);
      await sendMessage(lastUserMessage.content);
    } catch (err) {
      console.error('[regenerate] 重新生成失败:', err);
      setError(err instanceof Error ? err.message : 'Failed to regenerate');
      setIsLoading(false);
    }
  }, [deleteRound, messages, sendMessage, sessionKey]);

  const startSession = useCallback(() => {
    const newSessionKey = generateUuid();
    loadRequestIdRef.current += 1;
    activeSessionKeyRef.current = newSessionKey;
    setSessionKey(newSessionKey);
    resetSessionView();
  }, [resetSessionView]);

  const loadSession = useCallback(async (id: string): Promise<void> => {
    const requestId = loadRequestIdRef.current + 1;
    loadRequestIdRef.current = requestId;
    activeSessionKeyRef.current = id;
    setSessionKey(id);
    resetSessionView();

    try {
      const data = await getSessionMessages(id);
      if (loadRequestIdRef.current !== requestId || activeSessionKeyRef.current !== id) {
        return;
      }
      if (Array.isArray(data)) {
        setMessages(sortMessages(data));
      }
    } catch (err) {
      if (loadRequestIdRef.current !== requestId || activeSessionKeyRef.current !== id) {
        return;
      }
      console.error('[loadSession] 加载session失败:', err);
      setError(err instanceof Error ? err.message : 'Failed to load session');
    }
  }, [resetSessionView]);

  const clearSession = useCallback(() => {
    loadRequestIdRef.current += 1;
    activeSessionKeyRef.current = null;
    setSessionKey(null);
    resetSessionView();
  }, [resetSessionView]);

  const resetSession = useCallback(() => {
    startSession();
  }, [startSession]);

  return {
    error,
    messages,
    sessionKey,
    isLoading,
    pendingPermission,
    sendMessage,
    startSession,
    loadSession,
    clearSession,
    resetSession,
    stopGeneration,
    deleteRound,
    regenerate,
    sendPermissionResponse,
  };
}

export type { UseAgentSessionOptions, UseAgentSessionReturn } from './types';
