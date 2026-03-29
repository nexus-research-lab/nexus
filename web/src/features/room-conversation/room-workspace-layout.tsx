"use client";

import { RefObject, useState } from "react";

import { RoomContextPanel } from "@/features/room-context/room-context-panel";
import { RoomEditorPanel } from "@/features/room-context/room-editor-panel";
import {
  HOME_CHAT_PANEL_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { WorkspaceCanvasShell } from "@/shared/ui/workspace-canvas-shell";
import { WorkspaceInspectorShell } from "@/shared/ui/workspace-inspector-shell";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";

import { RoomAgentAboutView } from "./room-agent-about-view";
import { RoomChatPanel } from "./room-chat-panel";
import { RoomConversationHeader } from "./room-conversation-header";
import { RoomConversationHistoryView } from "./room-conversation-history-view";
import { RoomWorkspaceView } from "./room-workspace-view";

interface RoomWorkspaceLayoutProps {
  current_agent: Agent;
  current_agent_id: string | null;
  current_room_type: string;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  active_workspace_path: string | null;
  active_surface_tab: RoomSurfaceTabKey;
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
  on_open_workspace_file: (path: string | null) => void;
  on_close_workspace_pane: () => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
}

/**
 * Room 双栏工作区布局
 *
 * 左侧：ConversationArea（flex-1），包含 Header + 消息流 + 输入框
 * 右侧：可折叠 DetailPanel（成员列表等）
 *
 * 原三栏布局中的左侧 ObjectListPanel 已迁移到侧边栏宽面板。
 */
export function RoomWorkspaceLayout({
  current_agent,
  current_agent_id,
  current_room_type,
  room_members,
  available_room_agents,
  current_room_title,
  current_conversation,
  current_conversation_id,
  current_room_conversations,
  active_workspace_path,
  active_surface_tab,
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
  on_open_workspace_file,
  on_close_workspace_pane,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
}: RoomWorkspaceLayoutProps) {
  // 右侧详情面板折叠状态
  const [is_detail_panel_open, set_is_detail_panel_open] = useState(true);

  // 仅在 chat tab 且编辑器未打开时显示详情面板
  const show_detail_panel = is_detail_panel_open && !is_editor_open && active_surface_tab === "chat";

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1 gap-2 lg:gap-2.5 xl:gap-3")}>
      {/* 主内容区：编辑器 + 对话区 */}
      <section
        ref={workspace_split_ref}
        className={cn(
          "flex min-h-0 min-w-0 flex-1 gap-3",
          is_resizing_editor && "cursor-col-resize select-none",
        )}
      >
        <RoomEditorPanel
          agent_id={current_agent.agent_id}
          class_name="hidden lg:flex"
          is_open={is_editor_open}
          on_close={on_close_workspace_pane}
          on_resize_start={on_start_editor_resize}
          path={active_workspace_path}
          width_percent={editor_width_percent}
        />

        <WorkspaceCanvasShell
          class_name={HOME_CHAT_PANEL_CLASS}
          is_joined_with_inspector={show_detail_panel}
        >
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            <RoomConversationHeader
              active_tab={active_surface_tab}
              conversation_count={current_room_conversations.length}
              conversations={current_room_conversations}
              current_agent_name={current_agent.name}
              current_conversation_id={current_conversation_id}
              current_conversation_title={current_conversation?.title ?? null}
              current_room_title={current_room_title}
              current_room_type={current_room_type}
              is_detail_panel_open={is_detail_panel_open}
              is_loading={is_conversation_busy}
              member_count={room_members.length}
              on_change_tab={on_change_surface_tab}
              on_select_conversation={on_select_conversation}
              on_toggle_detail_panel={() => set_is_detail_panel_open((prev) => !prev)}
              room_members={room_members}
            />

            <div className="min-h-0 min-w-0 flex-1">
              {active_surface_tab === "chat" ? (
                <RoomChatPanel
                  agent_id={current_agent.agent_id}
                  current_agent_name={current_agent.name}
                  current_room_title={current_room_title}
                  hide_header
                  on_conversation_snapshot_change={on_conversation_snapshot_change}
                  on_create_conversation={on_create_conversation}
                  on_loading_change={on_loading_change}
                  on_open_workspace_file={on_open_workspace_file}
                  on_todos_change={on_todos_change}
                  session_key={current_conversation?.session_key ?? null}
                  session_title={current_conversation?.title ?? null}
                />
              ) : null}

              {active_surface_tab === "history" ? (
                <RoomConversationHistoryView
                  conversations={current_room_conversations}
                  current_conversation_id={current_conversation_id}
                  current_room_type={current_room_type}
                  on_create_conversation={on_create_conversation}
                  on_delete_conversation={on_delete_conversation}
                  on_select_conversation={on_select_conversation}
                />
              ) : null}

              {active_surface_tab === "workspace" ? (
                <RoomWorkspaceView
                  active_workspace_path={active_workspace_path}
                  agent_id={current_agent.agent_id}
                  on_open_workspace_file={on_open_workspace_file}
                />
              ) : null}

              {active_surface_tab === "about" && current_room_type === "dm" ? (
                <RoomAgentAboutView agent={current_agent} />
              ) : null}
            </div>
          </div>
        </WorkspaceCanvasShell>
      </section>

      {/* 右侧可折叠详情面板 */}
      {show_detail_panel ? (
        <WorkspaceInspectorShell>
          <RoomContextPanel
            active_conversation={current_conversation}
            agent={current_agent}
            available_room_agents={available_room_agents}
            current_agent_id={current_agent_id}
            current_room_type={current_room_type}
            is_conversation_busy={is_conversation_busy}
            on_add_room_member={on_add_room_member}
            on_edit_agent={on_edit_agent}
            on_remove_room_member={on_remove_room_member}
            on_select_agent={on_select_agent}
            room_conversations={current_room_conversations}
            room_members={room_members}
            room_name={current_room_title}
            todos={current_todos}
          />
        </WorkspaceInspectorShell>
      ) : null}
    </div>
  );
}
