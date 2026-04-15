"use client";

import { useCallback, useEffect, useState } from "react";

import { useMediaQuery } from "@/hooks/use-media-query";
import { Agent } from "@/types/agent";
import { AgentConversationIdentity } from "@/types/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { UpdateRoomParams } from "@/types/room";

import { RoomMobileWorkspace } from "./room-mobile-workspace";
import { RoomWorkspaceLayout } from "./room-workspace-layout";

interface RoomWorkspaceShellProps {
  current_agent: Agent;
  current_room_type: string;
  room_id: string | null;
  room_avatar?: string | null;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  current_room_conversation: RoomConversationView | null;
  current_agent_session_identity: AgentConversationIdentity | null;
  conversation_id: string | null;
  current_room_conversations: RoomConversationView[];
  active_workspace_path: string | null;
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  workspace_split_ref: React.RefObject<HTMLElement | null>;
  on_back_to_directory: () => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_update_conversation_title: (conversation_id: string, title: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_close_workspace_pane: () => void;
  on_start_editor_resize: () => void;
  on_loading_change: (is_loading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent-conversation").RoomEventPayload) => void;
}

export function RoomWorkspaceShell({
  current_agent,
  current_room_type,
  room_id,
  room_avatar,
  room_members,
  available_room_agents,
  current_room_title,
  current_room_conversation,
  current_agent_session_identity,
  conversation_id,
  current_room_conversations,
  active_workspace_path,
  initial_draft,
  on_initial_draft_consumed,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_conversation_busy,
  current_todos,
  workspace_split_ref,
  on_back_to_directory,
  on_create_conversation,
  on_select_conversation,
  on_delete_conversation,
  on_add_room_member,
  on_remove_room_member,
  on_update_room,
  on_update_conversation_title,
  on_open_workspace_file,
  on_close_workspace_pane,
  on_start_editor_resize,
  on_loading_change,
  on_todos_change,
  on_conversation_snapshot_change,
  on_room_event,
}: RoomWorkspaceShellProps) {
  const is_mobile = useMediaQuery("(max-width: 767px)");
  const [active_surface_tab, set_active_surface_tab] = useState<RoomSurfaceTabKey>("chat");

  useEffect(() => {
    if (current_room_type !== "dm" && active_surface_tab === "about") {
      set_active_surface_tab("chat");
    }
  }, [active_surface_tab, current_room_type]);

  const handle_select_conversation_in_shell = useCallback((conversation_id: string) => {
    set_active_surface_tab("chat");
    on_select_conversation(conversation_id);
  }, [on_select_conversation]);

  const handle_change_surface_tab = useCallback((next_tab: RoomSurfaceTabKey) => {
    set_active_surface_tab((current_tab) => {
      if (next_tab === "chat") {
        return "chat";
      }
      return current_tab === next_tab ? "chat" : next_tab;
    });
  }, []);

  const handle_create_conversation_in_shell = useCallback(async (title?: string) => {
    const next_conversation_id = await on_create_conversation(title);
    set_active_surface_tab("chat");
    return next_conversation_id;
  }, [on_create_conversation]);

  const handle_open_workspace_file_in_shell = useCallback((path: string | null) => {
    on_open_workspace_file(path);
    if (path) {
      set_active_surface_tab("workspace");
    }
  }, [on_open_workspace_file]);

  if (is_mobile) {
    return (
      <RoomMobileWorkspace
        current_agent={current_agent}
        current_room_type={current_room_type}
        room_id={room_id}
        room_members={room_members}
        current_room_conversation={current_room_conversation}
        current_agent_session_identity={current_agent_session_identity}
        conversation_id={conversation_id}
        current_room_conversations={current_room_conversations}
        current_room_title={current_room_title}
        initial_draft={initial_draft}
        on_initial_draft_consumed={on_initial_draft_consumed}
        on_back_to_directory={on_back_to_directory}
        on_conversation_snapshot_change={on_conversation_snapshot_change}
        on_create_conversation={handle_create_conversation_in_shell}
        on_loading_change={on_loading_change}
        on_room_event={on_room_event}
        on_select_conversation={handle_select_conversation_in_shell}
      />
    );
  }

  return (
    <RoomWorkspaceLayout
      active_workspace_path={active_workspace_path}
      active_surface_tab={active_surface_tab}
      available_room_agents={available_room_agents}
      current_agent={current_agent}
      current_room_type={current_room_type}
      room_id={room_id}
      room_avatar={room_avatar}
      room_members={room_members}
      current_room_title={current_room_title}
      current_agent_session_identity={current_agent_session_identity}
      conversation_id={conversation_id}
      current_room_conversations={current_room_conversations}
      initial_draft={initial_draft}
      on_initial_draft_consumed={on_initial_draft_consumed}
      current_todos={current_todos}
      editor_width_percent={editor_width_percent}
      is_editor_open={is_editor_open}
      is_resizing_editor={is_resizing_editor}
      is_conversation_busy={is_conversation_busy}
      on_add_room_member={on_add_room_member}
      on_remove_room_member={on_remove_room_member}
      on_change_surface_tab={handle_change_surface_tab}
      on_close_workspace_pane={on_close_workspace_pane}
      on_conversation_snapshot_change={on_conversation_snapshot_change}
      on_create_conversation={handle_create_conversation_in_shell}
      on_delete_conversation={on_delete_conversation}
      on_loading_change={on_loading_change}
      on_open_workspace_file={handle_open_workspace_file_in_shell}
      on_update_room={on_update_room}
      on_update_conversation_title={on_update_conversation_title}
      on_select_conversation={handle_select_conversation_in_shell}
      on_start_editor_resize={on_start_editor_resize}
      on_todos_change={on_todos_change}
      workspace_split_ref={workspace_split_ref}
      on_room_event={on_room_event}
    />
  );
}
