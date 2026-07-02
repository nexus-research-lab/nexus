"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { is_main_agent } from "@/config/options";
import { useRoomPageAgentDialog } from "@/hooks/room-page-controller/use-room-page-agent-dialog";
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import { subscribe_room_directory_updates } from "@/lib/api/room-api";
import { useAgentStore } from "@/store/agent";
import {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

export function useLauncherPageController() {
  const storedAgents = useAgentStore((state) => state.agents);
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const createAgent = useAgentStore((state) => state.create_agent);
  const updateAgent = useAgentStore((state) => state.update_agent);
  const deleteAgent = useAgentStore((state) => state.delete_agent);
  const setCurrentAgent = useAgentStore((state) => state.set_current_agent);
  const [isHydrated, setIsHydrated] = useState(false);
  const [agents, setAgents] = useState<LauncherAgentSummary[]>([]);
  const [rooms, setRooms] = useState<LauncherRoomSummary[]>([]);
  const [conversations, setConversations] = useState<
    LauncherConversationSummary[]
  >([]);
  const dialogAgents = useMemo(
    () => storedAgents.filter((agent) => !is_main_agent(agent.agent_id)),
    [storedAgents],
  );

  const refreshBootstrap = useCallback(() => {
    void get_launcher_bootstrap_api().then((payload) => {
      setAgents(payload.agents);
      setRooms(payload.rooms);
      setConversations(payload.conversations);
    });
  }, []);

  const agentDialog = useRoomPageAgentDialog({
    agents: dialogAgents,
    create_agent: async (params) => {
      const nextAgentId = await createAgent(params);
      setCurrentAgent(nextAgentId);
      refreshBootstrap();
      return nextAgentId;
    },
    update_agent: async (agentId, params) => {
      await updateAgent(agentId, params);
      refreshBootstrap();
    },
  });

  useEffect(() => {
    let isCancelled = false;

    void get_launcher_bootstrap_api()
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
    () => subscribe_room_directory_updates(refreshBootstrap),
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
      handle_delete_agent: async (agentId: string) => {
        await deleteAgent(agentId);
        refreshBootstrap();
      },
      ...agentDialog,
    }),
    [
      agents,
      rooms,
      conversations,
      currentAgentId,
      isHydrated,
      setCurrentAgent,
      deleteAgent,
      refreshBootstrap,
      agentDialog,
    ],
  );
}
