/**
 * Conversation 初始化 Hook
 *
 * 当前仍复用 session store 做持久化与服务端同步，
 * 这里只提供 conversation 语义层，方便页面和 controller 逐步迁移。
 */

import { useInitializeSessions } from "@/hooks/use-initialize-sessions";

interface UseInitializeConversationsOptions {
  loadConversationsFromServer: () => Promise<void>;
  setCurrentConversation: (key: string) => void;
  autoSelectFirst?: boolean;
  debugName?: string;
}

export const useInitializeConversations = ({
  loadConversationsFromServer,
  setCurrentConversation,
  autoSelectFirst = true,
  debugName = "useInitializeConversations",
}: UseInitializeConversationsOptions) => {
  return useInitializeSessions({
    loadSessionsFromServer: loadConversationsFromServer,
    setCurrentSession: setCurrentConversation,
    autoSelectFirst,
    debugName,
  });
};
