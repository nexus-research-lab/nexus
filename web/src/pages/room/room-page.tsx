import { useMemo } from "react";
import { useParams } from "react-router-dom";

import { GroupRouteEntry } from "@/features/conversation/room/group/group-route-entry";
import { RoomSurfaceShell } from "@/features/conversation/room/surface/room-surface-shell";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { resolveSelectedDraftConversationId } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model";
import { useRoomNavigationStore } from "@/store/room-navigation";
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
        externalSessionsReliability={conversation.externalSessions}
        sidePanelWidthPercent={workspace.sidePanelWidthPercent}
        initialDraft={navigation.initialDraft}
        isResizingSidePanel={workspace.isResizingSidePanel}
        onManageRoom={actions.manageRoom}
        onOpenMemberManager={actions.prepareAgentCatalog}
        onBackToDirectory={navigation.backToChatDirectory}
        onCloseConversation={actions.closeConversation}
        onDeleteConversation={navigation.deleteConversation}
        onForkConversation={actions.forkConversation}
        onCreateConversation={navigation.createConversation}
        onReplaceFinalConversation={navigation.replaceFinalConversation}
        onOpenWorkspaceFile={workspace.handleOpenWorkspaceFile}
        onSaveAgentOptions={actions.saveAgentOptions}
        onUpdateConversationTitle={actions.updateConversationTitle}
        onSelectConversation={navigation.selectConversation}
        onConversationSnapshotChange={conversation.handleSnapshotChange}
        onInitialDraftConsumed={navigation.consumeInitialDraft}
        onStartSidePanelResize={workspace.handleStartSidePanelResize}
        onTodosChange={workspace.setCurrentTodos}
        onValidateAgentName={actions.validateAgentName}
        surfaceSplitRef={workspace.surfaceSplitRef}
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
  const preferredConversationTabs = useRoomNavigationStore((state) => (
    params.roomId
      ? state.conversation_tabs_by_room[params.roomId]
      : undefined
  ));
  const preferredConversationIds = useMemo(() => {
    if (!preferredConversationTabs) {
      return [];
    }
    return [
      preferredConversationTabs.active_conversation_id,
      ...preferredConversationTabs.open_conversation_ids.filter(
        (id) => id !== preferredConversationTabs.active_conversation_id,
      ),
    ];
  }, [preferredConversationTabs]);
  const controller = useRoomPageController({
    roomId: params.roomId,
    conversationId: params.conversationId,
    preferredConversationIds: params.conversationId || params.sessionKey
      ? []
      : preferredConversationIds,
    sessionKey: params.sessionKey,
  });
  const { actions, conversation, room, status } = controller;
  const selectedDraftConversationId = useMemo(
    () => resolveSelectedDraftConversationId(
      conversation.items,
      conversation.selectedId,
    ),
    [conversation.items, conversation.selectedId],
  );
  const navigation = useRoomPageNavigation({
    roomId: params.roomId,
    routeConversationId: params.conversationId,
    routeSessionKey: params.sessionKey,
    currentRoomId: getCurrentRoomId(controller),
    selectedConversationId: conversation.selectedId,
    selectedDraftConversationId,
    isHydrated: status.isHydrated,
    closeConversation: actions.closeConversation,
    createConversation: actions.createConversation,
    deleteConversation: actions.deleteConversation,
  });
  useRoomPageTour({
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
      routeConversationId={params.conversationId}
      routeRoomId={params.roomId}
    />
  );
}
