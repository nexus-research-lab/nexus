/**
 * INPUT: 用户主动收起的局部内容节点与其真实高度差。
 * OUTPUT: 向所属会话 Feed 冒泡的显式收缩事件。
 * POS: 局部交互与 live Feed 高度保护之间的窄语义边界。
 */

export const CONVERSATION_EXPLICIT_SHRINK_EVENT =
  "nexus:conversation-explicit-shrink";

export interface ConversationExplicitShrinkDetail {
  heightDelta: number;
}

const SHRINK_TOLERANCE_PX = 0.5;

export function notifyConversationExplicitShrink(
  anchor: HTMLElement,
  heightDelta: number,
): void {
  if (!Number.isFinite(heightDelta) || heightDelta <= SHRINK_TOLERANCE_PX) {
    return;
  }
  anchor.dispatchEvent(new CustomEvent<ConversationExplicitShrinkDetail>(
    CONVERSATION_EXPLICIT_SHRINK_EVENT,
    {
      bubbles: true,
      detail: { heightDelta },
    },
  ));
}

export function getConversationExplicitShrinkDetail(
  event: Event,
): ConversationExplicitShrinkDetail | null {
  if (!(event instanceof CustomEvent)) {
    return null;
  }
  const detail = event.detail as Partial<ConversationExplicitShrinkDetail>;
  return typeof detail?.heightDelta === "number"
    ? { heightDelta: detail.heightDelta }
    : null;
}
