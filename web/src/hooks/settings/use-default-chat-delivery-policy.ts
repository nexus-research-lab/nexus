import { useEffect, useState } from "react";

import {
  get_default_chat_delivery_policy,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/options";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { UserPreferences } from "@/types/settings/preferences";

export function useDefaultChatDeliveryPolicy(): AgentConversationDefaultDeliveryPolicy {
  const [policy, setPolicy] = useState<AgentConversationDefaultDeliveryPolicy>(
    () => get_default_chat_delivery_policy(),
  );

  useEffect(() => {
    const handlePreferencesChange = (event: Event) => {
      const payload = (event as CustomEvent<UserPreferences>).detail;
      setPolicy(payload?.chat_default_delivery_policy ?? get_default_chat_delivery_policy());
    };
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, handlePreferencesChange);
    return () => {
      window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, handlePreferencesChange);
    };
  }, []);

  return policy;
}
