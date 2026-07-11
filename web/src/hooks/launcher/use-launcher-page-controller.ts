"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import { subscribeRoomDirectoryUpdates } from "@/lib/api/room-api";
import { useAgentStore } from "@/store/agent";
import {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

export function useLauncherPageController() {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const setCurrentAgent = useAgentStore((state) => state.set_current_agent);
  const [isHydrated, setIsHydrated] = useState(false);
  const [agents, setAgents] = useState<LauncherAgentSummary[]>([]);
  const [rooms, setRooms] = useState<LauncherRoomSummary[]>([]);
  const [conversations, setConversations] = useState<
    LauncherConversationSummary[]
  >([]);
  const refreshBootstrap = useCallback(() => {
    void getLauncherBootstrapApi().then((payload) => {
      setAgents(payload.agents);
      setRooms(payload.rooms);
      setConversations(payload.conversations);
    });
  }, []);

  useEffect(() => {
    let isCancelled = false;

    void getLauncherBootstrapApi()
      .then((payload) => {
        if (!isCancelled) {
          setAgents(payload.agents);
          setRooms(payload.rooms);
          setConversations(payload.conversations);
        }
      })
      .catch((error) => {
        console.error(
          "[useLauncherPageController] 初始化 Launcher 数据失败:",
          error,
        );
      })
      .finally(() => {
        if (!isCancelled) {
          setIsHydrated(true);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, []);

  useEffect(
    () => subscribeRoomDirectoryUpdates(refreshBootstrap),
    [refreshBootstrap],
  );

  return useMemo(
    () => ({
      agents,
      rooms,
      conversations,
      current_agent_id: currentAgentId,
      is_hydrated: isHydrated,
      handle_select_agent: setCurrentAgent,
    }),
    [
      agents,
      rooms,
      conversations,
      currentAgentId,
      isHydrated,
      setCurrentAgent,
    ],
  );
}
