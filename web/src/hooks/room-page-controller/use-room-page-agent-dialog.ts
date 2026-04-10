/**
 * =====================================================
 * @File   ：use-room-page-agent-dialog.ts
 * @Date   ：2026-04-08 11:42:07
 * @Author ：leemysw
 * 2026-04-08 11:42:07   Create
 * =====================================================
 */

"use client";

import { useCallback, useMemo, useState } from "react";

import { initialOptions } from "@/config/options";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { Agent, AgentOptions } from "@/types/agent";

interface UseRoomPageAgentDialogOptions {
  agents: Agent[];
  create_agent: (params: { name: string; options?: Partial<AgentOptions> }) => Promise<string>;
  update_agent: (
    agent_id: string,
    params: { name?: string; options?: Partial<AgentOptions> },
  ) => Promise<void>;
}

export function useRoomPageAgentDialog({
  agents,
  create_agent,
  update_agent,
}: UseRoomPageAgentDialogOptions) {
  const [is_dialog_open, set_is_dialog_open] = useState(false);
  const [dialog_mode, set_dialog_mode] = useState<"create" | "edit">("create");
  const [editing_agent_id, set_editing_agent_id] = useState<string | null>(null);

  const editing_agent = useMemo(
    () => agents.find((agent) => agent.agent_id === editing_agent_id) ?? null,
    [agents, editing_agent_id],
  );

  const dialog_initial_title = useMemo(
    () => (dialog_mode === "edit" ? editing_agent?.name : undefined),
    [dialog_mode, editing_agent?.name],
  );

  const dialog_initial_options = useMemo(() => {
    if (dialog_mode !== "edit" || !editing_agent) {
      return initialOptions;
    }

    return {
      model: editing_agent.options.model,
      permission_mode: editing_agent.options.permission_mode,
      allowed_tools: editing_agent.options.allowed_tools,
      disallowed_tools: editing_agent.options.disallowed_tools,
      max_turns: editing_agent.options.max_turns,
      max_thinking_tokens: editing_agent.options.max_thinking_tokens,
      setting_sources: editing_agent.options.setting_sources,
    };
  }, [dialog_mode, editing_agent]);

  const handle_open_create_agent = useCallback(() => {
    set_dialog_mode("create");
    set_editing_agent_id(null);
    set_is_dialog_open(true);
  }, []);

  const handle_edit_agent = useCallback((agent_id: string) => {
    set_dialog_mode("edit");
    set_editing_agent_id(agent_id);
    set_is_dialog_open(true);
  }, []);

  const handle_save_agent_options = useCallback(async (title: string, options: AgentOptions) => {
    const next_options = {
      model: options.model,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      setting_sources: options.setting_sources,
    };

    if (dialog_mode === "create") {
      await create_agent({
        name: title,
        options: next_options,
      });
      return;
    }

    if (dialog_mode === "edit" && editing_agent_id) {
      await update_agent(editing_agent_id, {
        name: title,
        options: next_options,
      });
    }
  }, [create_agent, dialog_mode, editing_agent_id, update_agent]);

  const handle_validate_agent_name = useCallback(async (name: string) => {
    const exclude_agent_id = dialog_mode === "edit" ? editing_agent_id ?? undefined : undefined;
    return validateAgentNameApi(name, exclude_agent_id);
  }, [dialog_mode, editing_agent_id]);

  return {
    is_dialog_open,
    dialog_mode,
    editing_agent_id,
    dialog_initial_title,
    dialog_initial_options,
    set_is_dialog_open,
    handle_open_create_agent,
    handle_edit_agent,
    handle_save_agent_options,
    handle_validate_agent_name,
  };
}
