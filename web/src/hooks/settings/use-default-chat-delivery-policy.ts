import { useEffect, useState } from "react";

import {
  getDefaultChatDeliveryPolicy,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/runtime-options";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { UserPreferences } from "@/types/settings/preferences";

export function useDefaultChatDeliveryPolicy(): AgentConversationDefaultDeliveryPolicy {
  const [policy, setPolicy] = useState<AgentConversationDefaultDeliveryPolicy>(
    () => getDefaultChatDeliveryPolicy(),
  );

  useEffect(() => {
    const handlePreferencesChange = (event: Event) => {
      const payload = (event as CustomEvent<UserPreferences>).detail;
      setPolicy(payload?.chat_default_delivery_policy ?? getDefaultChatDeliveryPolicy());
    };
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, handlePreferencesChange);
    return () => {
      window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, handlePreferencesChange);
    };
  }, []);

  return policy;
}
