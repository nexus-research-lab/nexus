/**
 * Session 初始化 Hook
 *
 * [INPUT]: 依赖 useSessionStore
 * [OUTPUT]: 对外提供 useInitializeSessions
 * [POS]: hooks 模块的初始化逻辑
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { useEffect, useState } from "react";
import { useSessionStore } from "@/store/session";

interface UseInitializeSessionsOptions {
  loadSessionsFromServer: () => Promise<void>;
  setCurrentSession: (key: string) => void;
  autoSelectFirst?: boolean;
  debugName?: string;
}

export const useInitializeSessions = ({
  loadSessionsFromServer,
  setCurrentSession,
  autoSelectFirst = true,
  debugName = "useInitializeSessions"
}: UseInitializeSessionsOptions) => {
  const [isHydrated, setIsHydrated] = useState(false);

  useEffect(() => {
    setIsHydrated(true);

    const currentState = useSessionStore.getState();
    if (currentState.sessions.length > 0) {
      return;
    }

    loadSessionsFromServer()
      .then(() => {
        const state = useSessionStore.getState();
        if (autoSelectFirst && !state.current_session_key && state.sessions.length > 0) {
          setCurrentSession(state.sessions[0].session_key);
        }
      })
      .catch((err) => {
        console.error(`[${debugName}] Failed to load sessions:`, err);
      });
  }, [loadSessionsFromServer, setCurrentSession, autoSelectFirst, debugName]);

  return isHydrated;
};