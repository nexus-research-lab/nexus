/**
 * INPUT: Room Agent 执行消息、权限、终态摘要、状态与稳定身份。
 * OUTPUT: 共享活动语义、无消息终态 Assistant 投影，以及终态证据判定。
 * POS: 稳定 execution shell 的纯模型；只投影已有执行证据，不复制运行状态机或正文规则。
 */
import {
  hasVisibleAssistantOutput,
  stripRoomControlMarkers,
} from "@/features/conversation/shared/message/message-content-model";
import { CONVERSATION_TASK_TOOL_NAMES } from "@/features/conversation/shared/todos/task-tool-names";
import {
  type MessageActivityState,
  resolvePermissionActivityState,
  resolveToolActivityState,
} from "@/features/conversation/shared/message/item/activity/message-activity-state";
import type {
  AssistantMessage,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type {
  TaskProgressContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import type { AgentRoundStatus } from "../../round/round-agent-model";
import { isAgentRoundActive } from "../../round/round-agent-model";

interface RoomAgentTerminalLabels {
  failed: string;
  stopped: string;
}

interface ProjectRoomAgentExecutionMessagesOptions {
  agentId: string;
  labels: RoomAgentTerminalLabels;
  messages: AssistantMessage[];
  resultSummary?: ResultSummary;
  roundId: string;
  status: AgentRoundStatus;
  timestamp: number;
}

interface ProjectRoomAgentActivityStateOptions {
  messages: AssistantMessage[];
  pendingPermissions: PendingPermission[];
  status: AgentRoundStatus;
}

const ROOM_HIDDEN_TOOL_NAMES = new Set<string>(
  CONVERSATION_TASK_TOOL_NAMES,
);

/** 判断 Thread inspector 是否存在主 Feed 最终回复之外的内容。 */
export function hasRoomAgentExecutionDetails(
  messages: AssistantMessage[],
): boolean {
  const finalMessageIndex = messages.length - 1;
  return messages.some((message, messageIndex) => message.content.some(
    (block) => {
      if (block.type === "text") {
        return messageIndex < finalMessageIndex
          && Boolean(stripRoomControlMarkers(block.text));
      }
      if (block.type === "thinking") {
        return Boolean(stripRoomControlMarkers(block.thinking));
      }
      if (block.type === "progress_update") {
        return false;
      }
      return true;
    },
  ));
}

/**
 * 公区只把 canonical execution/message/permission 证据翻译成共享活动词汇；
 * slot 身份、生命周期和终态仍由 RoomAgentExecutionState / round entry 决定。
 */
export function projectRoomAgentActivityState({
  messages,
  pendingPermissions,
  status,
}: ProjectRoomAgentActivityStateOptions): MessageActivityState | null {
  if (!isAgentRoundActive(status)) {
    return null;
  }
  const permissionActivity = resolvePermissionActivityState(
    pendingPermissions,
  );
  if (permissionActivity) {
    return permissionActivity;
  }

  const resolvedToolUseIds = collectResolvedToolUseIds(messages);
  const liveMessageActivity = resolveLiveMessageActivity(
    messages.at(-1),
    resolvedToolUseIds,
  );
  if (liveMessageActivity) {
    return liveMessageActivity;
  }
  const pendingToolUse = findLatestPendingToolUse(
    messages,
    resolvedToolUseIds,
  );
  return pendingToolUse
    ? resolveToolActivityState(pendingToolUse.name)
    : "thinking";
}

export function hasRoomAgentTerminalEvidence(
  messages: AssistantMessage[],
  resultSummary: ResultSummary | undefined,
  status: AgentRoundStatus,
): boolean {
  return (
    status === "cancelled"
    || status === "error"
    || Boolean(resultSummary)
    || messages.some(isTerminalAssistantMessage)
  );
}

export function isRoomAgentNoPublicReply(
  messages: AssistantMessage[],
  resultSummary: ResultSummary | undefined,
  status: AgentRoundStatus,
): boolean {
  if (
    status !== "done"
    || resultSummary?.is_error
    || (
      resultSummary
      && resultSummary.subtype !== "success"
    )
    || stripRoomControlMarkers(resultSummary?.result ?? "")
  ) {
    return false;
  }
  return !messages.some((message) => hasVisibleAssistantOutput(
    message,
    ROOM_HIDDEN_TOOL_NAMES,
  ));
}

function collectResolvedToolUseIds(
  messages: AssistantMessage[],
): Set<string> {
  const resolved = new Set<string>();
  for (const message of messages) {
    for (const block of message.content) {
      if (block.type === "tool_result" && block.tool_use_id) {
        resolved.add(block.tool_use_id);
      }
    }
  }
  return resolved;
}

function resolveLiveMessageActivity(
  message: AssistantMessage | undefined,
  resolvedToolUseIds: ReadonlySet<string>,
): MessageActivityState | null {
  if (
    !message
    || message.stream_status !== "streaming"
    || message.is_complete
    || message.stop_reason
  ) {
    return null;
  }
  for (let index = message.content.length - 1; index >= 0; index -= 1) {
    const block = message.content[index];
    if (block.type === "tool_result") {
      // 叶子工具已经收口；execution 仍 active 时属于 Agent 的下一步推理，
      // 不能回退到工具之前的旧“正在回复”。
      return "thinking";
    }
    if (
      block.type === "tool_use"
      && !resolvedToolUseIds.has(block.id)
    ) {
      return resolveToolActivityState(block.name);
    }
    if (block.type === "task_progress") {
      return resolveProgressActivityState(block);
    }
    if (block.type === "progress_update" && block.text.trim()) {
      // 文案由共享 MessageItem 投影到这一行；这里保留活动图标与布局身份。
      return "thinking";
    }
    if (block.type === "thinking" && block.thinking.trim()) {
      return "thinking";
    }
    if (block.type === "text" && block.text.trim()) {
      return "replying";
    }
    if (block.type === "workspace_file_artifact") {
      return "executing";
    }
  }
  return "thinking";
}

function findLatestPendingToolUse(
  messages: AssistantMessage[],
  resolvedToolUseIds: ReadonlySet<string>,
): ToolUseContent | null {
  for (let messageIndex = messages.length - 1; messageIndex >= 0; messageIndex -= 1) {
    const content = messages[messageIndex].content;
    for (let blockIndex = content.length - 1; blockIndex >= 0; blockIndex -= 1) {
      const block = content[blockIndex];
      if (
        block.type === "tool_use"
        && !resolvedToolUseIds.has(block.id)
      ) {
        return block;
      }
    }
  }
  return null;
}

function resolveProgressActivityState(
  block: TaskProgressContent,
): MessageActivityState {
  return resolveToolActivityState(block.last_tool_name);
}

export function projectRoomAgentExecutionMessages({
  agentId,
  labels,
  messages,
  resultSummary,
  roundId,
  status,
  timestamp,
}: ProjectRoomAgentExecutionMessagesOptions): AssistantMessage[] {
  if (messages.length > 0) {
    return messages;
  }

  const terminalStatus = resolveTerminalStatus(status, resultSummary);
  if (!terminalStatus) {
    return messages;
  }

  const fallbackText = terminalStatus === "cancelled"
    ? labels.stopped
    : terminalStatus === "error"
      ? labels.failed
      : "";
  const normalizedSummary = normalizeResultSummary(
    resultSummary,
    fallbackText,
  );
  if (!normalizedSummary && !fallbackText) {
    return messages;
  }

  return [{
    agent_id: agentId,
    content: normalizedSummary || !fallbackText
      ? []
      : [{ type: "text", text: fallbackText }],
    is_complete: true,
    is_synthetic: true,
    message_id:
      normalizedSummary?.message_id
      ?? `${roundId}:terminal-projection`,
    ...(normalizedSummary ? { result_summary: normalizedSummary } : {}),
    role: "assistant",
    round_id: roundId,
    session_key: `room:projection:${roundId}`,
    stream_status: terminalStatus,
    timestamp: normalizedSummary?.timestamp ?? timestamp,
  }];
}

function isTerminalAssistantMessage(message: AssistantMessage): boolean {
  return (
    message.stream_status === "done"
    || message.stream_status === "cancelled"
    || message.stream_status === "error"
    || message.is_complete === true
    || Boolean(message.stop_reason)
    || Boolean(message.result_summary)
  );
}

function normalizeResultSummary(
  resultSummary: ResultSummary | undefined,
  fallbackText: string,
): ResultSummary | undefined {
  if (!resultSummary || !fallbackText) {
    return resultSummary;
  }
  if (stripRoomControlMarkers(resultSummary.result ?? "").trim()) {
    return resultSummary;
  }
  return {
    ...resultSummary,
    result: fallbackText,
  };
}

function resolveTerminalStatus(
  status: AgentRoundStatus,
  resultSummary?: ResultSummary,
): "cancelled" | "done" | "error" | null {
  if (status === "cancelled" || status === "done" || status === "error") {
    return status;
  }
  if (!resultSummary) {
    return null;
  }
  if (resultSummary.is_error || resultSummary.subtype === "error") {
    return "error";
  }
  return resultSummary.subtype === "interrupted" ? "cancelled" : "done";
}
