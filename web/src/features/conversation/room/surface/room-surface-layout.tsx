"use client";

import { RefObject, useCallback, useEffect, useMemo, useState } from "react";

import { DmConversationHeader } from "@/features/conversation/room/dm/dm-conversation-header";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceTaskPanel } from "@/shared/ui/workspace/surface/workspace-task-strip";
import { Agent, AgentIdentityDraft, AgentNameValidationResult, AgentOptions } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import { TodoItem } from "@/types/conversation/todo";
import { UpdateRoomParams } from "@/types/conversation/room";

import { GroupConversationHeader } from "../group/header/group-conversation-header";
import { GroupThreadContextProvider } from "../group/thread/group-thread-context";
import { GroupThreadDetailPanel } from "../group/thread/group-thread-detail-panel";
import { useGroupThread } from "../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../group/chat/use-room-thread-panel-data";
import { RoomWorkspaceView } from "../workspace/room-workspace-view";
import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { SubagentTaskSurface } from "@/features/conversation/shared/subagent/subagent-task-surface";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";
import { RoomAgentAboutSurface } from "./room-agent-about-surface";
import { RoomChatSurface } from "./room-chat-surface";
import { RoomHistorySurface } from "./room-history-surface";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

type RoomAgentAboutRequestedTab = "identity" | "private_domain";

const RIGHT_PANEL_AUTO_COLLAPSE_SIDEBAR_QUERY = "(max-width: 1440px)";
const AUXILIARY_PANEL_WIDTH_LIMITS = {
  minWidth: "min(520px, 46vw)",
  maxWidth: "min(860px, 54vw)",
};

interface RoomSurfaceLayoutProps {
  currentAgent: Agent;
  currentRoomType: string;
  roomId: string | null;
  roomAvatar?: string | null;
  roomMembers: Agent[];
  availableRoomAgents: Agent[];
  currentRoomTitle: string;
  roomSkillNames: string[];
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomPrivateMessagesEnabled: boolean;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  currentRoomConversations: RoomConversationView[];
  activeWorkspacePath: string | null;
  activeSurfaceTab: RoomSurfaceTabKey;
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  isEditorOpen: boolean;
  editorWidthPercent: number;
  isResizingEditor: boolean;
  isConversationBusy: boolean;
  currentTodos: TodoItem[];
  workspaceSplitRef: RefObject<HTMLElement | null>;
  onReplayTour?: () => void;
  onChangeSurfaceTab: (tab: RoomSurfaceTabKey) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onAddRoomMember: (agentId: string) => Promise<void>;
  onRemoveRoomMember: (agentId: string) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onSaveAgentOptions: (agentId: string, title: string, options: AgentOptions, identity: AgentIdentityDraft) => Promise<void>;
  onValidateAgentName: (name: string, agentId?: string) => Promise<AgentNameValidationResult>;
  onUpdateRoom: (roomId: string, params: UpdateRoomParams) => Promise<void>;
  onUpdateConversationTitle: (conversationId: string, title: string) => Promise<void>;
  onOpenWorkspaceFile: (path: string | null, workspaceAgentId?: string | null) => void;
  onStartEditorResize: () => void;
  onLoadingChange: (isLoading: boolean) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

/**
 * Room 工作区主布局
 *
 * Thread 详情仍然作为聊天态右栏展示，
 * 文件编辑器则收进 workspace tab 自己的局部分栏。
 */
export function RoomSurfaceLayout(props: RoomSurfaceLayoutProps) {
  if (props.currentRoomType === "dm") {
    return <RoomSurfaceLayoutInner {...props} isThreadPanelOpen={false} />;
  }

  return (
    <GroupThreadContextProvider onOpenThread={() => props.onChangeSurfaceTab("chat")}>
      <RoomSurfaceLayoutWithThreadState {...props} />
    </GroupThreadContextProvider>
  );
}

function RoomSurfaceLayoutWithThreadState(props: RoomSurfaceLayoutProps) {
  // 只读 activeThread（ControlContext，稳定），不订阅 threadPanelData 对象：
  // 该对象每次产出新引用，而本组件是 GroupChatPanel（数据生产者）的祖先，
  // 一旦订阅就会形成「bump → 祖先重渲染 → 生产者重跑 → 再 bump」的死循环。
  // 真正需要数据的 GroupThreadDetailPanel 是生产者的兄弟叶子，自行订阅即可。
  const { activeThread, closeThread } = useGroupThread();

  useEffect(() => {
    if (props.activeSurfaceTab !== "chat" && activeThread) {
      closeThread();
    }
  }, [activeThread, closeThread, props.activeSurfaceTab]);

  return (
    <RoomSurfaceLayoutInner
      {...props}
      isThreadPanelOpen={Boolean(activeThread)}
    />
  );
}

type RoomSurfaceLayoutInnerProps = RoomSurfaceLayoutProps & {
  isThreadPanelOpen: boolean;
};

function RoomSurfaceLayoutInner({
  currentAgent: currentAgent,
  currentRoomType: currentRoomType,
  roomId: roomId,
  roomAvatar: roomAvatar,
  roomMembers: roomMembers,
  availableRoomAgents: availableRoomAgents,
  currentRoomTitle: currentRoomTitle,
  roomSkillNames: roomSkillNames,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  roomPrivateMessagesEnabled: roomPrivateMessagesEnabled,
  currentAgentSessionIdentity: currentAgentSessionIdentity,
  conversationId: conversationId,
  currentRoomConversations: currentRoomConversations,
  activeWorkspacePath: activeWorkspacePath,
  activeSurfaceTab: activeSurfaceTab,
  initialDraft: initialDraft = null,
  onInitialDraftConsumed: onInitialDraftConsumed,
  isEditorOpen: isEditorOpen,
  editorWidthPercent: editorWidthPercent,
  isResizingEditor: isResizingEditor,
  currentTodos: currentTodos,
  workspaceSplitRef: workspaceSplitRef,
  onReplayTour: onReplayTour,
  onChangeSurfaceTab: onChangeSurfaceTab,
  onCreateConversation: onCreateConversation,
  onSelectConversation: onSelectConversation,
  onCloseConversation: onCloseConversation,
  onDeleteConversation: onDeleteConversation,
  onAddRoomMember: onAddRoomMember,
  onRemoveRoomMember: onRemoveRoomMember,
  onOpenMemberManager: onOpenMemberManager,
  onSaveAgentOptions: onSaveAgentOptions,
  onValidateAgentName: onValidateAgentName,
  onUpdateRoom: onUpdateRoom,
  onUpdateConversationTitle: onUpdateConversationTitle,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onStartEditorResize: onStartEditorResize,
  onLoadingChange: onLoadingChange,
  onTodosChange: onTodosChange,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onRoomEvent: onRoomEvent,
  isThreadPanelOpen: isThreadPanelOpen,
}: RoomSurfaceLayoutInnerProps) {
  const isDm = currentRoomType === "dm";
  const isAuxiliaryPanelOpen = activeSurfaceTab !== "chat";
  const isRightPanelOpen = isAuxiliaryPanelOpen || isThreadPanelOpen;
  const subagentTaskSource = useMemo<SubagentTaskSource | null>(() => {
    if (isDm) {
      const sessionKey = currentAgentSessionIdentity?.session_key?.trim();
      return sessionKey ? { kind: "session", session_key: sessionKey } : null;
    }
    if (!roomId || !conversationId) {
      return null;
    }
    return {
      kind: "room",
      room_id: roomId,
      conversation_id: conversationId,
    };
  }, [conversationId, currentAgentSessionIdentity?.session_key, isDm, roomId]);
  const [aboutRequest, setAboutRequest] = useState<{
    agent_id: string | null;
    tab: RoomAgentAboutRequestedTab;
    key: number;
  }>({
    agent_id: null,
    tab: "private_domain",
    key: 0,
  });

  useWidePanelAutoCollapseForRightPanel(isRightPanelOpen);

  const handleOpenWorkspaceFile = useCallback((path: string | null, workspaceAgentId?: string | null) => {
    onOpenWorkspaceFile(path, workspaceAgentId);
  }, [onOpenWorkspaceFile]);

  const handleChangeSurfaceTab = useCallback((tab: RoomSurfaceTabKey) => {
    if (tab === "about") {
      setAboutRequest((current) => ({
        agent_id: currentAgent.agent_id,
        tab: "private_domain",
        key: current.key + 1,
      }));
    }
    onChangeSurfaceTab(tab);
  }, [currentAgent.agent_id, onChangeSurfaceTab]);

  const handleOpenAgentContact = useCallback((agentId: string) => {
    setAboutRequest((current) => ({
      agent_id: agentId,
      tab: "private_domain",
      key: current.key + 1,
    }));
    onChangeSurfaceTab("about");
  }, [onChangeSurfaceTab]);

  const handleCloseAuxiliaryPanel = useCallback(() => {
    onChangeSurfaceTab("chat");
  }, [onChangeSurfaceTab]);

  useEffect(() => {
    if (activeSurfaceTab === "subagents" && !subagentTaskSource) {
      onChangeSurfaceTab("chat");
    }
  }, [activeSurfaceTab, onChangeSurfaceTab, subagentTaskSource]);

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
            <div data-tour-anchor={CONVERSATION_TOUR_ANCHORS.header}>
              {isDm ? (
                <DmConversationHeader
                  activeTab={activeSurfaceTab}
                  conversationId={conversationId}
                  conversations={currentRoomConversations}
                  currentAgentName={currentAgent.name}
                  currentAgentAvatar={currentAgent.avatar ?? null}
                  onChangeTab={handleChangeSurfaceTab}
                  onCloseActiveTab={handleCloseAuxiliaryPanel}
                  onCloseConversation={onCloseConversation}
                  onCreateConversation={onCreateConversation}
                  onReplayTour={onReplayTour}
                  onSelectConversation={onSelectConversation}
                />
              ) : (
                <GroupConversationHeader
                  activeTab={activeSurfaceTab}
                  availableRoomAgents={availableRoomAgents}
                  conversationId={conversationId}
                  conversations={currentRoomConversations}
                  currentRoomTitle={currentRoomTitle}
                  onAddRoomMember={onAddRoomMember}
                  onOpenMemberManager={onOpenMemberManager}
                  onChangeTab={handleChangeSurfaceTab}
                  onCloseActiveTab={handleCloseAuxiliaryPanel}
                  onCloseConversation={onCloseConversation}
                  onCreateConversation={onCreateConversation}
                  onReplayTour={onReplayTour}
                  onRemoveRoomMember={onRemoveRoomMember}
                  onSelectConversation={onSelectConversation}
                  onUpdateRoom={onUpdateRoom}
                  roomAvatar={roomAvatar}
                  roomHostAgentId={roomHostAgentId}
                  roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
                  roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
                  roomId={roomId}
                  roomMembers={roomMembers}
                  roomSkillNames={roomSkillNames}
                />
              )}
            </div>
          )}
        >
          <div className="flex h-full min-h-0 min-w-0">
            <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
              {/* 中文注释：聊天面板必须常驻挂载，避免切换 surface tab 时卸载组件，
                    进而触发 useWebSocket 清理并关闭连接。 */}
              <RoomChatSurface
                conversationId={conversationId}
                currentAgent={currentAgent}
                currentAgentSessionIdentity={currentAgentSessionIdentity}
                currentRoomType={currentRoomType}
                initialDraft={initialDraft}
                onConversationSnapshotChange={onConversationSnapshotChange}
                onCreateConversation={onCreateConversation}
                onInitialDraftConsumed={onInitialDraftConsumed}
                onLoadingChange={onLoadingChange}
                onOpenAgentContact={handleOpenAgentContact}
                onOpenWorkspaceFile={handleOpenWorkspaceFile}
                onRoomEvent={onRoomEvent}
                onTodosChange={onTodosChange}
                roomHostAgentId={roomHostAgentId}
                roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
                roomId={roomId}
                roomMembers={roomMembers}
              />
              <WorkspaceTaskPanel key={conversationId ?? "conversation-tasks"} todos={currentTodos} />
            </div>

            {isAuxiliaryPanelOpen ? (
              <section
                className="relative ml-2 flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none"
                style={{
                  width: `${editorWidthPercent}%`,
                  ...AUXILIARY_PANEL_WIDTH_LIMITS,
                }}
              >
                <ConversationResizeHandle
                  ariaLabel="调整右侧面板宽度"
                  onMouseDown={onStartEditorResize}
                />

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", activeSurfaceTab !== "history" && "hidden")}>
                  <RoomHistorySurface
                    conversations={currentRoomConversations}
                    conversationId={conversationId}
                    currentRoomType={currentRoomType}
                    onCreateConversation={onCreateConversation}
                    onDeleteConversation={onDeleteConversation}
                    onSelectConversation={onSelectConversation}
                    onUpdateConversationTitle={onUpdateConversationTitle}
                  />
                </div>

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", activeSurfaceTab !== "workspace" && "hidden")}>
                  <RoomWorkspaceView
                    activeWorkspacePath={activeWorkspacePath}
                    agentId={currentAgent.agent_id}
                    isDm={isDm}
                    isEditorOpen={isEditorOpen}
                    roomMembers={roomMembers}
                    onOpenWorkspaceFile={handleOpenWorkspaceFile}
                  />
                </div>

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", activeSurfaceTab !== "about" && "hidden")}>
                  <RoomAgentAboutSurface
                    agent={currentAgent}
                    conversationId={conversationId}
                    roomId={roomId}
                    roomMembers={roomMembers}
                    isVisible={activeSurfaceTab === "about"}
                    requestedAgentId={aboutRequest.agent_id}
                    requestedTab={aboutRequest.tab}
                    requestKey={aboutRequest.key}
                    onSaveAgentOptions={onSaveAgentOptions}
                    onValidateAgentName={onValidateAgentName}
                  />
                </div>

                {activeSurfaceTab === "subagents" && subagentTaskSource ? (
                  <div className="flex h-full min-h-0 min-w-0 flex-1 flex-col">
                    <SubagentTaskSurface
                      onClose={handleCloseAuxiliaryPanel}
                      onOpenWorkspaceFile={(path, workspaceAgentId) =>
                        handleOpenWorkspaceFile(path, workspaceAgentId)}
                      source={subagentTaskSource}
                    />
                  </div>
                ) : null}
              </section>
            ) : !isDm ? (
              <GroupThreadInlinePanel
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

function useWidePanelAutoCollapseForRightPanel(isPanelOpen: boolean) {
  const shouldAutoCollapseSidebar = useMediaQuery(RIGHT_PANEL_AUTO_COLLAPSE_SIDEBAR_QUERY);
  const collapseWidePanelForRightPanel = useSidebarStore((s) => s.collapse_wide_panel_for_right_panel);
  const expandWidePanelAfterRightPanel = useSidebarStore((s) => s.expand_wide_panel_after_right_panel);

  useEffect(() => {
    if (isPanelOpen && shouldAutoCollapseSidebar) {
      collapseWidePanelForRightPanel();
      return;
    }
    expandWidePanelAfterRightPanel();
  }, [
    collapseWidePanelForRightPanel,
    expandWidePanelAfterRightPanel,
    isPanelOpen,
    shouldAutoCollapseSidebar,
  ]);

  useEffect(() => {
    return () => {
      expandWidePanelAfterRightPanel();
    };
  }, [expandWidePanelAfterRightPanel]);
}

function GroupThreadInlinePanel({
  activeSurfaceTab: activeSurfaceTab,
  editorWidthPercent: editorWidthPercent,
  className: className,
  onStartEditorResize: onStartEditorResize,
}: {
  activeSurfaceTab: RoomSurfaceTabKey;
  editorWidthPercent: number;
  className?: string;
  onStartEditorResize: () => void;
}) {
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (activeSurfaceTab !== "chat" || !activeThread || !threadPanelData) {
    return null;
  }

  return (
    <section
      className={cn(
        "relative ml-2 min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none",
        className,
      )}
      style={{
        width: `${editorWidthPercent}%`,
        minWidth: "360px",
        maxWidth: "560px",
      }}
    >
      <ConversationResizeHandle
        ariaLabel="调整 Thread 面板宽度"
        onMouseDown={onStartEditorResize}
      />

      <GroupThreadDetailPanel
        roundId={activeThread.roundId}
        agentId={activeThread.agentId}
        agentName={threadPanelData.agentName ?? activeThread.agentId}
        agentAvatar={threadPanelData.agentAvatar}
        userAvatar={threadPanelData.userAvatar}
        messages={threadPanelData.messages}
        pendingPermissions={threadPanelData.pendingPermissions}
        onPermissionResponse={threadPanelData.onPermissionResponse}
        canRespondToPermissions={threadPanelData.canRespondToPermissions}
        permissionReadOnlyReason={threadPanelData.permissionReadOnlyReason}
        onClose={closeThread}
        onStopMessage={threadPanelData.onStopMessage}
        onOpenWorkspaceFile={threadPanelData.onOpenWorkspaceFile}
        isLoading={threadPanelData.isLoading}
        layout="desktop"
      />
    </section>
  );
}
