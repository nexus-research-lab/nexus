/**
 * 会话类型定义
 *
 * [INPUT]: 依赖 @/types/sdk 的 SessionId
 * [OUTPUT]: 对外提供 SessionOptions、Session、ApiSession、CreateSessionParams、UpdateSessionParams
 * [POS]: types 模块的会话核心类型，被 agent-api.ts 和 agent-options.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { SessionId } from "@/types/sdk";

// ==================== 会话配置 ====================

/** 会话配置选项 */
export interface SessionOptions {
  model?: string;
  permissionMode?: string;
  allowedTools?: string[];
  disallowedTools?: string[];
  systemPrompt?: string;
  maxTurns?: number;
  maxThinkingTokens?: number;
  cwd?: string;
  includePartialMessages?: boolean;
  settingSources?: ('user' | 'project' | 'local')[];
  skillsEnabled?: boolean;
}

// ==================== 会话数据结构 ====================

/** 标准化的会话数据结构 */
export interface Session {
  /** 会话路由键（UUID，唯一标识一个会话） */
  session_key: string;
  /** 所属 Agent 实体 ID */
  agent_id?: string;
  /** SDK 会话 ID */
  session_id: SessionId | null;
  /** 会话标题 */
  title: string;
  /** 会话元数据 */
  options: Record<string, unknown>;
  /** 创建时间（时间戳） */
  created_at: number;
  /** 最后活动时间（时间戳） */
  last_activity_at: number;
  /** 是否活跃 */
  is_active?: boolean;
  /** 消息数量 */
  message_count?: number;
}

// ==================== API 相关类型 ====================

/** API 响应中的会话数据（后端格式） */
export interface ApiSession {
  session_key: string;
  agent_id: string;
  session_id: string | null;
  created_at: string;
  last_activity: string;
  is_active: boolean;
  title: string | null;
  message_count: number;
  options: Record<string, unknown> | null;
}

// ==================== 操作参数类型 ====================

/** 创建会话参数 */
export interface CreateSessionParams {
  title?: string;
  agent_id?: string;
}

/** 更新会话参数 */
export interface UpdateSessionParams {
  title?: string;
}
