import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { Loader2 } from "lucide-react";

import { GroupRouteEntry } from "@/features/conversation/room/group/group-route-entry";
import { RoomSurfaceShell } from "@/features/conversation/room/surface/room-surface-shell";
import { useRoomPageController } from "@/hooks/room-page-controller/use-room-page-controller";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { RoomRouteParams } from "@/types/app/route";
import { UpdateRoomParams } from "@/types/conversation/room";

export function RoomPage() {
  const params = useParams<RoomRouteParams>();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const [pending_initial_prompt, set_pending_initial_prompt] = useState<string | null>(null);
  const [pending_deleted_room, set_pending_deleted_room] = useState<{
    id: string;
    room_type: "room" | "dm";
  } | null>(null);
  const [pending_delete_agent, set_pending_delete_agent] = useState<{ id: string; name: string } | null>(null);
  const controller = useRoomPageController({
    room_id: params.room_id,
    conversation_id: params.conversation_id,
  });

  useEffect(() => {
    const initial_prompt = searchParams.get("initial")?.trim() ?? "";
    if (!initial_prompt) {
      return;
    }

    set_pending_initial_prompt((current_prompt) => current_prompt || initial_prompt);

    const next_search_params = new URLSearchParams(searchParams);
    next_search_params.delete("initial");
    setSearchParams(next_search_params, { replace: true });
  }, [searchParams, setSearchParams]);

  const handle_consume_initial_prompt = useCallback(() => {
    set_pending_initial_prompt(null);
  }, []);

  const handleBackToLauncher = useCallback(() => {
    controller.handle_back_to_directory();
    navigate(AppRouteBuilders.launcher());
  }, [controller, navigate]);

  const handleUpdateRoom = useCallback(async (_room_id: string, params: UpdateRoomParams) => {
    await controller.handle_update_room(params);
  }, [controller]);

  const handleSelectConversation = useCallback((conversation_id: string) => {
    controller.handle_select_conversation(conversation_id);
    const route_room_id = params.room_id;
    if (route_room_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, conversation_id));
    }
  }, [controller, navigate, params.room_id]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    const route_room_id = params.room_id;
    const next_conversation_id = await controller.handle_create_conversation(title);
    if (route_room_id && next_conversation_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, next_conversation_id));
    }
    return next_conversation_id;
  }, [controller, navigate, params.room_id]);

  const handleDeleteConversation = useCallback(async (conversation_id: string) => {
    const route_room_id = params.room_id;
    const next_conversation_id = await controller.handle_delete_conversation(conversation_id);
    if (!route_room_id) {
      return next_conversation_id;
    }
    if (next_conversation_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, next_conversation_id));
      return next_conversation_id;
    }
    navigate(AppRouteBuilders.room(route_room_id));
    return null;
  }, [controller, navigate, params.room_id]);

  const handleUpdateConversationTitle = useCallback(async (conversation_id: string, title: string) => {
    await controller.handle_update_conversation_title(conversation_id, title);
  }, [controller]);

  const handleRoomEvent = useCallback((event_type: string, _data: import("@/types/agent/agent-conversation").RoomEventPayload) => {
    if (event_type === "room_deleted") {
      if (_data.room_id && _data.room_id === params.room_id) {
        set_pending_deleted_room({
          id: _data.room_id,
          room_type: controller.current_room?.room_type === "dm" ? "dm" : "room",
        });
        void controller.handle_refresh_room_state();
      }
      return;
    }

    if (event_type === "room_resync_required") {
      void controller.handle_refresh_room_state();
    }
    // room_member_added / room_member_removed are handled by the next server-rendered
    // room context fetch; no extra action needed here.
  }, [controller, params.room_id]);

  useEffect(() => {
    if (!pending_deleted_room) {
      return;
    }

    if (!controller.is_hydrated) {
      return;
    }

    if (!params.room_id || params.room_id !== pending_deleted_room.id) {
      set_pending_deleted_room(null);
      return;
    }

    if (controller.current_room && !controller.room_error) {
      // Room 仍可访问，继续留在当前路径。
      set_pending_deleted_room(null);
      return;
    }

    const fallback_route = pending_deleted_room.room_type === "dm"
      ? AppRouteBuilders.dm_directory()
      : AppRouteBuilders.home();
    navigate(fallback_route, { replace: true });
    set_pending_deleted_room(null);
  }, [
    controller.current_room,
    controller.is_hydrated,
    controller.room_error,
    navigate,
    params.room_id,
    pending_deleted_room,
  ]);

  const handle_request_delete_agent = useCallback((agent_id: string) => {
    const target_agent = controller.agents.find((agent) => agent.agent_id === agent_id);
    controller.set_is_dialog_open(false);
    set_pending_delete_agent({
      id: agent_id,
      name: target_agent?.name ?? "该 Agent",
    });
  }, [controller]);

  const handle_confirm_delete_agent = useCallback(async () => {
    if (!pending_delete_agent) {
      return;
    }

    await controller.handle_delete_agent(pending_delete_agent.id);
    set_pending_delete_agent(null);
  }, [controller, pending_delete_agent]);

  useEffect(() => {
    // 原有逻辑：自动导航到当前对话
    if (
      controller.is_hydrated &&
      params.room_id &&
      controller.current_room?.id === params.room_id &&
      !params.conversation_id &&
      controller.conversation_id &&
      !pending_initial_prompt
    ) {
      navigate(
        AppRouteBuilders.room_conversation(
          params.room_id,
          controller.conversation_id,
        ),
        { replace: true },
      );
    }
  }, [
    controller.is_hydrated,
    searchParams,
    navigate,
    params.conversation_id,
    params.room_id,
    controller.current_room?.id,
    controller.conversation_id,
    pending_initial_prompt,
  ]);

  // 加载中 — 内联 loading，外层布局由路由层提供
  if (!controller.is_hydrated) {
    return (
      <WorkspacePageFrame content_padding_class_name="p-0">
        <div className="flex min-h-0 flex-1 items-center justify-center">
          <div className="flex flex-col items-center gap-3">
            <Loader2 className="h-6 w-6 animate-spin text-(--text-soft)" />
            <span className="text-sm text-(--text-soft)">加载对话...</span>
          </div>
        </div>
      </WorkspacePageFrame>
    );
  }

  if (controller.current_room && controller.current_agent) {
    return (
      <>
        <WorkspacePageFrame
          content_padding_class_name="p-0"
        >
          <RoomSurfaceShell
            active_workspace_path={controller.active_workspace_path}
            available_room_agents={controller.available_room_agents}
            current_agent={controller.current_agent}
            room_id={controller.route_room_id}
            current_room_type={controller.current_room_type}
            room_avatar={controller.current_room.avatar ?? null}
            room_members={controller.room_members}
            current_room_title={controller.current_room_title}
            current_room_conversations={controller.current_room_conversations}
            current_room_conversation={controller.current_room_conversation}
            current_agent_session_identity={controller.current_agent_session_identity}
            conversation_id={controller.conversation_id}
            current_todos={controller.current_todos}
            editor_width_percent={controller.editor_width_percent}
            initial_draft={pending_initial_prompt}
            is_editor_open={controller.is_editor_open}
            is_resizing_editor={controller.is_resizing_editor}
            is_conversation_busy={controller.is_conversation_busy}
            on_add_room_member={controller.handle_add_room_member}
            on_remove_room_member={controller.handle_remove_room_member}
            on_back_to_directory={handleBackToLauncher}
            on_close_workspace_pane={controller.handle_close_workspace_pane}
            on_delete_conversation={handleDeleteConversation}
            on_loading_change={controller.set_is_conversation_busy}
            on_create_conversation={handleCreateConversation}
            on_open_workspace_file={controller.handle_open_workspace_file}
            on_save_agent_options={controller.handle_save_existing_agent_options}
            on_update_room={handleUpdateRoom}
            on_update_conversation_title={handleUpdateConversationTitle}
            on_select_conversation={handleSelectConversation}
            on_conversation_snapshot_change={controller.handle_conversation_snapshot_change}
            on_initial_draft_consumed={handle_consume_initial_prompt}
            on_start_editor_resize={controller.handle_start_editor_resize}
            on_todos_change={controller.set_current_todos}
            on_validate_agent_name={controller.handle_validate_agent_name_for_agent}
            workspace_split_ref={controller.workspace_split_ref}
            on_room_event={handleRoomEvent}
          />
        </WorkspacePageFrame>

        <AgentOptions
          agent_id={controller.editing_agent_id ?? undefined}
          initial_avatar={controller.dialog_initial_avatar}
          initial_description={controller.dialog_initial_description}
          mode={controller.dialog_mode}
          is_open={controller.is_dialog_open}
          on_close={() => controller.set_is_dialog_open(false)}
          on_delete={handle_request_delete_agent}
          on_save={controller.handle_save_agent_options}
          on_validate_name={controller.handle_validate_agent_name}
          initial_title={controller.dialog_initial_title}
          initial_options={controller.dialog_initial_options}
          initial_vibe_tags={controller.dialog_initial_vibe_tags}
        />

        <ConfirmDialog
          confirm_text="删除成员"
          is_open={Boolean(pending_delete_agent)}
          message={`删除「${pending_delete_agent?.name ?? "该 Agent"}」后，该成员将不再出现在当前前端列表中。已有历史协作不会自动删除。`}
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

  return (
    <WorkspacePageFrame>
      <GroupRouteEntry
        agents={controller.room_members}
        conversations={controller.conversations}
        conversation_id={params.conversation_id}
        room_id={params.room_id}
      />
    </WorkspacePageFrame>
  );
}
