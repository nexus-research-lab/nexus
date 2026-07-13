import { useCallback, useEffect, useMemo } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { parseAgentRuntimeStatus } from "@/lib/agent-runtime-status";
import { useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import { useAgentStore } from "@/store/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import { refreshHomeDirectory, useHomeDirectory } from "../home-directory-resource";

export interface SidebarDirectoryState {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  isLoading: boolean;
  refreshDirectory: () => void;
  rooms: LauncherRoomSummary[];
}

export function useSidebarDirectory(): SidebarDirectoryState {
  const directory = useHomeDirectory();
  const applyAgentRuntimeStatus = useAgentStore((state) => state.apply_agent_runtime_status);
  const agentIds = useMemo(
    () => directory.agents.map((agent) => agent.id),
    [directory.agents],
  );
  const agentIdSet = useMemo(() => new Set(agentIds), [agentIds]);
  const handleRuntimeMessage = useCallback((message: unknown) => {
    const event = parseEventMessage(message);
    if (!event) {
      return;
    }
    if (event.event_type !== "agent_runtime_event" || !event.agent_id) {
      return;
    }
    if (!agentIdSet.has(event.agent_id)) {
      return;
    }
    const payload = parseAgentRuntimeStatus(event.data);
    if (payload) {
      applyAgentRuntimeStatus(payload);
    }
  }, [agentIdSet, applyAgentRuntimeStatus]);

  const { state: runtimeState, send: sendRuntimeMessage } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30_000,
    onMessage: handleRuntimeMessage,
  });

  useEffect(() => {
    if (runtimeState !== "connected" || agentIds.length === 0) {
      return undefined;
    }
    for (const agentId of agentIds) {
      sendRuntimeMessage({
        type: "subscribe_workspace",
        agent_id: agentId,
        watch_files: false,
      });
    }
    return () => {
      for (const agentId of agentIds) {
        sendRuntimeMessage({
          type: "unsubscribe_workspace",
          agent_id: agentId,
          watch_files: false,
        });
      }
    };
  }, [agentIds, runtimeState, sendRuntimeMessage]);

  return {
    ...directory,
    refreshDirectory: refreshHomeDirectory,
  };
}
