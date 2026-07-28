/**
 * INPUT: 会话消息快照与滚动容器测量值。
 * OUTPUT: 溢出/贴底/跟随转换/键盘意图判定，以及覆盖并行活动回复正文增长的稳定内容版本。
 * POS: DM、Room 与 Thread 跟随滚动的纯模型真相源。
 */
import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

const BOTTOM_THRESHOLD_PX = 80;
const FOLLOW_RESUME_THRESHOLD_PX = 8;
const MIN_ATOMIC_GROWTH_DETACH_PX = 160;
const ATOMIC_GROWTH_VIEWPORT_RATIO = 0.35;
const SCROLL_OVERFLOW_TOLERANCE_PX = 1;
const SCROLL_DIRECTION_TOLERANCE_PX = 0.5;
const VIEWPORT_SIZE_TOLERANCE_PX = 1;

interface ScrollMetrics {
  clientHeight: number;
  scrollHeight: number;
  scrollTop: number;
}

export interface ConversationViewportSize {
  height: number;
}

export interface ConversationViewportResizeState {
  scrollTop: number;
  shouldFollow: boolean;
  showScrollToBottom: boolean;
}

export interface ConversationViewportSizeRevision {
  baseline: ConversationViewportSize;
  changed: boolean;
}

interface ConversationViewportElement {
  clientHeight: number;
}

export type FollowScrollIntent = "down" | "up";

interface ScrollMessageIdentity {
  messageId: string;
  role: Message["role"] | "";
  streamStatus: string;
  timestamp: number;
}

const EMPTY_SCROLL_MESSAGE_IDENTITY: ScrollMessageIdentity = {
  messageId: "",
  role: "",
  streamStatus: "",
  timestamp: 0,
};

export function getScrollBottomTop(element: ScrollMetrics): number {
  return Math.max(0, element.scrollHeight - element.clientHeight);
}

export function hasScrollableOverflow(element: ScrollMetrics): boolean {
  return getScrollBottomTop(element) > SCROLL_OVERFLOW_TOLERANCE_PX;
}

export function isNearScrollBottom(element: ScrollMetrics): boolean {
  return getScrollBottomTop(element) - element.scrollTop <= BOTTOM_THRESHOLD_PX;
}

export function getConversationViewportSize(
  element: ConversationViewportElement,
): ConversationViewportSize {
  return {
    height: element.clientHeight,
  };
}

export function hasConversationViewportSizeChanged(
  previous: ConversationViewportSize,
  current: ConversationViewportSize,
): boolean {
  return (
    Math.abs(previous.height - current.height) > VIEWPORT_SIZE_TOLERANCE_PX
  );
}

export function resolveConversationViewportSizeRevision(
  previous: ConversationViewportSize | null,
  current: ConversationViewportSize,
): ConversationViewportSizeRevision {
  if (!previous) {
    return { baseline: current, changed: false };
  }
  if (!hasConversationViewportSizeChanged(previous, current)) {
    // 不推进容差内的基线；连续的小步窗口缩放仍须累计成一次真实变化。
    return { baseline: previous, changed: false };
  }
  return { baseline: current, changed: true };
}

export function resolveConversationViewportResizeState(
  element: ScrollMetrics,
  previousScrollTop: number,
  wasFollowing: boolean,
): ConversationViewportResizeState {
  const bottomTop = getScrollBottomTop(element);
  const scrollTop = Math.min(Math.max(0, previousScrollTop), bottomTop);
  if (!hasScrollableOverflow(element)) {
    return {
      scrollTop,
      shouldFollow: true,
      showScrollToBottom: false,
    };
  }
  const remainsAtBottom = (
    bottomTop - scrollTop <= SCROLL_OVERFLOW_TOLERANCE_PX
  );
  const shouldFollow = wasFollowing && remainsAtBottom;
  return {
    scrollTop,
    shouldFollow,
    showScrollToBottom: !shouldFollow && !remainsAtBottom,
  };
}

/**
 * Room 运行摘要切换完整结果、权限卡出现等原子布局变化不能让视口追赶
 * 整个新高度；小的真实流式增长仍保持自动跟随。
 */
export function shouldDetachFollowForAtomicGrowth(
  element: ScrollMetrics,
  previousScrollHeight: number,
): boolean {
  if (
    previousScrollHeight <= 0
    || element.scrollHeight <= element.clientHeight
  ) {
    return false;
  }
  const growth = element.scrollHeight - previousScrollHeight;
  const threshold = Math.max(
    MIN_ATOMIC_GROWTH_DETACH_PX,
    element.clientHeight * ATOMIC_GROWTH_VIEWPORT_RATIO,
  );
  return growth > threshold;
}

export function shouldResumeFollowOnScroll(
  element: ScrollMetrics,
  previousScrollTop: number,
  hasUserScrollIntent: boolean,
): boolean {
  return (
    hasUserScrollIntent
    && element.scrollTop
      > previousScrollTop + SCROLL_DIRECTION_TOLERANCE_PX
    && getScrollBottomTop(element) - element.scrollTop
      <= FOLLOW_RESUME_THRESHOLD_PX
  );
}

export function shouldPauseFollowOnScroll(
  element: ScrollMetrics,
  previousScrollTop: number,
  hasUserScrollIntent: boolean,
): boolean {
  return (
    hasUserScrollIntent
    && element.scrollTop
      < previousScrollTop - SCROLL_DIRECTION_TOLERANCE_PX
  );
}

export function resolveKeyboardFollowScrollIntent(
  key: string,
  shiftKey: boolean,
): FollowScrollIntent | null {
  switch (key) {
    case "ArrowUp":
    case "Home":
    case "PageUp":
      return "up";
    case "ArrowDown":
    case "End":
    case "PageDown":
      return "down";
    case " ":
      return shiftKey ? "up" : "down";
    default:
      return null;
  }
}

export function resolveTouchFollowScrollIntent(
  previousY: number,
  currentY: number,
): FollowScrollIntent | null {
  if (currentY > previousY) {
    return "up";
  }
  if (currentY < previousY) {
    return "down";
  }
  return null;
}

function projectScrollMessageIdentity(
  message: Message | undefined,
): ScrollMessageIdentity {
  if (!message) {
    return EMPTY_SCROLL_MESSAGE_IDENTITY;
  }
  return {
    messageId: message.message_id,
    role: message.role,
    streamStatus:
      message.role === "assistant" ? message.stream_status ?? "" : "",
    timestamp: message.timestamp,
  };
}

function projectAssistantScrollRevision(message: Message): string | null {
  if (message.role !== "assistant") {
    return null;
  }
  let renderedLength = message.result_summary?.result?.length ?? 0;
  for (const block of message.content) {
    switch (block.type) {
      case "text":
        renderedLength += block.text.length;
        break;
      case "thinking":
        renderedLength += block.thinking.length;
        break;
      case "tool_use_error":
      case "system_event":
        renderedLength += block.content.length;
        break;
      case "task_progress":
        renderedLength += block.description.length;
        break;
      case "search_result":
        renderedLength +=
          (block.title?.length ?? 0)
          + (block.snippet?.length ?? 0);
        break;
      default:
        // 非文本块的增删仍会改变正文高度；动态大负载只计块身份，避免逐 token 序列化。
        renderedLength += 1;
        break;
    }
  }
  return [
    message.message_id,
    message.agent_round_id ?? "",
    message.stream_status ?? "",
    message.content.length,
    renderedLength,
  ].join(":");
}

function buildRoomAgentNodeIdentity(
  rootRoundId: string,
  agentRoundId: string | null | undefined,
): string | null {
  const normalizedAgentRoundId = agentRoundId?.trim();
  return normalizedAgentRoundId
    ? `${rootRoundId}\u001e${normalizedAgentRoundId}`
    : null;
}

/**
 * 只描述会增删或搬动 Feed 节点的身份，不把 token 长度混进来。
 * Room 在历史 root 下追加 public wake 时，滚动层据此执行跨 renderer 锚点恢复。
 */
export function buildConversationScrollTopologyKey(
  sessionKey: string | null,
  messages: readonly Message[],
  pendingSlots: readonly RoomPendingAgentSlotState[] = [],
): string {
  const identities: string[] = [];
  const seen = new Set<string>();
  const append = (identity: string | null): void => {
    if (!identity || seen.has(identity)) {
      return;
    }
    seen.add(identity);
    identities.push(identity);
  };

  for (const message of messages) {
    if (message.role === "user") {
      append(`root:${message.client_message_id?.trim() || message.round_id}`);
      continue;
    }
    append(buildRoomAgentNodeIdentity(
      message.round_id,
      message.agent_round_id,
    ));
  }
  for (const slot of pendingSlots) {
    append(buildRoomAgentNodeIdentity(slot.round_id, slot.agent_round_id));
  }
  return [sessionKey ?? "", ...identities].join("\u001f");
}

/**
 * 权限模块和 terminal 组件切换是原子布局，不属于可追随的真实 token 增长。
 */
export function buildConversationAtomicLayoutKey(
  sessionKey: string | null,
  messages: readonly Message[],
  pendingPermissions: readonly PendingPermission[] = [],
): string {
  const terminalRevisions = messages.flatMap((message) => {
    if (message.role !== "assistant") {
      return [];
    }
    const terminalStatus = (
      message.stream_status === "done"
      || message.stream_status === "cancelled"
      || message.stream_status === "error"
    );
    if (
      !terminalStatus
      && !message.is_complete
      && !message.stop_reason
      && !message.result_summary
    ) {
      return [];
    }
    return [[
      message.message_id,
      terminalStatus ? 1 : 0,
      message.is_complete ? 1 : 0,
      message.stop_reason ?? "",
      message.result_summary ? 1 : 0,
    ].join(":")];
  });
  const permissionIds = pendingPermissions
    .map((permission) => permission.request_id)
    .filter(Boolean)
    .sort();
  return [
    sessionKey ?? "",
    ...terminalRevisions,
    "permissions",
    ...permissionIds,
  ].join("\u001f");
}

export function buildConversationScrollContentKey(
  sessionKey: string | null,
  messages: readonly Message[],
): string {
  const firstMessage = projectScrollMessageIdentity(messages[0]);
  const latestMessage = projectScrollMessageIdentity(messages.at(-1));
  const assistantRevisions = messages.flatMap((message) => {
    const revision = projectAssistantScrollRevision(message);
    return revision ? [revision] : [];
  });

  return [
    sessionKey ?? "",
    messages.length,
    firstMessage.messageId,
    latestMessage.messageId,
    latestMessage.timestamp,
    latestMessage.role,
    latestMessage.streamStatus,
    ...assistantRevisions,
  ].join("\u001f");
}
