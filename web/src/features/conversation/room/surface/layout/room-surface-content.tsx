"use client";

/**
 * INPUT: 桌面 Room 会话、任务快照、右栏状态与页面命令。
 * OUTPUT: 将任务快照交给聊天 Bottom Dock，并仅在托管 Plan 成功创建后暴露工作图右栏。
 * POS: Room 桌面 Surface 的主内容装配层；对话视觉效果必须裁剪在聊天栏内。
 */

import { cn } from "@/shared/ui/class-name";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { hasManagedExecutionGraph } from "@/features/conversation/shared/execution/execution-process-model";

import { RoomChatSurface } from "../room-chat-surface";
import { RoomSurfaceAuxiliaryPanel } from "./room-surface-auxiliary-panel";
import { RoomSurfaceHeader } from "./room-surface-header";
import type { RoomSurfaceLayoutProps } from "./room-surface-layout-types";
import { RoomThreadInlinePanel } from "./room-thread-inline-panel";
import { useRoomSurfaceLayoutController } from "./use-room-surface-layout-controller";

import "./room-surface-split.css";

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
  runtimeKind,
  currentTodos,
  executionResource,
  executionTaskRuns,
  sidePanelWidthPercent,
  initialDraft = null,
  isResizingSidePanel,
  isThreadPanelOpen,
  onChangeSurfaceTab,
  onCloseConversation,
  onConversationSnapshotChange,
  onCreateConversation,
  onDeleteConversation,
  onExecutionTaskRunsChange,
  onInitialDraftConsumed,
  onManageRoom,
  onOpenMemberManager,
  onOpenWorkspaceFile,
  onReplayTour,
  onRoomEvent,
  onSaveAgentOptions,
  onSelectConversation,
  onStartSidePanelResize,
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
  surfaceSplitRef,
}: RoomSurfaceContentProps) {
  const isDm = currentRoomType === "dm";
  const workgraphAvailable = hasManagedExecutionGraph(
    executionResource.execution,
  );
  const layout = useRoomSurfaceLayoutController({
    activeSurfaceTab,
    conversationId,
    currentAgentId: currentAgent.agent_id,
    currentAgentSessionIdentity,
    isDm,
    isThreadPanelOpen,
    onChangeSurfaceTab,
    roomId,
    workgraphAvailable,
  });

  return (
    <section
      ref={surfaceSplitRef}
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        isResizingSidePanel && "cursor-col-resize select-none",
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
              onDeleteConversation={onDeleteConversation}
              onManageRoom={onManageRoom}
              onOpenMemberManager={onOpenMemberManager}
              onReplayTour={onReplayTour}
              onSelectConversation={onSelectConversation}
              onUpdateConversationTitle={onUpdateConversationTitle}
              roomAvatar={roomAvatar}
              roomHostAgentId={roomHostAgentId}
              roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
              roomId={roomId}
              roomMembers={roomMembers}
              roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
              roomSkillNames={roomSkillNames}
              workgraphAvailable={workgraphAvailable}
            />
          )}
        >
          <div className="nexus-room-surface-split flex h-full min-h-0 min-w-0">
            <div className="nexus-room-surface-conversation flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
              <div
                className="nexus-room-conversation-reading-edge min-h-0 min-w-0 flex-1"
                data-room-conversation-reading-edge="true"
              >
                {/* 只挂载当前会话；标签列表只消费标题元数据，切换会话后按需加载，切换右栏不清理当前连接。 */}
                <RoomChatSurface
                  conversationId={conversationId}
                  currentAgent={currentAgent}
                  currentAgentSessionIdentity={currentAgentSessionIdentity}
                  currentRoomType={currentRoomType}
                  executionResource={executionResource}
                  initialDraft={initialDraft}
                  layout="desktop"
                  onConversationSnapshotChange={onConversationSnapshotChange}
                  onCreateConversation={onCreateConversation}
                  onExecutionTaskRunsChange={onExecutionTaskRunsChange}
                  onInitialDraftConsumed={onInitialDraftConsumed}
                  onOpenAgentContact={layout.handleOpenAgentContact}
                  onOpenWorkGraph={() => layout.handleChangeSurfaceTab("workgraph")}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  onRoomEvent={onRoomEvent}
                  onTodosChange={onTodosChange}
                  roomHostAgentId={roomHostAgentId}
                  roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
                  roomId={roomId}
                  roomMembers={roomMembers}
                  runtimeKind={runtimeKind}
                  todos={currentTodos}
                />
              </div>
            </div>

            {layout.isAuxiliaryPanelOpen ? (
              <RoomSurfaceAuxiliaryPanel
                aboutRequest={layout.aboutRequest}
                activeSurfaceTab={activeSurfaceTab}
                activeWorkspacePath={activeWorkspacePath}
                conversationId={conversationId}
                currentAgent={currentAgent}
                executionResource={executionResource}
                executionTaskRuns={executionTaskRuns}
                sidePanelWidthPercent={sidePanelWidthPercent}
                isDm={isDm}
                onClose={layout.handleCloseAuxiliaryPanel}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                onSaveAgentOptions={onSaveAgentOptions}
                onStartSidePanelResize={onStartSidePanelResize}
                onValidateAgentName={onValidateAgentName}
                roomId={roomId}
                roomMembers={roomMembers}
                subagentTaskSource={layout.subagentTaskSource}
              />
            ) : !isDm ? (
              <RoomThreadInlinePanel
                activeSurfaceTab={activeSurfaceTab}
                className="hidden lg:flex"
                sidePanelWidthPercent={sidePanelWidthPercent}
                onStartSidePanelResize={onStartSidePanelResize}
              />
            ) : null}
          </div>
        </WorkspaceSurfaceScaffold>
      </div>
    </section>
  );
}
