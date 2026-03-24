/**
 * Conversation 初始化 Hook
 *
 * 直接基于 conversation store 初始化，不再通过 session 语义中转。
 */

import { useEffect, useState } from "react";

import { getConversationStoreSnapshot } from "@/store/conversation";
import { InitializeConversationsOptions } from "@/types/conversation";

export const useInitializeConversations = ({
  load_conversations_from_server,
  set_current_conversation,
  auto_select_first = true,
  debug_name = "useInitializeConversations",
}: InitializeConversationsOptions) => {
  const [is_hydrated, set_is_hydrated] = useState(false);

  useEffect(() => {
    set_is_hydrated(true);

    const current_state = getConversationStoreSnapshot();
    if (current_state.conversations.length > 0) {
      return;
    }

    load_conversations_from_server()
      .then(() => {
        const state = getConversationStoreSnapshot();
        if (auto_select_first && !state.current_conversation_id && state.conversations.length > 0) {
          set_current_conversation(state.conversations[0].session_key);
        }
      })
      .catch((err) => {
        console.error(`[${debug_name}] Failed to load conversations:`, err);
      });
  }, [auto_select_first, debug_name, load_conversations_from_server, set_current_conversation]);

  return is_hydrated;
};
