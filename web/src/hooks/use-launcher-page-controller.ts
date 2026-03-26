"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { useHomeAgentConversationController } from "@/hooks/use-home-agent-conversation-controller";
import { listRooms } from "@/lib/room-api";
import { useAppConversationStore } from "@/store/app-conversation";
import { RuntimeRoomListItem } from "@/types/launcher";
import { LauncherSearchParams } from "@/types/route";

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
  const agent_conversation = useHomeAgentConversationController();
  const [search_params, set_search_params] = useSearchParams();
  const {
    conversation_key: app_conversation_key,
    set_conversation_key: set_app_conversation_key,
    clear_conversation_key: clear_app_conversation_key,
  } = useAppConversationStore();

  const surface: LauncherSurface = search_params.get("surface") === "app" ? "app" : "launcher";
  const route_app_prompt = search_params.get("app_prompt")?.trim() ?? "";
  const [app_conversation_draft, set_app_conversation_draft] = useState(route_app_prompt);
  const [runtime_rooms, set_runtime_rooms] = useState<RuntimeRoomListItem[]>([]);

  const refresh_runtime_rooms = useCallback(async () => {
    try {
      const rooms = await listRooms(20);
      set_runtime_rooms(rooms.map((item) => ({ room: item.room, members: item.members })));
    } catch {
      set_runtime_rooms([]);
    }
  }, []);

  useEffect(() => {
    set_app_conversation_draft(route_app_prompt);
  }, [route_app_prompt]);

  useEffect(() => {
    if (search_params.get("blobDebug") !== "1" || surface === "app") {
      return;
    }

    const next_search_params = new URLSearchParams(search_params);
    next_search_params.set("surface", "app");
    set_search_params(next_search_params, { replace: true });
  }, [search_params, set_search_params, surface]);

  useEffect(() => {
    let is_cancelled = false;
    void refresh_runtime_rooms().catch(() => {
      if (!is_cancelled) {
        set_runtime_rooms([]);
      }
    });

    return () => {
      is_cancelled = true;
    };
  }, [refresh_runtime_rooms]);

  const is_app_conversation_open = surface === "app";

  const set_launcher_search = useCallback((next_search: LauncherSearchParams) => {
    set_search_params(buildLauncherSearchParams(next_search, search_params), { replace: true });
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

  return useMemo(() => ({
    ...agent_conversation,
    surface,
    runtime_rooms,
    route_app_prompt,
    is_app_conversation_open,
    app_conversation_key,
    app_conversation_draft,
    open_app_conversation,
    close_app_conversation,
    clear_route_app_prompt,
    refresh_runtime_rooms,
    set_app_conversation_key,
    clear_app_conversation_key,
    set_app_conversation_draft,
  }), [
    agent_conversation,
    surface,
    runtime_rooms,
    route_app_prompt,
    is_app_conversation_open,
    app_conversation_key,
    app_conversation_draft,
    open_app_conversation,
    close_app_conversation,
    clear_route_app_prompt,
    refresh_runtime_rooms,
    set_app_conversation_key,
    clear_app_conversation_key,
  ]);
}
