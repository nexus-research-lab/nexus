import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { AgentOptions } from "@/types/agent/agent";

export interface ModelSelectionPreference {
  provider?: string;
  model?: string;
}

export interface UserPreferences {
  chat_default_delivery_policy: AgentConversationDefaultDeliveryPolicy;
  default_agent_options: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
  updated_at?: string;
}

export interface UpdateUserPreferencesParams {
  chat_default_delivery_policy?: AgentConversationDefaultDeliveryPolicy;
  default_agent_options?: Partial<AgentOptions>;
  default_image_model_selection?: ModelSelectionPreference;
  default_background_model_selection?: ModelSelectionPreference;
}
