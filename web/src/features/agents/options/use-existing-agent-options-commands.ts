import { useCallback } from "react";

import { validateAgentNameApi } from "@/lib/api/agent-manage-api";
import type {
  AgentIdentityDraft,
  AgentOptions,
  UpdateAgentParams,
} from "@/types/agent/agent";

import { buildAgentMutationParams } from "./agent-options-mutation";

interface UseExistingAgentOptionsCommandsOptions {
  updateAgent: (agentId: string, params: UpdateAgentParams) => Promise<void>;
}

export function useExistingAgentOptionsCommands({
  updateAgent,
}: UseExistingAgentOptionsCommandsOptions) {
  const saveAgentOptions = useCallback(async (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await updateAgent(agentId, buildAgentMutationParams(title, options, identity));
  }, [updateAgent]);

  return {
    saveAgentOptions,
    validateAgentName: validateAgentNameApi,
  };
}
