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
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface ComposerPanelProps {
  compact: boolean;
  isLoading: boolean;
  runtimePhase: AgentConversationRuntimePhase | null;
  runtimeKind: AgentRuntimeKind;
  onSendMessage: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: MessageAttachment[],
  ) => void | Promise<void>;
  inputQueueItems: InputQueueItem[];
  onEnqueueMessage: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: MessageAttachment[],
  ) => void | Promise<void>;
  onDeleteQueuedMessage: (itemId: string) => void | Promise<void>;
  onGuideQueuedMessage: (itemId: string) => void | Promise<void>;
  onReorderQueueMessages: (orderedIds: string[]) => void | Promise<void>;
  onStop: () => void;
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
    default: "px-2 py-2",
    goal: "px-2 pb-2 pt-1.5",
    queue: "px-2 pb-2 pt-1",
  },
  regular: {
    default: "px-3 py-3",
    goal: "px-3 pb-3 pt-2",
    queue: "px-3 pb-3 pt-1.5",
  },
};

export const COMPOSER_SHORTCUT_KEY_CLASS_NAME =
  "font-mono text-[11px] font-semibold leading-none text-(--text-muted)";
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
