import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { AgentOptions } from "@/types/agent/agent";

export type AgentRuntimeKind = "claude" | "nxs";

export function normalizeAgentRuntimeKind(value?: string | null): AgentRuntimeKind {
  switch (value?.trim().toLowerCase()) {
    case "nxs":
    case "go":
    case "go-native":
    case "gonative":
      return "nxs";
    case "claude":
    case "claude-code":
    case "claudecode":
      return "claude";
    default:
      return "nxs";
  }
}

export interface NXSRuntimeStatus {
  available: boolean;
  path?: string;
  source?: "env" | string;
  can_download: boolean;
  message?: string;
}

export interface ModelSelectionPreference {
  provider?: string;
  model?: string;
}

export interface RuntimeSettingsForKind {
  tool_search?: boolean;
}

export type RuntimeSettings = Partial<
  Record<AgentRuntimeKind, RuntimeSettingsForKind>
>;

export interface UserPreferences {
  chat_default_delivery_policy: AgentConversationDefaultDeliveryPolicy;
  agent_runtime_kind?: AgentRuntimeKind;
  agent_sdk_diagnostics_enabled?: boolean;
  runtime_settings?: RuntimeSettings;
  default_agent_options: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_vision_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
  updated_at?: string;
}

export interface UpdateUserPreferencesParams {
  chat_default_delivery_policy?: AgentConversationDefaultDeliveryPolicy;
  agent_runtime_kind?: AgentRuntimeKind;
  agent_sdk_diagnostics_enabled?: boolean;
  runtime_settings?: RuntimeSettings;
  default_agent_options?: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_vision_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
}
