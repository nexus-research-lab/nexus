import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { AgentOptions } from "@/types/agent/agent";

export interface UserPreferences {
  chat_default_delivery_policy: AgentConversationDefaultDeliveryPolicy;
  default_agent_options: Partial<AgentOptions>;
  updated_at?: string;
}

export interface UpdateUserPreferencesParams {
  chat_default_delivery_policy?: AgentConversationDefaultDeliveryPolicy;
  default_agent_options?: Partial<AgentOptions>;
}
