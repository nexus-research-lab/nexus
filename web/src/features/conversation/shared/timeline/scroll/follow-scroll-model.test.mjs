// @vitest-environment node

import assert from "node:assert/strict";
import { test } from "vitest";

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
  const visibleTopBeforeGrowth = visible.getBoundingClientRect().top;
  documentTops.visible += 40;
  assert.equal(
    anchor.restore(container, feed),
    440,
    "history loaded above the reading round during a scroll gesture must not push the viewport back",
  );
  assert.equal(visible.getBoundingClientRect().top, visibleTopBeforeGrowth);
  documentTops.visible += 120;
  assert.equal(anchor.restore(container, feed), 560);
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
  assert.equal(scrollTop, 560);
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
