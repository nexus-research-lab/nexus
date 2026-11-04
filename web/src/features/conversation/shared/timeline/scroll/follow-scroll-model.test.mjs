// @vitest-environment node

import assert from "node:assert/strict";
import { test } from "vitest";

test("scroll-to-latest requires real viewport overflow", async () => {
  const { hasScrollableOverflow } = await import("@/features/conversation/shared/timeline/scroll/follow-scroll-model.ts");
  assert.equal(
    hasScrollableOverflow(
      { clientHeight: 500, scrollHeight: 500, scrollTop: 0 },
    ),
    false,
    "an empty or short conversation must not expose a scroll-to-latest action",
  );
  assert.equal(
    hasScrollableOverflow(
      { clientHeight: 500, scrollHeight: 501, scrollTop: 0 },
    ),
    false,
    "sub-pixel layout rounding must not create a false scroll affordance",
  );
  assert.equal(
    hasScrollableOverflow(
      { clientHeight: 500, scrollHeight: 502, scrollTop: 0 },
    ),
    true,
    "real overflow must preserve the scroll-to-latest affordance",
  );
});

test("FOLLOW and READING preserve intent at the real bottom edge", async () => {
  const {
    getConversationViewportSize,
    hasConversationViewportSizeChanged,
    isAtScrollBottom,
    resolveKeyboardFollowScrollIntent,
    resolveTouchFollowScrollIntent,
    resolveConversationViewportResizeState,
    resolveConversationViewportSizeRevision,
    shouldPauseFollowOnScroll,
    shouldResumeFollowOnScroll,
  } = await import("@/features/conversation/shared/timeline/scroll/follow-scroll-model.ts");
  assert.equal(
    isAtScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 2_000 },
    ),
    false,
    "an intermediate position is not the real bottom",
  );
  assert.equal(
    isAtScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_450 },
    ),
    false,
    "the action remains visible even when the reader is only 50px from bottom",
  );
  assert.equal(
    isAtScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_499.5 },
    ),
    true,
    "subpixel rounding at the real edge still counts as bottom",
  );
  assert.equal(
    shouldResumeFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_480 },
      4_500,
      true,
    ),
    false,
    "a small explicit upward scroll must remain detached inside the threshold",
  );
  assert.equal(
    shouldResumeFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_450 },
      4_300,
      true,
    ),
    false,
    "moving down while still away from the edge must remain detached",
  );
  assert.equal(
    shouldResumeFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_494 },
      4_450,
      false,
    ),
    false,
    "a programmatic size correction must not restore following",
  );
  assert.equal(
    shouldResumeFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_494 },
      4_450,
      true,
    ),
    false,
    "being several pixels above the edge must keep READING ownership",
  );
  assert.equal(
    shouldResumeFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_499.5 },
      4_450,
      true,
    ),
    true,
    "only downward user movement to the real bottom may resume FOLLOW",
  );
  assert.equal(
    shouldPauseFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_420 },
      4_450,
      true,
    ),
    true,
    "an upward pointer or wheel movement must detach following",
  );
  assert.equal(
    shouldPauseFollowOnScroll(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_420 },
      4_450,
      false,
    ),
    false,
    "programmatic upward correction must not imitate user intent",
  );
  assert.equal(resolveKeyboardFollowScrollIntent("PageUp", false), "up");
  assert.equal(resolveKeyboardFollowScrollIntent("End", false), "down");
  assert.equal(resolveKeyboardFollowScrollIntent(" ", true), "up");
  assert.equal(resolveKeyboardFollowScrollIntent("a", false), null);
  assert.equal(resolveTouchFollowScrollIntent(400, 360), "down");
  assert.equal(
    resolveTouchFollowScrollIntent(360, 380),
    "up",
    "a reverse touch move must use the previous frame instead of the origin",
  );
  assert.deepEqual(
    getConversationViewportSize({
      clientHeight: 480,
      clientWidth: 320,
    }),
    { height: 480, width: 320 },
    "the reading viewport is defined by its available content height",
  );
  assert.equal(
    hasConversationViewportSizeChanged(
      { height: 500 },
      getConversationViewportSize({
        clientHeight: 500,
      }),
    ),
    false,
    "an unchanged viewport height must not detach following",
  );
  assert.equal(
    hasConversationViewportSizeChanged(
      { height: 500 },
      { height: 499 },
    ),
    false,
    "subpixel observer noise must not detach following",
  );
  const ignoredViewportRevision = resolveConversationViewportSizeRevision(
    { height: 500 },
    { height: 499 },
  );
  assert.deepEqual(
    ignoredViewportRevision,
    {
      baseline: { height: 500 },
      changed: false,
    },
    "ignored one-pixel resize noise must not advance the comparison baseline",
  );
  assert.deepEqual(
    resolveConversationViewportSizeRevision(
      ignoredViewportRevision.baseline,
      { height: 498 },
    ),
    {
      baseline: { height: 498 },
      changed: true,
    },
    "successive one-pixel App resizes must accumulate into a real viewport change",
  );
  assert.equal(
    hasConversationViewportSizeChanged(
      { height: 500 },
      { height: 420 },
    ),
    true,
    "Composer or App height changes must be treated as viewport changes",
  );
  assert.deepEqual(
    resolveConversationViewportResizeState(
      { clientHeight: 420, scrollHeight: 1_500, scrollTop: 1_000 },
      1_000,
      true,
    ),
    {
      scrollTop: 1_080,
      shouldFollow: true,
      showScrollToBottom: false,
    },
    "a shrinking viewport must preserve FOLLOW and synchronously use its new bottom",
  );
  assert.deepEqual(
    resolveConversationViewportResizeState(
      { clientHeight: 500, scrollHeight: 1_500, scrollTop: 1_000 },
      1_080,
      true,
    ),
    {
      scrollTop: 1_000,
      shouldFollow: true,
      showScrollToBottom: false,
    },
    "a growing viewport clamps to bottom without changing FOLLOW ownership",
  );
  assert.deepEqual(
    resolveConversationViewportResizeState(
      { clientHeight: 420, scrollHeight: 1_500, scrollTop: 700 },
      700,
      false,
    ),
    {
      scrollTop: 700,
      shouldFollow: false,
      showScrollToBottom: true,
    },
    "an explicitly detached reader must remain detached after viewport resize",
  );
  assert.deepEqual(
    resolveConversationViewportResizeState(
      { clientHeight: 500, scrollHeight: 1_500, scrollTop: 1_000 },
      1_000,
      false,
    ),
    {
      scrollTop: 1_000,
      shouldFollow: false,
      showScrollToBottom: false,
    },
    "a browser clamp may hide the action but cannot silently turn READING into FOLLOW",
  );
});

test("FOLLOW keeps one scroll owner while parallel Room Agents grow", async () => {
  const { resolveConversationFollowCommitOwner } = await import("@/features/conversation/shared/timeline/scroll/follow-scroll-model.ts");

  assert.equal(
    resolveConversationFollowCommitOwner({
      bottomScrollActive: false,
      isNewSession: false,
      isVirtualFeed: true,
      topologyChanged: false,
    }),
    "virtualizer",
    "an existing upper Agent stream must not issue a second shared bottom write",
  );
  assert.equal(
    resolveConversationFollowCommitOwner({
      bottomScrollActive: true,
      isNewSession: false,
      isVirtualFeed: true,
      topologyChanged: false,
    }),
    "bottom",
    "stream growth during an explicit return-to-latest transaction must still hand off to FOLLOW",
  );
  assert.equal(
    resolveConversationFollowCommitOwner({
      bottomScrollActive: false,
      isNewSession: false,
      isVirtualFeed: true,
      topologyChanged: true,
    }),
    "bottom",
    "a genuinely appended tail node still needs the shared bottom owner",
  );
  assert.equal(
    resolveConversationFollowCommitOwner({
      bottomScrollActive: false,
      isNewSession: false,
      isVirtualFeed: false,
      topologyChanged: false,
    }),
    "bottom",
    "every static Agent resize in FOLLOW belongs to the aggregate bottom transaction",
  );
});

test("FOLLOW always resolves the current real bottom without a high-water target", async () => {
  const { BottomScrollAnimator } = await import("@/features/conversation/shared/timeline/scroll/scroll-animation.ts");
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: () => {
      throw new Error("FOLLOW must not request an animation frame");
    },
  };
  try {
    const container = {
      clientHeight: 500,
      scrollHeight: 1_080,
      scrollTop: 580,
    };
    const animator = new BottomScrollAnimator(() => container, () => {});

    container.scrollHeight = 1_040;
    animator.follow();
    assert.equal(container.scrollTop, 540);

    container.scrollHeight = 1_120;
    animator.follow();
    assert.equal(container.scrollTop, 620);
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("READING preserves the first visible Room round during static growth", async () => {
  const { ConversationViewportAnchor } = await import("@/features/conversation/shared/timeline/scroll/conversation-viewport-anchor.ts");
  let scrollTop = 400;
  const documentTops = {
    above: 250,
    visible: 450,
  };
  const container = {
    clientHeight: 500,
    scrollHeight: 1_500,
    get scrollTop() {
      return scrollTop;
    },
    set scrollTop(value) {
      scrollTop = value;
    },
    getBoundingClientRect: () => ({ bottom: 600, top: 100 }),
  };
  const buildRound = (key, height) => ({
    dataset: {
      conversationRootRoundId: key,
      conversationRoundId: key,
    },
    isConnected: true,
    getBoundingClientRect: () => {
      const top = 100 + documentTops[key] - scrollTop;
      return { bottom: top + height, top };
    },
  });
  const above = buildRound("above", 100);
  const visible = buildRound("visible", 200);
  const rounds = [above, visible];
  const feed = {
    contains: (element) => rounds.includes(element),
    dataset: {},
    querySelectorAll: () => rounds,
  };
  const anchor = new ConversationViewportAnchor();

  anchor.capture(container, feed);
  documentTops.visible += 40;
  assert.equal(
    anchor.restore(container, feed, { userScrollActive: true }),
    null,
    "a live user gesture absorbs the current geometry instead of writing scrollTop",
  );
  assert.equal(scrollTop, 400);
  const visibleTopBeforeGrowth = visible.getBoundingClientRect().top;
  documentTops.visible += 120;
  assert.equal(anchor.restore(container, feed), 520);
  assert.equal(
    visible.getBoundingClientRect().top,
    visibleTopBeforeGrowth,
    "a permission or earlier member result must not move the visible reply",
  );

  assert.equal(
    anchor.restore(container, feed),
    null,
    "growth below the anchor must not manufacture a scroll correction",
  );

  feed.dataset.conversationVirtualFeed = "true";
  documentTops.visible += 80;
  assert.equal(
    anchor.restore(container, feed),
    null,
    "Virtualizer remains the only owner of virtual item size compensation",
  );
  assert.equal(scrollTop, 520);
});

test("history prepend preserves scrolling performed while the page was loading", async () => {
  const { HistoryPrependAnchor } = await import("@/features/conversation/shared/timeline/scroll/history-prepend-anchor.ts");
  let scrollHeight = 1_000;
  let scrollTop = 80;
  const container = {
    get scrollHeight() {
      return scrollHeight;
    },
    get scrollTop() {
      return scrollTop;
    },
    set scrollTop(value) {
      scrollTop = value;
    },
  };
  const anchor = new HistoryPrependAnchor();
  anchor.prepare(container);
  scrollTop = 210;
  scrollHeight = 1_480;
  assert.equal(anchor.restore(container), 690);
  assert.equal(
    scrollTop,
    690,
    "the prepend delta is added to the latest user position, not the request-start position",
  );
});

test("viewport anchor survives a static-to-virtual Room feed switch", async () => {
  const { ConversationViewportAnchor } = await import("@/features/conversation/shared/timeline/scroll/conversation-viewport-anchor.ts");
  let scrollTop = 400;
  let documentTop = 460;
  const container = {
    clientHeight: 500,
    scrollHeight: 1_600,
    get scrollTop() {
      return scrollTop;
    },
    set scrollTop(value) {
      scrollTop = value;
    },
    getBoundingClientRect: () => ({ bottom: 600, top: 100 }),
  };
  const buildRound = (
    roundId = "room-agent-round:root-visible:agent-visible",
    getDocumentTop = () => documentTop,
  ) => ({
    dataset: {
      conversationRootRoundId: "root-visible",
      conversationRoundId: roundId,
    },
    getBoundingClientRect: () => {
      const top = 100 + getDocumentTop() - scrollTop;
      return { bottom: top + 180, top };
    },
    isConnected: true,
  });
  const staticRound = buildRound();
  let rounds = [staticRound];
  const feed = {
    contains: (element) => rounds.includes(element),
    dataset: {},
    querySelectorAll: () => rounds,
  };
  const anchor = new ConversationViewportAnchor();
  anchor.capture(container, feed);
  const visibleTop = staticRound.getBoundingClientRect().top;

  staticRound.isConnected = false;
  documentTop += 140;
  const virtualRound = buildRound();
  const earlierSibling = buildRound(
    "room-agent-round:root-visible:agent-earlier",
    () => 300,
  );
  rounds = [earlierSibling, virtualRound];
  feed.dataset.conversationVirtualFeed = "true";
  assert.equal(
    anchor.restore(container, feed, { allowVirtualFeed: true }),
    540,
  );
  assert.equal(
    virtualRound.getBoundingClientRect().top,
    visibleTop,
    "crossing the virtualization threshold must preserve the same visible node",
  );
});
