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
  create_session: (params?: CreateSessionParams) => Promise<string>;
  delete_session: (key: string) => void;
  update_session: (key: string, params: UpdateSessionParams) => void;
  set_current_session: (key: string | null) => void;
  sync_session_snapshot: (
    key: string,
    patch: Partial<Pick<Session, 'message_count' | 'last_activity_at' | 'session_id'>>
  ) => void;

  // 查询
  get_session: (key: string) => Session | undefined;

  // 服务器同步
  load_sessions_from_server: () => Promise<void>;

  // 清理
  clear_all_sessions: () => void;
}
