"use client";

import { useMediaQuery } from "@/hooks/use-media-query";
import { HOME_WORKSPACE_SECTION_GAP_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { AgentCostSummary, ConversationCostSummary } from "@/types/cost";
import { TodoItem } from "@/types/todo";

import { RoomMobileWorkspace } from "./room-mobile-workspace";
import { RoomWorkspaceLayout } from "./room-workspace-layout";

interface RoomWorkspaceShellProps {
  agents: Agent[];
  current_agent: Agent;
  current_agent_id: string | null;
  recent_agents: Agent[];
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
  workspace_split_ref: React.RefObject<HTMLElement | null>;
  on_select_agent: (agent_id: string) => void;
  on_open_create_agent: () => void;
  on_back_to_directory: () => void;
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

export function RoomWorkspaceShell({
  agents,
  current_agent,
  current_agent_id,
  recent_agents,
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
  on_open_create_agent,
  on_back_to_directory,
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
}: RoomWorkspaceShellProps) {
  const is_mobile = useMediaQuery("(max-width: 767px)");

  if (is_mobile) {
    return (
      <RoomMobileWorkspace
        current_agent={current_agent}
        current_conversation={current_conversation}
        current_conversation_id={current_conversation_id}
        current_room_conversations={current_room_conversations}
        on_back_to_directory={on_back_to_directory}
        on_conversation_snapshot_change={on_conversation_snapshot_change}
        on_create_conversation={on_create_conversation}
        on_loading_change={on_loading_change}
        on_select_conversation={on_select_conversation}
      />
    );
  }

  return (
    <section className={cn("flex min-h-0 flex-1 flex-col", HOME_WORKSPACE_SECTION_GAP_CLASS)}>
      <div className="workspace-shell relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[30px]">
        <RoomWorkspaceLayout
          active_workspace_path={active_workspace_path}
          agent_cost_summary={agent_cost_summary}
          agents={agents}
          current_agent={current_agent}
          current_agent_id={current_agent_id}
          current_conversation={current_conversation}
          current_conversation_id={current_conversation_id}
          current_room_conversations={current_room_conversations}
          current_todos={current_todos}
          editor_width_percent={editor_width_percent}
          is_editor_open={is_editor_open}
          is_resizing_editor={is_resizing_editor}
          is_conversation_busy={is_conversation_busy}
          on_close_workspace_pane={on_close_workspace_pane}
          on_conversation_snapshot_change={on_conversation_snapshot_change}
          on_create_agent={on_open_create_agent}
          on_create_conversation={on_create_conversation}
          on_delete_conversation={on_delete_conversation}
          on_edit_agent={on_edit_agent}
          on_loading_change={on_loading_change}
          on_open_directory={on_back_to_directory}
          on_open_workspace_file={on_open_workspace_file}
          on_select_agent={on_select_agent}
          on_select_conversation={on_select_conversation}
          on_start_editor_resize={on_start_editor_resize}
          on_todos_change={on_todos_change}
          recent_agents={recent_agents}
          conversation_cost_summary={conversation_cost_summary}
          workspace_split_ref={workspace_split_ref}
        />
      </div>
    </section>
  );
}
