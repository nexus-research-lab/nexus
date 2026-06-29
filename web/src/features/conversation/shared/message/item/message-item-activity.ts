import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type { ContentBlock } from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";

import {
  get_input_summary,
  get_tool_title,
} from "../blocks/tool-block-model";
import type { MessageActivityState } from "../ui/message-primitives";
import {
  find_latest_streaming_block,
  map_runtime_phase_to_activity_state,
} from "./message-item-support";

const PROCESS_SUMMARY_DETAIL_LIMIT = 72;

export function build_process_summary({
  pending_permission_count,
  process_content,
}: {
  pending_permission_count: number;
  process_content: ContentBlock[];
}): string {
  let tool_count = 0;
  let thinking_count = 0;
  let error_count = 0;
  let guidance_count = 0;

  for (const block of process_content) {
    if (block.type === "thinking") {
      thinking_count += 1;
      continue;
    }
    if (block.type === "tool_use") {
      tool_count += 1;
      continue;
    }
    if (block.type === "tool_result" && block.is_error) {
      error_count += 1;
      continue;
    }
    if (
      block.type === "system_event" &&
      block.subtype === "guided_input"
    ) {
      guidance_count += 1;
    }
  }

  if (pending_permission_count > 0) {
    return "等待你的确认后继续";
  }

  const summary_parts: string[] = [];
  if (thinking_count > 0) {
    summary_parts.push(`${thinking_count} 段思路`);
  }
  if (tool_count > 0) {
    summary_parts.push(`${tool_count} 次动作`);
  }
  if (error_count > 0) {
    summary_parts.push(`${error_count} 个异常`);
  }
  if (guidance_count > 0) {
    summary_parts.push(`${guidance_count} 次引导`);
  }

  const summary = summary_parts.length > 0 ? summary_parts.join(" · ") : "查看过程";
  const latest_detail = latest_process_detail(process_content);
  return latest_detail ? `${summary} · 最近：${latest_detail}` : summary;
}

function latest_process_detail(process_content: ContentBlock[]): string | null {
  for (let index = process_content.length - 1; index >= 0; index -= 1) {
    const block = process_content[index];
    if (block.type === "task_progress") {
      return compact_process_detail(
        block.description || block.last_tool_name || "后台任务正在执行",
      );
    }
    if (block.type === "tool_use") {
      const detail = get_input_summary(block.input);
      return compact_process_detail(
        detail ? `${get_tool_title(block.name)}：${detail}` : get_tool_title(block.name),
      );
    }
    if (block.type === "system_event") {
      return compact_process_detail(block.content || block.label);
    }
    if (block.type === "tool_use_error") {
      return compact_process_detail(block.content);
    }
  }
  return null;
}

function compact_process_detail(value: string): string | null {
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) {
    return null;
  }
  if (text.length <= PROCESS_SUMMARY_DETAIL_LIMIT) {
    return text;
  }
  return `${text.slice(0, PROCESS_SUMMARY_DETAIL_LIMIT - 1)}…`;
}

export function resolve_live_activity_state({
  is_last_round,
  is_loading,
  merged_content,
  pending_permissions,
  runtime_phase,
  stream_status,
  streaming_block_indexes,
}: {
  is_last_round?: boolean;
  is_loading?: boolean;
  merged_content: ContentBlock[];
  pending_permissions: PendingPermission[];
  runtime_phase?: AgentConversationRuntimePhase | null;
  stream_status?: string | null;
  streaming_block_indexes: ReadonlySet<number>;
}): MessageActivityState | null {
  if (!is_last_round || !is_loading) {
    return null;
  }

  if (pending_permissions.length > 0) {
    return pending_permissions.some(
      (permission) =>
        permission.interaction_mode === "question" ||
        permission.tool_name === "AskUserQuestion",
    )
      ? "waiting_input"
      : "waiting_permission";
  }

  const runtime_activity_state =
    map_runtime_phase_to_activity_state(runtime_phase);
  if (runtime_activity_state === "sending") {
    return "sending";
  }

  const latest_streaming_block = find_latest_streaming_block(
    merged_content,
    streaming_block_indexes,
  );
  if (latest_streaming_block?.type === "thinking") {
    return "thinking";
  }
  if (latest_streaming_block?.type === "text") {
    return "replying";
  }

  const has_visible_reply_text = merged_content.some(
    (block) => block.type === "text" && Boolean(block.text.trim()),
  );
  if (has_visible_reply_text && stream_status === "streaming") {
    return "replying";
  }

  if (stream_status === "pending") {
    return "thinking";
  }

  return runtime_activity_state;
}
