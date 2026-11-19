// INPUT: Durable user content, control-record metadata, delivery policy and timestamp.
// OUTPUT: Display content, Goal/guidance facts, time and available actions without CSS; runtime self-wake trigger facts.
// POS: User-message semantic projection; reading geometry belongs to message-reading-layout.

import { formatMessageTime } from "../../../message-time";
import type { MessageExternalSource, UserMessage } from "@/types/conversation/message/entity";

export interface UserMessagePresentation {
  displayContent: string;
  externalSource: MessageExternalSource | null;
  goal: boolean;
  guided: boolean;
  hasContent: boolean;
  timestamp: string;
}

export function projectUserMessagePresentation(
  content: string,
  message: UserMessage,
): UserMessagePresentation {
  const goal = message.metadata?.subtype === "goal_set";
  const displayContent = goal
    ? content.replace(/^\s*\/goal(?:\s+|$)/i, "").trim()
    : content;
  return {
    displayContent,
    externalSource: message.external_source?.channel ? message.external_source : null,
    goal,
    guided: message.delivery_policy === "guide",
    hasContent: Boolean(displayContent),
    timestamp: formatMessageTime(message.timestamp),
  };
}

export type RuntimeWakeLabelKey =
  | "message.runtime_wake.task_completed"
  | "message.runtime_wake.task_failed"
  | "message.runtime_wake.task_stopped"
  | "message.runtime_wake.resumed";

export interface RuntimeWakePresentation {
  labelKey: RuntimeWakeLabelKey;
  /** runtime 原文摘要；没有后台任务证据时为空，视图不得补占位。 */
  summary: string;
  timestamp: string;
}

// 后台任务状态到触发文案的完整映射；未知状态统一归入“继续执行”。
const RUNTIME_WAKE_TASK_LABELS: Record<string, RuntimeWakeLabelKey> = {
  completed: "message.runtime_wake.task_completed",
  failed: "message.runtime_wake.task_failed",
  killed: "message.runtime_wake.task_stopped",
  stopped: "message.runtime_wake.task_stopped",
};

/**
 * runtime 自唤醒行（后端 metadata.input_kind=runtime_wake）不是用户输入：
 * 返回触发事实供视图渲染为分隔条；普通用户消息返回 null。
 */
export function projectRuntimeWakePresentation(message: UserMessage): RuntimeWakePresentation | null {
  const metadata = message.metadata;
  if (metadata?.input_kind !== "runtime_wake") {
    return null;
  }
  const backgroundTask = metadata.trigger_kind === "background_task";
  const status = typeof metadata.task_status === "string" ? metadata.task_status : "";
  return {
    labelKey: backgroundTask
      ? RUNTIME_WAKE_TASK_LABELS[status] ?? "message.runtime_wake.task_completed"
      : "message.runtime_wake.resumed",
    summary: backgroundTask ? message.content.trim() : "",
    timestamp: formatMessageTime(message.timestamp),
  };
}

export function projectAvailableUserMessageAction<Action>(
  available: boolean,
  action: Action,
): Action | undefined {
  return available ? action : undefined;
}
