import { ReactNode, RefObject } from "react";

import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";
import { Session } from "@/types/session";
import { TodoItem } from "@/types/todo";

export interface RoomChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  session_key: string | null;
  session_title?: string | null;
  on_create_conversation: () => void;
  layout?: "desktop" | "mobile";
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: ConversationSnapshotPayload) => void;
}

export interface RoomMobileWorkspaceProps {
  current_agent: Agent;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  on_back_to_directory: () => void;
  on_create_conversation: () => void;
  on_select_conversation: (conversation_id: string) => void;
  on_loading_change: (is_loading: boolean) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
}

export interface RoomWorkspaceShellProps {
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
  is_session_busy: boolean;
  current_todos: TodoItem[];
  session_cost_summary: SessionCostSummary;
  agent_cost_summary: AgentCostSummary;
  workspace_split_ref: RefObject<HTMLElement | null>;
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

export interface RoomSidebarPanelProps {
  agents: Agent[];
  agent: Agent;
  current_agent_id: string | null;
  recent_agents: Agent[];
  conversations: Conversation[];
  current_conversation_id: string | null;
  active_workspace_path: string | null;
  on_select_agent: (agent_id: string) => void;
  on_open_directory: () => void;
  on_create_agent: () => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation: () => void;
  on_delete_conversation: (conversation_id: string) => void;
  on_open_workspace_file: (path: string | null) => void;
}

export interface RoomContextPanelProps {
  agent: Agent;
  sessions: Session[];
  active_session: Session | null;
  todos: TodoItem[];
  is_session_busy: boolean;
  session_cost_summary: SessionCostSummary;
  agent_cost_summary: AgentCostSummary;
  on_edit_agent: (agent_id: string) => void;
}

export interface RoomEditorPanelProps {
  agent_id: string;
  path: string | null;
  is_open: boolean;
  width_percent: number;
  embedded?: boolean;
  class_name?: string;
  on_close: () => void;
  on_resize_start: () => void;
}

export interface FileTreeNode {
  entry: WorkspaceFileEntry;
  children: FileTreeNode[];
}

export interface RoomMemberSummaryCardProps {
  agent: Agent;
  model_name: string;
  runtime_status: "Running" | "Idle" | "Active";
  localized_runtime_status: string;
  on_edit_agent: (agent_id: string) => void;
}

export interface RoomWorkspaceContextSectionProps {
  allowed_tool_count: number;
  localized_agent_skill: string;
  permission_mode: string;
  served_room_count: number;
}

export interface RoomMembersSectionProps {
  current_agent_id: string | null;
  members: Agent[];
  on_create_agent: () => void;
  on_select_agent: (agent_id: string) => void;
}

export interface RoomSidebarHeaderProps {
  active_room_title: string;
  current_agent_name: string;
  is_refreshing: boolean;
  on_open_directory: () => void;
  on_refresh: () => void;
}

export interface RoomConversationsSectionProps {
  conversations: Conversation[];
  current_conversation_id: string | null;
  on_create_conversation: () => void;
  on_delete_conversation: (conversation_id: string) => void;
  on_select_conversation: (conversation_id: string) => void;
}

export interface RoomContextSectionProps {
  contextualFiles: WorkspaceFileEntry[];
  current_conversation: Conversation | null;
  file_explorer_content: ReactNode;
  filesystem_error: string | null;
  is_file_explorer_visible: boolean;
  memory_file_count: number;
  on_create_directory: () => void;
  on_create_file: () => void;
  on_open_workspace_file: (path: string) => void;
  on_toggle_file_explorer: () => void;
  total_conversation_count: number;
  active_workspace_path: string | null;
}

export interface RoomProgressSectionProps {
  todos: TodoItem[];
}

export interface RoomCollaborationStatusSectionProps {
  active_conversation: Conversation | null;
  localized_agent_skill: string;
  localized_runtime_status: string;
  model_name: string;
  total_room_count: number;
}

export interface RoomUsageSectionProps {
  agent_cost_summary: AgentCostSummary;
  session_cost_summary: SessionCostSummary;
}
