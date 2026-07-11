import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import { isAskUserQuestionTimedOutResult } from "@/types/conversation/ask-user-question";
import type { ContentBlock } from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";

import {
  getInputSummary,
  getToolTitle,
} from "../../blocks/tool/tool-block-model";
import type { MessageActivityState } from "../../ui/message-primitives";

const PROCESS_SUMMARY_DETAIL_LIMIT = 72;
interface ProcessMetric {
  label: string;
  matches: (block: ContentBlock) => boolean;
}

const PROCESS_METRICS: ProcessMetric[] = [
  { label: "段思路", matches: (block) => block.type === "thinking" },
  { label: "次动作", matches: (block) => block.type === "tool_use" },
  {
    label: "个异常",
    matches: (block) => block.type === "tool_result" && Boolean(block.is_error),
  },
  {
    label: "次引导",
    matches: (block) => block.type === "system_event"
      && block.subtype === "guided_input",
  },
];
const PROCESS_DETAIL_RESOLVERS: Array<
  (block: ContentBlock) => string | null
> = [
  (block) => block.type === "task_progress"
    ? compactProcessDetail(
      block.description || block.last_tool_name || "后台任务正在执行",
    )
    : null,
  (block) => {
    if (block.type !== "tool_use") {
      return null;
    }
    const detail = getInputSummary(block.input);
    return compactProcessDetail(
      detail ? `${getToolTitle(block.name)}：${detail}` : getToolTitle(block.name),
    );
  },
  (block) => block.type === "system_event"
    ? compactProcessDetail(block.content || block.label)
    : null,
  (block) => block.type === "tool_use_error"
    ? compactProcessDetail(block.content)
    : null,
];
const RUNTIME_PHASE_ACTIVITY: Partial<
  Record<AgentConversationRuntimePhase, MessageActivityState>
> = {
  awaiting_permission: "waiting_permission",
  running: "thinking",
  sending: "sending",
  streaming: "replying",
};
const STREAMING_BLOCK_ACTIVITY: Partial<
  Record<ContentBlock["type"], MessageActivityState>
> = {
  text: "replying",
  thinking: "thinking",
};

export function buildProcessSummary({
  pendingPermissionCount,
  processContent,
}: {
  pendingPermissionCount: number;
  processContent: ContentBlock[];
}): string {
  if (pendingPermissionCount > 0) {
    return "等待你的确认后继续";
  }

  const summaryParts = PROCESS_METRICS.flatMap(({ label, matches }) => {
    const count = processContent.filter(matches).length;
    return count > 0 ? [`${count} ${label}`] : [];
  });
  const summary = summaryParts.length > 0 ? summaryParts.join(" · ") : "查看过程";
  const latestDetail = latestProcessDetail(processContent);
  return latestDetail ? `${summary} · 最近：${latestDetail}` : summary;
}

function latestProcessDetail(processContent: ContentBlock[]): string | null {
  for (let index = processContent.length - 1; index >= 0; index -= 1) {
    const block = processContent[index];
    for (const resolveDetail of PROCESS_DETAIL_RESOLVERS) {
      const detail = resolveDetail(block);
      if (detail) {
        return detail;
      }
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

export function resolveLiveActivityState({
  isLastRound,
  isLoading,
  mergedContent,
  pendingPermissions,
  runtimePhase,
  streamStatus,
  streamingBlockIndexes,
}: {
  isLastRound?: boolean;
  isLoading?: boolean;
  mergedContent: ContentBlock[];
  pendingPermissions: PendingPermission[];
  runtimePhase?: AgentConversationRuntimePhase | null;
  streamStatus?: string | null;
  streamingBlockIndexes: ReadonlySet<number>;
}): MessageActivityState | null {
  if (!isLastRound || !isLoading) {
    return null;
  }

  const pendingPermissionActivity = resolvePendingPermissionActivity(
    pendingPermissions,
  );
  if (pendingPermissionActivity) {
    return pendingPermissionActivity;
  }

  const runtimeActivityState =
    mapRuntimePhaseToActivityState(runtimePhase);
  const latestStreamingBlock = findLatestStreamingBlock(
    mergedContent,
    streamingBlockIndexes,
  );
  const hasVisibleReplyText = mergedContent.some(
    (block) => block.type === "text" && Boolean(block.text.trim()),
  );
  const candidates: Array<MessageActivityState | null> = [
    runtimeActivityState === "sending" ? "sending" : null,
    latestStreamingBlock
      ? STREAMING_BLOCK_ACTIVITY[latestStreamingBlock.type] ?? null
      : null,
    hasVisibleReplyText && streamStatus === "streaming" ? "replying" : null,
    streamStatus === "pending" ? "thinking" : null,
    runtimeActivityState,
  ];
  return candidates.find((candidate) => candidate !== null) ?? null;
}

function resolvePendingPermissionActivity(
  permissions: PendingPermission[],
): MessageActivityState | null {
  if (permissions.length === 0) {
    return null;
  }
  const waitingForAnswer = permissions.some(
    (permission) => permission.interaction_mode === "question"
      || permission.tool_name === "AskUserQuestion",
  );
  return waitingForAnswer ? "waiting_input" : "waiting_permission";
}

function mapRuntimePhaseToActivityState(
  phase?: AgentConversationRuntimePhase | null,
): MessageActivityState | null {
  return phase ? RUNTIME_PHASE_ACTIVITY[phase] ?? null : null;
}

function findLatestStreamingBlock(
  content: ContentBlock[],
  streamingBlockIndexes: ReadonlySet<number>,
): ContentBlock | null {
  const indexes = Array.from(streamingBlockIndexes).sort(
    (left, right) => right - left,
  );
  for (const index of indexes) {
    const block = content[index];
    if (!block || isEmptyStreamingBlock(block)) {
      continue;
    }
    return block;
  }
  return null;
}

function isEmptyStreamingBlock(block: ContentBlock): boolean {
  const textByType: Partial<Record<ContentBlock["type"], string>> = {
    text: block.type === "text" ? block.text : "",
    thinking: block.type === "thinking" ? block.thinking : "",
    tool_use_error: block.type === "tool_use_error" ? block.content : "",
  };
  const text = textByType[block.type];
  return typeof text === "string" && !text.trim();
}

export function hasTimedOutAskUserQuestion(content: ContentBlock[]): boolean {
  const questionToolUseIds = new Set<string>();
  for (const block of content) {
    if (block.type === "tool_use" && block.name === "AskUserQuestion") {
      questionToolUseIds.add(block.id);
    }
  }
  return content.some(
    (block) => block.type === "tool_result"
      && questionToolUseIds.has(block.tool_use_id)
      && isAskUserQuestionTimedOutResult(block),
  );
}
