"use client";

import { useLayoutEffect, useRef } from "react";

import { SessionLoaderOptions } from "@/types/conversation/conversation";

/**
 * Session 加载器，监听 sessionKey 变化并触发加载。
 */
export const useSessionLoader = ({
  session_key: sessionKey,
  load_session: loadSession,
  debug_name: debugName = "useSessionLoader",
}: SessionLoaderOptions) => {
  const prevKey = useRef<string | null>(null);

  useLayoutEffect(() => {
    if (prevKey.current === sessionKey) {
      return;
    }

    prevKey.current = sessionKey;

    if (sessionKey) {
      console.debug(`[${debugName}] Loading session:`, sessionKey);
      void loadSession(sessionKey);
    }
  }, [sessionKey, debugName, loadSession]);
};
