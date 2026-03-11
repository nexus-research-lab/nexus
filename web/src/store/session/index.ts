/**
 * Session Store - 主入口
 *
 * [INPUT]: 依赖 zustand, ./types, ./actions
 * [OUTPUT]: 对外提供 useSessionStore
 * [POS]: store/session 模块主入口
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { createBrowserJSONStorage } from '@/lib/browser-storage';
import { SessionStoreState } from './types';
import * as actions from './actions';

// ==================== Store 创建 ====================

export const useSessionStore = create<SessionStoreState>()(
  persist(
    (set, get) => ({
      sessions: [],
      current_session_key: null,
      loading: false,
      error: null,

      createSession: actions.createSessionAction(set, get),
      deleteSession: actions.deleteSessionAction(set, get),
      updateSession: actions.updateSessionAction(set),
      setCurrentSession: actions.setCurrentSessionAction(set),
      syncSessionSnapshot: actions.syncSessionSnapshotAction(set),
      getSession: actions.getSessionAction(get),
      loadSessionsFromServer: actions.loadSessionsFromServerAction(set, get),
      clearAllSessions: actions.clearAllSessionsAction(set),
    }),
    {
      name: 'agent-ui-sessions',
      storage: createBrowserJSONStorage(),
      partialize: (state) => ({
        sessions: state.sessions,
        current_session_key: state.current_session_key,
      }),
    }
  )
);

// ==================== 导出 ====================

export type { SessionStoreState } from './types';
export { generateSessionKey, createDefaultSession } from './utils';
