import { RefObject } from "react";

import { ContentBlock, Message, ToolResultContent, ToolUseContent } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload, PermissionRiskLevel, PermissionUpdate } from "@/types/permission";

export interface AttachmentFile {
  id: string;
  file: File;
  preview?: string;
  type: "image" | "document";
}

export interface RoomComposerPanelProps {
  compact: boolean;
  current_agent_name: string | null;
  is_loading: boolean;
  on_send_message: (content: string) => void | Promise<void>;
  on_stop: () => void;
  disabled?: boolean;
  placeholder?: string;
  max_length?: number;
}

export interface RoomConversationHeaderProps {
  current_agent_name: string | null;
  current_conversation_id: string | null;
  current_conversation_title: string | null;
  is_loading: boolean;
}

export interface RoomConversationFeedProps {
  bottom_anchor_ref: RefObject<HTMLDivElement | null>;
  current_agent_name: string | null;
  is_last_round_pending_permission: PendingPermission | null;
  is_loading: boolean;
  is_mobile_layout: boolean;
  message_groups: Map<string, Message[]>;
  on_delete_round: (round_id: string) => Promise<void>;
  on_open_workspace_file?: (path: string) => void;
  on_permission_response: (payload: PermissionDecisionPayload) => void;
  on_regenerate_round?: (round_id: string) => Promise<void>;
  round_ids: string[];
}

export interface RoomConversationEmptyStateProps {
  on_create_conversation: () => void;
}

export interface RoomScrollToLatestButtonProps {
  is_loading: boolean;
  is_mobile_layout: boolean;
  on_click: () => void;
}

export interface MessageStatsData {
  duration: string;
  tokens: string | null;
  cost: string | null;
  cache_hit: string | null;
}

export interface MessageStatsProps {
  stats?: MessageStatsData;
  show_cursor?: boolean;
  copied_assistant?: boolean;
  is_regenerating?: boolean;
  is_deleting?: boolean;
  on_copy_assistant?: () => void;
  on_regenerate?: () => void;
  on_delete?: () => void;
}

export interface MessageItemProps {
  current_agent_name?: string | null;
  round_id: string;
  messages: Message[];
  is_last_round?: boolean;
  is_loading?: boolean;
  pending_permission?: PendingPermission | null;
  on_permission_response?: (payload: PermissionDecisionPayload) => void;
  hidden_tool_names?: string[];
  on_delete?: (round_id: string) => Promise<void>;
  on_regenerate?: (round_id: string) => Promise<void>;
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
  class_name?: string;
}

export interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  on_open_workspace_file?: (path: string) => void;
}

export interface ThinkingBlockProps {
  thinking: string;
  is_streaming?: boolean;
}

export interface CodeBlockProps {
  language: string;
  value: string;
}

export interface ToolPermissionRequest {
  request_id: string;
  tool_input: Record<string, any>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  on_allow: (updated_permissions?: PermissionUpdate[]) => void;
  on_deny: (updated_permissions?: PermissionUpdate[]) => void;
}

export interface ToolBlockProps {
  tool_use: ToolUseContent;
  tool_result?: ToolResultContent;
  status?: "pending" | "running" | "success" | "error" | "waiting_permission";
  start_time?: number;
  end_time?: number;
  permission_request?: ToolPermissionRequest;
}

export interface ContentRendererProps {
  content: string | ContentBlock[];
  is_streaming?: boolean;
  streaming_block_indexes?: Set<number>;
  pending_permission?: PendingPermission | null;
  on_permission_response?: (payload: PermissionDecisionPayload) => void;
  on_open_workspace_file?: (path: string) => void;
  hidden_tool_names?: string[];
}
