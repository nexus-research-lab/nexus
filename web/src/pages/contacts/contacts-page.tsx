import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Loader2 } from "lucide-react";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { createRoom, ensureDirectRoom } from "@/lib/room-api";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspacePageFrame } from "@/shared/ui/workspace/workspace-page-frame";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { AgentIdentityDraft, AgentOptions as AgentConfigOptions } from "@/types/agent";
import { initialOptions } from "@/config/options";

export function ContactsPage() {
  const navigate = useNavigate();
  const {
    agents,
    create_agent,
    update_agent,
    delete_agent,
    load_agents_from_server,
    loading,
  } = useAgentStore();
  const { conversations, load_conversations_from_server } = useConversationStore();
  const [is_dialog_open, set_is_dialog_open] = useState(false);
  const [dialog_mode, set_dialog_mode] = useState<"create" | "edit">("create");
  const [editing_agent_id, set_editing_agent_id] = useState<string | null>(null);
  const [pending_delete_agent, set_pending_delete_agent] = useState<{ id: string; name: string } | null>(null);

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
      system_prompt: editing_agent.options.system_prompt,
    };
  }, [dialog_mode, editing_agent]);

  // 💬 Chat → ensureDirectRoom 发起 DM
  const handle_open_direct_room = useCallback((agent_id: string) => {
    void ensureDirectRoom(agent_id).then((context) => {
      navigate(
        AppRouteBuilders.room_conversation(
          context.room.id,
          context.conversation.id,
        ),
      );
    });
  }, [navigate]);

  // 👥 Create Team → 用该 Agent 创建单人成员 Room
  const handle_create_team = useCallback((agent_id: string) => {
    void createRoom({ agent_ids: [agent_id] }).then((context) => {
      navigate(
        AppRouteBuilders.room_conversation(
          context.room.id,
          context.conversation.id,
        ),
      );
    });
  }, [navigate]);

  // 新建 Agent → 打开 AgentOptions 对话框（create 模式）
  const handle_open_create_agent = useCallback(() => {
    set_dialog_mode("create");
    set_editing_agent_id(null);
    set_is_dialog_open(true);
  }, []);

  // 点击卡片 → 打开 AgentOptions 对话框（edit 模式）
  const handle_open_edit_agent = useCallback((agent_id: string) => {
    set_dialog_mode("edit");
    set_editing_agent_id(agent_id);
    set_is_dialog_open(true);
  }, []);

  const handle_validate_agent_name = useCallback(async (name: string) => {
    const exclude_agent_id = dialog_mode === "edit" ? editing_agent_id ?? undefined : undefined;
    return validateAgentNameApi(name, exclude_agent_id);
  }, [dialog_mode, editing_agent_id]);

  const handle_save_agent = useCallback(async (
    title: string,
    options: AgentConfigOptions,
    identity: AgentIdentityDraft,
  ) => {
    const next_options = {
      model: options.model,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      setting_sources: options.setting_sources,
      system_prompt: options.system_prompt,
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

  const handle_confirm_delete_agent = useCallback(async () => {
    if (!pending_delete_agent) {
      return;
    }

    await delete_agent(pending_delete_agent.id);
    set_pending_delete_agent(null);
  }, [delete_agent, pending_delete_agent]);

  const handle_request_delete_agent = useCallback((agent_id: string) => {
    const target_agent = agents.find((agent) => agent.agent_id === agent_id);
    set_is_dialog_open(false);
    set_pending_delete_agent({
      id: agent_id,
      name: target_agent?.name ?? "该 Agent",
    });
  }, [agents]);

  useEffect(() => {
    void load_agents_from_server();
    void load_conversations_from_server();
  }, [load_agents_from_server, load_conversations_from_server]);

  // 加载中 — 内联 loading，外层布局由路由层提供
  if (loading && !agents.length) {
    return (
      <WorkspacePageFrame content_padding_class_name="p-0">
        <div className="flex min-h-0 flex-1 items-center justify-center">
          <div className="flex flex-col items-center gap-3">
            <Loader2 className="h-6 w-6 animate-spin text-(--text-soft)" />
            <span className="text-sm text-(--text-soft)">加载成员...</span>
          </div>
        </div>
      </WorkspacePageFrame>
    );
  }

  return (
    <>
      <WorkspacePageFrame content_padding_class_name="p-0">
        <ContactsDirectory
          agents={agents}
          conversations={conversations}
          on_create_agent={handle_open_create_agent}
          on_create_team={handle_create_team}
          on_edit_agent={handle_open_edit_agent}
          on_open_direct_room={handle_open_direct_room}
        />
      </WorkspacePageFrame>

      <AgentOptions
        agent_id={editing_agent_id ?? undefined}
        initial_options={dialog_initial_options}
        initial_avatar={editing_agent?.avatar ?? ""}
        initial_description={editing_agent?.description ?? ""}
        initial_title={dialog_initial_title}
        initial_vibe_tags={editing_agent?.vibe_tags ?? []}
        is_open={is_dialog_open}
        mode={dialog_mode}
        on_close={() => set_is_dialog_open(false)}
        on_delete={handle_request_delete_agent}
        on_save={handle_save_agent}
        on_validate_name={handle_validate_agent_name}
      />

      <ConfirmDialog
        confirm_text="删除成员"
        is_open={Boolean(pending_delete_agent)}
        message={`删除「${pending_delete_agent?.name ?? "该 Agent"}」后，该成员将不再出现在 Contacts 中。已有历史协作不会自动删除。`}
        on_cancel={() => set_pending_delete_agent(null)}
        on_confirm={() => {
          void handle_confirm_delete_agent();
        }}
        title="删除成员"
        variant="danger"
      />
    </>
  );
}
