"use client";

import { useHomeAgentSessionController } from "@/hooks/use-home-agent-session-controller";
import { useHomeWorkspaceController } from "@/hooks/use-home-workspace-controller";

export function useHomePageController() {
  const agentSession = useHomeAgentSessionController();
  const workspace = useHomeWorkspaceController({
    currentAgentId: agentSession.currentAgentId,
    currentSession: agentSession.currentSession,
  });

  return {
    ...agentSession,
    ...workspace,
  };
}
