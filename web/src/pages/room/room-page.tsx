import { useParams } from "react-router-dom";

import { GroupRouteEntry } from "@/features/conversation/room/group/group-route-entry";
import { RoomSurfaceShell } from "@/features/conversation/room/surface/room-surface-shell";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import type { RoomRouteParams } from "@/types/app/route";

import { useRoomPageController } from "./controller/use-room-page-controller";
import { useRoomPageEvents } from "./orchestration/use-room-page-events";
import { useRoomPageNavigation } from "./orchestration/use-room-page-navigation";
import { useRoomPageTour } from "./orchestration/use-room-page-tour";

export function RoomPage() {
  const params = useParams<RoomRouteParams>();
  const controller = useRoomPageController({
    roomId: params.roomId,
    conversationId: params.conversationId,
    sessionKey: params.sessionKey,
  });
  const navigation = useRoomPageNavigation({
    roomId: params.roomId,
    routeConversationId: params.conversationId,
    routeSessionKey: params.sessionKey,
    currentRoomId: controller.currentRoom?.id ?? null,
    selectedConversationId: controller.conversationId,
    isHydrated: controller.isHydrated,
    createConversation: controller.handleCreateConversation,
    deleteConversation: controller.handleDeleteConversation,
  });
  const {startCurrentTour} = useRoomPageTour({
    roomType: controller.currentRoom?.room_type ?? null,
    hasConversation: Boolean(controller.currentRoomConversation),
    enabled: controller.isHydrated && Boolean(controller.currentRoom),
  });
  const handleRoomEvent = useRoomPageEvents({
    roomId: params.roomId,
    roomType: controller.currentRoomType,
    refreshRoomState: controller.handleRefreshRoomState,
  });

  if (!controller.isHydrated) {
    return (
      <WorkspacePageFrame contentPaddingClassName="p-0">
        <WorkspaceLoadingState label="加载对话..." />
      </WorkspacePageFrame>
    );
  }

  if (controller.currentRoom && controller.currentAgent) {
    return (
      <WorkspacePageFrame contentPaddingClassName="p-0">
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
          initialDraft={navigation.initialDraft}
          isEditorOpen={controller.isEditorOpen}
          isResizingEditor={controller.isResizingEditor}
          onReplayTour={startCurrentTour}
          onManageRoom={controller.handleManageRoom}
          onOpenMemberManager={controller.handlePrepareRoomAgentCatalog}
          onBackToDirectory={navigation.backToLauncher}
          onCloseConversation={controller.handleCloseConversation}
          onDeleteConversation={navigation.deleteConversation}
          onCreateConversation={navigation.createConversation}
          onOpenWorkspaceFile={controller.handleOpenWorkspaceFile}
          onSaveAgentOptions={controller.handleSaveExistingAgentOptions}
          onUpdateConversationTitle={controller.handleUpdateConversationTitle}
          onSelectConversation={navigation.selectConversation}
          onConversationSnapshotChange={controller.handleConversationSnapshotChange}
          onInitialDraftConsumed={navigation.consumeInitialDraft}
          onStartEditorResize={controller.handleStartEditorResize}
          onTodosChange={controller.setCurrentTodos}
          onValidateAgentName={controller.handleValidateAgentNameForAgent}
          workspaceSplitRef={controller.workspaceSplitRef}
          onRoomEvent={handleRoomEvent}
        />
      </WorkspacePageFrame>
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
