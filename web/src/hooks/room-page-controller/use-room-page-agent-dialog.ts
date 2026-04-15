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

import { get_initial_agent_options } from "@/config/options";
import { validate_agent_name_api } from "@/lib/agent-manage-api";
import { Agent, AgentIdentityDraft, AgentOptions } from "@/types/agent";

interface UseRoomPageAgentDialogOptions {
  agents: Agent[];
  create_agent: (params: {
    name: string;
    options?: Partial<AgentOptions>;
    avatar?: string;
    description?: string;
    vibe_tags?: string[];
  }) => Promise<string>;
  update_agent: (
    agent_id: string,
    params: {
      name?: string;
      options?: Partial<AgentOptions>;
      avatar?: string;
      description?: string;
      vibe_tags?: string[];
    },
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
  const dialog_initial_avatar = useMemo(
    () => (dialog_mode === "edit" ? editing_agent?.avatar ?? "" : ""),
    [dialog_mode, editing_agent?.avatar],
  );
  const dialog_initial_description = useMemo(
    () => (dialog_mode === "edit" ? editing_agent?.description ?? "" : ""),
    [dialog_mode, editing_agent?.description],
  );
  const dialog_initial_vibe_tags = useMemo(
    () => (dialog_mode === "edit" ? editing_agent?.vibe_tags ?? [] : []),
    [dialog_mode, editing_agent?.vibe_tags],
  );

  const dialog_initial_options = useMemo(() => {
    if (dialog_mode !== "edit" || !editing_agent) {
      return get_initial_agent_options();
    }

    return {
      provider: editing_agent.options.provider,
      permission_mode: editing_agent.options.permission_mode,
      allowed_tools: editing_agent.options.allowed_tools,
      disallowed_tools: editing_agent.options.disallowed_tools,
      system_prompt: editing_agent.options.system_prompt,
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

  const handle_save_agent_options = useCallback(async (
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    const next_options = {
      provider: options.provider,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      system_prompt: options.system_prompt,
      setting_sources: options.setting_sources,
    };

    if (dialog_mode === "create") {
      await create_agent({
        name: title,
        options: next_options,
        avatar: identity.avatar,
        description: identity.description,
        vibe_tags: identity.vibe_tags,
      });
      return;
    }

    if (dialog_mode === "edit" && editing_agent_id) {
      await update_agent(editing_agent_id, {
        name: title,
        options: next_options,
        avatar: identity.avatar,
        description: identity.description,
        vibe_tags: identity.vibe_tags,
      });
    }
  }, [create_agent, dialog_mode, editing_agent_id, update_agent]);

  const handle_save_existing_agent_options = useCallback(async (
    agent_id: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    const next_options = {
      provider: options.provider,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      system_prompt: options.system_prompt,
      setting_sources: options.setting_sources,
    };

    await update_agent(agent_id, {
      name: title,
      options: next_options,
      avatar: identity.avatar,
      description: identity.description,
      vibe_tags: identity.vibe_tags,
    });
  }, [update_agent]);

  const handle_validate_agent_name = useCallback(async (name: string) => {
    const exclude_agent_id = dialog_mode === "edit" ? editing_agent_id ?? undefined : undefined;
    return validate_agent_name_api(name, exclude_agent_id);
  }, [dialog_mode, editing_agent_id]);

  const handle_validate_agent_name_for_agent = useCallback(async (name: string, agent_id?: string) => {
    return validate_agent_name_api(name, agent_id);
  }, []);

  return {
    is_dialog_open,
    dialog_mode,
    editing_agent_id,
    dialog_initial_title,
    dialog_initial_avatar,
    dialog_initial_description,
    dialog_initial_options,
    dialog_initial_vibe_tags,
    set_is_dialog_open,
    handle_open_create_agent,
    handle_edit_agent,
    handle_save_agent_options,
    handle_save_existing_agent_options,
    handle_validate_agent_name,
    handle_validate_agent_name_for_agent,
  };
}
