/**
 * Session Store Actions
 *
 * [INPUT]: 依赖 @/types, @/lib/agent-api
 * [OUTPUT]: 对外提供 session CRUD actions
 * [POS]: store/session 模块的操作函数
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { CreateSessionParams, Session, UpdateSessionParams, } from '@/types';
import { SessionStoreState } from './types';
import { createDefaultSession } from './utils';
import { createSession, deleteSession, getSessions, updateSession } from "@/lib/agent-api";

// ==================== 基础操作 ====================

export const createSessionAction = (
  set: (fn: (state: SessionStoreState) => Partial<SessionStoreState>) => void,
  get: () => SessionStoreState
) => async (params?: CreateSessionParams): Promise<string> => {
  const newSession = createDefaultSession(params);

  try {
    const created = await createSession(newSession.session_key, {
      title: params?.title,
      options: params?.options,
      agent_id: params?.agent_id,
    });

    set((state) => ({
      sessions: [created, ...state.sessions.filter(s => s.session_key !== newSession.session_key)],
      error: null,
    }));
    console.debug('[SessionStore] Session synced:', created.session_key);
    return created.session_key;
  } catch (error) {
    console.error('[SessionStore] Failed to sync session:', error);
    set((state) => ({
      sessions: [newSession, ...state.sessions],
      error: null,
    }));
    return newSession.session_key;
  }
};


export const deleteSessionAction = (
  set: (fn: (state: SessionStoreState) => Partial<SessionStoreState>) => void,
  get: () => SessionStoreState
) => async (key: string): Promise<void> => {
  try {
    await deleteSession(key);

    set((state) => {
      const newSessions = state.sessions.filter(s => s.session_key !== key);
      const newCurrentKey = state.current_session_key === key
        ? (newSessions[0]?.session_key || null)
        : state.current_session_key;

      return {
        sessions: newSessions,
        current_session_key: newCurrentKey,
        error: null,
      };
    });
  } catch (error) {
    console.error('[SessionStore] Failed to delete session:', error);
    set(() => ({ error: 'Failed to delete session' }));
  }
};

export const updateSessionAction = (
  set: (fn: (state: SessionStoreState) => Partial<SessionStoreState>) => void
) => async (key: string, params: UpdateSessionParams): Promise<void> => {
  try {
    await updateSession(key, params);

    set((state) => ({
      sessions: state.sessions.map(session =>
        session.session_key === key
          ? {
            ...session,
            ...(params.title && { title: params.title }),
            ...(params.options && { options: { ...session.options, ...params.options } }),
            last_activity_at: Date.now(),
          }
          : session
      ),
      error: null,
    }));
  } catch (error) {
    console.error('[SessionStore] Failed to update session:', error);
    set(() => ({ error: 'Failed to sync update with server' }));
  }
};

export const setCurrentSessionAction = (
  set: any
) => (key: string | null): void => {
  set({ current_session_key: key, error: null });
};


// ==================== 查询操作 ====================

export const getSessionAction = (get: () => SessionStoreState) => (key: string): Session | undefined => {
  return get().sessions.find(s => s.session_key === key);
};


// ==================== 服务器同步 ====================

export const loadSessionsFromServerAction = (
  set: any,
  get: () => SessionStoreState
) => async (): Promise<void> => {
  try {
    set({ loading: true, error: null });

    const sessions = await getSessions();

    if (sessions && Array.isArray(sessions)) {
      const sortedSessions = [...sessions].sort((a, b) => b.last_activity_at - a.last_activity_at);
      console.debug(`[SessionStore] Loaded ${sortedSessions.length} sessions`);
      set({ sessions: sortedSessions, loading: false, error: null });
    } else {
      set({ loading: false, error: 'Invalid response format' });
    }
  } catch (err) {
    console.error('[SessionStore] Failed to load sessions:', err);
    set({
      loading: false,
      error: err instanceof Error ? err.message : 'Unknown error',
    });
  }
};

// ==================== 清理 ====================

export const clearAllSessionsAction = (
  set: any
) => (): void => {
  set({
    sessions: [],
    current_session_key: null,
    error: null,
  });
};
