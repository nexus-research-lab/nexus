/**
 * Conversation Store 辅助函数
 *
 * [INPUT]: 依赖 @/types/conversation 的 CreateConversationParams, Conversation
 * [OUTPUT]: 对外提供 generateConversationKey, createDefaultConversation
 * [POS]: store 模块的工具函数
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Conversation, CreateConversationParams } from '@/types/conversation';
import { generateUuid } from '@/lib/uuid';

export const generateConversationKey = (): string => {
  return generateUuid();
};

export const createDefaultConversation = (params?: CreateConversationParams): Conversation => {
  const now = Date.now();
  return {
    session_key: generateConversationKey(),
    agent_id: params?.agent_id,
    session_id: null,
    title: params?.title || 'New Chat',
    options: {},
    created_at: now,
    last_activity_at: now,
  };
};
