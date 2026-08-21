/**
 * INPUT: 会话轮次 DOM 协议、滚动视口与目标轮次/对齐选项。
 * OUTPUT: 可定位用户锚点或完整轮次 wrapper 的 static/virtual Feed 滚动句柄。
 * POS: DM/Room 轮次精确定位边界；不拥有导航目标或未读状态。
 */
import type { MutableRefObject } from "react";

export const CONVERSATION_ROUND_SELECTOR = "[data-conversation-round-id]";
const CONVERSATION_ROUND_USER_ANCHOR_SELECTOR =
  '[data-conversation-round-user-anchor="true"]';
const ROUND_NAVIGATION_TARGET_DATA_KEY = "conversationRoundNavigationTarget";
const ROUND_FOCUS_LANDING_INSET_PX = 4;

export interface ConversationRoundScrollOptions {
  align?: "start" | "focus";
  behavior?: ScrollBehavior;
  target?: "round" | "user";
}

export interface ConversationRoundScrollHandle {
  scrollToRoundId: (
    roundId: string,
    options?: ConversationRoundScrollOptions,
  ) => boolean;
}

export type ConversationRoundScrollHandleRef =
  MutableRefObject<ConversationRoundScrollHandle | null>;

export function findConversationRoundElement(
  scrollElement: HTMLDivElement,
  roundId: string,
): HTMLElement | null {
  return (
    Array.from(
      scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
    ).find((element) => (
      element.dataset.conversationRoundId === roundId
      || element.dataset.conversationRootRoundId === roundId
    )) ?? null
  );
}

export function getConversationRoundFocusOffset(
  scrollElement: HTMLDivElement | null,
): number {
  if (!scrollElement) {
    return 180;
  }
  return Math.min(180, scrollElement.clientHeight * 0.34);
}

export function getConversationRoundNavigationTarget(
  scrollElement: HTMLDivElement,
): string | null {
  return scrollElement.dataset[ROUND_NAVIGATION_TARGET_DATA_KEY] ?? null;
}

export function setConversationRoundNavigationTarget(
  scrollElement: HTMLDivElement,
  roundId: string,
): void {
  scrollElement.dataset[ROUND_NAVIGATION_TARGET_DATA_KEY] = roundId;
}

export function clearConversationRoundNavigationTarget(
  scrollElement: HTMLDivElement,
  roundId?: string | null,
): void {
  const currentRoundId = getConversationRoundNavigationTarget(scrollElement);
  if (roundId && currentRoundId && currentRoundId !== roundId) {
    return;
  }
  delete scrollElement.dataset[ROUND_NAVIGATION_TARGET_DATA_KEY];
}

export function isConversationRoundScrollTargetVisible(
  scrollElement: HTMLDivElement,
  target: HTMLElement,
): boolean {
  const containerRect = scrollElement.getBoundingClientRect();
  const targetRect = resolveConversationRoundScrollTarget(
    target,
  ).getBoundingClientRect();
  return (
    targetRect.top >= containerRect.top + 8 &&
    targetRect.top < containerRect.bottom - 8
  );
}

export function scrollToConversationRoundElement(
  scrollElement: HTMLDivElement,
  target: HTMLElement,
  options?: ConversationRoundScrollOptions,
): void {
  const containerRect = scrollElement.getBoundingClientRect();
  const targetRect = resolveConversationRoundScrollTarget(
    target,
    options?.target,
  ).getBoundingClientRect();
  const offset =
    options?.align === "focus"
      // 把目标顶部稳定放到焦点线之前。若精确落在同一坐标，浏览器的
      // sub-pixel scrollTop 量化可能让上一轮仍包含焦点并抢走 active 状态。
      ? Math.max(
        24,
        getConversationRoundFocusOffset(scrollElement)
          - ROUND_FOCUS_LANDING_INSET_PX,
      )
      : 24;
  const maxScrollTop = Math.max(
    0,
    scrollElement.scrollHeight - scrollElement.clientHeight,
  );
  const nextScrollTop = Math.min(
    maxScrollTop,
    Math.max(
      0,
      scrollElement.scrollTop + targetRect.top - containerRect.top - offset,
    ),
  );
  scrollElement.scrollTo({
    behavior: options?.behavior ?? "smooth",
    top: nextScrollTop,
  });
}

function resolveConversationRoundScrollTarget(
  target: HTMLElement,
  preference: ConversationRoundScrollOptions["target"] = "user",
): HTMLElement {
  if (preference === "round") {
    return target;
  }
  return (
    target.querySelector<HTMLElement>(
      CONVERSATION_ROUND_USER_ANCHOR_SELECTOR,
    ) ?? target
  );
}
