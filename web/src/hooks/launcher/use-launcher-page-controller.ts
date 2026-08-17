"use client";

import { useMemo } from "react";

import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { useAgentStore } from "@/store/agent";

export function useLauncherPageController() {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const setCurrentAgent = useAgentStore((state) => state.set_current_agent);
  const { agents, conversations, isLoading, rooms } = useHomeDirectory();

  return useMemo(
    () => ({
      agents,
      rooms,
      conversations,
      current_agent_id: currentAgentId,
      is_hydrated: !isLoading,
      handle_select_agent: setCurrentAgent,
    }),
    [
      agents,
      rooms,
      conversations,
      currentAgentId,
      isLoading,
      setCurrentAgent,
    ],
  );
}
