/**
 * Session 操作函数
 *
 * [INPUT]: 依赖 @/types, @/lib/agent-api
 * [OUTPUT]: 对外提供 createStartSession, createLoadSession, createClearSession, createResetSession, createLoadHistoryMessages
 * [POS]: hooks/agent 模块的会话生命周期操作，被 useAgentSession 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Message } from '@/types';
import { getSessionMessages } from "@/lib/agent-api";
import { generateUuid } from '@/lib/uuid';

/**
 * 创建新session操作
 */
export function createStartSession(
  setSessionKey: (id: string) => void,
  setMessages: (messages: Message[]) => void,
  setToolCalls: (calls: any[]) => void,
  setError: (error: string | null) => void,
  setIsLoading: (loading: boolean) => void
) {
  return () => {
    const newSessionKey = generateUuid();
    setSessionKey(newSessionKey);
    setMessages([]);
    setToolCalls([]);
    setError(null);
    setIsLoading(false);
  };
}

/**
 * 检测并标记未完成的工具调用
 * 当页面刷新时，如果有工具调用没有对应的结果，将其标记为中断状态
 * 同时添加虚拟的 ResultMessage 以便 UI 正确显示完成状态
 */
export function markInterruptedToolCalls(messages: Message[]): Message[] {
  // 收集所有 tool_use id 和 tool_result 的 tool_use_id
  const toolUseIds = new Set<string>();
  const toolResultIds = new Set<string>();
  // 检查是否已经有 resultMessage
  let hasResultMessage = false;

  for (const msg of messages) {
    if (msg.role === 'result') {
      hasResultMessage = true;
    }
    if (msg.role === 'assistant' && Array.isArray((msg as any).content)) {
      for (const block of (msg as any).content) {
        if (block.type === 'tool_use' && block.id) {
          toolUseIds.add(block.id);
        }
        if (block.type === 'tool_result' && block.tool_use_id) {
          toolResultIds.add(block.tool_use_id);
        }
      }
    }
  }

  // 找出未完成的工具调用
  const incompleteToolIds = [...toolUseIds].filter(id => !toolResultIds.has(id));

  if (incompleteToolIds.length === 0) {
    return messages;
  }

  console.debug('[markInterruptedToolCalls] 发现未完成的工具调用:', incompleteToolIds);

  // 为未完成的工具添加中断状态的 tool_result
  const updatedMessages = messages.map(msg => {
    if (msg.role === 'assistant' && Array.isArray((msg as any).content)) {
      const content = (msg as any).content;
      const hasIncompleteTools = content.some(
        (block: any) => block.type === 'tool_use' && incompleteToolIds.includes(block.id)
      );

      if (hasIncompleteTools) {
        // 为每个未完成的工具添加中断状态的 tool_result
        const additionalResults = content
          .filter((block: any) => block.type === 'tool_use' && incompleteToolIds.includes(block.id))
          .map((block: any) => ({
            type: 'tool_result',
            tool_use_id: block.id,
            content: '任务已中断（页面刷新或连接断开）',
            is_error: true,
          }));

        return {
          ...msg,
          content: [...content, ...additionalResults],
        };
      }
    }
    return msg;
  });

  // 如果没有 resultMessage，添加一个虚拟的中断状态 ResultMessage
  if (!hasResultMessage && updatedMessages.length > 0) {
    // 获取最后一条消息的信息用于构造 ResultMessage
    const lastMessage = updatedMessages[updatedMessages.length - 1];
    const interruptedResultMessage: Message = {
      message_id: `interrupted_result_${Date.now()}`,
      round_id: lastMessage.round_id,
      agent_id: lastMessage.agent_id,
      session_id: lastMessage.session_id,
      role: 'result',
      timestamp: Date.now(),
      subtype: 'error',
      duration_ms: 0,
      duration_api_ms: 0,
      num_turns: 0,
      result: '任务已中断（页面刷新或连接断开）',
      is_error: true,
    } as Message;

    console.debug('[markInterruptedToolCalls] 添加虚拟 ResultMessage:', interruptedResultMessage);
    updatedMessages.push(interruptedResultMessage);
  }

  return updatedMessages;
}

/**
 * 加载指定会话
 * 设置sessionKey并从后端加载历史消息
 */
export const createLoadSession = (
  setSessionKey: (id: string) => void,
  setMessages: (messages: Message[]) => void,
  setError: (error: string | null) => void,
) => async (id: string): Promise<void> => {
  try {
    console.debug('[loadSession] 开始加载session:', id);

    // 1. 设置sessionKey
    console.debug('[loadSession] 设置sessionKey:', id);
    setSessionKey(id);

    // 2. 清空当前消息
    setMessages([]);
    setError(null);

    // 3. 加载历史消息
    console.debug('[loadSession] 调用getSessionMessages API');
    const data = await getSessionMessages(id);

    if (data && Array.isArray(data)) {
      // 检测并标记未完成的工具调用（页面刷新时中断的任务）
      const finalMessages = markInterruptedToolCalls(data);

      setMessages(finalMessages);
    } else {
      console.debug(`[loadSession] 没有收到有效消息数据:`, data);
    }
  } catch (err) {
    console.error('[loadSession] 加载session失败:', err);
    setError(err instanceof Error ? err.message : 'Failed to load session');
  }
};

/**
 * 清除session操作
 */
export function createClearSession(
  setMessages: (messages: Message[]) => void,
  setToolCalls: (calls: any[]) => void,
  setError: (error: string | null) => void,
  setIsLoading: (loading: boolean) => void,
  setSessionKey: (id: string | null) => void,
  abortControllerRef: React.RefObject<AbortController | null>
) {
  return () => {
    setMessages([]);
    setToolCalls([]);
    setError(null);
    setIsLoading(false);
    setSessionKey(null);

    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
  };
}

/**
 * 重置session操作（创建新session）
 */
export function createResetSession(startSession: () => void) {
  return () => {
    startSession();
  };
}

/**
 * 加载历史消息
 */
export function createLoadHistoryMessages(
  setMessages: (messages: Message[]) => void,
  updateSession: (id: string, params: any) => void,
) {
  return async (sessionKey: string) => {
    try {
      const messages = await getSessionMessages(sessionKey);
      if (Array.isArray(messages)) {
        const finalMessages = markInterruptedToolCalls(messages);
        console.debug(`[useAgentSession] Loaded ${finalMessages.length} messages`);
        setMessages(finalMessages);

        // 同时更新到session store中缓存
        updateSession(sessionKey, { messages: finalMessages });
      }
    } catch (err) {
      console.error('[useAgentSession] Failed to load history:', err);
    }
  };
}
