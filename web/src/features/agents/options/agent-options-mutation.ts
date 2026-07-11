import type {
  AgentIdentityDraft,
  AgentOptions,
  CreateAgentParams,
} from "@/types/agent/agent";

import { pickAgentEditableOptions } from "./agent-options-constants";

export function buildAgentMutationParams(
  name: string,
  options: AgentOptions,
  identity: AgentIdentityDraft,
): CreateAgentParams {
  return {
    name,
    options: pickAgentEditableOptions(options),
    avatar: identity.avatar,
    description: identity.description,
    vibe_tags: identity.vibe_tags,
  };
}
