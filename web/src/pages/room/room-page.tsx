import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";

import { getExternalSessionKeyFromConversationId } from "@/features/conversation/external-session-labels";
import { GroupRouteEntry } from "@/features/conversation/room/group/group-route-entry";
import { RoomSurfaceShell } from "@/features/conversation/room/surface/room-surface-shell";
import { useRoomPageController } from "@/hooks/room-page-controller/use-room-page-controller";
import { useI18n } from "@/shared/i18n/i18n-context";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { RoomRouteParams } from "@/types/app/route";
import { UpdateRoomParams } from "@/types/conversation/room";
import {
  buildDmConversationTour,
  buildRoomConversationTour,
  buildRoomEmptyConversationTour,
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
  const controller = useRoomPageController({
    roomId: params.roomId,
    conversationId: params.conversationId,
    sessionKey: params.sessionKey,
  });
  const conversationTour = useMemo(() => {
    if (!controller.currentRoom) {
      return null;
    }
    if (controller.currentRoom.room_type === "dm") {
      return buildDmConversationTour(t);
    }
    if (controller.currentRoomConversation) {
      return buildRoomConversationTour(t);
    }
    return buildRoomEmptyConversationTour(t);
  }, [
    controller.currentRoom,
    controller.currentRoomConversation,
    t,
  ]);

  const { startCurrentTour: startCurrentTour } = usePageOnboardingTour({
    tour: conversationTour,
    enabled: controller.isHydrated && Boolean(controller.currentRoom),
    autoStartDelayMs: 260,
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
    navigate(AppRouteBuilders.launcher());
  }, [navigate]);

  const handleUpdateRoom = useCallback(async (_room_id: string, params: UpdateRoomParams) => {
    await controller.handleUpdateRoom(params);
  }, [controller]);

  const handleSelectConversation = useCallback((conversationId: string) => {
    const routeRoomId = params.roomId;
    if (routeRoomId) {
      const externalSessionKey = getExternalSessionKeyFromConversationId(conversationId);
      if (externalSessionKey) {
        navigate(AppRouteBuilders.roomSession(routeRoomId, externalSessionKey));
        return;
      }
      navigate(AppRouteBuilders.roomConversation(routeRoomId, conversationId));
    }
  }, [navigate, params.roomId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    const routeRoomId = params.roomId;
    const nextConversationId = await controller.handleCreateConversation(title);
    if (routeRoomId && nextConversationId) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, nextConversationId));
    }
    return nextConversationId;
  }, [controller, navigate, params.roomId]);

  const handleDeleteConversation = useCallback(async (conversationId: string) => {
    const routeRoomId = params.roomId;
    const isDeletingActiveConversation = conversationId === controller.conversationId;
    const nextConversationId = await controller.handleDeleteConversation(conversationId);
    if (!routeRoomId) {
      return nextConversationId;
    }
    if (!isDeletingActiveConversation) {
      return nextConversationId;
    }
    if (nextConversationId) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, nextConversationId));
      return nextConversationId;
    }
    navigate(AppRouteBuilders.room(routeRoomId));
    return null;
  }, [controller, navigate, params.roomId]);

  const handleUpdateConversationTitle = useCallback(async (conversationId: string, title: string) => {
    await controller.handleUpdateConversationTitle(conversationId, title);
  }, [controller]);

  const handleRoomEvent = useCallback((eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => {
    if (eventType === "room_deleted") {
      if (data.room_id && data.room_id === params.roomId) {
        setPendingDeletedRoom({
          id: data.room_id,
          room_type: controller.currentRoom?.room_type === "dm" ? "dm" : "room",
        });
        void controller.handleRefreshRoomState();
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
      void controller.handleRefreshRoomState();
    }
    // roomMemberAdded / roomMemberRemoved are handled by the next server-rendered
    // room context fetch; no extra action needed here.
  }, [controller, params.roomId]);

  useEffect(() => {
    if (!pendingDeletedRoom) {
      return;
    }

    if (!controller.isHydrated) {
      return;
    }

    if (!params.roomId || params.roomId !== pendingDeletedRoom.id) {
      setPendingDeletedRoom(null);
      return;
    }

    if (controller.currentRoom && !controller.roomError) {
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
    controller.currentRoom,
    controller.isHydrated,
    controller.roomError,
    navigate,
    params.roomId,
    pendingDeletedRoom,
  ]);

  useEffect(() => {
    // 原有逻辑：自动导航到当前对话
    if (
      controller.isHydrated &&
      params.roomId &&
      controller.currentRoom?.id === params.roomId &&
      !params.conversationId &&
      !params.sessionKey &&
      controller.conversationId &&
      !pendingInitialPrompt
    ) {
      const externalSessionKey = getExternalSessionKeyFromConversationId(
        controller.conversationId,
      );
      navigate(
        externalSessionKey
          ? AppRouteBuilders.roomSession(params.roomId, externalSessionKey)
          : AppRouteBuilders.roomConversation(
            params.roomId,
            controller.conversationId,
          ),
        { replace: true },
      );
    }
  }, [
    controller.isHydrated,
    searchParams,
    navigate,
    params.conversationId,
    params.roomId,
    params.sessionKey,
    controller.currentRoom?.id,
    controller.conversationId,
    pendingInitialPrompt,
  ]);

  // 加载中 — 内联 loading，外层布局由路由层提供
  if (!controller.isHydrated) {
    return (
      <WorkspacePageFrame contentPaddingClassName="p-0">
        <WorkspaceLoadingState label="加载对话..." />
      </WorkspacePageFrame>
    );
  }

  if (controller.currentRoom && controller.currentAgent) {
    return (
      <>
        <WorkspacePageFrame
          contentPaddingClassName="p-0"
        >
          <RoomSurfaceShell
            activeWorkspacePath={controller.activeWorkspacePath}
            availableRoomAgents={controller.availableRoomAgents}
            currentAgent={controller.currentAgent}
            roomId={controller.routeRoomId}
            currentRoomType={controller.currentRoomType}
            roomAvatar={controller.currentRoom.avatar ?? null}
            roomMembers={controller.roomMembers}
            currentRoomTitle={controller.currentRoomTitle}
            roomSkillNames={controller.currentRoomSkillNames}
            roomHostAgentId={controller.currentRoom.host_agent_id ?? null}
            roomHostAutoReplyEnabled={controller.currentRoom.host_auto_reply_enabled ?? false}
            roomPrivateMessagesEnabled={controller.currentRoom.private_messages_enabled ?? false}
            currentRoomConversations={controller.currentRoomConversations}
            currentRoomConversation={controller.currentRoomConversation}
            currentAgentSessionIdentity={controller.currentAgentSessionIdentity}
            conversationId={controller.conversationId}
            currentTodos={controller.currentTodos}
            editorWidthPercent={controller.editorWidthPercent}
            initialDraft={pendingInitialPrompt}
            isEditorOpen={controller.isEditorOpen}
            isResizingEditor={controller.isResizingEditor}
            onReplayTour={startCurrentTour}
            onAddRoomMember={controller.handleAddRoomMember}
            onOpenMemberManager={controller.handlePrepareRoomAgentCatalog}
            onRemoveRoomMember={controller.handleRemoveRoomMember}
            onBackToDirectory={handleBackToLauncher}
            onCloseConversation={controller.handleCloseConversation}
            onDeleteConversation={handleDeleteConversation}
            onCreateConversation={handleCreateConversation}
            onOpenWorkspaceFile={controller.handleOpenWorkspaceFile}
            onSaveAgentOptions={controller.handleSaveExistingAgentOptions}
            onUpdateRoom={handleUpdateRoom}
            onUpdateConversationTitle={handleUpdateConversationTitle}
            onSelectConversation={handleSelectConversation}
            onConversationSnapshotChange={controller.handleConversationSnapshotChange}
            onInitialDraftConsumed={handleConsumeInitialPrompt}
            onStartEditorResize={controller.handleStartEditorResize}
            onTodosChange={controller.setCurrentTodos}
            onValidateAgentName={controller.handleValidateAgentNameForAgent}
            workspaceSplitRef={controller.workspaceSplitRef}
            onRoomEvent={handleRoomEvent}
          />
        </WorkspacePageFrame>

      </>
    );
  }

  return (
    <WorkspacePageFrame>
      <GroupRouteEntry
        agents={controller.roomMembers}
        conversations={controller.currentRoomConversations}
        conversationId={params.conversationId}
        roomId={params.roomId}
      />
    </WorkspacePageFrame>
  );
}
