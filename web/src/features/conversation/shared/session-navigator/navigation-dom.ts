import type {
  ConversationRoundScrollHandle,
  ConversationRoundScrollOptions,
} from "../timeline/scroll/round-scroll";
import {
  CONVERSATION_ROUND_SELECTOR,
  findConversationRoundElement,
  getConversationRoundFocusOffset,
  scrollToConversationRoundElement,
} from "../timeline/scroll/round-scroll";

const SCROLL_BOUNDARY_EPSILON_PX = 2;

function estimateRoundIndex(
  scrollElement: HTMLDivElement,
  roundIds: string[],
): number {
  if (roundIds.length <= 1) {
    return 0;
  }
  const maxScroll = Math.max(
    1,
    scrollElement.scrollHeight - scrollElement.clientHeight,
  );
  const ratio = Math.min(
    1,
    Math.max(0, scrollElement.scrollTop / maxScroll),
  );
  return Math.min(
    roundIds.length - 1,
    Math.max(0, Math.round(ratio * (roundIds.length - 1))),
  );
}

function resolveBoundaryRoundId(
  scrollElement: HTMLDivElement,
  roundIds: string[],
): string | undefined {
  if (scrollElement.scrollTop <= SCROLL_BOUNDARY_EPSILON_PX) {
    return roundIds[0];
  }
  const maxScroll = Math.max(
    0,
    scrollElement.scrollHeight - scrollElement.clientHeight,
  );
  if (scrollElement.scrollTop >= maxScroll - SCROLL_BOUNDARY_EPSILON_PX) {
    return roundIds[roundIds.length - 1];
  }
  return undefined;
}

function findFocusedVisibleRoundId(
  scrollElement: HTMLDivElement,
  roundIdSet: Set<string>,
): string | null {
  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
  );
  const containerRect = scrollElement.getBoundingClientRect();
  const focusY =
    containerRect.top + getConversationRoundFocusOffset(scrollElement);
  let closestRoundId: string | null = null;
  let closestDistance = Number.POSITIVE_INFINITY;
  let containingRoundId: string | null = null;
  let containingTop = Number.NEGATIVE_INFINITY;

  for (const element of elements) {
    const roundId = element.dataset.conversationRoundId;
    if (!roundId || !roundIdSet.has(roundId)) {
      continue;
    }
    const rect = element.getBoundingClientRect();
    if (rect.bottom < containerRect.top || rect.top > containerRect.bottom) {
      continue;
    }
    if (rect.top <= focusY && rect.bottom >= focusY && rect.top > containingTop) {
      containingTop = rect.top;
      containingRoundId = roundId;
    }
    const distance = Math.abs(rect.top - focusY);
    if (distance < closestDistance) {
      closestDistance = distance;
      closestRoundId = roundId;
    }
  }
  return containingRoundId ?? closestRoundId;
}

export function resolveVisibleRoundId(
  scrollElement: HTMLDivElement,
  roundIds: string[],
  roundIdSet: Set<string>,
): string | null {
  if (roundIds.length === 0) {
    return null;
  }
  const boundaryRoundId = resolveBoundaryRoundId(scrollElement, roundIds);
  if (boundaryRoundId) {
    return boundaryRoundId;
  }
  return (
    findFocusedVisibleRoundId(scrollElement, roundIdSet) ??
    roundIds[estimateRoundIndex(scrollElement, roundIds)] ??
    null
  );
}

export function scrollToTimelineRound(
  scrollElement: HTMLDivElement | null,
  roundScrollHandle: ConversationRoundScrollHandle | null,
  roundId: string,
  options?: ConversationRoundScrollOptions,
): boolean {
  if (roundScrollHandle?.scrollToRoundId(roundId, options)) {
    return true;
  }
  const target = scrollElement
    ? findConversationRoundElement(scrollElement, roundId)
    : null;
  if (!scrollElement || !target) {
    return false;
  }
  scrollToConversationRoundElement(scrollElement, target, options);
  return true;
}
