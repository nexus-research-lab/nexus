"use client";

import { RefObject } from "react";

import { RoomContextPanel } from "@/features/room-context/room-context-panel";
import { RoomEditorPanel } from "@/features/room-context/room-editor-panel";
import { RoomObjectListPanel } from "@/features/room-navigation/room-object-list-panel";
import {
  HOME_AGENT_INSPECTOR_WRAPPER_CLASS,
  HOME_CHAT_PANEL_CLASS,
  HOME_WORKSPACE_MAIN_GAP_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { RoomAggregate } from "@/types/room";
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
  current_room_id: string | null;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  rooms: RoomAggregate[];
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

export function RoomWorkspaceLayout({
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
  active_surface_tab,
  is_editor_open,
  editor_width_percent,
  is_resizing_editor,
  is_conversation_busy,
  current_todos,
  workspace_split_ref,
  on_change_surface_tab,
  on_select_agent,
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
}: RoomWorkspaceLayoutProps) {
  const show_detail_panel = !is_editor_open && active_surface_tab === "chat";

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1", HOME_WORKSPACE_MAIN_GAP_CLASS)}>
      <RoomObjectListPanel
        active_space={current_room_type === "dm" ? "dm" : "room"}
        agents={available_room_agents.concat(room_members.filter((member) => (
          !available_room_agents.some((agent) => agent.agent_id === member.agent_id)
        )))}
        conversations={current_room_conversations}
        current_room_id={current_room_id}
        current_room_title={current_room_title}
        on_delete_room={on_delete_room}
        on_open_contacts={on_open_contacts}
        on_open_room={on_open_room}
        on_update_room={on_update_room}
        rooms={rooms}
      />

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
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            <RoomConversationHeader
              active_tab={active_surface_tab}
              current_agent_name={current_agent.name}
              current_conversation_id={current_conversation_id}
              current_conversation_title={current_conversation?.title ?? null}
              current_room_title={current_room_title}
              current_room_type={current_room_type}
              is_loading={is_conversation_busy}
              member_count={room_members.length}
              on_change_tab={on_change_surface_tab}
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
        </div>
      </section>

      {show_detail_panel ? (
        <div className={HOME_AGENT_INSPECTOR_WRAPPER_CLASS}>
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
        </div>
      ) : null}
    </div>
  );
}
