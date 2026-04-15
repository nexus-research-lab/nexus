/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：use-launcher-page-controller.ts
# @Date   ：2026-04-15 16:39
# @Author ：leemysw
# 2026-04-15 16:39   Create
# =====================================================
*/

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { is_main_agent } from "@/config/options";
import { useRoomPageAgentDialog } from "@/hooks/room-page-controller/use-room-page-agent-dialog";
import { list_rooms, subscribe_room_list_updates } from "@/lib/api/room-api";
import { useConversationStore } from "@/store/conversation";
import { useAgentStore } from "@/store/agent";
import { RoomAggregate } from "@/types/conversation/room";

export function useLauncherPageController() {
  const agents = useAgentStore((state) => state.agents);
  const current_agent_id = useAgentStore((state) => state.current_agent_id);
  const create_agent = useAgentStore((state) => state.create_agent);
  const update_agent = useAgentStore((state) => state.update_agent);
  const delete_agent = useAgentStore((state) => state.delete_agent);
  const set_current_agent = useAgentStore((state) => state.set_current_agent);
  const load_agents_from_server = useAgentStore((state) => state.load_agents_from_server);
  const conversations = useConversationStore((state) => state.conversations);
  const load_conversations_from_server = useConversationStore((state) => state.load_conversations_from_server);
  const [is_hydrated, set_is_hydrated] = useState(false);
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const regular_agents = useMemo(
    () => agents.filter((agent) => !is_main_agent(agent.agent_id)),
    [agents],
  );

  const agent_dialog = useRoomPageAgentDialog({
    agents: regular_agents,
    create_agent: async (params) => {
      const next_agent_id = await create_agent(params);
      set_current_agent(next_agent_id);
      return next_agent_id;
    },
    update_agent,
  });

  const refresh_rooms = useCallback(() => {
    void list_rooms(200).then(set_rooms);
  }, []);

  useEffect(() => {
    let is_cancelled = false;

    void Promise.all([
      load_agents_from_server(),
      load_conversations_from_server(),
      list_rooms(200).then(set_rooms),
    ])
      .catch((error) => {
        console.error("[useLauncherPageController] 初始化 Launcher 数据失败:", error);
      })
      .finally(() => {
        if (!is_cancelled) {
          set_is_hydrated(true);
        }
      });

    return () => {
      is_cancelled = true;
    };
  }, [load_agents_from_server, load_conversations_from_server]);

  useEffect(() => subscribe_room_list_updates(refresh_rooms), [refresh_rooms]);

  return useMemo(() => ({
    agents: regular_agents,
    rooms,
    conversations,
    current_agent_id,
    is_hydrated,
    refresh_conversations: load_conversations_from_server,
    handle_select_agent: set_current_agent,
    handle_delete_agent: delete_agent,
    ...agent_dialog,
  }), [
    regular_agents,
    rooms,
    conversations,
    current_agent_id,
    is_hydrated,
    load_conversations_from_server,
    set_current_agent,
    delete_agent,
    agent_dialog,
  ]);
}
