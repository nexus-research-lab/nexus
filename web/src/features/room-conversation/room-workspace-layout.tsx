"use client";

import { RefObject } from "react";

import { RoomContextPanel } from "@/features/room-context/room-context-panel";
import { RoomEditorPanel } from "@/features/room-context/room-editor-panel";
import { RoomSidebarPanel } from "@/features/room-navigation/room-sidebar-panel";
import {
  HOME_AGENT_INSPECTOR_WRAPPER_CLASS,
  HOME_CHAT_PANEL_CLASS,
  HOME_WORKSPACE_MAIN_GAP_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { AgentCostSummary, ConversationCostSummary } from "@/types/cost";
import { TodoItem } from "@/types/todo";

import { RoomChatPanel } from "./room-chat-panel";

interface RoomWorkspaceLayoutProps {
  current_agent: Agent;
  current_agent_id: string | null;
  room_members: Agent[];
  current_room_title: string;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  active_workspace_path: string | null;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  conversation_cost_summary: ConversationCostSummary;
  agent_cost_summary: AgentCostSummary;
  workspace_split_ref: RefObject<HTMLElement | null>;
  on_select_agent: (agent_id: string) => void;
  on_open_directory: () => void;
  on_create_agent: () => void;
  on_edit_agent: (agent_id: string) => void;
  on_create_conversation: () => void;
  on_select_conversation: (conversation_id: string) => void;
  on_delete_conversation: (conversation_id: string) => void;
  on_open_workspace_file: (path: string | null) => void;
  on_close_workspace_pane: () => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
}

export function RoomWorkspaceLayout({
  current_agent,
  current_agent_id,
  room_members,
  current_room_title,
  current_conversation,
  current_conversation_id,
  current_room_conversations,
  active_workspace_path,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_conversation_busy,
  current_todos,
  conversation_cost_summary,
  agent_cost_summary,
  workspace_split_ref,
  on_select_agent,
  on_open_directory,
  on_create_agent,
  on_edit_agent,
  on_create_conversation,
  on_select_conversation,
  on_delete_conversation,
  on_open_workspace_file,
  on_close_workspace_pane,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
}: RoomWorkspaceLayoutProps) {
  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1", HOME_WORKSPACE_MAIN_GAP_CLASS)}>
      <div className="hidden lg:flex lg:min-h-0 lg:shrink-0 lg:border-r lg:workspace-divider">
        <RoomSidebarPanel
          active_workspace_path={active_workspace_path}
          agent={current_agent}
          members={room_members}
          conversations={current_room_conversations}
          current_agent_id={current_agent_id}
          current_conversation_id={current_conversation_id}
          room_name={current_room_title}
          on_create_agent={on_create_agent}
          on_create_conversation={on_create_conversation}
          on_delete_conversation={on_delete_conversation}
          on_open_directory={on_open_directory}
          on_open_workspace_file={on_open_workspace_file}
          on_select_agent={on_select_agent}
          on_select_conversation={on_select_conversation}
        />
      </div>

      <section
        ref={workspace_split_ref}
        className={cn(
          "flex min-h-0 min-w-0 flex-1",
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

        <div className={cn(HOME_CHAT_PANEL_CLASS, !is_editor_open && "min-[1280px]:border-r min-[1280px]:workspace-divider")}>
          <div className="min-h-0 min-w-0 flex-1">
            <RoomChatPanel
              agent_id={current_agent.agent_id}
              current_agent_name={current_agent.name}
              current_room_title={current_room_title}
              on_conversation_snapshot_change={on_conversation_snapshot_change}
              on_create_conversation={on_create_conversation}
              on_loading_change={on_loading_change}
              on_open_workspace_file={on_open_workspace_file}
              on_todos_change={on_todos_change}
              session_key={current_conversation?.session_key ?? null}
              session_title={current_conversation?.title ?? null}
            />
          </div>
        </div>
      </section>

      {!is_editor_open ? (
        <div className={HOME_AGENT_INSPECTOR_WRAPPER_CLASS}>
          <RoomContextPanel
            active_conversation={current_conversation}
            agent={current_agent}
            agent_cost_summary={agent_cost_summary}
            is_conversation_busy={is_conversation_busy}
            on_edit_agent={on_edit_agent}
            conversation_cost_summary={conversation_cost_summary}
            room_conversations={current_room_conversations}
            room_members={room_members}
            room_name={current_room_title}
            todos={current_todos}
          />
        </div>
      ) : null}
    </div>
  );
}
