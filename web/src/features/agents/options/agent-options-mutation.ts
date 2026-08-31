import type {
  AgentIdentityDraft,
  AgentOptions,
  CreateAgentParams,
  UpdateAgentParams,
} from "@/types/agent/agent";

import { pickAgentEditableOptions } from "@/lib/agent-options";

export function buildAgentMutationParams(
  name: string,
  options: AgentOptions,
  identity: AgentIdentityDraft,
): UpdateAgentParams {
  return {
    name,
    options: pickAgentEditableOptions(options),
    avatar: identity.avatar,
    business_tags: identity.business_tags,
    description: identity.description,
    vibe_tags: identity.vibe_tags,
  };
}

export function buildCreateAgentMutationParams(
  name: string,
  options: AgentOptions,
  identity: AgentIdentityDraft,
): CreateAgentParams {
  return {
    ...buildAgentMutationParams(name, options, identity),
    name,
    profile_template: identity.profile_template,
  };
}
