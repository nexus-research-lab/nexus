/**
 * useAgentSession Hook 类型定义
 *
 * [INPUT]: 依赖 @/types 的 Message
 * [OUTPUT]: 对外提供 UseAgentSessionOptions, UseAgentSessionReturn
 * [POS]: types 模块的会话交互类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Message } from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/permission';

export interface UseAgentSessionOptions {
  wsUrl?: string;
  agentId?: string | null;
  onError?: (error: Error) => void;
}

export interface UseAgentSessionReturn {
  messages: Message[];
  sessionKey: string | null;
  isLoading: boolean;
  error: string | null;
  sendMessage: (content: string) => Promise<void>;
  startSession: () => void;
  loadSession: (key: string) => Promise<void>;
  clearSession: () => void;
  resetSession: () => void;
  stopGeneration: () => void;
  deleteRound: (roundId: string) => Promise<void>;
  regenerate: (roundId: string) => Promise<void>;
  pendingPermission: PendingPermission | null;
  sendPermissionResponse: (payload: PermissionDecisionPayload) => void;
}

export interface SessionSnapshot {
  sessionKey: string;
  messageCount: number;
  lastActivityAt: number;
  sessionId: string | null;
}
