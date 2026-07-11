import { useCallback } from "react";

import { buildAgentOptionsSavePayload } from "@/features/agents/options/agent-options-constants";
import { validateAgentNameApi } from "@/lib/api/agent-manage-api";
import type { AgentIdentityDraft, AgentOptions } from "@/types/agent/agent";

interface UseRoomAgentOptionsOptions {
  updateAgent: (
    agentId: string,
    params: {
      name?: string;
      options?: Partial<AgentOptions>;
      avatar?: string;
      description?: string;
      vibe_tags?: string[];
    },
  ) => Promise<void>;
}

export function useRoomAgentOptions({updateAgent}: UseRoomAgentOptionsOptions) {
  const saveExistingAgentOptions = useCallback(async (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await updateAgent(agentId, {
      name: title,
      options: buildAgentOptionsSavePayload(options),
      avatar: identity.avatar,
      description: identity.description,
      vibe_tags: identity.vibe_tags,
    });
  }, [updateAgent]);

  const validateAgentName = useCallback((name: string, agentId?: string) => (
    validateAgentNameApi(name, agentId)
  ), []);

  return {
    saveExistingAgentOptions,
    validateAgentName,
  };
}
