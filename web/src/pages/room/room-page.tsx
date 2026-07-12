import { useParams } from "react-router-dom";

import { GroupRouteEntry } from "@/features/conversation/room/group/group-route-entry";
import { RoomSurfaceShell } from "@/features/conversation/room/surface/room-surface-shell";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import type { RoomEventPayload } from "@/types/agent/agent-conversation";
import type { RoomRouteParams } from "@/types/app/route";

import { useRoomPageController } from "./controller/use-room-page-controller";
import { useRoomPageEvents } from "./orchestration/use-room-page-events";
import { useRoomPageNavigation } from "./orchestration/use-room-page-navigation";
import { useRoomPageTour } from "./orchestration/use-room-page-tour";

type RoomPageController = ReturnType<typeof useRoomPageController>;
type RoomPageNavigation = ReturnType<typeof useRoomPageNavigation>;

interface RoomPageContentProps {
  controller: RoomPageController;
  handleRoomEvent: (eventType: string, data: RoomEventPayload) => void;
  navigation: RoomPageNavigation;
  onReplayTour?: () => void;
  routeConversationId?: string;
  routeRoomId?: string;
}

interface ActiveRoomPageProps extends RoomPageContentProps {
  currentAgent: NonNullable<RoomPageController["agent"]["current"]>;
  currentRoom: NonNullable<RoomPageController["room"]["current"]>;
}

function ActiveRoomPage({
  controller,
  currentAgent,
  currentRoom,
  handleRoomEvent,
  navigation,
  onReplayTour,
}: ActiveRoomPageProps) {
  const { actions, agent, conversation, room, workspace } = controller;
  return (
    <WorkspacePageFrame contentPaddingClassName="p-0">
      <RoomSurfaceShell
        activeWorkspacePath={workspace.activeWorkspacePath}
        availableRoomAgents={room.availableAgents}
        currentAgent={currentAgent}
        roomId={room.routeId}
        currentRoomType={room.type}
        roomAvatar={currentRoom.avatar ?? null}
        roomMembers={room.members}
        currentRoomTitle={room.title}
        roomSkillNames={room.skillNames}
        roomHostAgentId={currentRoom.host_agent_id ?? null}
        roomHostAutoReplyEnabled={currentRoom.host_auto_reply_enabled}
        roomPrivateMessagesEnabled={currentRoom.private_messages_enabled}
        currentRoomConversations={conversation.items}
        currentRoomConversation={conversation.current}
        currentAgentSessionIdentity={agent.sessionIdentity}
        conversationId={conversation.selectedId}
        currentTodos={workspace.currentTodos}
        editorWidthPercent={workspace.editorWidthPercent}
        initialDraft={navigation.initialDraft}
        isEditorOpen={workspace.isEditorOpen}
        isResizingEditor={workspace.isResizingEditor}
        onReplayTour={onReplayTour}
        onManageRoom={actions.manageRoom}
        onOpenMemberManager={actions.prepareAgentCatalog}
        onBackToDirectory={navigation.backToLauncher}
        onCloseConversation={actions.closeConversation}
        onDeleteConversation={navigation.deleteConversation}
        onCreateConversation={navigation.createConversation}
        onOpenWorkspaceFile={workspace.handleOpenWorkspaceFile}
        onSaveAgentOptions={actions.saveAgentOptions}
        onUpdateConversationTitle={actions.updateConversationTitle}
        onSelectConversation={navigation.selectConversation}
        onConversationSnapshotChange={conversation.handleSnapshotChange}
        onInitialDraftConsumed={navigation.consumeInitialDraft}
        onStartEditorResize={workspace.handleStartEditorResize}
        onTodosChange={workspace.setCurrentTodos}
        onValidateAgentName={actions.validateAgentName}
        workspaceSplitRef={workspace.workspaceSplitRef}
        onRoomEvent={handleRoomEvent}
      />
    </WorkspacePageFrame>
  );
}

function RoomPageContent(props: RoomPageContentProps) {
  const { agent, conversation, room, status } = props.controller;
  if (!status.isHydrated) {
    return (
      <WorkspacePageFrame contentPaddingClassName="p-0">
        <WorkspaceLoadingState label="加载对话..." />
      </WorkspacePageFrame>
    );
  }
  if (!room.current || !agent.current) {
    return (
      <WorkspacePageFrame>
        <GroupRouteEntry
          agents={room.members}
          conversations={conversation.items}
          conversationId={props.routeConversationId}
          roomId={props.routeRoomId}
        />
      </WorkspacePageFrame>
    );
  }
  return (
    <ActiveRoomPage
      {...props}
      currentAgent={agent.current}
      currentRoom={room.current}
    />
  );
}

function getCurrentRoomId(controller: RoomPageController): string | null {
  return controller.room.current?.id ?? null;
}

function getCurrentRoomType(controller: RoomPageController): string | null {
  return controller.room.current?.room_type ?? null;
}

export function RoomPage() {
  const params = useParams<RoomRouteParams>();
  const controller = useRoomPageController({
    roomId: params.roomId,
    conversationId: params.conversationId,
    sessionKey: params.sessionKey,
  });
  const { actions, conversation, room, status } = controller;
  const navigation = useRoomPageNavigation({
    roomId: params.roomId,
    routeConversationId: params.conversationId,
    routeSessionKey: params.sessionKey,
    currentRoomId: getCurrentRoomId(controller),
    selectedConversationId: conversation.selectedId,
    isHydrated: status.isHydrated,
    createConversation: actions.createConversation,
    deleteConversation: actions.deleteConversation,
  });
  const { startCurrentTour } = useRoomPageTour({
    roomType: getCurrentRoomType(controller),
    hasConversation: Boolean(conversation.current),
    enabled: status.isHydrated && Boolean(room.current),
  });
  const handleRoomEvent = useRoomPageEvents({
    roomId: params.roomId,
    roomType: room.type,
    refreshRoomState: actions.refreshRoomState,
  });

  return (
    <RoomPageContent
      controller={controller}
      handleRoomEvent={handleRoomEvent}
      navigation={navigation}
      onReplayTour={startCurrentTour}
      routeConversationId={params.conversationId}
      routeRoomId={params.roomId}
    />
  );
}
