import { useCallback, useEffect, useMemo, useState } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/options";
import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import { subscribeRoomDirectoryUpdates } from "@/lib/api/room-api";
import { useWebSocket } from "@/lib/websocket";
import { AGENT_LIST_UPDATED_EVENT_NAME, useAgentStore } from "@/store/agent";
import type { AgentRuntimeStatus } from "@/types/agent/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import type { EventMessage } from "@/types/conversation/message";

export interface SidebarDirectoryState {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
  isLoading: boolean;
  refreshDirectory: () => void;
}

interface SidebarDirectorySnapshot {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
}

let sidebarDirectoryCache: SidebarDirectorySnapshot | null = null;

const SIDEBAR_DIRECTORY_FALLBACK_REFRESH_INTERVAL_MS = 120000;

export function useSidebarDirectory(): SidebarDirectoryState {
  const wsUrl = getAgentWsUrl();
  const applyAgentRuntimeStatus = useAgentStore((s) => s.apply_agent_runtime_status);
  const [agents, setAgents] = useState<LauncherAgentSummary[]>(() => sidebarDirectoryCache?.agents ?? []);
  const [rooms, setRooms] = useState<LauncherRoomSummary[]>(() => sidebarDirectoryCache?.rooms ?? []);
  const [conversations, setConversations] = useState<LauncherConversationSummary[]>(
    () => sidebarDirectoryCache?.conversations ?? [],
  );
  const [isLoading, setIsLoading] = useState(sidebarDirectoryCache === null);

  const refreshDirectory = useCallback(() => {
    if (sidebarDirectoryCache === null) {
      setIsLoading(true);
    }
    void getLauncherBootstrapApi().then((payload) => {
      sidebarDirectoryCache = {
        agents: payload.agents,
        rooms: payload.rooms,
        conversations: payload.conversations,
      };
      setAgents(payload.agents);
      setRooms(payload.rooms);
      setConversations(payload.conversations);
      setIsLoading(false);
    }).catch((error) => {
      console.error("[HomeSidebarPanel] 加载侧边栏目录失败:", error);
      if (sidebarDirectoryCache === null) {
        setAgents([]);
        setRooms([]);
        setConversations([]);
      }
      setIsLoading(false);
    });
  }, []);

  useEffect(() => {
    refreshDirectory();
  }, [refreshDirectory]);

  useEffect(() => {
    const refreshIfVisible = () => {
      if (document.visibilityState === "hidden") {
        return;
      }
      refreshDirectory();
    };

    const intervalId = window.setInterval(refreshIfVisible, SIDEBAR_DIRECTORY_FALLBACK_REFRESH_INTERVAL_MS);
    window.addEventListener("focus", refreshIfVisible);
    document.addEventListener("visibilitychange", refreshIfVisible);
    return () => {
      window.clearInterval(intervalId);
      window.removeEventListener("focus", refreshIfVisible);
      document.removeEventListener("visibilitychange", refreshIfVisible);
    };
  }, [refreshDirectory]);

  useEffect(() => subscribeRoomDirectoryUpdates(refreshDirectory), [refreshDirectory]);

  useEffect(() => {
    window.addEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refreshDirectory);
    return () => {
      window.removeEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refreshDirectory);
    };
  }, [refreshDirectory]);

  const agentIds = useMemo(() => agents.map((agent) => agent.id), [agents]);
  const agentIdSet = useMemo(() => new Set(agentIds), [agentIds]);
  const handleRuntimeMessage = useCallback((message: unknown) => {
    const event = message as EventMessage;
    if (event.event_type !== "agent_runtime_event") {
      return;
    }
    if (!event.agent_id || !agentIdSet.has(event.agent_id)) {
      return;
    }
    const payload = event.data as AgentRuntimeStatus | undefined;
    if (!payload?.agent_id) {
      return;
    }
    applyAgentRuntimeStatus(payload);
  }, [agentIdSet, applyAgentRuntimeStatus]);

  const { state: runtimeWsState, send: runtimeWsSend } = useWebSocket({
    url: wsUrl,
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleRuntimeMessage,
  });

  useEffect(() => {
    if (runtimeWsState !== "connected" || agentIds.length === 0) {
      return;
    }

    for (const agentId of agentIds) {
      runtimeWsSend({
        type: "subscribe_workspace",
        agent_id: agentId,
        watch_files: false,
      });
    }

    return () => {
      for (const agentId of agentIds) {
        runtimeWsSend({
          type: "unsubscribe_workspace",
          agent_id: agentId,
          watch_files: false,
        });
      }
    };
  }, [agentIds, runtimeWsSend, runtimeWsState]);

  return {
    agents,
    rooms,
    conversations,
    isLoading,
    refreshDirectory,
  };
}
