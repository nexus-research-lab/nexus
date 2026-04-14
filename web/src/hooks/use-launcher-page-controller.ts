/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：use-launcher-page-controller.ts
# @Date   ：2026-04-12 18:11
# @Author ：leemysw
# 2026-04-12 18:11   Create
# =====================================================
*/

"use client";

import { SetStateAction, useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { getDefaultAgentId } from "@/config/options";
import { useRoomPageAgentDialog } from "@/hooks/room-page-controller/use-room-page-agent-dialog";
import { listRooms, subscribe_room_list_updates } from "@/lib/room-api";
import { buildLauncherAppSessionKey } from "@/lib/session-key";
import { useConversationStore } from "@/store/conversation";
import { useAgentStore } from "@/store/agent";
import { LauncherSearchParams } from "@/types/route";
import { RoomAggregate } from "@/types/room";

type LauncherSurface = NonNullable<LauncherSearchParams["surface"]>;

function buildLauncherSearchParams(
  search_params: LauncherSearchParams,
  current_search_params: URLSearchParams,
) {
  const next_search_params = new URLSearchParams(current_search_params);

  if (search_params.surface && search_params.surface !== "launcher") {
    next_search_params.set("surface", search_params.surface);
  } else {
    next_search_params.delete("surface");
  }

  if (search_params.app_prompt?.trim()) {
    next_search_params.set("app_prompt", search_params.app_prompt.trim());
  } else {
    next_search_params.delete("app_prompt");
  }

  return next_search_params;
}

export function useLauncherPageController() {
  const [search_params, set_search_params] = useSearchParams();
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
  const app_session_key = buildLauncherAppSessionKey(getDefaultAgentId());

  const agent_dialog = useRoomPageAgentDialog({
    agents,
    create_agent: async (params) => {
      const next_agent_id = await create_agent(params);
      set_current_agent(next_agent_id);
      return next_agent_id;
    },
    update_agent,
  });

  const surface: LauncherSurface = search_params.get("surface") === "app" ? "app" : "launcher";
  const route_app_prompt = search_params.get("app_prompt")?.trim() ?? "";
  const [app_conversation_draft, set_app_conversation_draft] = useState("");

  const refresh_rooms = useCallback(() => {
    void listRooms(200).then(set_rooms);
  }, []);

  useEffect(() => {
    let is_cancelled = false;

    void Promise.all([
      load_agents_from_server(),
      load_conversations_from_server(),
      listRooms(200).then(set_rooms),
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

  useEffect(() => {
    if (search_params.get("blobDebug") !== "1" || surface === "app") {
      return;
    }

    const next_search_params = new URLSearchParams(search_params);
    next_search_params.set("surface", "app");
    set_search_params(next_search_params, { replace: true });
  }, [search_params, set_search_params, surface]);

  const is_app_conversation_open = surface === "app";

  const set_launcher_search = useCallback((next_search: LauncherSearchParams) => {
    const next_search_params = buildLauncherSearchParams(next_search, search_params);
    if (next_search_params.toString() === search_params.toString()) {
      return;
    }
    set_search_params(next_search_params, { replace: true });
  }, [search_params, set_search_params]);

  const open_app_conversation = useCallback((next_prompt?: string) => {
    const trimmed_prompt = next_prompt?.trim() ?? "";
    set_app_conversation_draft("");
    set_launcher_search({
      surface: "app",
      app_prompt: trimmed_prompt || undefined,
    });
  }, [set_launcher_search]);

  const close_app_conversation = useCallback(() => {
    set_app_conversation_draft("");
    set_launcher_search({});
  }, [set_launcher_search]);

  const clear_route_app_prompt = useCallback(() => {
    set_launcher_search({
      surface: "app",
      app_prompt: undefined,
    });
  }, [set_launcher_search]);

  const handle_change_app_conversation_draft = useCallback((next_value: SetStateAction<string>) => {
    set_app_conversation_draft((current_value) => (
      typeof next_value === "function"
        ? next_value(current_value)
        : next_value
    ));

    // 中文注释：路由里的 app_prompt 只用于一次性启动。
    // 用户一旦开始手动输入，就应立刻丢弃这份启动 prompt，避免再次回灌到输入链路。
    if (route_app_prompt) {
      set_launcher_search({
        surface: "app",
        app_prompt: undefined,
      });
    }
  }, [route_app_prompt, set_launcher_search]);

  return useMemo(() => ({
    agents,
    rooms,
    conversations,
    current_agent_id,
    is_hydrated,
    surface,
    route_app_prompt,
    is_app_conversation_open,
    app_session_key,
    app_conversation_draft,
    refresh_conversations: load_conversations_from_server,
    handle_select_agent: set_current_agent,
    handle_delete_agent: delete_agent,
    open_app_conversation,
    close_app_conversation,
    clear_route_app_prompt,
    set_app_conversation_draft: handle_change_app_conversation_draft,
    ...agent_dialog,
  }), [
    agents,
    rooms,
    conversations,
    current_agent_id,
    is_hydrated,
    surface,
    route_app_prompt,
    is_app_conversation_open,
    app_session_key,
    app_conversation_draft,
    load_conversations_from_server,
    set_current_agent,
    delete_agent,
    open_app_conversation,
    close_app_conversation,
    clear_route_app_prompt,
    handle_change_app_conversation_draft,
    agent_dialog,
  ]);
}
