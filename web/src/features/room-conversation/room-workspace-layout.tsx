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
          activeWorkspacePath={active_workspace_path}
          agent={current_agent}
          agents={agents}
          currentAgentId={current_agent_id}
          currentSessionKey={current_conversation_id}
          onCreateAgent={on_create_agent}
          onCreateSession={on_create_conversation}
          onDeleteSession={on_delete_conversation}
          onOpenDirectory={on_open_directory}
          onOpenWorkspaceFile={on_open_workspace_file}
          onSelectAgent={on_select_agent}
          onSelectSession={on_select_conversation}
          recentAgents={recent_agents}
          sessions={current_room_conversations}
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
          agentId={current_agent.agent_id}
          className="hidden lg:flex"
          isOpen={is_editor_open}
          onClose={on_close_workspace_pane}
          onResizeStart={on_start_editor_resize}
          path={active_workspace_path}
          widthPercent={editor_width_percent}
        />

        <div className={cn(HOME_CHAT_PANEL_CLASS, !is_editor_open && "min-[1280px]:border-r min-[1280px]:workspace-divider")}>
          <div className="min-h-0 min-w-0 flex-1">
            <RoomChatPanel
              agentId={current_agent.agent_id}
              currentAgentName={current_agent.name}
              onLoadingChange={on_loading_change}
              onNewSession={on_create_conversation}
              onOpenWorkspaceFile={on_open_workspace_file}
              onSessionSnapshotChange={(snapshot) =>
                on_conversation_snapshot_change({
                  conversation_id: snapshot.sessionKey,
                  message_count: snapshot.messageCount,
                  last_activity_at: snapshot.lastActivityAt,
                  session_id: snapshot.sessionId,
                })
              }
              onTodosChange={on_todos_change}
              sessionKey={current_conversation?.session_key ?? null}
              sessionTitle={current_conversation?.title ?? null}
            />
          </div>
        </div>
      </section>

      {!is_editor_open ? (
        <div className={HOME_AGENT_INSPECTOR_WRAPPER_CLASS}>
          <RoomContextPanel
            activeSession={current_conversation}
            agent={current_agent}
            agentCostSummary={agent_cost_summary}
            isSessionBusy={is_session_busy}
            onEditAgent={on_edit_agent}
            sessionCostSummary={session_cost_summary}
            sessions={current_room_conversations}
            todos={current_todos}
          />
        </div>
      ) : null}
    </div>
  );
}
