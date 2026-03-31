"use client";

import { Fragment, RefObject, useState } from "react";

import { RoomContextPanel } from "@/features/conversation-shared/context/room-context-panel";
import { EditorPanel } from "@/features/conversation-shared/context/editor-panel";
import { DmChatPanel } from "@/features/dm-conversation/dm-chat-panel";
import {
  HOME_CHAT_PANEL_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { WorkspaceCanvasShell } from "@/shared/ui/workspace/workspace-canvas-shell";
import { WorkspaceInspectorShell } from "@/shared/ui/workspace/workspace-inspector-shell";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { UpdateRoomParams } from "@/types/room";

import { RoomChatPanel } from "./room-chat-panel";
import { RoomChatErrorBoundary } from "./room-chat-error-boundary";
import { RoomConversationHistoryView } from "./room-conversation-history-view";
import { RoomWorkspaceView } from "./room-workspace-view";
import { RoomAgentAboutView } from "./room-agent-about-view";

interface RoomWorkspaceLayoutProps {
  current_agent: Agent;
  current_agent_id: string | null;
  current_room_type: string;
  room_id: string | null;
  room_description: string;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  active_workspace_path: string | null;
  active_surface_tab: RoomSurfaceTabKey;
  initial_draft?: string | null;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  workspace_split_ref: RefObject<HTMLElement | null>;
  on_change_surface_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_agent: (agent_id: string) => void;
  on_edit_agent: (agent_id: string) => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
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
 * Room 双栏工作区布局
 *
 * 左侧：ConversationArea（flex-1），包含 Header + 消息流 + 输入框
 * 右侧：可折叠 DetailPanel（成员列表等）
 */
export function RoomWorkspaceLayout({
  current_agent,
  current_agent_id,
  current_room_type,
  room_id,
  room_description,
  room_members,
  available_room_agents,
  current_room_title,
  current_conversation,
  current_conversation_id,
  current_room_conversations,
  active_workspace_path,
  active_surface_tab,
  initial_draft = null,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_conversation_busy,
  current_todos,
  workspace_split_ref,
  on_change_surface_tab,
  on_select_agent,
  on_edit_agent,
  on_create_conversation,
  on_select_conversation,
  on_delete_conversation,
  on_add_room_member,
  on_remove_room_member,
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
  const [is_detail_panel_open, set_is_detail_panel_open] = useState(true);
  const is_dm = current_room_type === "dm";
  const ChatBoundary = import.meta.env.DEV ? RoomChatErrorBoundary : Fragment;

  // 仅在 chat tab 且编辑器未打开时显示详情面板
  const show_detail_panel = is_detail_panel_open && !is_editor_open && active_surface_tab === "chat";

  const handle_open_workspace_file = (path: string) => on_open_workspace_file(path);

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1 gap-2")}>
      {/* 主内容区：编辑器 + 对话区 */}
      <section
        ref={workspace_split_ref}
        className={cn(
          "flex min-h-0 min-w-0 flex-1",
          is_editor_open && "gap-2",
          is_resizing_editor && "cursor-col-resize select-none",
        )}
      >
        <WorkspaceCanvasShell
          class_name={HOME_CHAT_PANEL_CLASS}
          is_joined_with_inspector={show_detail_panel}
        >
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            <div className="min-h-0 min-w-0 flex-1">
              {active_surface_tab === "chat" ? (
                is_dm ? (
                  <ChatBoundary>
                    <DmChatPanel
                      active_tab={active_surface_tab}
                      agent_id={current_agent.agent_id}
                      conversations={current_room_conversations}
                      current_agent_name={current_agent.name}
                      initial_draft={initial_draft}
                      is_detail_panel_open={is_detail_panel_open}
                      on_change_tab={on_change_surface_tab}
                      on_conversation_snapshot_change={on_conversation_snapshot_change}
                      on_create_conversation={on_create_conversation}
                      on_loading_change={on_loading_change}
                      on_open_workspace_file={handle_open_workspace_file}
                      on_todos_change={on_todos_change}
                      on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
                      session_key={current_conversation?.session_key ?? null}
                      session_title={current_conversation?.title ?? null}
                    />
                  </ChatBoundary>
                ) : (
                  <ChatBoundary>
                    <RoomChatPanel
                      active_tab={active_surface_tab}
                      agent_id={current_agent.agent_id}
                      conversation_id={current_conversation_id}
                      conversations={current_room_conversations}
                      current_agent_name={current_agent.name}
                      current_room_title={current_room_title}
                      initial_draft={initial_draft}
                      is_detail_panel_open={is_detail_panel_open}
                      on_change_tab={on_change_surface_tab}
                      on_conversation_snapshot_change={on_conversation_snapshot_change}
                      on_create_conversation={on_create_conversation}
                      on_loading_change={on_loading_change}
                      on_open_workspace_file={handle_open_workspace_file}
                      on_room_event={on_room_event}
                      on_select_conversation={on_select_conversation}
                      on_todos_change={on_todos_change}
                      on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
                      room_id={room_id}
                      room_members={room_members}
                      session_title={current_conversation?.title ?? null}
                    />
                  </ChatBoundary>
                )
              ) : null}

              {active_surface_tab === "history" ? (
                <RoomConversationHistoryView
                  conversations={current_room_conversations}
                  current_conversation_id={current_conversation_id}
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
                  on_open_workspace_file={on_open_workspace_file}
                />
              ) : null}

              {active_surface_tab === "about" && is_dm ? (
                <RoomAgentAboutView agent={current_agent} />
              ) : null}
            </div>
          </div>
        </WorkspaceCanvasShell>

        <EditorPanel
          agent_id={current_agent.agent_id}
          class_name="hidden lg:flex"
          is_open={is_editor_open}
          on_close={on_close_workspace_pane}
          on_resize_start={on_start_editor_resize}
          path={active_workspace_path}
          width_percent={editor_width_percent}
        />
      </section>

      {/* 右侧可折叠详情面板 */}
      {
        show_detail_panel ? (
          <WorkspaceInspectorShell>
            <RoomContextPanel
              active_conversation={current_conversation}
              agent={current_agent}
              available_room_agents={available_room_agents}
              current_agent_id={current_agent_id}
              current_room_type={current_room_type}
              room_id={room_id}
              room_name={current_room_title}
              room_description={room_description}
              is_conversation_busy={is_conversation_busy}
              on_add_room_member={on_add_room_member}
              on_edit_agent={on_edit_agent}
              on_remove_room_member={on_remove_room_member}
              on_update_room={on_update_room}
              on_delete_room={on_delete_room}
              on_select_agent={on_select_agent}
              room_conversations={current_room_conversations}
              room_members={room_members}
              todos={current_todos}
            />
          </WorkspaceInspectorShell>
        ) : null
      }
    </div >
  );
}
