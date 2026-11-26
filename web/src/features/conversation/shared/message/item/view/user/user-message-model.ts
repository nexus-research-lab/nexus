// INPUT: Durable user content, control-record metadata, delivery policy and timestamp.
// OUTPUT: Display content, Goal/guidance facts, time and available actions without CSS; runtime self-wake and
// host-event trigger facts, including the host run outcome folded into the divider.
// POS: User-message semantic projection; reading geometry belongs to message-reading-layout.

import { formatMessageTime } from "../../../message-time";
import type { Message, MessageExternalSource, UserMessage } from "@/types/conversation/message/entity";
import { HOST_RUN_OUTCOME_SUBTYPE } from "../../controller/projection/message-item-system-events";

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
  | "message.scheduled_task.triggered"
  | "message.follow_up.triggered"
  | "message.runtime_wake.task_completed"
  | "message.runtime_wake.task_failed"
  | "message.runtime_wake.task_stopped"
  | "message.runtime_wake.resumed";

export type HostRunOutcomeLabelKey =
  | "message.host_run.quiet"
  | "message.host_run.failed"
  | "message.host_run.stopped";

// 宿主 run 没有回复时的结局（system 行 metadata.outcome）到分隔条后缀的完整映射；有回复时回复本身就是记录。
const HOST_RUN_OUTCOME_LABELS: Record<string, HostRunOutcomeLabelKey | undefined> = {
  failed: "message.host_run.failed",
  quiet: "message.host_run.quiet",
  stopped: "message.host_run.stopped",
};

/** 从一轮消息里读出宿主 run 的结局；它只并进触发分隔条，不作为系统事件单独渲染。 */
export function projectHostRunOutcome(messages: readonly Message[]): HostRunOutcomeLabelKey | null {
  for (const message of messages) {
    if (message.role === "system" && message.metadata?.subtype === HOST_RUN_OUTCOME_SUBTYPE) {
      return HOST_RUN_OUTCOME_LABELS[String(message.metadata.outcome ?? "")] ?? null;
    }
  }
  return null;
}

export interface RuntimeWakePresentation {
  /** 触发来源决定图标；文案由 labelKey 单独表达。 */
  source: RuntimeWakeSource;
  labelKey: RuntimeWakeLabelKey;
  /** 宿主 run 没有回复时的结局后缀（无需提醒 / 未完成 / 已停止）；运行中或有回复时为空。 */
  outcomeKey: HostRunOutcomeLabelKey | null;
  /** runtime 原文摘要、定时任务名称或跟进备注；没有证据时为空，视图不得补占位。 */
  summary: string;
  timestamp: string;
}

export type RuntimeWakeSource = "runtime_wake" | "scheduled_task" | "follow_up";

// 主窗口宿主事件 input_kind 到分隔条的完整映射：来源图标、文案与摘要所在的 metadata 字段。
const HOST_EVENT_TRIGGERS: Record<string, {
  labelKey: RuntimeWakeLabelKey;
  source: RuntimeWakeSource;
  summaryKey: string;
} | undefined> = {
  follow_up: { labelKey: "message.follow_up.triggered", source: "follow_up", summaryKey: "follow_up_note" },
  scheduled_task: { labelKey: "message.scheduled_task.triggered", source: "scheduled_task", summaryKey: "task_name" },
};

// 后台任务状态到触发文案的完整映射；未知状态统一归入“继续执行”。
const RUNTIME_WAKE_TASK_LABELS: Record<string, RuntimeWakeLabelKey> = {
  completed: "message.runtime_wake.task_completed",
  failed: "message.runtime_wake.task_failed",
  killed: "message.runtime_wake.task_stopped",
  stopped: "message.runtime_wake.task_stopped",
};

/**
 * 非用户输入的触发行不是用户气泡：runtime 自唤醒（metadata.input_kind=runtime_wake）
 * 与主窗口宿主事件到点（metadata.input_kind=scheduled_task / follow_up）都返回触发事实供视图渲染为分隔条；
 * 普通用户消息返回 null。宿主事件行的正文是给模型的指令包装，只展示元数据里的名称或备注摘要。
 */
export function projectRuntimeWakePresentation(
  message: UserMessage,
  outcomeKey: HostRunOutcomeLabelKey | null = null,
): RuntimeWakePresentation | null {
  const metadata = message.metadata;
  const hostEvent = HOST_EVENT_TRIGGERS[String(metadata?.input_kind ?? "")];
  if (metadata && hostEvent) {
    const summary = metadata[hostEvent.summaryKey];
    return {
      source: hostEvent.source,
      labelKey: hostEvent.labelKey,
      outcomeKey,
      summary: typeof summary === "string" ? summary.trim() : "",
      timestamp: formatMessageTime(message.timestamp),
    };
  }
  if (metadata?.input_kind !== "runtime_wake") {
    return null;
  }
  const backgroundTask = metadata.trigger_kind === "background_task";
  const status = typeof metadata.task_status === "string" ? metadata.task_status : "";
  return {
    source: "runtime_wake",
    labelKey: backgroundTask
      ? RUNTIME_WAKE_TASK_LABELS[status] ?? "message.runtime_wake.task_completed"
      : "message.runtime_wake.resumed",
    outcomeKey: null,
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
