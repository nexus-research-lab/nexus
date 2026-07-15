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

export type WebSearchProvider =
  | "brave"
  | "tavily"
  | "exa"
  | "firecrawl"
  | "searxng"
  | "anysearch";

export const DEFAULT_WEB_SEARCH_PROVIDER: WebSearchProvider = "anysearch";

export interface AnySearchSettings {
  domain?: string;
  tag?: string;
  content_types?: string[];
  params?: Record<string, unknown>;
}

export interface WebSearchSettings {
  enabled: boolean;
  provider?: WebSearchProvider;
  base_url?: string;
  allow_private_network?: boolean;
  use_provider_extract?: boolean;
  default_count?: number;
  timeout_seconds?: number;
  cache_ttl_seconds?: number;
  country?: string;
  language?: string;
  search_language?: string;
  freshness?: string;
  search_depth?: "basic" | "advanced";
  extract_depth?: "basic" | "advanced";
  anysearch?: AnySearchSettings;
  api_key_configured?: boolean;
  api_key_masked?: string;
}

export type RuntimeSettings = Partial<
  Record<AgentRuntimeKind, RuntimeSettingsForKind>
>;

export interface UserPreferences {
  chat_default_delivery_policy: AgentConversationDefaultDeliveryPolicy;
  agent_runtime_kind?: AgentRuntimeKind;
  agent_sdk_diagnostics_enabled?: boolean;
  runtime_settings?: RuntimeSettings;
  web_search?: WebSearchSettings;
  web_search_api_key?: string;
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
  web_search?: WebSearchSettings;
  web_search_api_key?: string;
  default_agent_options?: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_vision_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
}
