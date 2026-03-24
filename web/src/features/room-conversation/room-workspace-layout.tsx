"use client";

import { RefObject } from "react";

import { TodoItem } from "@/components/workspace/agent-task-widget";
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
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";

import { RoomChatPanel } from "./room-chat-panel";

interface RoomWorkspaceLayoutProps {
  agents: Agent[];
  current_agent: Agent;
  current_agent_id: string | null;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  recent_agents: Agent[];
  active_workspace_path: string | null;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_session_busy: boolean;
  current_todos: TodoItem[];
  session_cost_summary: SessionCostSummary;
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
  agents,
  current_agent,
  current_agent_id,
  current_conversation,
  current_conversation_id,
  current_room_conversations,
  recent_agents,
  active_workspace_path,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_session_busy,
  current_todos,
  session_cost_summary,
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
          agents={agents}
          conversations={current_room_conversations}
          current_agent_id={current_agent_id}
          current_conversation_id={current_conversation_id}
          on_create_agent={on_create_agent}
          on_create_conversation={on_create_conversation}
          on_delete_conversation={on_delete_conversation}
          on_open_directory={on_open_directory}
          on_open_workspace_file={on_open_workspace_file}
          on_select_agent={on_select_agent}
          on_select_conversation={on_select_conversation}
          recent_agents={recent_agents}
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
              on_conversation_snapshot_change={(snapshot) =>
                on_conversation_snapshot_change({
                  conversation_id: snapshot.session_key,
                  message_count: snapshot.message_count,
                  last_activity_at: snapshot.last_activity_at,
                  session_id: snapshot.session_id,
                })
              }
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
            active_session={current_conversation}
            agent={current_agent}
            agent_cost_summary={agent_cost_summary}
            is_session_busy={is_session_busy}
            on_edit_agent={on_edit_agent}
            session_cost_summary={session_cost_summary}
            sessions={current_room_conversations}
            todos={current_todos}
          />
        </div>
      ) : null}
    </div>
  );
}
