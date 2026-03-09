/**
 * useAgentSession Hook - 主入口
 *
 * 管理Agent会话的WebSocket连接、消息处理和会话状态
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { useWebSocket } from '@/lib/websocket';
import { useSessionStore } from '@/store/session';
import { AssistantMessage, Message, StreamEvent, ToolCall, UserMessage } from '@/types';
import { UserQuestionAnswer } from '@/types/ask-user-question';
import { UseAgentSessionOptions, UseAgentSessionReturn } from './types';
import {
  createClearSession,
  createLoadHistoryMessages,
  createLoadSession,
  createResetSession,
  createStartSession,
} from './session-operations';
import { deleteRound as deleteRoundApi } from '@/lib/agent-api';

// ==================== Hook实现 ====================

interface ConversationEventPayload {
  event_id: string;
  seq: number;
  turn_id: string;
  kind: 'message_upsert' | 'message_delta';
  message?: Message;
  delta?: StreamEvent;
}

function isStreamEventMessage(message: Message | StreamEvent): message is StreamEvent {
  return 'type' in message && !('role' in message);
}

function getContentBlockKey(block: any): string | null {
  if (!block || typeof block !== 'object') {
    return null;
  }
  if (block.type === 'thinking') {
    return 'thinking';
  }
  if (block.type === 'tool_use' && block.id) {
    return `tool_use:${block.id}`;
  }
  if (block.type === 'tool_result' && block.tool_use_id) {
    return `tool_result:${block.tool_use_id}`;
  }
  if (block.type === 'text' && typeof block.text === 'string') {
    return `text:${block.text}`;
  }
  return null;
}

function mergeAssistantContent(existingContent: any[], incomingContent: any[]): any[] {
  const merged = [...existingContent];
  const indexMap = new Map<string, number>();

  merged.forEach((block, index) => {
    const key = getContentBlockKey(block);
    if (key) {
      indexMap.set(key, index);
    }
  });

  incomingContent.forEach((block) => {
    const key = getContentBlockKey(block);
    if (!key) {
      merged.push(block);
      return;
    }

    const existingIndex = indexMap.get(key);
    if (existingIndex === undefined) {
      merged.push(block);
      indexMap.set(key, merged.length - 1);
      return;
    }

    merged[existingIndex] = block;
  });

  const thinkingIndex = merged.findIndex(block => block?.type === 'thinking');
  if (thinkingIndex > 0) {
    const [thinkingBlock] = merged.splice(thinkingIndex, 1);
    merged.unshift(thinkingBlock);
  }

  return merged;
}

function findAssistantMessageIndex(messages: Message[], messageId?: string): number {
  if (messageId) {
    const exactIndex = messages.findIndex(
      msg => msg.role === 'assistant' && msg.message_id === messageId
    );
    if (exactIndex !== -1) {
      return exactIndex;
    }
  }

  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].role === 'assistant') {
      return index;
    }
  }
  return -1;
}

function createAssistantMessageFromStreamStart(
  event: StreamEvent,
  messageSessionKey: string,
  roundId: string
): AssistantMessage {
  return {
    message_id: event.message_id || crypto.randomUUID(),
    agent_id: messageSessionKey,
    round_id: roundId,
    role: 'assistant',
    content: [],
    timestamp: Date.now(),
    model: event.message?.model,
  };
}

function applyStreamEventToAssistantMessage(
  message: AssistantMessage,
  event: StreamEvent
): AssistantMessage {
  const eventAny = event as any;
  const updatedMessage: AssistantMessage = {
    ...message,
    content: [...message.content],
  };

  if (event.type === 'content_block_start' && event.content_block && typeof event.index === 'number') {
    updatedMessage.content[event.index] = event.content_block;
    return updatedMessage;
  }

  if (event.type === 'content_block_delta' && event.delta && typeof event.index === 'number') {
    const block = updatedMessage.content[event.index] as any;
    const delta = event.delta;
    if (!block) {
      return updatedMessage;
    }

    if (block.type === 'text' && delta.type === 'text_delta') {
      updatedMessage.content[event.index] = {
        ...block,
        text: block.text + (delta.text || '')
      };
      return updatedMessage;
    }

    if (block.type === 'thinking' && delta.type === 'thinking_delta') {
      updatedMessage.content[event.index] = {
        ...block,
        thinking: block.thinking + (delta.thinking || '')
      };
      return updatedMessage;
    }

    if (block.type === 'tool_use' && delta.type === 'input_json_delta') {
      try {
        updatedMessage.content[event.index] = {
          ...block,
          input: JSON.parse(delta.partial_json),
        };
      } catch {
        // 忽略不完整JSON，等待后续delta
      }
      return updatedMessage;
    }
  }

  if (event.type === 'message_delta') {
    if (event.delta?.stop_reason) {
      updatedMessage.stop_reason = event.delta.stop_reason;
    }
  }

  return updatedMessage;
}

function handleStreamEventMessage(
  messages: Message[],
  event: StreamEvent,
  messageSessionKey: string,
  roundId: string
): Message[] {
  if (event.type === 'message_start') {
    if (event.message_id) {
      const exists = messages.some(
        msg => msg.role === 'assistant' && msg.message_id === event.message_id
      );
      if (exists) {
        return messages;
      }
    }
    return [...messages, createAssistantMessageFromStreamStart(event, messageSessionKey, roundId)];
  }

  const targetIndex = findAssistantMessageIndex(messages, event.message_id);
  if (targetIndex === -1) {
    return messages;
  }

  const assistantMessage = messages[targetIndex] as AssistantMessage;
  const updatedMessage = applyStreamEventToAssistantMessage(assistantMessage, event);
  const nextMessages = [...messages];
  nextMessages[targetIndex] = updatedMessage;
  return nextMessages;
}

function mergeToolResultMessage(messages: Message[], message: AssistantMessage): Message[] | null {
  if (!message.is_tool_result) {
    return null;
  }

  const toolResultBlock = message.content.find(
    (block): block is Extract<AssistantMessage['content'][number], { type: 'tool_result' }> =>
      block.type === 'tool_result'
  );
  if (!toolResultBlock || !toolResultBlock.tool_use_id) {
    return null;
  }

  const reverseIndex = [...messages].reverse().findIndex(msg =>
    msg.role === 'assistant' &&
    Array.isArray(msg.content) &&
    msg.content.some((block: any) => block.type === 'tool_use' && block.id === toolResultBlock.tool_use_id)
  );
  if (reverseIndex === -1) {
    return null;
  }

  const targetIndex = messages.length - 1 - reverseIndex;
  const targetMessage = messages[targetIndex] as AssistantMessage;
  const updatedMessage: AssistantMessage = {
    ...targetMessage,
    content: mergeAssistantContent(targetMessage.content, message.content),
  };

  const nextMessages = [...messages];
  nextMessages[targetIndex] = updatedMessage;
  return nextMessages;
}

function upsertMessageById(messages: Message[], message: Message): Message[] | null {
  const existingIndex = message.message_id
    ? messages.findIndex(item => item.message_id === message.message_id)
    : -1;
  if (existingIndex === -1) {
    return null;
  }

  const nextMessages = [...messages];
  if (message.role !== 'assistant' || nextMessages[existingIndex].role !== 'assistant') {
    nextMessages[existingIndex] = message;
    return nextMessages;
  }

  const existingAssistant = nextMessages[existingIndex] as AssistantMessage;
  const incomingAssistant = message as AssistantMessage;
  nextMessages[existingIndex] = {
    ...incomingAssistant,
    content: mergeAssistantContent(existingAssistant.content, incomingAssistant.content),
  };
  return nextMessages;
}

function reduceIncomingMessage(
  messages: Message[],
  incoming: Message | StreamEvent,
  messageSessionKey: string,
  roundId: string
): Message[] {
  if (isStreamEventMessage(incoming)) {
    return handleStreamEventMessage(messages, incoming, messageSessionKey, roundId);
  }

  if (incoming.role === 'assistant') {
    const mergedToolResult = mergeToolResultMessage(messages, incoming);
    if (mergedToolResult) {
      return mergedToolResult;
    }
  }

  const upserted = upsertMessageById(messages, incoming);
  if (upserted) {
    return upserted;
  }

  return [...messages, incoming];
}

function extractToolCallsFromMessage(message: Message): ToolCall[] {
  if (message.role !== 'assistant' || !Array.isArray(message.content)) {
    return [];
  }
  return message.content
    .filter((block): block is Extract<AssistantMessage['content'][number], { type: 'tool_use' }> =>
      block.type === 'tool_use'
    )
    .map(block => ({
      id: block.id,
      tool_name: block.name,
      input: block.input || {},
      status: 'running',
      start_time: Date.now(),
    }));
}

function mergeToolCalls(prev: ToolCall[], incoming: ToolCall[]): ToolCall[] {
  if (incoming.length === 0) {
    return prev;
  }
  const merged = new Map<string, ToolCall>();
  prev.forEach(call => merged.set(call.id, call));
  incoming.forEach(call => {
    const existing = merged.get(call.id);
    if (!existing) {
      merged.set(call.id, call);
      return;
    }
    merged.set(call.id, { ...existing, ...call });
  });
  return [...merged.values()];
}

export function useAgentSession(options: UseAgentSessionOptions = {}): UseAgentSessionReturn {
  const wsUrl = options.wsUrl || process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8010/agent/v1/chat/ws';

  // 状态
  const [messages, setMessages] = useState<Message[]>([]);
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // sessionKey 初始为 null，只在创建或加载 session 时设置
  const [sessionKey, setstring] = useState<string | null>(null);
  // 权限请求状态
  const [pendingPermission, setPendingPermission] = useState<{
    request_id: string;
    tool_name: string;
    tool_input: Record<string, any>;
  } | null>(null);

  // Store
  const { updateSession } = useSessionStore();

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
    }
  }, [sessionKey]);

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
  const sendPermissionResponse = useCallback((decision: 'allow' | 'deny', userAnswers?: UserQuestionAnswer[]) => {
    if (!pendingPermission) return;

    const response: Record<string, any> = {
      type: 'permission_response',
      request_id: pendingPermission.request_id,
      session_key: sessionKey,
      agent_id: sessionKey,
      decision,
      message: decision === 'deny' ? 'User denied permission' : '',
    };

    // 如果是 AskUserQuestion，附带用户答案
    if (userAnswers && userAnswers.length > 0) {
      response.user_answers = userAnswers;
    }

    console.debug('[useAgentSession] Sending permission response:', response);
    wsSend(response as any);
    setPendingPermission(null);
  }, [pendingPermission, sessionKey, wsSend]);

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
    createStartSession(setstring, setMessages, setToolCalls, setError, setIsLoading),
    []
  );

  const loadSession = useCallback(
    createLoadSession(setstring, setMessages, setError),
    []
  );

  const clearSession = useCallback(
    createClearSession(setMessages, setToolCalls, setError, setIsLoading, setstring, abortControllerRef),
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
