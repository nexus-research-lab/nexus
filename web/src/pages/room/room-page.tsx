import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";

import { get_external_session_key_from_conversation_id } from "@/features/conversation/external-session-labels";
import { GroupRouteEntry } from "@/features/conversation/room/group/group-route-entry";
import { RoomSurfaceShell } from "@/features/conversation/room/surface/room-surface-shell";
import { useRoomPageController } from "@/hooks/room-page-controller/use-room-page-controller";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { RoomRouteParams } from "@/types/app/route";
import { UpdateRoomParams } from "@/types/conversation/room";
import {
  build_dm_conversation_tour,
  build_room_conversation_tour,
  build_room_empty_conversation_tour,
} from "@/features/conversation/room/room-tour";

export function RoomPage() {
  const { t } = useI18n();
  const params = useParams<RoomRouteParams>();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const [pendingInitialPrompt, setPendingInitialPrompt] = useState<string | null>(null);
  const [pendingDeletedRoom, setPendingDeletedRoom] = useState<{
    id: string;
    room_type: "room" | "dm";
  } | null>(null);
  const [pendingDeleteAgent, setPendingDeleteAgent] = useState<{ id: string; name: string } | null>(null);
  const controller = useRoomPageController({
    room_id: params.room_id,
    conversation_id: params.conversation_id,
    session_key: params.session_key,
  });
  const conversationTour = useMemo(() => {
    if (!controller.current_room) {
      return null;
    }
    if (controller.current_room.room_type === "dm") {
      return build_dm_conversation_tour(t);
    }
    if (controller.current_room_conversation) {
      return build_room_conversation_tour(t);
    }
    return build_room_empty_conversation_tour(t);
  }, [
    controller.current_room,
    controller.current_room_conversation,
    t,
  ]);

  const { start_current_tour: startCurrentTour } = usePageOnboardingTour({
    tour: conversationTour,
    enabled: controller.is_hydrated && Boolean(controller.current_room),
    auto_start_delay_ms: 260,
  });

  useEffect(() => {
    const initialPrompt = searchParams.get("initial")?.trim() ?? "";
    if (!initialPrompt) {
      return;
    }

    setPendingInitialPrompt((currentPrompt) => currentPrompt || initialPrompt);

    const nextSearchParams = new URLSearchParams(searchParams);
    nextSearchParams.delete("initial");
    setSearchParams(nextSearchParams, { replace: true });
  }, [searchParams, setSearchParams]);

  const handleConsumeInitialPrompt = useCallback(() => {
    setPendingInitialPrompt(null);
  }, []);

  const handleBackToLauncher = useCallback(() => {
    controller.handle_back_to_directory();
    navigate(AppRouteBuilders.launcher());
  }, [controller, navigate]);

  const handleUpdateRoom = useCallback(async (_room_id: string, params: UpdateRoomParams) => {
    await controller.handle_update_room(params);
  }, [controller]);

  const handleSelectConversation = useCallback((conversationId: string) => {
    controller.handle_select_conversation(conversationId);
    const routeRoomId = params.room_id;
    if (routeRoomId) {
      const externalSessionKey = get_external_session_key_from_conversation_id(conversationId);
      if (externalSessionKey) {
        navigate(AppRouteBuilders.room_session(routeRoomId, externalSessionKey));
        return;
      }
      navigate(AppRouteBuilders.room_conversation(routeRoomId, conversationId));
    }
  }, [controller, navigate, params.room_id]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    const routeRoomId = params.room_id;
    const nextConversationId = await controller.handle_create_conversation(title);
    if (routeRoomId && nextConversationId) {
      navigate(AppRouteBuilders.room_conversation(routeRoomId, nextConversationId));
    }
    return nextConversationId;
  }, [controller, navigate, params.room_id]);

  const handleDeleteConversation = useCallback(async (conversationId: string) => {
    const routeRoomId = params.room_id;
    const isDeletingActiveConversation = conversationId === controller.conversation_id;
    const nextConversationId = await controller.handle_delete_conversation(conversationId);
    if (!routeRoomId) {
      return nextConversationId;
    }
    if (!isDeletingActiveConversation) {
      return nextConversationId;
    }
    if (nextConversationId) {
      navigate(AppRouteBuilders.room_conversation(routeRoomId, nextConversationId));
      return nextConversationId;
    }
    navigate(AppRouteBuilders.room(routeRoomId));
    return null;
  }, [controller, navigate, params.room_id]);

  const handleUpdateConversationTitle = useCallback(async (conversationId: string, title: string) => {
    await controller.handle_update_conversation_title(conversationId, title);
  }, [controller]);

  const handleRoomEvent = useCallback((eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => {
    if (eventType === "room_deleted") {
      if (data.room_id && data.room_id === params.room_id) {
        setPendingDeletedRoom({
          id: data.room_id,
          room_type: controller.current_room?.room_type === "dm" ? "dm" : "room",
        });
        void controller.handle_refresh_room_state();
      }
      return;
    }

    if (eventType === "room_directed_message") {
      console.debug("[Room] room_directed_message", {
        message_id: data.message_id,
        event_kind: data.event_kind,
        room_id: data.room_id,
        conversation_id: data.conversation_id,
        source_agent_id: data.source_agent_id,
        recipients: data.recipients,
        target_agent_id: data.target_agent_id,
        reply_route: data.reply_route,
        wake_policy: data.wake_policy,
        delay_seconds: data.delay_seconds,
        correlation_id: data.correlation_id,
        content_chars: data.content_chars,
        has_content: typeof data.content === "string" && data.content.length > 0,
      });
      return;
    }

    if (eventType === "room_directed_message_consumed") {
      console.debug("[Room] room_directed_message_consumed", {
        room_id: data.room_id,
        conversation_id: data.conversation_id,
        agent_id: data.agent_id,
        round_id: data.round_id,
        last_message_id: data.last_message_id,
        last_message_timestamp: data.last_message_timestamp,
      });
      return;
    }

    if (eventType === "room_resync_required" || eventType === "session_resync_required") {
      void controller.handle_refresh_room_state();
    }
    // room_member_added / room_member_removed are handled by the next server-rendered
    // room context fetch; no extra action needed here.
  }, [controller, params.room_id]);

  useEffect(() => {
    if (!pendingDeletedRoom) {
      return;
    }

    if (!controller.is_hydrated) {
      return;
    }

    if (!params.room_id || params.room_id !== pendingDeletedRoom.id) {
      setPendingDeletedRoom(null);
      return;
    }

    if (controller.current_room && !controller.room_error) {
      // Room 仍可访问，继续留在当前路径。
      setPendingDeletedRoom(null);
      return;
    }

    const fallbackRoute = pendingDeletedRoom.room_type === "dm"
      ? AppRouteBuilders.contacts()
      : AppRouteBuilders.home();
    navigate(fallbackRoute, { replace: true });
    setPendingDeletedRoom(null);
  }, [
    controller.current_room,
    controller.is_hydrated,
    controller.room_error,
    navigate,
    params.room_id,
    pendingDeletedRoom,
  ]);

  const handleRequestDeleteAgent = useCallback((agentId: string) => {
    const targetAgent = controller.agents.find((agent) => agent.agent_id === agentId);
    controller.set_is_dialog_open(false);
    setPendingDeleteAgent({
      id: agentId,
      name: targetAgent?.name ?? "该 Agent",
    });
  }, [controller]);

  const handleConfirmDeleteAgent = useCallback(async () => {
    if (!pendingDeleteAgent) {
      return;
    }

    await controller.handle_delete_agent(pendingDeleteAgent.id);
    setPendingDeleteAgent(null);
  }, [controller, pendingDeleteAgent]);

  useEffect(() => {
    // 原有逻辑：自动导航到当前对话
    if (
      controller.is_hydrated &&
      params.room_id &&
      controller.current_room?.id === params.room_id &&
      !params.conversation_id &&
      !params.session_key &&
      controller.conversation_id &&
      !pendingInitialPrompt
    ) {
      const externalSessionKey = get_external_session_key_from_conversation_id(
        controller.conversation_id,
      );
      navigate(
        externalSessionKey
          ? AppRouteBuilders.room_session(params.room_id, externalSessionKey)
          : AppRouteBuilders.room_conversation(
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
    params.session_key,
    controller.current_room?.id,
    controller.conversation_id,
    pendingInitialPrompt,
  ]);

  // 加载中 — 内联 loading，外层布局由路由层提供
  if (!controller.is_hydrated) {
    return (
      <WorkspacePageFrame content_padding_class_name="p-0">
        <WorkspaceLoadingState label="加载对话..." />
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
            room_skill_names={controller.current_room_skill_names}
            room_host_agent_id={controller.current_room.host_agent_id ?? null}
            room_host_auto_reply_enabled={controller.current_room.host_auto_reply_enabled ?? false}
            room_private_messages_enabled={controller.current_room.private_messages_enabled ?? false}
            current_room_conversations={controller.current_room_conversations}
            current_room_conversation={controller.current_room_conversation}
            current_agent_session_identity={controller.current_agent_session_identity}
            conversation_id={controller.conversation_id}
            current_todos={controller.current_todos}
            editor_width_percent={controller.editor_width_percent}
            initial_draft={pendingInitialPrompt}
            is_editor_open={controller.is_editor_open}
            is_resizing_editor={controller.is_resizing_editor}
            is_conversation_busy={controller.is_conversation_busy}
            on_replay_tour={startCurrentTour}
            on_add_room_member={controller.handle_add_room_member}
            on_open_member_manager={controller.handle_prepare_room_agent_catalog}
            on_remove_room_member={controller.handle_remove_room_member}
            on_back_to_directory={handleBackToLauncher}
            on_close_conversation={controller.handle_close_conversation}
            on_delete_conversation={handleDeleteConversation}
            on_loading_change={controller.set_is_conversation_busy}
            on_create_conversation={handleCreateConversation}
            on_open_workspace_file={controller.handle_open_workspace_file}
            on_save_agent_options={controller.handle_save_existing_agent_options}
            on_update_room={handleUpdateRoom}
            on_update_conversation_title={handleUpdateConversationTitle}
            on_select_conversation={handleSelectConversation}
            on_conversation_snapshot_change={controller.handle_conversation_snapshot_change}
            on_initial_draft_consumed={handleConsumeInitialPrompt}
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
          on_delete={handleRequestDeleteAgent}
          on_save={controller.handle_save_agent_options}
          on_validate_name={controller.handle_validate_agent_name}
          initial_title={controller.dialog_initial_title}
          initial_options={controller.dialog_initial_options}
          initial_vibe_tags={controller.dialog_initial_vibe_tags}
        />

        <ConfirmDialog
          confirm_text="删除成员"
          is_open={Boolean(pendingDeleteAgent)}
          message={`删除「${pendingDeleteAgent?.name ?? "该 Agent"}」后，该成员将不再出现在当前前端列表中。已有历史协作不会自动删除。`}
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

  return (
    <WorkspacePageFrame>
      <GroupRouteEntry
        agents={controller.room_members}
        conversations={controller.current_room_conversations}
        conversation_id={params.conversation_id}
        room_id={params.room_id}
      />
    </WorkspacePageFrame>
  );
}
