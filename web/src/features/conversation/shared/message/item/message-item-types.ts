/**
 * =====================================================
 * @File   ：message-item-types.ts
 * @Date   ：2026-04-16 15:54
 * @Author ：leemysw
 * 2026-04-16 15:54   Create
 * =====================================================
 */

import type { CSSProperties, ReactNode } from "react";

import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type {
  AssistantMessage,
  ContentBlock,
  Message,
} from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import type { ContentProjection } from "./message-item-support";
import type { MessageActivityState } from "../ui/message-primitives";

export interface MessageItemProps {
  compact?: boolean;
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  current_user_avatar?: string | null;
  round_id: string;
  messages: Message[];
  is_last_round?: boolean;
  is_loading?: boolean;
  runtime_phase?: AgentConversationRuntimePhase | null;
  pending_permissions?: PendingPermission[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  hidden_tool_names?: string[];
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
  on_stop_message?: (msg_id: string) => void;
  default_process_expanded?: boolean;
  assistant_header_action?: ReactNode;
  assistant_content_mode?: "dm_live" | "dm_archived" | "room_thread" | "room_result";
  class_name?: string;
}

export interface MessageStatsData {
  duration: string | null;
  tokens: string | null;
  cost: string | null;
  cache_hit: string | null;
}

export interface MessageItemState {
  copied_user: boolean;
  copied_assistant: boolean;
  user_message: Message | undefined;
  user_content: string;
  model: string | undefined;
  timestamp: number | undefined;
  stream_status: AssistantMessage["stream_status"] | null;
  stats: MessageStatsData | null;
  matched_pending_permissions_by_tool_use_id: ReadonlyMap<string, PendingPermission>;
  unmatched_pending_permissions: PendingPermission[];
  direct_ordered_projection: ContentProjection;
  process_projection: ContentProjection;
  final_assistant_content: string | ContentBlock[] | null;
  final_assistant_streaming_indexes: Set<number>;
  final_assistant_text: string;
  should_render_direct_assistant_content: boolean;
  should_render_process_callchain: boolean;
  should_render_assistant_text: boolean;
  should_render_standalone_activity_status: boolean;
  should_show_assistant_footer: boolean;
  show_cursor: boolean;
  final_assistant_is_streaming: boolean;
  should_hide_assistant_content: boolean;
  process_summary: string;
  live_activity_state: MessageActivityState | null;
  is_process_expanded: boolean;
  toggle_process_expanded: () => void;
  process_anchor_ref: React.RefObject<HTMLElement | null>;
  can_copy_assistant: boolean;
  can_stop_message: boolean;
  handle_copy_user: () => Promise<void>;
  handle_copy_assistant: () => Promise<void>;
  handle_stop_message: () => void;
  content_area_ref: React.RefObject<HTMLDivElement | null>;
  content_area_style: CSSProperties | undefined;
  merged_content_length: number;
}
