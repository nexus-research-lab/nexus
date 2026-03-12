/**
 * Session Store 辅助函数
 *
 * [INPUT]: 依赖 @/types 的 CreateSessionParams, Session
 * [OUTPUT]: 对外提供 generateSessionKey, createDefaultSession
 * [POS]: store 模块的工具函数
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { CreateSessionParams, Session } from '@/types';
import { generateUuid } from '@/lib/uuid';

// ==================== ID生成 ====================

/** 生成新的 session 路由键 */
export const generateSessionKey = (): string => {
  return generateUuid();
};

// ==================== 默认值创建 ====================

/** 创建默认会话 */
export const createDefaultSession = (params?: CreateSessionParams): Session => {
  const now = Date.now();
  return {
    session_key: generateSessionKey(),
    agent_id: params?.agent_id,
    session_id: null,
    title: params?.title || 'New Chat',
    options: {},
    created_at: now,
    last_activity_at: now,
  };
};
