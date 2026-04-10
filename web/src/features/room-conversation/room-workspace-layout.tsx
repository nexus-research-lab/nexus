"use client";

import { Fragment, RefObject, useCallback } from "react";

import { EditorPanel } from "@/features/conversation-shared/context/editor-panel";
import { DmChatPanel } from "@/features/dm-conversation/dm-chat-panel";
import { DmConversationHeader } from "@/features/dm-conversation/dm-conversation-header";
import { cn } from "@/lib/utils";
import { WorkspaceCanvasShell } from "@/shared/ui/workspace/workspace-canvas-shell";
import { Agent } from "@/types/agent";
import { AgentConversationIdentity } from "@/types/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { UpdateRoomParams } from "@/types/room";

import { RoomChatPanel } from "./room-chat-panel";
import { RoomChatErrorBoundary } from "./room-chat-error-boundary";
import { RoomConversationHeader } from "./room-conversation-header";
import { RoomConversationHistoryView } from "./room-conversation-history-view";
import { RoomWorkspaceView } from "./room-workspace-view";
import { RoomAgentAboutView } from "./room-agent-about-view";
import { RoomThreadContextProvider } from "./thread/room-thread-context";
import { useRoomThread, useThreadPanelData } from "./thread/room-thread-state";
import { ThreadDetailPanel } from "./thread-detail-panel";

const ChatBoundary = import.meta.env.DEV ? RoomChatErrorBoundary : Fragment;

interface RoomWorkspaceLayoutProps {
  current_agent: Agent;
  current_room_type: string;
  room_id: string | null;
  room_description: string;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
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
  on_change_surface_tab: (tab: RoomSurfaceTabKey) => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_update_conversation_title: (conversation_id: string, title: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_close_workspace_pane: () => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent-conversation").RoomEventPayload) => void;
}

/**
 * Room 工作区主布局
 *
 * 右侧常驻栏已经移除，任务状态收进 Header，
 * Thread 详情与文件编辑一样，作为独立右栏展示。
 */
export function RoomWorkspaceLayout(props: RoomWorkspaceLayoutProps) {
  if (props.current_room_type === "dm") {
    return <RoomWorkspaceLayoutInner {...props} />;
  }

  return (
    <RoomThreadContextProvider>
      <RoomWorkspaceLayoutInner {...props} />
    </RoomThreadContextProvider>
  );
}

function RoomWorkspaceLayoutInner({
  current_agent,
  current_room_type,
  room_id,
  room_description,
  room_members,
  available_room_agents,
  current_room_title,
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
  is_conversation_busy,
  current_todos,
  workspace_split_ref,
  on_change_surface_tab,
  on_create_conversation,
  on_select_conversation,
  on_delete_conversation,
  on_add_room_member,
  on_update_room,
  on_delete_room,
  on_update_conversation_title,
  on_open_workspace_file,
  on_close_workspace_pane,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
  on_room_event,
}: RoomWorkspaceLayoutProps) {
  const is_dm = current_room_type === "dm";

  const handle_open_workspace_file = useCallback((path: string | null) => {
    on_open_workspace_file(path);
  }, [on_open_workspace_file]);

  return (
    <div className="flex min-h-0 min-w-0 flex-1">
      <section
        ref={workspace_split_ref}
        className={cn(
          "flex min-h-0 min-w-0 flex-1",
          is_resizing_editor && "cursor-col-resize select-none",
        )}
      >
        <WorkspaceCanvasShell>
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            {is_dm ? (
              <DmConversationHeader
                active_tab={active_surface_tab}
                conversation_id={conversation_id}
                conversations={current_room_conversations}
                current_agent_name={current_agent.name}
                is_loading={is_conversation_busy}
                on_change_tab={on_change_surface_tab}
                on_create_conversation={on_create_conversation}
                on_select_conversation={on_select_conversation}
                todos={current_todos}
              />
            ) : (
              <RoomConversationHeader
                active_tab={active_surface_tab}
                available_room_agents={available_room_agents}
                conversation_id={conversation_id}
                conversations={current_room_conversations}
                current_room_title={current_room_title}
                is_loading={is_conversation_busy}
                on_add_room_member={on_add_room_member}
                on_change_tab={on_change_surface_tab}
                on_create_conversation={on_create_conversation}
                on_delete_room={on_delete_room}
                on_select_conversation={on_select_conversation}
                on_update_room={on_update_room}
                room_description={room_description}
                room_id={room_id}
                room_members={room_members}
                todos={current_todos}
              />
            )}

            <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
              <div className={cn("h-full min-h-0 min-w-0", active_surface_tab !== "chat" && "hidden")}>
                {/* 中文注释：聊天面板必须常驻挂载，避免切换 surface tab 时卸载组件，
                    进而触发 useWebSocket 清理并关闭连接。 */}
                {is_dm ? (
                  <ChatBoundary>
                    <DmChatPanel
                      current_agent_name={current_agent.name}
                      initial_draft={initial_draft}
                      on_initial_draft_consumed={on_initial_draft_consumed}
                      on_conversation_snapshot_change={on_conversation_snapshot_change}
                      on_loading_change={on_loading_change}
                      on_open_workspace_file={handle_open_workspace_file}
                      on_todos_change={on_todos_change}
                      session_identity={current_agent_session_identity}
                    />
                  </ChatBoundary>
                ) : (
                  <ChatBoundary>
                    <RoomChatPanel
                      agent_id={current_agent.agent_id}
                      conversation_id={conversation_id}
                      current_agent_name={current_agent.name}
                      initial_draft={initial_draft}
                      on_initial_draft_consumed={on_initial_draft_consumed}
                      on_conversation_snapshot_change={on_conversation_snapshot_change}
                      on_create_conversation={on_create_conversation}
                      on_loading_change={on_loading_change}
                      on_open_workspace_file={handle_open_workspace_file}
                      on_room_event={on_room_event}
                      on_todos_change={on_todos_change}
                      room_id={room_id}
                      room_members={room_members}
                    />
                  </ChatBoundary>
                )}
              </div>

              {active_surface_tab === "history" ? (
                <RoomConversationHistoryView
                  conversations={current_room_conversations}
                  conversation_id={conversation_id}
                  current_room_type={current_room_type}
                  on_create_conversation={on_create_conversation}
                  on_delete_conversation={on_delete_conversation}
                  on_select_conversation={on_select_conversation}
                  on_update_conversation_title={on_update_conversation_title}
                />
              ) : null}

              {active_surface_tab === "workspace" ? (
                <RoomWorkspaceView
                  active_workspace_path={active_workspace_path}
                  agent_id={current_agent.agent_id}
                  is_dm={is_dm}
                  room_members={room_members}
                  on_open_workspace_file={on_open_workspace_file}
                />
              ) : null}

              {active_surface_tab === "about" && is_dm ? (
                <RoomAgentAboutView agent={current_agent} />
              ) : null}

            </div>
          </div>
        </WorkspaceCanvasShell>

        {!is_dm ? (
          <RoomThreadSidePanel
            active_surface_tab={active_surface_tab}
            class_name="hidden lg:flex lg:ml-2"
          />
        ) : null}

        <EditorPanel
          agent_id={current_agent.agent_id}
          class_name={cn("hidden lg:flex", is_editor_open && "lg:ml-2")}
          is_open={is_editor_open}
          on_close={on_close_workspace_pane}
          on_resize_start={on_start_editor_resize}
          path={active_workspace_path}
          width_percent={editor_width_percent}
        />
      </section>
    </div>
  );
}

function RoomThreadSidePanel({
  active_surface_tab,
  class_name,
}: {
  active_surface_tab: RoomSurfaceTabKey;
  class_name?: string;
}) {
  const { active_thread, close_thread } = useRoomThread();
  const { thread_panel_data } = useThreadPanelData();

  if (active_surface_tab !== "chat" || !active_thread || !thread_panel_data) {
    return null;
  }

  return (
    <section
      className={cn(
        "glass-card radius-shell-lg min-h-0 min-w-0 shrink-0 overflow-hidden",
        class_name,
      )}
      style={{ width: "clamp(320px,34vw,436px)" }}
    >
      <ThreadDetailPanel
        round_id={active_thread.round_id}
        agent_id={active_thread.agent_id}
        agent_name={thread_panel_data.agent_name ?? active_thread.agent_id}
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
