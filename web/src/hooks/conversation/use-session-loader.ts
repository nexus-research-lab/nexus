"use client";

import { useLayoutEffect, useRef } from "react";

import { SessionLoaderOptions } from "@/types/conversation/conversation";

/**
 * Session 加载器，监听 session_key 变化并触发加载。
 */
export const useSessionLoader = ({
  session_key,
  load_session,
  debug_name = "useSessionLoader",
}: SessionLoaderOptions) => {
  const prev_key = useRef<string | null>(null);

  useLayoutEffect(() => {
    if (prev_key.current === session_key) {
      return;
    }

    prev_key.current = session_key;

    if (session_key) {
      console.debug(`[${debug_name}] Loading session:`, session_key);
      void load_session(session_key);
    }
  }, [session_key, debug_name, load_session]);
};
