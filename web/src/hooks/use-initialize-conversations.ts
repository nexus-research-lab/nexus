/**
 * Conversation 初始化 Hook
 *
 * 当前仍复用 session store 做持久化与服务端同步，
 * 这里只提供 conversation 语义层，方便页面和 controller 逐步迁移。
 */

import { useInitializeSessions } from "@/hooks/use-initialize-sessions";
import { InitializeConversationsOptions } from "@/types/conversation";

export const useInitializeConversations = ({
  load_conversations_from_server,
  set_current_conversation,
  auto_select_first = true,
  debug_name = "useInitializeConversations",
}: InitializeConversationsOptions) => {
  return useInitializeSessions({
    load_sessions_from_server: load_conversations_from_server,
    set_current_session: set_current_conversation,
    auto_select_first,
    debug_name,
  });
};
