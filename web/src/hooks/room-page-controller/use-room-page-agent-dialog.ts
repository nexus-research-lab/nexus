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

import { getInitialAgentOptions } from "@/config/options";
import { buildAgentOptionsSavePayload } from "@/features/agents/options/agent-options-constants";
import { validateAgentNameApi } from "@/lib/api/agent-manage-api";
import { Agent, AgentIdentityDraft, AgentOptions } from "@/types/agent/agent";

interface UseRoomPageAgentDialogOptions {
  agents: Agent[];
  createAgent: (params: {
    name: string;
    options?: Partial<AgentOptions>;
    avatar?: string;
    description?: string;
    vibe_tags?: string[];
  }) => Promise<string>;
  updateAgent: (
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
  createAgent: createAgent,
  updateAgent: updateAgent,
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
      return getInitialAgentOptions();
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
    const nextOptions = buildAgentOptionsSavePayload(options);

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
    const nextOptions = buildAgentOptionsSavePayload(options);

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
    return validateAgentNameApi(name, excludeAgentId);
  }, [dialogMode, editingAgentId]);

  const handleValidateAgentNameForAgent = useCallback(async (name: string, agentId?: string) => {
    return validateAgentNameApi(name, agentId);
  }, []);

  return {
    isDialogOpen: isDialogOpen,
    dialogMode: dialogMode,
    editingAgentId: editingAgentId,
    dialogInitialTitle: dialogInitialTitle,
    dialogInitialAvatar: dialogInitialAvatar,
    dialogInitialDescription: dialogInitialDescription,
    dialogInitialOptions: dialogInitialOptions,
    dialogInitialVibeTags: dialogInitialVibeTags,
    setIsDialogOpen: setIsDialogOpen,
    handleOpenCreateAgent: handleOpenCreateAgent,
    handleEditAgent: handleEditAgent,
    handleSaveAgentOptions: handleSaveAgentOptions,
    handleSaveExistingAgentOptions: handleSaveExistingAgentOptions,
    handleValidateAgentName: handleValidateAgentName,
    handleValidateAgentNameForAgent: handleValidateAgentNameForAgent,
  };
}
