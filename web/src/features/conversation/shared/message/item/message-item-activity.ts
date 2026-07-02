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
  pending_permission_count: pendingPermissionCount,
  process_content: processContent,
}: {
  pending_permission_count: number;
  process_content: ContentBlock[];
}): string {
  let toolCount = 0;
  let thinkingCount = 0;
  let errorCount = 0;
  let guidanceCount = 0;

  for (const block of processContent) {
    if (block.type === "thinking") {
      thinkingCount += 1;
      continue;
    }
    if (block.type === "tool_use") {
      toolCount += 1;
      continue;
    }
    if (block.type === "tool_result" && block.is_error) {
      errorCount += 1;
      continue;
    }
    if (
      block.type === "system_event" &&
      block.subtype === "guided_input"
    ) {
      guidanceCount += 1;
    }
  }

  if (pendingPermissionCount > 0) {
    return "等待你的确认后继续";
  }

  const summaryParts: string[] = [];
  if (thinkingCount > 0) {
    summaryParts.push(`${thinkingCount} 段思路`);
  }
  if (toolCount > 0) {
    summaryParts.push(`${toolCount} 次动作`);
  }
  if (errorCount > 0) {
    summaryParts.push(`${errorCount} 个异常`);
  }
  if (guidanceCount > 0) {
    summaryParts.push(`${guidanceCount} 次引导`);
  }

  const summary = summaryParts.length > 0 ? summaryParts.join(" · ") : "查看过程";
  const latestDetail = latestProcessDetail(processContent);
  return latestDetail ? `${summary} · 最近：${latestDetail}` : summary;
}

function latestProcessDetail(processContent: ContentBlock[]): string | null {
  for (let index = processContent.length - 1; index >= 0; index -= 1) {
    const block = processContent[index];
    if (block.type === "task_progress") {
      return compactProcessDetail(
        block.description || block.last_tool_name || "后台任务正在执行",
      );
    }
    if (block.type === "tool_use") {
      const detail = get_input_summary(block.input);
      return compactProcessDetail(
        detail ? `${get_tool_title(block.name)}：${detail}` : get_tool_title(block.name),
      );
    }
    if (block.type === "system_event") {
      return compactProcessDetail(block.content || block.label);
    }
    if (block.type === "tool_use_error") {
      return compactProcessDetail(block.content);
    }
  }
  return null;
}

function compactProcessDetail(value: string): string | null {
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
  is_last_round: isLastRound,
  is_loading: isLoading,
  merged_content: mergedContent,
  pending_permissions: pendingPermissions,
  runtime_phase: runtimePhase,
  stream_status: streamStatus,
  streaming_block_indexes: streamingBlockIndexes,
}: {
  is_last_round?: boolean;
  is_loading?: boolean;
  merged_content: ContentBlock[];
  pending_permissions: PendingPermission[];
  runtime_phase?: AgentConversationRuntimePhase | null;
  stream_status?: string | null;
  streaming_block_indexes: ReadonlySet<number>;
}): MessageActivityState | null {
  if (!isLastRound || !isLoading) {
    return null;
  }

  if (pendingPermissions.length > 0) {
    return pendingPermissions.some(
      (permission) =>
        permission.interaction_mode === "question" ||
        permission.tool_name === "AskUserQuestion",
    )
      ? "waiting_input"
      : "waiting_permission";
  }

  const runtimeActivityState =
    map_runtime_phase_to_activity_state(runtimePhase);
  if (runtimeActivityState === "sending") {
    return "sending";
  }

  const latestStreamingBlock = find_latest_streaming_block(
    mergedContent,
    streamingBlockIndexes,
  );
  if (latestStreamingBlock?.type === "thinking") {
    return "thinking";
  }
  if (latestStreamingBlock?.type === "text") {
    return "replying";
  }

  const hasVisibleReplyText = mergedContent.some(
    (block) => block.type === "text" && Boolean(block.text.trim()),
  );
  if (hasVisibleReplyText && streamStatus === "streaming") {
    return "replying";
  }

  if (streamStatus === "pending") {
    return "thinking";
  }

  return runtimeActivityState;
}
