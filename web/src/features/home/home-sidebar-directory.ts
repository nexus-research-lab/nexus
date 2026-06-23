import { useCallback, useEffect, useMemo, useState } from "react";

import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { get_agent_ws_url } from "@/config/options";
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import { subscribe_room_directory_updates } from "@/lib/api/room-api";
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
  is_loading: boolean;
  refresh_directory: () => void;
}

interface SidebarDirectorySnapshot {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
}

let sidebar_directory_cache: SidebarDirectorySnapshot | null = null;

const SIDEBAR_DIRECTORY_FALLBACK_REFRESH_INTERVAL_MS = 120000;

export function useSidebarDirectory(): SidebarDirectoryState {
  const ws_url = get_agent_ws_url();
  const apply_agent_runtime_status = useAgentStore((s) => s.apply_agent_runtime_status);
  const [agents, set_agents] = useState<LauncherAgentSummary[]>(() => sidebar_directory_cache?.agents ?? []);
  const [rooms, set_rooms] = useState<LauncherRoomSummary[]>(() => sidebar_directory_cache?.rooms ?? []);
  const [conversations, set_conversations] = useState<LauncherConversationSummary[]>(
    () => sidebar_directory_cache?.conversations ?? [],
  );
  const [is_loading, set_is_loading] = useState(sidebar_directory_cache === null);

  const refresh_directory = useCallback(() => {
    if (sidebar_directory_cache === null) {
      set_is_loading(true);
    }
    void get_launcher_bootstrap_api().then((payload) => {
      sidebar_directory_cache = {
        agents: payload.agents,
        rooms: payload.rooms,
        conversations: payload.conversations,
      };
      set_agents(payload.agents);
      set_rooms(payload.rooms);
      set_conversations(payload.conversations);
      set_is_loading(false);
    }).catch((error) => {
      console.error("[HomeSidebarPanel] 加载侧边栏目录失败:", error);
      if (sidebar_directory_cache === null) {
        set_agents([]);
        set_rooms([]);
        set_conversations([]);
      }
      set_is_loading(false);
    });
  }, []);

  useEffect(() => {
    refresh_directory();
  }, [refresh_directory]);

  useEffect(() => {
    const refresh_if_visible = () => {
      if (document.visibilityState === "hidden") {
        return;
      }
      refresh_directory();
    };

    const interval_id = window.setInterval(refresh_if_visible, SIDEBAR_DIRECTORY_FALLBACK_REFRESH_INTERVAL_MS);
    window.addEventListener("focus", refresh_if_visible);
    document.addEventListener("visibilitychange", refresh_if_visible);
    return () => {
      window.clearInterval(interval_id);
      window.removeEventListener("focus", refresh_if_visible);
      document.removeEventListener("visibilitychange", refresh_if_visible);
    };
  }, [refresh_directory]);

  useEffect(() => subscribe_room_directory_updates(refresh_directory), [refresh_directory]);

  useEffect(() => {
    window.addEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refresh_directory);
    return () => {
      window.removeEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refresh_directory);
    };
  }, [refresh_directory]);

  const agent_ids = useMemo(() => agents.map((agent) => agent.id), [agents]);
  const agent_id_set = useMemo(() => new Set(agent_ids), [agent_ids]);
  const handle_runtime_message = useCallback((message: unknown) => {
    const event = message as EventMessage;
    if (event.event_type !== "agent_runtime_event") {
      return;
    }
    if (!event.agent_id || !agent_id_set.has(event.agent_id)) {
      return;
    }
    const payload = event.data as AgentRuntimeStatus | undefined;
    if (!payload?.agent_id) {
      return;
    }
    apply_agent_runtime_status(payload);
  }, [agent_id_set, apply_agent_runtime_status]);

  const { state: runtime_ws_state, send: runtime_ws_send } = useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_runtime_message,
  });

  useEffect(() => {
    if (runtime_ws_state !== "connected" || agent_ids.length === 0) {
      return;
    }

    for (const agent_id of agent_ids) {
      runtime_ws_send({
        type: "subscribe_workspace",
        agent_id,
        watch_files: false,
      });
    }

    return () => {
      for (const agent_id of agent_ids) {
        runtime_ws_send({
          type: "unsubscribe_workspace",
          agent_id,
          watch_files: false,
        });
      }
    };
  }, [agent_ids, runtime_ws_send, runtime_ws_state]);

  return {
    agents,
    rooms,
    conversations,
    is_loading,
    refresh_directory,
  };
}
