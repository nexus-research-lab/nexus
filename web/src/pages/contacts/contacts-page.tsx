import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { ContactsAgentDetail } from "@/features/contacts/contacts-agent-detail";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { validate_agent_name_api } from "@/lib/api/agent-manage-api";
import { create_room, ensure_direct_room } from "@/lib/api/room-api";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { useAgentStore } from "@/store/agent";
import {
  AgentIdentityDraft,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";
import { get_initial_agent_options, is_main_agent } from "@/config/options";
import { build_agent_options_save_payload } from "@/features/agents/options/agent-options-constants";

export function ContactsPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const agents = useAgentStore((state) => state.agents);
  const createAgent = useAgentStore((state) => state.create_agent);
  const updateAgent = useAgentStore((state) => state.update_agent);
  const deleteAgent = useAgentStore((state) => state.delete_agent);
  const loadAgentsFromServer = useAgentStore(
    (state) => state.load_agents_from_server,
  );
  const loading = useAgentStore((state) => state.loading);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingAgentId, setEditingAgentId] = useState<string | null>(
    null,
  );
  const [pendingDeleteAgent, setPendingDeleteAgent] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const regularAgents = useMemo(
    () => agents.filter((agent) => !is_main_agent(agent.agent_id)),
    [agents],
  );
  const selectedAgentId = searchParams.get("agent");

  const editingAgent = useMemo(
    () =>
      regularAgents.find((agent) => agent.agent_id === editingAgentId) ??
      null,
    [editingAgentId, regularAgents],
  );
  const selectedAgent = useMemo(
    () =>
      selectedAgentId
        ? regularAgents.find((agent) => agent.agent_id === selectedAgentId) ?? null
        : null,
    [regularAgents, selectedAgentId],
  );
  const dialogInitialTitle = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.name : undefined),
    [dialogMode, editingAgent?.name],
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

  // 💬 Chat → ensureDirectRoom 发起 DM
  const handleOpenDirectRoom = useCallback(
    (agentId: string) => {
      void ensure_direct_room(agentId).then((context) => {
        navigate(
          AppRouteBuilders.room_conversation(
            context.room.id,
            context.conversation.id,
          ),
        );
      });
    },
    [navigate],
  );

  // 👥 Create Team → 用该 Agent 创建单人成员 Room
  const handleCreateTeam = useCallback(
    (agentId: string) => {
      void create_room({ agent_ids: [agentId] }).then((context) => {
        navigate(
          AppRouteBuilders.room_conversation(
            context.room.id,
            context.conversation.id,
          ),
        );
      });
    },
    [navigate],
  );

  // 新建 Agent → 打开 AgentOptions 对话框（create 模式）
  const handleOpenCreateAgent = useCallback(() => {
    setDialogMode("create");
    setEditingAgentId(null);
    setIsDialogOpen(true);
  }, []);

  // 点击卡片 → 打开 AgentOptions 对话框（edit 模式）
  const handleOpenEditAgent = useCallback((agentId: string) => {
    setDialogMode("edit");
    setEditingAgentId(agentId);
    setIsDialogOpen(true);
  }, []);

  const handleValidateAgentName = useCallback(
    async (name: string) => {
      const excludeAgentId =
        dialogMode === "edit" ? (editingAgentId ?? undefined) : undefined;
      return validate_agent_name_api(name, excludeAgentId);
    },
    [dialogMode, editingAgentId],
  );

  const handleSaveAgent = useCallback(
    async (
      title: string,
      options: AgentConfigOptions,
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
    },
    [createAgent, dialogMode, editingAgentId, updateAgent],
  );

  const handleSaveAgentOptions = useCallback(
    async (
      agentId: string,
      title: string,
      options: AgentConfigOptions,
      identity: AgentIdentityDraft,
    ) => {
      await updateAgent(agentId, {
        name: title,
        options: build_agent_options_save_payload(options),
        avatar: identity.avatar,
        description: identity.description,
        vibe_tags: identity.vibe_tags,
      });
    },
    [updateAgent],
  );

  const handleValidateAgentDetailName = useCallback(
    async (name: string, agentId?: string) => {
      return validate_agent_name_api(name, agentId);
    },
    [],
  );

  const handleConfirmDeleteAgent = useCallback(async () => {
    if (!pendingDeleteAgent) {
      return;
    }

    const deletedAgentId = pendingDeleteAgent.id;
    await deleteAgent(deletedAgentId);
    setPendingDeleteAgent(null);
    if (selectedAgentId === deletedAgentId) {
      navigate(AppRouteBuilders.contacts(), { replace: true });
    }
  }, [deleteAgent, navigate, pendingDeleteAgent, selectedAgentId]);

  const handleRequestDeleteAgent = useCallback(
    (agentId: string) => {
      const targetAgent = agents.find((agent) => agent.agent_id === agentId);
      if (!targetAgent || is_main_agent(targetAgent.agent_id)) {
        return;
      }
      setIsDialogOpen(false);
      setPendingDeleteAgent({
        id: agentId,
        name: targetAgent?.name ?? "该 Agent",
      });
    },
    [agents],
  );

  useEffect(() => {
    void loadAgentsFromServer();
  }, [loadAgentsFromServer]);

  useEffect(() => {
    if (!selectedAgentId || loading) {
      return;
    }
    if (!regularAgents.some((agent) => agent.agent_id === selectedAgentId)) {
      navigate(AppRouteBuilders.contacts(), { replace: true });
    }
  }, [loading, navigate, regularAgents, selectedAgentId]);

  // 加载中 — 内联 loading，外层布局由路由层提供
  if (loading && !regularAgents.length) {
    return (
      <WorkspacePageFrame content_padding_class_name="p-0">
        <WorkspaceLoadingState label="加载成员..." />
      </WorkspacePageFrame>
    );
  }

  return (
    <>
      <WorkspacePageFrame content_padding_class_name="p-0">
        {selectedAgent ? (
          <ContactsAgentDetail
            agent={selectedAgent}
            on_back={() => navigate(AppRouteBuilders.contacts())}
            on_create_team={handleCreateTeam}
            on_delete_agent={handleRequestDeleteAgent}
            on_open_direct_room={handleOpenDirectRoom}
            on_save_agent_options={handleSaveAgentOptions}
            on_validate_agent_name={handleValidateAgentDetailName}
          />
        ) : (
          <ContactsDirectory
            agents={regularAgents}
            on_create_agent={handleOpenCreateAgent}
            on_create_team={handleCreateTeam}
            on_edit_agent={handleOpenEditAgent}
            on_open_direct_room={handleOpenDirectRoom}
          />
        )}
      </WorkspacePageFrame>

      <AgentOptions
        agent_id={editingAgentId ?? undefined}
        initial_options={dialogInitialOptions}
        initial_avatar={editingAgent?.avatar ?? ""}
        initial_description={editingAgent?.description ?? ""}
        initial_title={dialogInitialTitle}
        initial_vibe_tags={editingAgent?.vibe_tags ?? []}
        is_open={isDialogOpen}
        mode={dialogMode}
        on_close={() => setIsDialogOpen(false)}
        on_delete={handleRequestDeleteAgent}
        on_save={handleSaveAgent}
        on_validate_name={handleValidateAgentName}
      />

      <ConfirmDialog
        confirm_text="删除成员"
        is_open={Boolean(pendingDeleteAgent)}
        message={`删除「${pendingDeleteAgent?.name ?? "该 Agent"}」后，该成员将不再出现在 Contacts 中。已有历史协作不会自动删除。`}
        on_cancel={() => setPendingDeleteAgent(null)}
        on_confirm={() => {
          void handleConfirmDeleteAgent();
        }}
        title="删除成员"
        variant="danger"
      />
    </>
  );
}
