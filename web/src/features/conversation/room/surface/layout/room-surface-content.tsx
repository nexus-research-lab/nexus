"use client";

import { cn } from "@/shared/ui/class-name";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceTaskPanel } from "@/shared/ui/workspace/surface/workspace-task-strip";

import { RoomChatSurface } from "../room-chat-surface";
import { RoomSurfaceAuxiliaryPanel } from "./room-surface-auxiliary-panel";
import { RoomSurfaceHeader } from "./room-surface-header";
import type { RoomSurfaceLayoutProps } from "./room-surface-layout-types";
import { RoomThreadInlinePanel } from "./room-thread-inline-panel";
import { useRoomSurfaceLayoutController } from "./use-room-surface-layout-controller";

type RoomSurfaceContentProps = RoomSurfaceLayoutProps & {
  isThreadPanelOpen: boolean;
};

export function RoomSurfaceContent({
  activeSurfaceTab,
  activeWorkspacePath,
  availableRoomAgents,
  conversationId,
  currentAgent,
  currentAgentSessionIdentity,
  currentRoomConversations,
  currentRoomTitle,
  currentRoomType,
  currentTodos,
  editorWidthPercent,
  initialDraft = null,
  isEditorOpen,
  isResizingEditor,
  isThreadPanelOpen,
  onChangeSurfaceTab,
  onCloseConversation,
  onConversationSnapshotChange,
  onCreateConversation,
  onDeleteConversation,
  onInitialDraftConsumed,
  onManageRoom,
  onOpenMemberManager,
  onOpenWorkspaceFile,
  onReplayTour,
  onRoomEvent,
  onSaveAgentOptions,
  onSelectConversation,
  onStartEditorResize,
  onTodosChange,
  onUpdateConversationTitle,
  onValidateAgentName,
  roomAvatar,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
  roomPrivateMessagesEnabled,
  roomSkillNames,
  workspaceSplitRef,
}: RoomSurfaceContentProps) {
  const isDm = currentRoomType === "dm";
  const layout = useRoomSurfaceLayoutController({
    activeSurfaceTab,
    conversationId,
    currentAgentId: currentAgent.agent_id,
    currentAgentSessionIdentity,
    isDm,
    isThreadPanelOpen,
    onChangeSurfaceTab,
    roomId,
  });

  return (
    <section
      ref={workspaceSplitRef}
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        isResizingEditor && "cursor-col-resize select-none",
      )}
    >
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <WorkspaceSurfaceScaffold
          bodyClassName="relative"
          header={(
            <RoomSurfaceHeader
              activeSurfaceTab={activeSurfaceTab}
              availableRoomAgents={availableRoomAgents}
              conversationId={conversationId}
              conversations={currentRoomConversations}
              currentAgent={currentAgent}
              currentRoomTitle={currentRoomTitle}
              isDm={isDm}
              onChangeSurfaceTab={layout.handleChangeSurfaceTab}
              onCloseAuxiliaryPanel={layout.handleCloseAuxiliaryPanel}
              onCloseConversation={onCloseConversation}
              onCreateConversation={onCreateConversation}
              onManageRoom={onManageRoom}
              onOpenMemberManager={onOpenMemberManager}
              onReplayTour={onReplayTour}
              onSelectConversation={onSelectConversation}
              roomAvatar={roomAvatar}
              roomHostAgentId={roomHostAgentId}
              roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
              roomId={roomId}
              roomMembers={roomMembers}
              roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
              roomSkillNames={roomSkillNames}
            />
          )}
        >
          <div className="flex h-full min-h-0 min-w-0">
            <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
              {/* 聊天面板常驻挂载，切换右栏时不能触发 WebSocket 清理。 */}
              <RoomChatSurface
                conversationId={conversationId}
                currentAgent={currentAgent}
                currentAgentSessionIdentity={currentAgentSessionIdentity}
                currentRoomType={currentRoomType}
                initialDraft={initialDraft}
                onConversationSnapshotChange={onConversationSnapshotChange}
                onCreateConversation={onCreateConversation}
                onInitialDraftConsumed={onInitialDraftConsumed}
                onOpenAgentContact={layout.handleOpenAgentContact}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                onRoomEvent={onRoomEvent}
                onTodosChange={onTodosChange}
                roomHostAgentId={roomHostAgentId}
                roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
                roomId={roomId}
                roomMembers={roomMembers}
              />
              <WorkspaceTaskPanel
                key={conversationId ?? "conversation-tasks"}
                todos={currentTodos}
              />
            </div>

            {layout.isAuxiliaryPanelOpen ? (
              <RoomSurfaceAuxiliaryPanel
                aboutRequest={layout.aboutRequest}
                activeSurfaceTab={activeSurfaceTab}
                activeWorkspacePath={activeWorkspacePath}
                conversationId={conversationId}
                conversations={currentRoomConversations}
                currentAgent={currentAgent}
                currentRoomType={currentRoomType}
                editorWidthPercent={editorWidthPercent}
                isDm={isDm}
                isEditorOpen={isEditorOpen}
                onClose={layout.handleCloseAuxiliaryPanel}
                onCreateConversation={onCreateConversation}
                onDeleteConversation={onDeleteConversation}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                onSaveAgentOptions={onSaveAgentOptions}
                onSelectConversation={onSelectConversation}
                onStartEditorResize={onStartEditorResize}
                onUpdateConversationTitle={onUpdateConversationTitle}
                onValidateAgentName={onValidateAgentName}
                roomId={roomId}
                roomMembers={roomMembers}
                subagentTaskSource={layout.subagentTaskSource}
              />
            ) : !isDm ? (
              <RoomThreadInlinePanel
                activeSurfaceTab={activeSurfaceTab}
                className="hidden lg:flex"
                editorWidthPercent={editorWidthPercent}
                onStartEditorResize={onStartEditorResize}
              />
            ) : null}
          </div>
        </WorkspaceSurfaceScaffold>
      </div>
    </section>
  );
}
