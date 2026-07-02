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
import { build_agent_options_save_payload } from "@/features/agents/options/agent-options-constants";
import { validate_agent_name_api } from "@/lib/api/agent-manage-api";
import { Agent, AgentIdentityDraft, AgentOptions } from "@/types/agent/agent";

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
    agentId: string,
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
  create_agent: createAgent,
  update_agent: updateAgent,
}: UseRoomPageAgentDialogOptions) {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingAgentId, setEditingAgentId] = useState<string | null>(null);

  const editingAgent = useMemo(
    () => agents.find((agent) => agent.agent_id === editingAgentId) ?? null,
    [agents, editingAgentId],
  );

  const dialogInitialTitle = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.name : undefined),
    [dialogMode, editingAgent?.name],
  );
  const dialogInitialAvatar = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.avatar ?? "" : ""),
    [dialogMode, editingAgent?.avatar],
  );
  const dialogInitialDescription = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.description ?? "" : ""),
    [dialogMode, editingAgent?.description],
  );
  const dialogInitialVibeTags = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.vibe_tags ?? [] : []),
    [dialogMode, editingAgent?.vibe_tags],
  );

  const dialogInitialOptions = useMemo(() => {
    if (dialogMode !== "edit" || !editingAgent) {
      return get_initial_agent_options();
    }

    return {
      provider: editingAgent.options.provider,
      model: editingAgent.options.model,
      permission_mode: editingAgent.options.permission_mode,
      allowed_tools: editingAgent.options.allowed_tools,
      disallowed_tools: editingAgent.options.disallowed_tools,
      max_turns: editingAgent.options.max_turns,
      max_thinking_tokens: editingAgent.options.max_thinking_tokens,
      mcp_servers: editingAgent.options.mcp_servers,
      setting_sources: editingAgent.options.setting_sources,
    };
  }, [dialogMode, editingAgent]);

  const handleOpenCreateAgent = useCallback(() => {
    setDialogMode("create");
    setEditingAgentId(null);
    setIsDialogOpen(true);
  }, []);

  const handleEditAgent = useCallback((agentId: string) => {
    setDialogMode("edit");
    setEditingAgentId(agentId);
    setIsDialogOpen(true);
  }, []);

  const handleSaveAgentOptions = useCallback(async (
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    const nextOptions = build_agent_options_save_payload(options);

    if (dialogMode === "create") {
      await createAgent({
        name: title,
        options: nextOptions,
        avatar: identity.avatar,
        description: identity.description,
        vibe_tags: identity.vibe_tags,
      });
      return;
    }

    if (dialogMode === "edit" && editingAgentId) {
      await updateAgent(editingAgentId, {
        name: title,
        options: nextOptions,
        avatar: identity.avatar,
        description: identity.description,
        vibe_tags: identity.vibe_tags,
      });
    }
  }, [createAgent, dialogMode, editingAgentId, updateAgent]);

  const handleSaveExistingAgentOptions = useCallback(async (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    const nextOptions = build_agent_options_save_payload(options);

    await updateAgent(agentId, {
      name: title,
      options: nextOptions,
      avatar: identity.avatar,
      description: identity.description,
      vibe_tags: identity.vibe_tags,
    });
  }, [updateAgent]);

  const handleValidateAgentName = useCallback(async (name: string) => {
    const excludeAgentId = dialogMode === "edit" ? editingAgentId ?? undefined : undefined;
    return validate_agent_name_api(name, excludeAgentId);
  }, [dialogMode, editingAgentId]);

  const handleValidateAgentNameForAgent = useCallback(async (name: string, agentId?: string) => {
    return validate_agent_name_api(name, agentId);
  }, []);

  return {
    is_dialog_open: isDialogOpen,
    dialog_mode: dialogMode,
    editing_agent_id: editingAgentId,
    dialog_initial_title: dialogInitialTitle,
    dialog_initial_avatar: dialogInitialAvatar,
    dialog_initial_description: dialogInitialDescription,
    dialog_initial_options: dialogInitialOptions,
    dialog_initial_vibe_tags: dialogInitialVibeTags,
    set_is_dialog_open: setIsDialogOpen,
    handle_open_create_agent: handleOpenCreateAgent,
    handle_edit_agent: handleEditAgent,
    handle_save_agent_options: handleSaveAgentOptions,
    handle_save_existing_agent_options: handleSaveExistingAgentOptions,
    handle_validate_agent_name: handleValidateAgentName,
    handle_validate_agent_name_for_agent: handleValidateAgentNameForAgent,
  };
}
