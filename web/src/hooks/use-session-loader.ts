import { useEffect, useRef } from "react";
import { SessionLoaderOptions } from "@/types/session";

/**
 * Session 加载器 — 监听 session_key 变化并触发加载
 *
 * [INPUT]: 外部传入 session_key + loadSession 回调
 * [OUTPUT]: 无（副作用 hook）
 * [POS]: hooks 模块的 session 加载逻辑
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */
export const useSessionLoader = (
  {
    session_key,
    load_session,
    debug_name = "useSessionLoader",
  }: SessionLoaderOptions,
) => {
  const prev_key = useRef<string | null>(null);

  useEffect(() => {
    if (prev_key.current === session_key) return;
    prev_key.current = session_key;

    if (session_key) {
      console.debug(`[${debug_name}] Loading session:`, session_key);
      load_session(session_key);
    }
  }, [debug_name, load_session, session_key]);
};
