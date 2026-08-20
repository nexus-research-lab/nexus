/**
 * INPUT: 会话 Feed、滚动视口与稳定的 conversation round DOM 身份。
 * OUTPUT: 内容在当前可见轮次之前增减高度或 Feed renderer 切换时，保持该轮次的视口内位置不变。
 * POS: DM/Room/Thread 的连续阅读锚点；普通虚拟项测高继续由 Virtualizer 负责补偿。
 */
import { CONVERSATION_ROUND_SELECTOR } from "./round-scroll";

const VIEWPORT_ANCHOR_TOLERANCE_PX = 0.5;

interface ConversationViewportAnchorSnapshot {
  element: HTMLElement;
  offsetTop: number;
  rootRoundId: string;
  roundId: string;
}

interface ConversationViewportRestoreOptions {
  allowVirtualFeed?: boolean;
  userScrollActive?: boolean;
}

export class ConversationViewportAnchor {
  private snapshot: ConversationViewportAnchorSnapshot | null = null;

  capture(
    container: HTMLDivElement,
    feed: HTMLDivElement | null,
  ): void {
    if (
      !feed
      || container.scrollTop <= 0
    ) {
      this.reset();
      return;
    }

    const element = findFirstVisibleRound(container, feed);
    if (!element) {
      this.reset();
      return;
    }
    this.snapshot = {
      element,
      offsetTop: getViewportOffsetTop(container, element),
      rootRoundId: element.dataset.conversationRootRoundId ?? "",
      roundId: element.dataset.conversationRoundId ?? "",
    };
  }

  restore(
    container: HTMLDivElement,
    feed: HTMLDivElement | null,
    options: ConversationViewportRestoreOptions = {},
  ): number | null {
    const snapshot = this.snapshot;
    if (
      !snapshot
      || !feed
      || options.userScrollActive
      || (
        feed.dataset.conversationVirtualFeed === "true"
        && !options.allowVirtualFeed
      )
    ) {
      this.capture(container, feed);
      return null;
    }
    const element = resolveSnapshotElement(feed, snapshot);
    if (!element) {
      this.capture(container, feed);
      return null;
    }
    snapshot.element = element;

    const offsetDelta = (
      getViewportOffsetTop(container, element)
      - snapshot.offsetTop
    );
    if (Math.abs(offsetDelta) <= VIEWPORT_ANCHOR_TOLERANCE_PX) {
      this.capture(container, feed);
      return null;
    }

    const previousScrollTop = container.scrollTop;
    container.scrollTop = previousScrollTop + offsetDelta;
    this.capture(container, feed);
    return container.scrollTop === previousScrollTop
      ? null
      : container.scrollTop;
  }

  reset(): void {
    this.snapshot = null;
  }
}

function resolveSnapshotElement(
  feed: HTMLDivElement,
  snapshot: ConversationViewportAnchorSnapshot,
): HTMLElement | null {
  if (
    snapshot.element.isConnected
    && feed.contains(snapshot.element)
  ) {
    return snapshot.element;
  }
  const rounds = feed.querySelectorAll<HTMLElement>(
    CONVERSATION_ROUND_SELECTOR,
  );
  if (snapshot.roundId) {
    for (const round of rounds) {
      if (round.dataset.conversationRoundId === snapshot.roundId) {
        return round;
      }
    }
  }
  if (snapshot.rootRoundId) {
    for (const round of rounds) {
      if (round.dataset.conversationRootRoundId === snapshot.rootRoundId) {
        return round;
      }
    }
  }
  return null;
}

function findFirstVisibleRound(
  container: HTMLDivElement,
  feed: HTMLDivElement,
): HTMLElement | null {
  const viewport = container.getBoundingClientRect();
  const rounds = feed.querySelectorAll<HTMLElement>(
    CONVERSATION_ROUND_SELECTOR,
  );
  for (const round of rounds) {
    const bounds = round.getBoundingClientRect();
    if (
      bounds.bottom > viewport.top + VIEWPORT_ANCHOR_TOLERANCE_PX
      && bounds.top < viewport.bottom - VIEWPORT_ANCHOR_TOLERANCE_PX
    ) {
      return round;
    }
  }
  return null;
}

function getViewportOffsetTop(
  container: HTMLDivElement,
  element: HTMLElement,
): number {
  return (
    element.getBoundingClientRect().top
    - container.getBoundingClientRect().top
  );
}
