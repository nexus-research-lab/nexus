import type { ReactNode } from "react";

import type { Agent } from "@/types/agent/agent";
import type {
  AgentConversationDefaultDeliveryPolicy,
  AgentConversationDeliveryPolicy,
  AgentConversationRuntimePhase,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import type { LoopCatalogItem } from "@/types/capability/loop";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type {
  CommandCatalogData,
  ContextUsageData,
} from "@/types/generated/protocol";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface ComposerPanelProps {
  compact: boolean;
  commandCatalog: CommandCatalogData;
  contextUsage: ContextUsageData | null;
  /** 纯文本消费者可隐藏附件、Goal 与 Loop 动作入口。 */
  showActionMenu?: boolean;
  contextUsageItems?: readonly ComposerContextUsageItem[];
  /**
   * DM/Room 等待用户回应时原位替换输入壳内容；草稿状态继续保留。
   */
  interactionSurface?: ReactNode;
  /** 用于最后一个 interaction 消失后恢复输入焦点。 */
  interactionIdentity?: string | null;
  /** 包含 Session 身份的完整待发送草稿作用域。 */
  draftScopeKey: string;
  /** 同一逻辑聊天共享、刻意不包含 Session ID 的已发送输入历史作用域。 */
  historyScopeKey: string;
  isLoading: boolean;
  runtimePhase: AgentConversationRuntimePhase | null;
  runtimeKind: AgentRuntimeKind;
  sessionSettings?: ComposerSessionSettingsScope;
  /** 桌面端可为该 DM Session 挂载本机目录。 */
  localDirectorySessionKey?: string;
  onSendMessage: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: MessageAttachment[],
    targetAgentIDs?: string[],
  ) => void | Promise<void>;
  inputQueueItems: InputQueueItem[];
  onEnqueueMessage: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: MessageAttachment[],
    targetAgentIDs?: string[],
  ) => void | Promise<void>;
  onDeleteQueuedMessage: (itemId: string) => void | Promise<void>;
  onGuideQueuedMessage: (itemId: string) => void | Promise<void>;
  onReorderQueueMessages: (orderedIds: string[]) => void | Promise<void>;
  /** DM 停止当前会话；Room 可把点击时的精确 Agent target 快照聚合到此入口。 */
  onStop?: () => void;
  /** 默认沿用单会话“停止生成”，Room 可显式显示“全部停止”。 */
  stopLabel?: string;
  defaultDeliveryPolicy: AgentConversationDefaultDeliveryPolicy;
  queueWhenSessionBusy?: boolean;
  roomMembers?: Agent[];
  onPrepareAttachments: (
    files: File[],
  ) => Promise<MessageAttachment[]>;
  onCreateGoal?: (objective: string) => Promise<void>;
  enableLoops?: boolean;
  onCreateLoopGoal?: (loop: LoopCatalogItem) => Promise<void>;
  goalCreateDisabledReason?: string | null;
  goalModeExtra?: ReactNode;
  goalScopeLabel: string;
  tourAnchor: string;
}

/** Room 将同一共享会话内的上下文快照按 Agent 显式归属。 */
export interface ComposerContextUsageItem {
  agentId: string;
  avatar?: string | null;
  name: string;
  usage: ContextUsageData | null;
}

export interface ComposerSessionSettingsTarget {
  agentId: string;
  avatar?: string | null;
  defaultConnectorIds?: string[];
  defaultModel?: string;
  defaultPermissionMode?: string;
  defaultProvider?: string;
  name: string;
  sessionKey: string;
}

export interface ComposerSessionSettingsScope {
  initialTargetId: string;
  runtimeKind: AgentRuntimeKind;
  targets: ComposerSessionSettingsTarget[];
}

export type ComposerInputMode = "message" | "goal";
export type ComposerRuntimeActivity =
  | "sending"
  | "compacting"
  | "replying"
  | null;

export type ComposerNativeKeyboardEvent = globalThis.KeyboardEvent & {
  keyCode?: number;
  which?: number;
};

interface ComposerDelivery {
  handler: "enqueue" | "send";
  policy: AgentConversationDeliveryPolicy;
}

const INPUT_ROW_PADDING: Record<
  "compact" | "regular",
  Record<"default" | "goal" | "queue", string>
> = {
  compact: {
    default: "px-3.5 pb-0.5 pt-1",
    goal: "px-3.5 pb-0.5 pt-1",
    queue: "px-3.5 pb-0.5 pt-0",
  },
  regular: {
    default: "px-3.5 pb-0.5 pt-1.5",
    goal: "px-3.5 pb-0.5 pt-1.5",
    queue: "px-3.5 pb-0.5 pt-0.5",
  },
};

export const MAX_COMPOSER_INPUT_LENGTH = 10_000;
export const MENTION_NAVIGATION_KEYS = new Set([
  "ArrowDown",
  "ArrowUp",
  "Enter",
  "Tab",
  "Escape",
]);
const IME_COMPOSITION_KEY_CODE = 229;
export const COMPOSITION_END_ENTER_GUARD_MS = 80;

export function focusComposerInputAtEnd(
  target: HTMLTextAreaElement,
): void {
  const caretPosition = target.value.length;
  target.focus({ preventScroll: true });
  target.setSelectionRange(caretPosition, caretPosition);
  target.scrollTop = target.scrollHeight;
}

export function isCaretOnFirstLine(target: HTMLTextAreaElement): boolean {
  const { end, start } = readSelectionRange(target);
  return [
    start === end,
    !target.value.slice(0, start).includes("\n"),
  ].every(Boolean);
}

export function isCaretOnLastLine(target: HTMLTextAreaElement): boolean {
  const { end, start } = readSelectionRange(target);
  return [
    start === end,
    !target.value.slice(end).includes("\n"),
  ].every(Boolean);
}

function readSelectionRange(target: HTMLTextAreaElement) {
  return {
    end: target.selectionEnd ?? 0,
    start: target.selectionStart ?? 0,
  };
}

export function isImeKeyboardEvent(
  event: ComposerNativeKeyboardEvent,
): boolean {
  return [
    event.isComposing,
    event.key === "Process",
    event.keyCode === IME_COMPOSITION_KEY_CODE,
    event.which === IME_COMPOSITION_KEY_CODE,
  ].some(Boolean);
}

export function isWithinCompositionEndEnterGuard(
  eventTime: number,
  compositionEndTime: number,
): boolean {
  return [
    compositionEndTime > 0,
    eventTime >= compositionEndTime,
    eventTime - compositionEndTime <= COMPOSITION_END_ENTER_GUARD_MS,
  ].every(Boolean);
}

export function resolveComposerDelivery(
  busy: boolean,
  queueWhenSessionBusy: boolean,
  defaultPolicy: AgentConversationDeliveryPolicy,
): ComposerDelivery {
  return {
    handler: resolveComposerDeliveryHandler(busy, queueWhenSessionBusy),
    policy: resolveComposerDeliveryPolicy(busy, defaultPolicy),
  };
}

function resolveComposerDeliveryHandler(
  busy: boolean,
  queueWhenSessionBusy: boolean,
): ComposerDelivery["handler"] {
  const shouldEnqueue = [busy, queueWhenSessionBusy].every(Boolean);
  return shouldEnqueue ? "enqueue" : "send";
}

function resolveComposerDeliveryPolicy(
  busy: boolean,
  defaultPolicy: AgentConversationDeliveryPolicy,
): AgentConversationDeliveryPolicy {
  return busy ? defaultPolicy : "queue";
}

export function getComposerInputRowPaddingClass(
  compact: boolean,
  hasPendingQueue: boolean,
  isGoalMode: boolean,
): string {
  const density = compact ? "compact" : "regular";
  const candidates = [
    { active: isGoalMode, state: "goal" },
    { active: hasPendingQueue, state: "queue" },
  ] as const;
  const state = candidates.find((candidate) => candidate.active)?.state
    ?? "default";
  return INPUT_ROW_PADDING[density][state];
}
