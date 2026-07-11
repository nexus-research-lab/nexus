import type { MutableRefObject } from "react";

export const CONVERSATION_ROUND_SELECTOR = "[data-conversation-round-id]";
export const CONVERSATION_ROUND_USER_ANCHOR_SELECTOR =
  '[data-conversation-round-user-anchor="true"]';
const ROUND_NAVIGATION_TARGET_DATA_KEY = "conversationRoundNavigationTarget";

export interface ConversationRoundScrollOptions {
  align?: "start" | "focus";
  behavior?: ScrollBehavior;
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
    ).find((element) => element.dataset.conversationRoundId === roundId) ?? null
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
  ).getBoundingClientRect();
  const offset =
    options?.align === "focus"
      ? getConversationRoundFocusOffset(scrollElement)
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

function resolveConversationRoundScrollTarget(target: HTMLElement): HTMLElement {
  return (
    target.querySelector<HTMLElement>(
      CONVERSATION_ROUND_USER_ANCHOR_SELECTOR,
    ) ?? target
  );
}
