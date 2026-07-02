"use client";

import { RefObject, useCallback, useEffect, useState } from "react";
import { X } from "lucide-react";

import { DmConversationHeader } from "@/features/conversation/room/dm/dm-conversation-header";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
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
import { RoomAgentAboutSurface } from "./room-agent-about-surface";
import { RoomChatSurface } from "./room-chat-surface";
import { RoomHistorySurface } from "./room-history-surface";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

type RoomAgentAboutRequestedTab = "identity" | "private_domain";

const RIGHT_PANEL_AUTO_COLLAPSE_SIDEBAR_QUERY = "(max-width: 1440px)";
const WIDE_AUXILIARY_PANEL_WIDTH_LIMITS = {
  minWidth: "min(520px, 46vw)",
  maxWidth: "min(860px, 54vw)",
};
const AUXILIARY_PANEL_WIDTH_LIMITS = {
  minWidth: "min(420px, 40vw)",
  maxWidth: "min(600px, 48vw)",
};

interface RoomSurfaceLayoutProps {
  current_agent: Agent;
  current_room_type: string;
  room_id: string | null;
  room_avatar?: string | null;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  room_skill_names: string[];
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  room_private_messages_enabled: boolean;
  current_agent_session_identity: AgentConversationIdentity | null;
  conversation_id: string | null;
  current_room_conversations: RoomConversationView[];
  active_workspace_path: string | null;
  active_surface_tab: RoomSurfaceTabKey;
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  workspace_split_ref: RefObject<HTMLElement | null>;
  on_replay_tour?: () => void;
  on_change_surface_tab: (tab: RoomSurfaceTabKey) => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversationId: string) => void;
  on_close_conversation: (conversationId: string) => Promise<void>;
  on_delete_conversation: (conversationId: string) => Promise<string | null>;
  on_add_room_member: (agentId: string) => Promise<void>;
  on_remove_room_member: (agentId: string) => Promise<void>;
  on_open_member_manager: () => Promise<void>;
  on_save_agent_options: (agentId: string, title: string, options: AgentOptions, identity: AgentIdentityDraft) => Promise<void>;
  on_validate_agent_name: (name: string, agentId?: string) => Promise<AgentNameValidationResult>;
  on_update_room: (roomId: string, params: UpdateRoomParams) => Promise<void>;
  on_update_conversation_title: (conversationId: string, title: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_start_editor_resize: () => void;
  on_loading_change: (isLoading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

/**
 * Room 工作区主布局
 *
 * Thread 详情仍然作为聊天态右栏展示，
 * 文件编辑器则收进 workspace tab 自己的局部分栏。
 */
export function RoomSurfaceLayout(props: RoomSurfaceLayoutProps) {
  if (props.current_room_type === "dm") {
    return <RoomSurfaceLayoutInner {...props} is_thread_panel_open={false} />;
  }

  return (
    <GroupThreadContextProvider on_open_thread={() => props.on_change_surface_tab("chat")}>
      <RoomSurfaceLayoutWithThreadState {...props} />
    </GroupThreadContextProvider>
  );
}

function RoomSurfaceLayoutWithThreadState(props: RoomSurfaceLayoutProps) {
  // 只读 active_thread（ControlContext，稳定），不订阅 thread_panel_data 对象：
  // 该对象每次产出新引用，而本组件是 GroupChatPanel（数据生产者）的祖先，
  // 一旦订阅就会形成「bump → 祖先重渲染 → 生产者重跑 → 再 bump」的死循环。
  // 真正需要数据的 GroupThreadDetailPanel 是生产者的兄弟叶子，自行订阅即可。
  const { active_thread: activeThread, close_thread: closeThread } = useGroupThread();

  useEffect(() => {
    if (props.active_surface_tab !== "chat" && activeThread) {
      closeThread();
    }
  }, [activeThread, closeThread, props.active_surface_tab]);

  return (
    <RoomSurfaceLayoutInner
      {...props}
      is_thread_panel_open={Boolean(activeThread)}
    />
  );
}

type RoomSurfaceLayoutInnerProps = RoomSurfaceLayoutProps & {
  is_thread_panel_open: boolean;
};

function RoomSurfaceLayoutInner({
  current_agent: currentAgent,
  current_room_type: currentRoomType,
  room_id: roomId,
  room_avatar: roomAvatar,
  room_members: roomMembers,
  available_room_agents: availableRoomAgents,
  current_room_title: currentRoomTitle,
  room_skill_names: roomSkillNames,
  room_host_agent_id: roomHostAgentId,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled,
  room_private_messages_enabled: roomPrivateMessagesEnabled,
  current_agent_session_identity: currentAgentSessionIdentity,
  conversation_id: conversationId,
  current_room_conversations: currentRoomConversations,
  active_workspace_path: activeWorkspacePath,
  active_surface_tab: activeSurfaceTab,
  initial_draft: initialDraft = null,
  on_initial_draft_consumed: onInitialDraftConsumed,
  is_editor_open: isEditorOpen,
  editor_width_percent: editorWidthPercent,
  is_resizing_editor: isResizingEditor,
  current_todos: currentTodos,
  workspace_split_ref: workspaceSplitRef,
  on_replay_tour: onReplayTour,
  on_change_surface_tab: onChangeSurfaceTab,
  on_create_conversation: onCreateConversation,
  on_select_conversation: onSelectConversation,
  on_close_conversation: onCloseConversation,
  on_delete_conversation: onDeleteConversation,
  on_add_room_member: onAddRoomMember,
  on_remove_room_member: onRemoveRoomMember,
  on_open_member_manager: onOpenMemberManager,
  on_save_agent_options: onSaveAgentOptions,
  on_validate_agent_name: onValidateAgentName,
  on_update_room: onUpdateRoom,
  on_update_conversation_title: onUpdateConversationTitle,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_start_editor_resize: onStartEditorResize,
  on_loading_change: onLoadingChange,
  on_todos_change: onTodosChange,
  on_conversation_snapshot_change: onConversationSnapshotChange,
  on_room_event: onRoomEvent,
  is_thread_panel_open: isThreadPanelOpen,
}: RoomSurfaceLayoutInnerProps) {
  const isDm = currentRoomType === "dm";
  const isAuxiliaryPanelOpen = activeSurfaceTab !== "chat";
  const isRightPanelOpen = isAuxiliaryPanelOpen || isThreadPanelOpen;
  const isWideAuxiliaryPanel =
    activeSurfaceTab === "history" ||
    activeSurfaceTab === "workspace" ||
    activeSurfaceTab === "about";
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

  const handleOpenWorkspaceFile = useCallback((path: string | null) => {
    onOpenWorkspaceFile(path);
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

  const auxiliaryCloseAction = (
    <WorkspaceSurfaceToolbarAction onClick={handleCloseAuxiliaryPanel}>
      <X className="h-3.5 w-3.5" />
      关闭
    </WorkspaceSurfaceToolbarAction>
  );

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
          body_class_name="relative"
          header={(
            <div data-tour-anchor={CONVERSATION_TOUR_ANCHORS.header}>
              {isDm ? (
                <DmConversationHeader
                  active_tab={activeSurfaceTab}
                  conversation_id={conversationId}
                  conversations={currentRoomConversations}
                  current_agent_name={currentAgent.name}
                  current_agent_avatar={currentAgent.avatar ?? null}
                  on_change_tab={handleChangeSurfaceTab}
                  on_close_conversation={onCloseConversation}
                  on_create_conversation={onCreateConversation}
                  on_replay_tour={onReplayTour}
                  on_select_conversation={onSelectConversation}
                  todos={currentTodos}
                />
              ) : (
                <GroupConversationHeader
                  active_tab={activeSurfaceTab}
                  available_room_agents={availableRoomAgents}
                  conversation_id={conversationId}
                  conversations={currentRoomConversations}
                  current_room_title={currentRoomTitle}
                  on_add_room_member={onAddRoomMember}
                  on_open_member_manager={onOpenMemberManager}
                  on_change_tab={handleChangeSurfaceTab}
                  on_close_conversation={onCloseConversation}
                  on_create_conversation={onCreateConversation}
                  on_replay_tour={onReplayTour}
                  on_remove_room_member={onRemoveRoomMember}
                  on_select_conversation={onSelectConversation}
                  on_update_room={onUpdateRoom}
                  room_avatar={roomAvatar}
                  room_host_agent_id={roomHostAgentId}
                  room_host_auto_reply_enabled={roomHostAutoReplyEnabled}
                  room_private_messages_enabled={roomPrivateMessagesEnabled}
                  room_id={roomId}
                  room_members={roomMembers}
                  room_skill_names={roomSkillNames}
                  todos={currentTodos}
                />
              )}
            </div>
          )}
        >
          <div className="flex h-full min-h-0 min-w-0">
            <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
              {/* 中文注释：聊天面板必须常驻挂载，避免切换 surface tab 时卸载组件，
                    进而触发 useWebSocket 清理并关闭连接。 */}
              <RoomChatSurface
                conversation_id={conversationId}
                current_agent={currentAgent}
                current_agent_session_identity={currentAgentSessionIdentity}
                current_room_type={currentRoomType}
                initial_draft={initialDraft}
                on_conversation_snapshot_change={onConversationSnapshotChange}
                on_create_conversation={onCreateConversation}
                on_initial_draft_consumed={onInitialDraftConsumed}
                on_loading_change={onLoadingChange}
                on_open_agent_contact={handleOpenAgentContact}
                on_open_workspace_file={handleOpenWorkspaceFile}
                on_room_event={onRoomEvent}
                on_todos_change={onTodosChange}
                room_host_agent_id={roomHostAgentId}
                room_host_auto_reply_enabled={roomHostAutoReplyEnabled}
                room_id={roomId}
                room_members={roomMembers}
              />
            </div>

            {isAuxiliaryPanelOpen ? (
              <section
                className="relative ml-2 flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none"
                style={{
                  width: `${editorWidthPercent}%`,
                  ...(isWideAuxiliaryPanel
                    ? WIDE_AUXILIARY_PANEL_WIDTH_LIMITS
                    : AUXILIARY_PANEL_WIDTH_LIMITS),
                }}
              >
                <ConversationResizeHandle
                  aria_label="调整右侧面板宽度"
                  on_mouse_down={onStartEditorResize}
                />

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", activeSurfaceTab !== "history" && "hidden")}>
                  <RoomHistorySurface
                    conversations={currentRoomConversations}
                    conversation_id={conversationId}
                    current_room_type={currentRoomType}
                    header_action={auxiliaryCloseAction}
                    on_create_conversation={onCreateConversation}
                    on_delete_conversation={onDeleteConversation}
                    on_select_conversation={onSelectConversation}
                    on_update_conversation_title={onUpdateConversationTitle}
                  />
                </div>

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", activeSurfaceTab !== "workspace" && "hidden")}>
                  <RoomWorkspaceView
                    active_workspace_path={activeWorkspacePath}
                    agent_id={currentAgent.agent_id}
                    header_action={auxiliaryCloseAction}
                    is_dm={isDm}
                    is_editor_open={isEditorOpen}
                    room_members={roomMembers}
                    on_open_workspace_file={handleOpenWorkspaceFile}
                  />
                </div>

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", activeSurfaceTab !== "about" && "hidden")}>
                  <RoomAgentAboutSurface
                    agent={currentAgent}
                    conversation_id={conversationId}
                    room_id={roomId}
                    room_members={roomMembers}
                    header_action={auxiliaryCloseAction}
                    is_visible={activeSurfaceTab === "about"}
                    requested_agent_id={aboutRequest.agent_id}
                    requested_tab={aboutRequest.tab}
                    request_key={aboutRequest.key}
                    on_save_agent_options={onSaveAgentOptions}
                    on_validate_agent_name={onValidateAgentName}
                  />
                </div>
              </section>
            ) : !isDm ? (
              <GroupThreadInlinePanel
                active_surface_tab={activeSurfaceTab}
                class_name="hidden lg:flex"
                editor_width_percent={editorWidthPercent}
                on_start_editor_resize={onStartEditorResize}
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
  active_surface_tab: activeSurfaceTab,
  editor_width_percent: editorWidthPercent,
  class_name: className,
  on_start_editor_resize: onStartEditorResize,
}: {
  active_surface_tab: RoomSurfaceTabKey;
  editor_width_percent: number;
  class_name?: string;
  on_start_editor_resize: () => void;
}) {
  const { active_thread: activeThread, close_thread: closeThread } = useGroupThread();
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
        aria_label="调整 Thread 面板宽度"
        on_mouse_down={onStartEditorResize}
      />

      <GroupThreadDetailPanel
        round_id={activeThread.round_id}
        agent_id={activeThread.agent_id}
        agent_name={threadPanelData.agent_name ?? activeThread.agent_id}
        agent_avatar={threadPanelData.agent_avatar}
        user_avatar={threadPanelData.user_avatar}
        messages={threadPanelData.messages}
        pending_permissions={threadPanelData.pending_permissions}
        on_permission_response={threadPanelData.on_permission_response}
        can_respond_to_permissions={threadPanelData.can_respond_to_permissions}
        permission_read_only_reason={threadPanelData.permission_read_only_reason}
        on_close={closeThread}
        on_stop_message={threadPanelData.on_stop_message}
        on_open_workspace_file={threadPanelData.on_open_workspace_file}
        is_loading={threadPanelData.is_loading}
        layout="desktop"
      />
    </section>
  );
}
