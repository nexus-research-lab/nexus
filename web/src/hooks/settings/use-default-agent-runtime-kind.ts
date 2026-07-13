import { useEffect, useState } from "react";

import {
  getDefaultAgentRuntimeKind,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/runtime-options";
import type {
  AgentRuntimeKind,
  UserPreferences,
} from "@/types/settings/preferences";

export function useDefaultAgentRuntimeKind(): AgentRuntimeKind {
  const [runtimeKind, setRuntimeKind] = useState<AgentRuntimeKind>(
    () => getDefaultAgentRuntimeKind(),
  );

  useEffect(() => {
    const handlePreferencesChange = (event: Event) => {
      const payload = (event as CustomEvent<UserPreferences>).detail;
      setRuntimeKind(
        payload?.agent_runtime_kind ?? getDefaultAgentRuntimeKind(),
      );
    };
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, handlePreferencesChange);
    return () => {
      window.removeEventListener(
        USER_PREFERENCES_CHANGED_EVENT,
        handlePreferencesChange,
      );
    };
  }, []);

  return runtimeKind;
}
