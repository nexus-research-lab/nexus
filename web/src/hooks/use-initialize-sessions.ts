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
import { InitializeSessionsOptions } from "@/types/session";

export const useInitializeSessions = ({
  load_sessions_from_server,
  set_current_session,
  auto_select_first = true,
  debug_name = "useInitializeSessions",
}: InitializeSessionsOptions) => {
  const [is_hydrated, set_is_hydrated] = useState(false);

  useEffect(() => {
    set_is_hydrated(true);

    const current_state = useSessionStore.getState();
    if (current_state.sessions.length > 0) {
      return;
    }

    load_sessions_from_server()
      .then(() => {
        const state = useSessionStore.getState();
        if (auto_select_first && !state.current_session_key && state.sessions.length > 0) {
          set_current_session(state.sessions[0].session_key);
        }
      })
      .catch((err) => {
        console.error(`[${debug_name}] Failed to load sessions:`, err);
      });
  }, [auto_select_first, debug_name, load_sessions_from_server, set_current_session]);

  return is_hydrated;
};
