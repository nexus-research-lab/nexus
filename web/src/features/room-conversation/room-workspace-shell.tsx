"use client";

import { useEffect, useState } from "react";

import { useMediaQuery } from "@/hooks/use-media-query";
import { HOME_WORKSPACE_SECTION_GAP_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { RoomAggregate } from "@/types/room";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";

import { RoomMobileWorkspace } from "./room-mobile-workspace";
import { RoomWorkspaceLayout } from "./room-workspace-layout";

interface RoomWorkspaceShellProps {
  current_agent: Agent;
  current_agent_id: string | null;
  current_room_type: string;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  current_room_id: string | null;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  rooms: RoomAggregate[];
  active_workspace_path: string | null;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  workspace_split_ref: React.RefObject<HTMLElement | null>;
  on_select_agent: (agent_id: string) => void;
  on_back_to_directory: () => void;
  on_open_contacts: () => void;
  on_open_room: (room_id: string) => void;
  on_edit_agent: (agent_id: string) => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_update_room: (params: {name?: string; description?: string; title?: string}) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_close_workspace_pane: () => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
}

export function RoomWorkspaceShell({
  current_agent,
  current_agent_id,
  current_room_type,
  room_members,
  available_room_agents,
  current_room_title,
  current_room_id,
  current_conversation,
  current_conversation_id,
  current_room_conversations,
  rooms,
  active_workspace_path,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_conversation_busy,
  current_todos,
  workspace_split_ref,
  on_select_agent,
  on_back_to_directory,
  on_open_contacts,
  on_open_room,
  on_edit_agent,
  on_create_conversation,
  on_select_conversation,
  on_delete_conversation,
  on_update_room,
  on_delete_room,
  on_add_room_member,
  on_remove_room_member,
  on_open_workspace_file,
  on_close_workspace_pane,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
}: RoomWorkspaceShellProps) {
  const is_mobile = useMediaQuery("(max-width: 767px)");
  const [active_surface_tab, set_active_surface_tab] = useState<RoomSurfaceTabKey>("chat");

  useEffect(() => {
    if (current_room_type !== "dm" && active_surface_tab === "about") {
      set_active_surface_tab("chat");
    }
  }, [active_surface_tab, current_room_type]);

  if (is_mobile) {
    return (
        <RoomMobileWorkspace
          current_agent={current_agent}
          current_conversation={current_conversation}
          current_conversation_id={current_conversation_id}
          current_room_conversations={current_room_conversations}
          current_room_title={current_room_title}
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
      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
        <RoomWorkspaceLayout
          active_workspace_path={active_workspace_path}
          active_surface_tab={active_surface_tab}
          available_room_agents={available_room_agents}
          current_agent={current_agent}
          current_agent_id={current_agent_id}
          current_room_id={current_room_id}
          current_room_type={current_room_type}
          room_members={room_members}
          current_room_title={current_room_title}
          current_conversation={current_conversation}
          current_conversation_id={current_conversation_id}
          current_room_conversations={current_room_conversations}
          current_todos={current_todos}
          rooms={rooms}
          editor_width_percent={editor_width_percent}
          is_editor_open={is_editor_open}
          is_resizing_editor={is_resizing_editor}
          is_conversation_busy={is_conversation_busy}
          on_add_room_member={on_add_room_member}
          on_change_surface_tab={set_active_surface_tab}
          on_close_workspace_pane={on_close_workspace_pane}
          on_conversation_snapshot_change={on_conversation_snapshot_change}
          on_create_conversation={on_create_conversation}
          on_delete_conversation={on_delete_conversation}
          on_delete_room={on_delete_room}
          on_edit_agent={on_edit_agent}
          on_loading_change={on_loading_change}
          on_open_contacts={on_open_contacts}
          on_open_room={on_open_room}
          on_open_workspace_file={on_open_workspace_file}
          on_remove_room_member={on_remove_room_member}
          on_select_agent={on_select_agent}
          on_select_conversation={on_select_conversation}
          on_start_editor_resize={on_start_editor_resize}
          on_todos_change={on_todos_change}
          on_update_room={on_update_room}
          workspace_split_ref={workspace_split_ref}
        />
      </div>
    </section>
  );
}
