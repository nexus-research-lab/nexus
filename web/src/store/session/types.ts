/**
 * Session Store 类型定义
 *
 * [INPUT]: 依赖 @/types 的 Session 类型
 * [OUTPUT]: 对外提供 SessionStoreState
 * [POS]: store/session 模块的类型，被 index.ts 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { CreateSessionParams, Session, UpdateSessionParams, } from '@/types';

// ==================== Store State ====================

export interface SessionStoreState {
  // 数据
  sessions: Session[];
  current_session_key: string | null;

  // UI 状态
  loading: boolean;
  error: string | null;

  // 基础操作
  createSession: (params?: CreateSessionParams) => Promise<string>;
  deleteSession: (key: string) => void;
  updateSession: (key: string, params: UpdateSessionParams) => void;
  setCurrentSession: (key: string | null) => void;

  // 查询
  getSession: (key: string) => Session | undefined;

  // 服务器同步
  loadSessionsFromServer: () => Promise<void>;

  // 清理
  clearAllSessions: () => void;
}
