"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { useHomeAgentConversationController } from "@/hooks/use-home-agent-conversation-controller";
import { LauncherSearchParams } from "@/types/route";

type LauncherSurface = NonNullable<LauncherSearchParams["surface"]>;

function buildLauncherSearchParams(search_params: LauncherSearchParams) {
  const next_search_params = new URLSearchParams();

  if (search_params.surface && search_params.surface !== "launcher") {
    next_search_params.set("surface", search_params.surface);
  }

  if (search_params.app_prompt?.trim()) {
    next_search_params.set("app_prompt", search_params.app_prompt.trim());
  }

  return next_search_params;
}

export function useLauncherPageController() {
  const agent_conversation = useHomeAgentConversationController();
  const [search_params, set_search_params] = useSearchParams();

  const surface: LauncherSurface = search_params.get("surface") === "app" ? "app" : "launcher";
  const route_app_prompt = search_params.get("app_prompt")?.trim() ?? "";
  const [app_conversation_draft, set_app_conversation_draft] = useState(route_app_prompt);

  useEffect(() => {
    set_app_conversation_draft(route_app_prompt);
  }, [route_app_prompt]);

  const is_app_conversation_open = surface === "app";

  const set_launcher_search = useCallback((next_search: LauncherSearchParams) => {
    set_search_params(buildLauncherSearchParams(next_search), { replace: true });
  }, [set_search_params]);

  const open_app_conversation = useCallback((next_prompt?: string) => {
    const trimmed_prompt = next_prompt?.trim() ?? "";
    set_app_conversation_draft(trimmed_prompt);
    set_launcher_search({
      surface: "app",
      app_prompt: trimmed_prompt || undefined,
    });
  }, [set_launcher_search]);

  const close_app_conversation = useCallback(() => {
    set_app_conversation_draft("");
    set_launcher_search({});
  }, [set_launcher_search]);

  const submit_app_conversation = useCallback((next_prompt: string) => {
    const trimmed_prompt = next_prompt.trim();
    set_app_conversation_draft(trimmed_prompt);
    set_launcher_search({
      surface: "app",
      app_prompt: trimmed_prompt || undefined,
    });
  }, [set_launcher_search]);

  return useMemo(() => ({
    ...agent_conversation,
    surface,
    is_app_conversation_open,
    app_conversation_draft,
    open_app_conversation,
    close_app_conversation,
    set_app_conversation_draft,
    submit_app_conversation,
  }), [
    agent_conversation,
    surface,
    is_app_conversation_open,
    app_conversation_draft,
    open_app_conversation,
    close_app_conversation,
    submit_app_conversation,
  ]);
}
