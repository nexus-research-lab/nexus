/**
 * Assistant 结构化内容块与当前历史 generation 大内容引用契约。
 */

import type { ToolInput } from "../../system/sdk";
import type { MessageAttachmentScope } from "./attachment";

interface DeferredMessageDetail {
  detail_ref?: string;
  detail_session_key?: string;
  detail_kind?: "image" | "tool_result";
  detail_size?: number;
  detail_truncated?: boolean;
}

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
  metadata?: Record<string, unknown>;
  source_type?: string;
}

export interface ToolResultContent extends DeferredMessageDetail {
  type: "tool_result";
  tool_use_id: string;
  /** Provider 原生工具结果不保证是字符串或数组，保留其 JSON 形状。 */
  content: unknown;
  is_error?: boolean;
  error_code?: string | null;
  metadata?: Record<string, unknown>;
  structured_output?: unknown;
  source_type?: string;
}

export interface ThinkingContent {
  type: "thinking";
  thinking: string;
  signature?: string | null;
}

export interface RedactedThinkingContent {
  type: "redacted_thinking";
  data?: string;
}

export interface DocumentContent {
  type: "document";
  source?: unknown;
  title?: string | null;
  context?: string | null;
  mime_type?: string | null;
  citations?: unknown;
}

export interface SearchResultContent {
  type: "search_result";
  query?: string | null;
  source?: string | null;
  title?: string | null;
  url?: string | null;
  snippet?: string | null;
  content?: unknown;
}

export interface ResourceLinkContent {
  type: "resource_link";
  name?: string | null;
  uri?: string | null;
  description?: string | null;
}

/** 未识别的 Provider 内容块。保留原始负载，但默认不参与渲染。 */
export interface UnsupportedContent {
  type: "unsupported";
  original_type: string;
  payload: Record<string, unknown>;
}

export interface ImageContent extends DeferredMessageDetail {
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

/** Provider 在执行中给出的当前轮次自然语言旁白；收到即展示且只以 ephemeral 消息存在。 */
export interface ProgressUpdateContent {
  type: "progress_update";
  text: string;
  preceding_tool_use_ids?: string[];
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
  | RedactedThinkingContent
  | ImageContent
  | DocumentContent
  | SearchResultContent
  | ResourceLinkContent
  | TaskProgressContent
  | ProgressUpdateContent
  | WorkspaceFileArtifactContent
  | SystemEventContent
  | UnsupportedContent;
