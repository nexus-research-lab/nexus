import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { AgentOptions } from "@/types/agent/agent";

export type AgentRuntimeKind = "claude" | "nxs";

export function normalize_agent_runtime_kind(value?: string | null): AgentRuntimeKind {
  switch (value?.trim().toLowerCase()) {
    case "nxs":
    case "go":
    case "go-native":
    case "gonative":
      return "nxs";
    default:
      return "claude";
  }
}

export interface NXSRuntimeStatus {
  available: boolean;
  path?: string;
  source?: "env" | "app_root" | "cache" | string;
  can_download: boolean;
  message?: string;
}

export interface ModelSelectionPreference {
  provider?: string;
  model?: string;
}

export interface UserPreferences {
  chat_default_delivery_policy: AgentConversationDefaultDeliveryPolicy;
  agent_runtime_kind?: AgentRuntimeKind;
  default_agent_options: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
  updated_at?: string;
}

export interface UpdateUserPreferencesParams {
  chat_default_delivery_policy?: AgentConversationDefaultDeliveryPolicy;
  agent_runtime_kind?: AgentRuntimeKind;
  default_agent_options?: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
}
