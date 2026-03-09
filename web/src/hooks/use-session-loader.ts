import { useEffect, useRef } from "react";

/**
 * Session 加载器 — 监听 session_key 变化并触发加载
 *
 * [INPUT]: 外部传入 session_key + loadSession 回调
 * [OUTPUT]: 无（副作用 hook）
 * [POS]: hooks 模块的 session 加载逻辑
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */
export const useSessionLoader = (
  sessionKey: string | null,
  loadSession: (key: string) => void,
  debugName = "useSessionLoader"
) => {
  const prevKey = useRef<string | null>(null);

  useEffect(() => {
    if (prevKey.current === sessionKey) return;
    prevKey.current = sessionKey;

    if (sessionKey) {
      console.debug(`[${debugName}] Loading session:`, sessionKey);
      loadSession(sessionKey);
    }
  }, [sessionKey, loadSession, debugName]);
};