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

interface VisibleRoundCandidate {
  containsFocus: boolean;
  distance: number;
  roundId: string;
  top: number;
}

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
  const candidates = elements
    .map((element) => projectVisibleRoundCandidate(
      element,
      containerRect,
      focusY,
      roundIdSet,
    ))
    .filter((candidate): candidate is VisibleRoundCandidate => (
      candidate !== null
    ));
  const containing = selectBestCandidate(
    candidates.filter((candidate) => candidate.containsFocus),
    (candidate) => candidate.top,
  );
  const closest = selectBestCandidate(
    candidates,
    (candidate) => -candidate.distance,
  );
  return containing?.roundId ?? closest?.roundId ?? null;
}

function projectVisibleRoundCandidate(
  element: HTMLElement,
  containerRect: DOMRect,
  focusY: number,
  roundIdSet: Set<string>,
): VisibleRoundCandidate | null {
  const roundId = element.dataset.conversationRoundId;
  if (!roundId || !roundIdSet.has(roundId)) {
    return null;
  }
  const rect = element.getBoundingClientRect();
  if (rect.bottom < containerRect.top || rect.top > containerRect.bottom) {
    return null;
  }
  return {
    containsFocus: rect.top <= focusY && rect.bottom >= focusY,
    distance: Math.abs(rect.top - focusY),
    roundId,
    top: rect.top,
  };
}

function selectBestCandidate(
  candidates: VisibleRoundCandidate[],
  getScore: (candidate: VisibleRoundCandidate) => number,
): VisibleRoundCandidate | null {
  return candidates.reduce<VisibleRoundCandidate | null>(
    (best, candidate) => (
      !best || getScore(candidate) > getScore(best) ? candidate : best
    ),
    null,
  );
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
