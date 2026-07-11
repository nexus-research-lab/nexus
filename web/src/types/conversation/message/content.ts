/**
 * Assistant 结构化内容块契约。
 */

import type { ToolInput } from "../../system/sdk";
import type { MessageAttachmentScope } from "./attachment";

export interface TextContent {
  type: "text";
  text: string;
}

interface ToolUseErrorContent {
  type: "tool_use_error";
  content: string;
}

export interface ToolUseContent {
  type: "tool_use";
  id: string;
  name: string;
  input: ToolInput;
}

export interface ToolResultContent {
  type: "tool_result";
  tool_use_id: string;
  content: string | unknown[];
  is_error?: boolean;
  error_code?: string | null;
}

export interface ThinkingContent {
  type: "thinking";
  thinking: string;
  signature?: string | null;
}

export interface ImageContent {
  type: "image";
  data?: string;
  mime_type?: string | null;
  alt?: string | null;
  path?: string | null;
  url?: string | null;
  uri?: string | null;
  source?: {
    type?: string;
    data?: string;
    media_type?: string;
    mime_type?: string;
    url?: string;
    uri?: string;
    path?: string;
  } | null;
}

export interface TaskProgressContent {
  type: "task_progress";
  task_id: string;
  description: string;
  tool_use_id?: string | null;
  last_tool_name?: string | null;
  usage?: Record<string, unknown>;
}

export interface WorkspaceFileArtifactContent {
  type: "workspace_file_artifact";
  id?: string;
  path: string;
  display_path?: string | null;
  label?: string | null;
  title?: string | null;
  artifact_kind?: string | null;
  mime_type?: string | null;
  operation?: string | null;
  scope?: MessageAttachmentScope;
  workspace_agent_id?: string | null;
  source_tool_use_id?: string | null;
  source_tool_name?: string | null;
}

type SystemEventTone = "neutral" | "warning";
export type SystemEventIcon = "retry" | "progress" | "status" | "guide";

export interface SystemEventContent {
  type: "system_event";
  content: string;
  label: string;
  tone: SystemEventTone;
  icon: SystemEventIcon;
  source_message_id: string;
  timestamp: number;
  subtype?: string;
  tool_use_id?: string | null;
  attempt?: number;
  max_retries?: number;
  retry_delay_ms?: number;
  error_status?: string | number | null;
  error?: string | null;
}

export type ContentBlock =
  | TextContent
  | ToolUseErrorContent
  | ToolUseContent
  | ToolResultContent
  | ThinkingContent
  | ImageContent
  | TaskProgressContent
  | WorkspaceFileArtifactContent
  | SystemEventContent;
