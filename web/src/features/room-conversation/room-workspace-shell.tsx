"use client";

import { useMediaQuery } from "@/hooks/use-media-query";
import { HOME_WORKSPACE_SECTION_GAP_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { RoomWorkspaceShellProps } from "@/types/room";

import { RoomMobileWorkspace } from "./room-mobile-workspace";
import { RoomWorkspaceLayout } from "./room-workspace-layout";

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
  is_session_busy,
  current_todos,
  session_cost_summary,
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
          is_session_busy={is_session_busy}
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
          session_cost_summary={session_cost_summary}
          workspace_split_ref={workspace_split_ref}
        />
      </div>
    </section>
  );
}
