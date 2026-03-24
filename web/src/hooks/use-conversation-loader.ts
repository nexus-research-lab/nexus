"use client";

import { useEffect, useRef } from "react";

import { ConversationLoaderOptions } from "@/types/conversation";

/**
 * Conversation 加载器，监听 conversation_id 变化并触发加载。
 */
export const useConversationLoader = ({
  conversation_id,
  load_conversation,
  debug_name = "useConversationLoader",
}: ConversationLoaderOptions) => {
  const prev_key = useRef<string | null>(null);

  useEffect(() => {
    if (prev_key.current === conversation_id) {
      return;
    }

    prev_key.current = conversation_id;

    if (conversation_id) {
      console.debug(`[${debug_name}] Loading conversation:`, conversation_id);
      load_conversation(conversation_id);
    }
  }, [conversation_id, debug_name, load_conversation]);
};
