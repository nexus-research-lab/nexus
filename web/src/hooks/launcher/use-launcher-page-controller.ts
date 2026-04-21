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
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import { subscribe_room_directory_updates } from "@/lib/api/room-api";
import { useAgentStore } from "@/store/agent";
import {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

interface LauncherBootstrapState {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
}

let launcher_bootstrap_inflight: Promise<LauncherBootstrapState> | null = null;

function run_launcher_bootstrap(): Promise<LauncherBootstrapState> {
  if (launcher_bootstrap_inflight) {
    return launcher_bootstrap_inflight;
  }

  launcher_bootstrap_inflight = get_launcher_bootstrap_api()
    .then((payload) => ({
      agents: payload.agents,
      rooms: payload.rooms,
      conversations: payload.conversations,
    }))
    .finally(() => {
      launcher_bootstrap_inflight = null;
    });
  return launcher_bootstrap_inflight;
}

export function useLauncherPageController() {
  const stored_agents = useAgentStore((state) => state.agents);
  const current_agent_id = useAgentStore((state) => state.current_agent_id);
  const create_agent = useAgentStore((state) => state.create_agent);
  const update_agent = useAgentStore((state) => state.update_agent);
  const delete_agent = useAgentStore((state) => state.delete_agent);
  const set_current_agent = useAgentStore((state) => state.set_current_agent);
  const [is_hydrated, set_is_hydrated] = useState(false);
  const [agents, set_agents] = useState<LauncherAgentSummary[]>([]);
  const [rooms, set_rooms] = useState<LauncherRoomSummary[]>([]);
  const [conversations, set_conversations] = useState<
    LauncherConversationSummary[]
  >([]);
  const dialog_agents = useMemo(
    () => stored_agents.filter((agent) => !is_main_agent(agent.agent_id)),
    [stored_agents],
  );

  const refresh_bootstrap = useCallback(() => {
    void get_launcher_bootstrap_api().then((payload) => {
      set_agents(payload.agents);
      set_rooms(payload.rooms);
      set_conversations(payload.conversations);
    });
  }, []);

  const agent_dialog = useRoomPageAgentDialog({
    agents: dialog_agents,
    create_agent: async (params) => {
      const next_agent_id = await create_agent(params);
      set_current_agent(next_agent_id);
      refresh_bootstrap();
      return next_agent_id;
    },
    update_agent: async (agent_id, params) => {
      await update_agent(agent_id, params);
      refresh_bootstrap();
    },
  });

  useEffect(() => {
    let is_cancelled = false;

    void run_launcher_bootstrap()
      .then((payload) => {
        if (!is_cancelled) {
          set_agents(payload.agents);
          set_rooms(payload.rooms);
          set_conversations(payload.conversations);
        }
      })
      .catch((error) => {
        console.error(
          "[useLauncherPageController] 初始化 Launcher 数据失败:",
          error,
        );
      })
      .finally(() => {
        if (!is_cancelled) {
          set_is_hydrated(true);
        }
      });

    return () => {
      is_cancelled = true;
    };
  }, []);

  useEffect(
    () => subscribe_room_directory_updates(refresh_bootstrap),
    [refresh_bootstrap],
  );

  return useMemo(
    () => ({
      agents,
      rooms,
      conversations,
      current_agent_id,
      is_hydrated,
      handle_select_agent: set_current_agent,
      handle_delete_agent: async (agent_id: string) => {
        await delete_agent(agent_id);
        refresh_bootstrap();
      },
      ...agent_dialog,
    }),
    [
      agents,
      rooms,
      conversations,
      current_agent_id,
      is_hydrated,
      set_current_agent,
      delete_agent,
      refresh_bootstrap,
      agent_dialog,
    ],
  );
}
