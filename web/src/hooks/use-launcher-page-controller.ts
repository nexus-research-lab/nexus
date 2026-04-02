"use client";

import { SetStateAction, useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { useHomeAgentConversationController } from "@/hooks/use-home-agent-conversation-controller";
import { useAppConversationStore } from "@/store/app-conversation";
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
    session_key: app_session_key,
    set_session_key: set_app_session_key,
    clear_session_key: clear_app_session_key,
  } = useAppConversationStore();

  const surface: LauncherSurface = search_params.get("surface") === "app" ? "app" : "launcher";
  const route_app_prompt = search_params.get("app_prompt")?.trim() ?? "";
  const [app_conversation_draft, set_app_conversation_draft] = useState("");

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
    ...agent_conversation,
    surface,
    route_app_prompt,
    is_app_conversation_open,
    app_session_key,
    app_conversation_draft,
    open_app_conversation,
    close_app_conversation,
    clear_route_app_prompt,
    set_app_session_key,
    clear_app_session_key,
    set_app_conversation_draft: handle_change_app_conversation_draft,
  }), [
    agent_conversation,
    surface,
    route_app_prompt,
    is_app_conversation_open,
    app_session_key,
    app_conversation_draft,
    open_app_conversation,
    close_app_conversation,
    clear_route_app_prompt,
    set_app_session_key,
    clear_app_session_key,
    handle_change_app_conversation_draft,
  ]);
}
