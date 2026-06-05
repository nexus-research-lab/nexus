"use client";

import { Fragment, RefObject, useCallback, useEffect, useState } from "react";
import { X } from "lucide-react";

import { DmChatPanel } from "@/features/conversation/room/dm/dm-chat-panel";
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

import { GroupChatPanel } from "../group/chat/group-chat-panel";
import { GroupChatErrorBoundary } from "../group/chat/group-chat-error-boundary";
import { GroupConversationHeader } from "../group/header/group-conversation-header";
import { GroupThreadContextProvider } from "../group/thread/group-thread-context";
import { GroupThreadDetailPanel } from "../group/thread/group-thread-detail-panel";
import { useGroupThread, useGroupThreadPanelData } from "../group/thread/group-thread-state";
import { RoomWorkspaceView } from "../workspace/room-workspace-view";
import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { RoomAgentAboutSurface } from "./room-agent-about-surface";
import { RoomHistorySurface } from "./room-history-surface";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

type RoomAgentAboutRequestedTab = "identity" | "private_domain";

const ChatBoundary = import.meta.env.DEV ? GroupChatErrorBoundary : Fragment;
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
  on_select_conversation: (conversation_id: string) => void;
  on_close_conversation: (conversation_id: string) => Promise<void>;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_open_member_manager: () => Promise<void>;
  on_save_agent_options: (agent_id: string, title: string, options: AgentOptions, identity: AgentIdentityDraft) => Promise<void>;
  on_validate_agent_name: (name: string, agent_id?: string) => Promise<AgentNameValidationResult>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_update_conversation_title: (conversation_id: string, title: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
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
  const {active_thread, close_thread} = useGroupThread();
  const {thread_panel_data} = useGroupThreadPanelData();

  useEffect(() => {
    if (props.active_surface_tab !== "chat" && active_thread) {
      close_thread();
    }
  }, [active_thread, close_thread, props.active_surface_tab]);

  return (
    <RoomSurfaceLayoutInner
      {...props}
      is_thread_panel_open={Boolean(active_thread && thread_panel_data)}
    />
  );
}

type RoomSurfaceLayoutInnerProps = RoomSurfaceLayoutProps & {
  is_thread_panel_open: boolean;
};

function RoomSurfaceLayoutInner({
  current_agent,
  current_room_type,
  room_id,
  room_avatar,
  room_members,
  available_room_agents,
  current_room_title,
  room_skill_names,
  room_host_agent_id,
  room_host_auto_reply_enabled,
  current_agent_session_identity,
  conversation_id,
  current_room_conversations,
  active_workspace_path,
  active_surface_tab,
  initial_draft = null,
  on_initial_draft_consumed,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  current_todos,
  workspace_split_ref,
  on_replay_tour,
  on_change_surface_tab,
  on_create_conversation,
  on_select_conversation,
  on_close_conversation,
  on_delete_conversation,
  on_add_room_member,
  on_remove_room_member,
  on_open_member_manager,
  on_save_agent_options,
  on_validate_agent_name,
  on_update_room,
  on_update_conversation_title,
  on_open_workspace_file,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
  on_room_event,
  is_thread_panel_open,
}: RoomSurfaceLayoutInnerProps) {
  const is_dm = current_room_type === "dm";
  const is_auxiliary_panel_open = active_surface_tab !== "chat";
  const is_right_panel_open = is_auxiliary_panel_open || is_thread_panel_open;
  const is_wide_auxiliary_panel =
    active_surface_tab === "history" ||
    active_surface_tab === "workspace" ||
    active_surface_tab === "about";
  const [about_request, set_about_request] = useState<{
    agent_id: string | null;
    tab: RoomAgentAboutRequestedTab;
    key: number;
  }>({
    agent_id: null,
    tab: "private_domain",
    key: 0,
  });

  useWidePanelAutoCollapseForRightPanel(is_right_panel_open);

  const handle_open_workspace_file = useCallback((path: string | null) => {
    on_open_workspace_file(path);
  }, [on_open_workspace_file]);

  const handle_change_surface_tab = useCallback((tab: RoomSurfaceTabKey) => {
    if (tab === "about") {
      set_about_request((current) => ({
        agent_id: current_agent.agent_id,
        tab: "private_domain",
        key: current.key + 1,
      }));
    }
    on_change_surface_tab(tab);
  }, [current_agent.agent_id, on_change_surface_tab]);

  const handle_open_agent_contact = useCallback((agent_id: string) => {
    set_about_request((current) => ({
      agent_id,
      tab: "private_domain",
      key: current.key + 1,
    }));
    on_change_surface_tab("about");
  }, [on_change_surface_tab]);

  const handle_close_auxiliary_panel = useCallback(() => {
    on_change_surface_tab("chat");
  }, [on_change_surface_tab]);

  const auxiliary_close_action = (
    <WorkspaceSurfaceToolbarAction onClick={handle_close_auxiliary_panel}>
      <X className="h-3.5 w-3.5" />
      关闭
    </WorkspaceSurfaceToolbarAction>
  );

  return (
    <section
      ref={workspace_split_ref}
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        is_resizing_editor && "cursor-col-resize select-none",
      )}
    >
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <WorkspaceSurfaceScaffold
          body_class_name="relative"
          header={(
            <div data-tour-anchor={CONVERSATION_TOUR_ANCHORS.header}>
              {is_dm ? (
                <DmConversationHeader
                  active_tab={active_surface_tab}
                  conversation_id={conversation_id}
                  conversations={current_room_conversations}
                  current_agent_name={current_agent.name}
                  current_agent_avatar={current_agent.avatar ?? null}
                  on_change_tab={handle_change_surface_tab}
                  on_close_conversation={on_close_conversation}
                  on_create_conversation={on_create_conversation}
                  on_replay_tour={on_replay_tour}
                  on_select_conversation={on_select_conversation}
                  todos={current_todos}
                />
              ) : (
                <GroupConversationHeader
                  active_tab={active_surface_tab}
                  available_room_agents={available_room_agents}
                  conversation_id={conversation_id}
                  conversations={current_room_conversations}
                  current_room_title={current_room_title}
                  on_add_room_member={on_add_room_member}
                  on_open_member_manager={on_open_member_manager}
                  on_change_tab={handle_change_surface_tab}
                  on_close_conversation={on_close_conversation}
                  on_create_conversation={on_create_conversation}
                  on_replay_tour={on_replay_tour}
                  on_remove_room_member={on_remove_room_member}
                  on_select_conversation={on_select_conversation}
                  on_update_room={on_update_room}
                  room_avatar={room_avatar}
                  room_host_agent_id={room_host_agent_id}
                  room_host_auto_reply_enabled={room_host_auto_reply_enabled}
                  room_id={room_id}
                  room_members={room_members}
                  room_skill_names={room_skill_names}
                  todos={current_todos}
                />
              )}
            </div>
          )}
        >
          <div className="flex h-full min-h-0 min-w-0">
            <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
              {/* 中文注释：聊天面板必须常驻挂载，避免切换 surface tab 时卸载组件，
                    进而触发 useWebSocket 清理并关闭连接。 */}
              {is_dm ? (
                <ChatBoundary>
                  <DmChatPanel
                    current_agent_name={current_agent.name}
                    current_agent_avatar={current_agent.avatar ?? null}
                    current_agent_permission_mode={current_agent.options.permission_mode ?? null}
                    initial_draft={initial_draft}
                    on_initial_draft_consumed={on_initial_draft_consumed}
                    on_conversation_snapshot_change={on_conversation_snapshot_change}
                    on_loading_change={on_loading_change}
                    on_open_agent_contact={handle_open_agent_contact}
                    on_open_workspace_file={handle_open_workspace_file}
                    on_room_event={on_room_event}
                    on_todos_change={on_todos_change}
                    session_identity={current_agent_session_identity}
                  />
                </ChatBoundary>
              ) : (
                <ChatBoundary>
                  <GroupChatPanel
                    agent_id={current_agent.agent_id}
                    conversation_id={conversation_id}
                    current_agent_name={current_agent.name}
                    current_agent_avatar={current_agent.avatar ?? null}
                    initial_draft={initial_draft}
                    on_initial_draft_consumed={on_initial_draft_consumed}
                    on_conversation_snapshot_change={on_conversation_snapshot_change}
                    on_create_conversation={on_create_conversation}
                    on_loading_change={on_loading_change}
                    on_open_agent_contact={handle_open_agent_contact}
                    on_open_workspace_file={handle_open_workspace_file}
                    on_room_event={on_room_event}
                    on_todos_change={on_todos_change}
                    room_host_agent_id={room_host_agent_id}
                    room_host_auto_reply_enabled={room_host_auto_reply_enabled}
                    room_id={room_id}
                    room_members={room_members}
                  />
                </ChatBoundary>
              )}
            </div>

            {is_auxiliary_panel_open ? (
              <section
                className="relative ml-2 flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none"
                style={{
                  width: `${editor_width_percent}%`,
                  ...(is_wide_auxiliary_panel
                    ? WIDE_AUXILIARY_PANEL_WIDTH_LIMITS
                    : AUXILIARY_PANEL_WIDTH_LIMITS),
                }}
              >
                <ConversationResizeHandle
                  aria_label="调整右侧面板宽度"
                  on_mouse_down={on_start_editor_resize}
                />

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", active_surface_tab !== "history" && "hidden")}>
                  <RoomHistorySurface
                    conversations={current_room_conversations}
                    conversation_id={conversation_id}
                    current_room_type={current_room_type}
                    header_action={auxiliary_close_action}
                    on_create_conversation={on_create_conversation}
                    on_delete_conversation={on_delete_conversation}
                    on_select_conversation={on_select_conversation}
                    on_update_conversation_title={on_update_conversation_title}
                  />
                </div>

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", active_surface_tab !== "workspace" && "hidden")}>
                  <RoomWorkspaceView
                    active_workspace_path={active_workspace_path}
                    agent_id={current_agent.agent_id}
                    header_action={auxiliary_close_action}
                    is_dm={is_dm}
                    is_editor_open={is_editor_open}
                    room_members={room_members}
                    on_open_workspace_file={on_open_workspace_file}
                  />
                </div>

                <div
                  className={cn("flex h-full min-h-0 min-w-0 flex-1 flex-col", active_surface_tab !== "about" && "hidden")}>
                  <RoomAgentAboutSurface
                    agent={current_agent}
                    conversation_id={conversation_id}
                    room_id={room_id}
                    room_members={room_members}
                    header_action={auxiliary_close_action}
                    is_visible={active_surface_tab === "about"}
                    requested_agent_id={about_request.agent_id}
                    requested_tab={about_request.tab}
                    request_key={about_request.key}
                    on_save_agent_options={on_save_agent_options}
                    on_validate_agent_name={on_validate_agent_name}
                  />
                </div>
              </section>
            ) : !is_dm ? (
              <GroupThreadInlinePanel
                active_surface_tab={active_surface_tab}
                class_name="hidden lg:flex"
                editor_width_percent={editor_width_percent}
                on_start_editor_resize={on_start_editor_resize}
              />
            ) : null}
          </div>
        </WorkspaceSurfaceScaffold>
      </div>
    </section>
  );
}

function useWidePanelAutoCollapseForRightPanel(is_panel_open: boolean) {
  const should_auto_collapse_sidebar = useMediaQuery(RIGHT_PANEL_AUTO_COLLAPSE_SIDEBAR_QUERY);
  const collapse_wide_panel_for_right_panel = useSidebarStore((s) => s.collapse_wide_panel_for_right_panel);
  const expand_wide_panel_after_right_panel = useSidebarStore((s) => s.expand_wide_panel_after_right_panel);

  useEffect(() => {
    if (is_panel_open && should_auto_collapse_sidebar) {
      collapse_wide_panel_for_right_panel();
      return;
    }
    expand_wide_panel_after_right_panel();
  }, [
    collapse_wide_panel_for_right_panel,
    expand_wide_panel_after_right_panel,
    is_panel_open,
    should_auto_collapse_sidebar,
  ]);

  useEffect(() => {
    return () => {
      expand_wide_panel_after_right_panel();
    };
  }, [expand_wide_panel_after_right_panel]);
}

function GroupThreadInlinePanel({
                                 active_surface_tab,
                                 editor_width_percent,
                                 class_name,
                                 on_start_editor_resize,
                               }: {
  active_surface_tab: RoomSurfaceTabKey;
  editor_width_percent: number;
  class_name?: string;
  on_start_editor_resize: () => void;
}) {
  const {active_thread, close_thread} = useGroupThread();
  const {thread_panel_data} = useGroupThreadPanelData();

  if (active_surface_tab !== "chat" || !active_thread || !thread_panel_data) {
    return null;
  }

  return (
    <section
      className={cn(
        "relative ml-2 min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none",
        class_name,
      )}
      style={{
        width: `${editor_width_percent}%`,
        minWidth: "360px",
        maxWidth: "560px",
      }}
    >
      <ConversationResizeHandle
        aria_label="调整 Thread 面板宽度"
        on_mouse_down={on_start_editor_resize}
      />

      <GroupThreadDetailPanel
        round_id={active_thread.round_id}
        agent_id={active_thread.agent_id}
        agent_name={thread_panel_data.agent_name ?? active_thread.agent_id}
        agent_avatar={thread_panel_data.agent_avatar}
        user_avatar={thread_panel_data.user_avatar}
        messages={thread_panel_data.messages}
        pending_permissions={thread_panel_data.pending_permissions}
        on_permission_response={thread_panel_data.on_permission_response}
        can_respond_to_permissions={thread_panel_data.can_respond_to_permissions}
        permission_read_only_reason={thread_panel_data.permission_read_only_reason}
        on_close={close_thread}
        on_stop_message={thread_panel_data.on_stop_message}
        on_open_workspace_file={thread_panel_data.on_open_workspace_file}
        is_loading={thread_panel_data.is_loading}
        layout="desktop"
      />
    </section>
  );
}
