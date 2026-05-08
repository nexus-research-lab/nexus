import { useEffect, useState } from "react";

import {
  get_default_chat_delivery_policy,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/options";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { UserPreferences } from "@/types/settings/preferences";

export function useDefaultChatDeliveryPolicy(): AgentConversationDefaultDeliveryPolicy {
  const [policy, set_policy] = useState<AgentConversationDefaultDeliveryPolicy>(
    () => get_default_chat_delivery_policy(),
  );

  useEffect(() => {
    const handle_preferences_change = (event: Event) => {
      const payload = (event as CustomEvent<UserPreferences>).detail;
      set_policy(payload?.chat_default_delivery_policy ?? get_default_chat_delivery_policy());
    };
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, handle_preferences_change);
    return () => {
      window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, handle_preferences_change);
    };
  }, []);

  return policy;
}
