"use client";

import { useHomeAgentConversationController } from "@/hooks/use-home-agent-conversation-controller";

export function useLauncherPageController() {
  return useHomeAgentConversationController();
}
