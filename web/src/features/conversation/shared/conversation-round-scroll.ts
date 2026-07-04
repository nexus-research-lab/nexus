import type { MutableRefObject } from "react";

export const CONVERSATION_ROUND_SELECTOR = "[data-conversation-round-id]";

export interface ConversationRoundScrollOptions {
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
  return Array.from(
    scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
  ).find((element) => element.dataset.conversationRoundId === roundId) ?? null;
}

export function scrollToConversationRoundElement(
  scrollElement: HTMLDivElement,
  target: HTMLElement,
  options?: ConversationRoundScrollOptions,
): void {
  const containerRect = scrollElement.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  scrollElement.scrollTo({
    behavior: options?.behavior ?? "smooth",
    top: scrollElement.scrollTop + targetRect.top - containerRect.top - 24,
  });
}
