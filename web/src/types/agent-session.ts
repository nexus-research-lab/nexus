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
  ws_url?: string;
  agent_id?: string | null;
  on_error?: (error: Error) => void;
}

export interface UseAgentSessionReturn {
  messages: Message[];
  session_key: string | null;
  is_loading: boolean;
  error: string | null;
  send_message: (content: string) => Promise<void>;
  start_session: () => void;
  load_session: (key: string) => Promise<void>;
  clear_session: () => void;
  reset_session: () => void;
  stop_generation: () => void;
  delete_round: (roundId: string) => Promise<void>;
  regenerate: (roundId: string) => Promise<void>;
  pending_permission: PendingPermission | null;
  send_permission_response: (payload: PermissionDecisionPayload) => void;
}

export interface SessionSnapshot {
  session_key: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
}
