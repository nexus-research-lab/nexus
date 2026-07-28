import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("conversation viewport suppresses the browser scroll-region outline", async () => {
  const { ConversationPanelViewport } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const html = renderToStaticMarkup(React.createElement(
    ConversationPanelViewport,
    {
      isMobileLayout: false,
      viewport: {
        error: null,
        isHistoryLoading: false,
        scrollRef: { current: null },
      },
    },
    React.createElement("div", null, "message"),
  ));

  assert.match(
    html,
    /class="[^"]*overflow-y-auto[^"]*outline-none[^"]*"/,
    "the programmatically focusable viewport must not expose Safari's native blue outline",
  );
  assert.match(html, /tabindex="-1"/);
});

test("scroll-to-latest requires real viewport overflow", async () => {
  const { hasScrollableOverflow } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
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

test("scroll-to-latest stays clear of the feed boundary", async () => {
  const { ScrollToLatestButton } = await server.ssrLoadModule(
    "/src/features/conversation/shared/scroll-to-latest-button.tsx",
  );
  const visibleHtml = renderToStaticMarkup(React.createElement(
    ScrollToLatestButton,
    {
      isLoading: false,
      onClick: () => {},
      visible: true,
    },
  ));
  const hiddenHtml = renderToStaticMarkup(React.createElement(
    ScrollToLatestButton,
    {
      isLoading: false,
      onClick: () => {},
      visible: false,
    },
  ));

  assert.match(
    visibleHtml,
    /<div class="pointer-events-none relative z-20 h-10 shrink-0">/,
    "the action needs a dedicated band outside the scrollable feed",
  );
  assert.match(
    visibleHtml,
    /class="[^"]*absolute[^"]*top-1\/2[^"]*"/,
    "the action must stay centered inside its reserved band",
  );
  assert.doesNotMatch(
    hiddenHtml,
    /aria-label="回到底部"/,
    "the reserved band must remain stable without exposing a hidden action",
  );
  assert.match(
    hiddenHtml,
    /<div class="pointer-events-none relative z-20 h-10 shrink-0"><\/div>/,
    "showing or hiding the action must not resize the feed",
  );
});

test("scroll events resume only while moving down near the bottom", async () => {
  const {
    getConversationViewportSize,
    hasConversationViewportSizeChanged,
    isNearScrollBottom,
    resolveKeyboardFollowScrollIntent,
    resolveTouchFollowScrollIntent,
    resolveConversationViewportResizeState,
    resolveConversationViewportSizeRevision,
    shouldDetachFollowForAtomicGrowth,
    shouldPauseFollowOnScroll,
    shouldResumeFollowOnScroll,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  assert.equal(
    isNearScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 2_000 },
    ),
    false,
    "an intermediate downward animation frame must not disable following",
  );
  assert.equal(
    isNearScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 1_800 },
    ),
    false,
    "Room layout movement must not be mistaken for user upward scrolling",
  );
  assert.equal(
    isNearScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_450 },
    ),
    true,
    "scrolling back near the bottom must restore following",
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
    true,
    "only downward movement back to the bottom edge may resume following",
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
    }),
    { height: 480 },
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
      scrollTop: 1_000,
      shouldFollow: false,
      showScrollToBottom: true,
    },
    "a shrinking viewport must retain the visible content instead of following the new bottom",
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
    "a growing viewport may clamp an existing bottom position without detaching",
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
  assert.equal(
    shouldDetachFollowForAtomicGrowth(
      { clientHeight: 500, scrollHeight: 1_080, scrollTop: 500 },
      1_000,
    ),
    false,
    "small streamed layout growth remains a normal follow target",
  );
  assert.equal(
    shouldDetachFollowForAtomicGrowth(
      { clientHeight: 500, scrollHeight: 1_700, scrollTop: 500 },
      1_000,
    ),
    true,
    "a terminal Room body replacing one summary must not drag the viewport across the whole result",
  );
});

test("Room streaming revisions keep the follow key fresh for non-last Agent output", async () => {
  const { buildConversationScrollContentKey } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  const streaming = assistantMessage({
    agentId: "agent-streaming",
    agentRoundId: "agent-round-streaming",
    messageId: "assistant-streaming",
    text: "第一段",
    timestamp: 1,
  });
  const later = assistantMessage({
    agentId: "agent-later",
    agentRoundId: "agent-round-later",
    messageId: "assistant-later",
    text: "较晚进入数组的并行回复",
    timestamp: 2,
  });

  const before = buildConversationScrollContentKey(
    "room:group:conversation",
    [streaming, later],
  );
  const after = buildConversationScrollContentKey(
    "room:group:conversation",
    [{
      ...streaming,
      content: [{ type: "text", text: "第一段继续输出" }],
    }, later],
  );

  assert.notEqual(
    before,
    after,
    "任意并行 Agent 的流式正文增长都必须触发主 Room 的贴底事务",
  );
});

test("auto follow settles again after virtual Room measurement", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  };
  try {
    const container = {
      clientHeight: 500,
      scrollHeight: 1_000,
      scrollTop: 0,
    };
    const animator = new BottomScrollAnimator(() => container, () => {});
    animator.scroll("auto");
    assert.equal(container.scrollTop, 500);
    assert.equal(
      frames.length,
      1,
      "auto follow needs one post-measurement settlement frame",
    );

    container.scrollHeight = 1_300;
    frames.shift()(performance.now());
    assert.equal(
      container.scrollTop,
      800,
      "virtual list height changes after layout must still finish at the bottom",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("streaming follow keeps one spring while the bottom target grows", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const cancelledFrames = new Set();
  const originalWindow = globalThis.window;
  let nextFrameId = 0;
  globalThis.window = {
    cancelAnimationFrame: (frameId) => {
      cancelledFrames.add(frameId);
    },
    requestAnimationFrame: (callback) => {
      nextFrameId += 1;
      frames.push({ callback, frameId: nextFrameId });
      return nextFrameId;
    },
  };
  const runNextFrame = (timestamp) => {
    while (frames.length > 0) {
      const frame = frames.shift();
      if (!cancelledFrames.has(frame.frameId)) {
        frame.callback(timestamp);
        return true;
      }
    }
    return false;
  };

  try {
    const positions = [];
    const container = {
      clientHeight: 500,
      scrollHeight: 1_000,
      scrollTop: 500,
    };
    const animator = new BottomScrollAnimator(
      () => container,
      (scrollTop) => positions.push(scrollTop),
    );

    animator.follow();
    animator.follow();
    assert.equal(
      frames.length,
      1,
      "multiple revisions before paint must share one animation frame",
    );

    container.scrollHeight = 1_040;
    runNextFrame(0);
    assert.ok(
      container.scrollTop > 500 && container.scrollTop < 540,
      "the first streamed line must be followed smoothly instead of jumping",
    );

    container.scrollHeight = 1_080;
    animator.follow();
    assert.equal(
      cancelledFrames.size,
      0,
      "a growing target must retain the current spring instead of restarting it",
    );

    for (let frame = 1; frame <= 120 && frames.length > 0; frame += 1) {
      animator.follow();
      runNextFrame(frame * (1_000 / 60));
    }
    assert.equal(
      frames.length,
      0,
      "content revisions without height changes must not keep RAF alive",
    );
    assert.ok(
      Math.abs(container.scrollTop - 580) <= 0.001,
      "the retained spring must settle at the latest measured bottom",
    );
    assert.ok(
      positions.every((position, index) =>
        index === 0 || position >= positions[index - 1]
      ),
      "a monotonically growing stream must never move the viewport backward",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("auto follow never reverses across transient bottom measurements", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const originalWindow = globalThis.window;

  try {
    for (const frameIntervalMs of [1_000 / 20, 1_000 / 30]) {
      const frames = [];
      const positions = [];
      globalThis.window = {
        cancelAnimationFrame: () => {},
        requestAnimationFrame: (callback) => {
          frames.push(callback);
          return frames.length;
        },
      };
      const container = {
        clientHeight: 500,
        scrollHeight: 1_000,
        scrollTop: 500,
      };
      const animator = new BottomScrollAnimator(
        () => container,
        (scrollTop) => positions.push(scrollTop),
      );
      const runFrame = (index) => {
        const callback = frames.shift();
        assert.ok(callback, `expected a queued frame at ${frameIntervalMs}ms`);
        callback(index * frameIntervalMs);
      };

      animator.follow();
      container.scrollHeight = 1_080;
      for (let frame = 0; frame < 3; frame += 1) {
        runFrame(frame);
      }
      const beforeTransientShrink = container.scrollTop;

      container.scrollHeight = 1_040;
      animator.follow();
      for (let frame = 3; frame < 6 && frames.length > 0; frame += 1) {
        runFrame(frame);
      }
      assert.ok(
        container.scrollTop >= beforeTransientShrink,
        `${Math.round(1_000 / frameIntervalMs)}fps follow must ignore a temporary lower target`,
      );

      container.scrollHeight = 1_120;
      animator.follow();
      for (
        let frame = 6;
        frame < 180 && frames.length > 0;
        frame += 1
      ) {
        runFrame(frame);
      }
      assert.ok(
        Math.abs(container.scrollTop - 620) <= 0.001,
        `${Math.round(1_000 / frameIntervalMs)}fps follow must settle at the later higher target`,
      );
      assert.ok(
        positions.every((position, index) =>
          index === 0 || position >= positions[index - 1]
        ),
        `${Math.round(1_000 / frameIntervalMs)}fps follow must keep every programmatic write monotonic`,
      );
    }
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("explicit smooth scroll may close against a lower bottom target", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  };

  try {
    const container = {
      clientHeight: 500,
      scrollHeight: 1_080,
      scrollTop: 500,
    };
    const animator = new BottomScrollAnimator(() => container, () => {});
    animator.scroll("smooth");

    for (let frame = 0; frame < 6; frame += 1) {
      frames.shift()(frame * (1_000 / 30));
    }
    assert.ok(container.scrollTop > 540);

    container.scrollHeight = 1_040;
    for (let frame = 6; frame < 180 && frames.length > 0; frame += 1) {
      frames.shift()(frame * (1_000 / 30));
    }
    assert.ok(
      Math.abs(container.scrollTop - 540) <= 0.001,
      "an explicit scroll-to-bottom transaction must use the real lower target",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("auto follow releases its high-water target after a real browser clamp", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  };

  try {
    let scrollHeight = 1_080;
    let scrollTop = 500;
    const container = {
      clientHeight: 500,
      get scrollHeight() {
        return scrollHeight;
      },
      set scrollHeight(value) {
        scrollHeight = value;
        scrollTop = Math.min(scrollTop, value - this.clientHeight);
      },
      get scrollTop() {
        return scrollTop;
      },
      set scrollTop(value) {
        scrollTop = Math.min(value, scrollHeight - this.clientHeight);
      },
    };
    const animator = new BottomScrollAnimator(() => container, () => {});
    animator.follow();
    frames.shift()(0);
    assert.ok(container.scrollTop > 500);

    container.scrollHeight = 1_040;
    for (let frame = 1; frame < 30 && frames.length > 0; frame += 1) {
      frames.shift()(frame * (1_000 / 60));
    }
    assert.equal(container.scrollTop, 540);
    assert.equal(
      frames.length,
      0,
      "a permanent smaller layout must not leave the old high-water RAF spinning",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("streaming spring escapes subpixel scrollTop quantization", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const originalWindow = globalThis.window;
  let nextFrameId = 0;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      nextFrameId += 1;
      frames.push(callback);
      return nextFrameId;
    },
  };

  try {
    let quantizedScrollTop = 498.5;
    const container = {
      clientHeight: 500,
      scrollHeight: 1_000,
      get scrollTop() {
        return quantizedScrollTop;
      },
      set scrollTop(value) {
        quantizedScrollTop = Math.round(value * 2) / 2;
      },
    };
    const animator = new BottomScrollAnimator(() => container, () => {});
    animator.follow();

    for (let frame = 0; frame < 10 && frames.length > 0; frame += 1) {
      const callback = frames.shift();
      callback(frame * (1_000 / 120));
    }

    assert.equal(container.scrollTop, 500);
    assert.equal(
      frames.length,
      0,
      "a rounded scrollTop must settle instead of keeping RAF alive",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("streaming spring treats App resume as a fresh visible frame", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  };
  try {
    const container = {
      clientHeight: 500,
      scrollHeight: 1_500,
      scrollTop: 500,
    };
    const animator = new BottomScrollAnimator(() => container, () => {});
    animator.follow();
    frames.shift()(0);
    const beforeResume = container.scrollTop;
    frames.shift()(5_000);
    assert.ok(
      container.scrollTop - beforeResume < 200,
      "a long minimized/hidden gap must not be consumed as one giant spring step",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("active follow stops before a Composer or App viewport resize", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const observedPositions = [];
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  };
  try {
    const container = {
      clientHeight: 500,
      scrollHeight: 1_080,
      scrollTop: 500,
    };
    const animator = new BottomScrollAnimator(
      () => container,
      (scrollTop) => observedPositions.push(scrollTop),
    );
    animator.follow();
    container.clientHeight = 420;
    frames.shift()(0);

    assert.equal(container.scrollTop, 500);
    assert.deepEqual(
      observedPositions,
      [],
      "the resize guard must stop before the animator writes scrollTop",
    );
    assert.equal(
      frames.length,
      0,
      "viewport resize must terminate the active follow transaction",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("virtual resize correction ignores a long reply crossing the viewport", async () => {
  const {
    resolveConversationVirtualInitialOffset,
    shouldAdjustConversationVirtualScrollPosition,
  } =
    await server.ssrLoadModule(
      "/src/features/conversation/shared/feed/use-conversation-virtual-scroll-policy.ts",
    );
  assert.equal(resolveConversationVirtualInitialOffset(null), 0);
  assert.equal(
    resolveConversationVirtualInitialOffset({ scrollTop: -20 }),
    0,
    "Safari overscroll must not become a negative virtual initial offset",
  );
  assert.equal(
    resolveConversationVirtualInitialOffset({ scrollTop: 640 }),
    640,
    "static-to-virtual switching must inherit the existing viewport offset",
  );
  assert.equal(
    shouldAdjustConversationVirtualScrollPosition(
      { end: 500 },
      28,
      { scrollOffset: 500 },
    ),
    true,
    "a round fully above the viewport must preserve the visible anchor",
  );
  assert.equal(
    shouldAdjustConversationVirtualScrollPosition(
      { end: 900 },
      28,
      { scrollOffset: 500 },
    ),
    false,
    "growth at the tail of a visible long reply must not push paused reading",
  );
});

test("non-virtual content growth preserves the first visible Room round", async () => {
  const { ConversationViewportAnchor } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/conversation-viewport-anchor.ts",
  );
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

test("viewport anchor survives a static-to-virtual Room feed switch", async () => {
  const { ConversationViewportAnchor } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/conversation-viewport-anchor.ts",
  );
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

test("Room topology and atomic layout revisions exclude token speed", async () => {
  const {
    buildConversationAtomicLayoutKey,
    buildConversationScrollTopologyKey,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  const streamingMessage = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-1",
    content: [{ type: "text", text: "第一段" }],
    message_id: "assistant-1",
    role: "assistant",
    round_id: "root-1",
    session_key: "room:group:conversation-1",
    stream_status: "streaming",
    timestamp: 1,
  };
  const topologyBefore = buildConversationScrollTopologyKey(
    "room:group:conversation-1",
    [streamingMessage],
    [],
  );
  const topologyAfterToken = buildConversationScrollTopologyKey(
    "room:group:conversation-1",
    [{
      ...streamingMessage,
      content: [{ type: "text", text: "第一段继续增长" }],
    }],
    [],
  );
  assert.equal(
    topologyAfterToken,
    topologyBefore,
    "real token growth must not look like a structural insertion",
  );
  assert.notEqual(
    buildConversationScrollTopologyKey(
      "room:group:conversation-1",
      [streamingMessage],
      [{
        agent_id: "agent-2",
        agent_round_id: "agent-round-2",
        msg_id: "slot-2",
        round_id: "historical-root",
        status: "pending",
        timestamp: 2,
      }],
    ),
    topologyBefore,
    "a new Room member slot must change the topology revision",
  );

  const permission = {
    request_id: "permission-1",
    tool_input: { command: "echo one" },
    tool_name: "Bash",
  };
  const atomicBefore = buildConversationAtomicLayoutKey(
    "room:group:conversation-1",
    [streamingMessage],
    [permission],
  );
  assert.notEqual(
    buildConversationAtomicLayoutKey(
      "room:group:conversation-1",
      [streamingMessage],
      [{ ...permission, request_id: "permission-2" }],
    ),
    atomicBefore,
    "equal permission counts with a different request still change layout identity",
  );
  assert.notEqual(
    buildConversationAtomicLayoutKey(
      "room:group:conversation-1",
      [{ ...streamingMessage, stream_status: "done" }],
      [permission],
    ),
    atomicBefore,
    "terminal component replacement must be an explicit atomic revision",
  );
});

test("Room handles every pending runtime human interaction without opening Thread", async () => {
  const { GroupAgentStatusCard } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-status-card.tsx",
  );
  const { GroupConversationRound } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-round.tsx",
  );
  const { resolveGroupConversationRound } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-feed-model.ts",
  );
  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const { ThreadControlContext } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/group-thread-state.ts",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const permission = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-1",
    interaction_mode: "permission",
    request_id: "permission-1",
    risk_label: "执行命令",
    risk_level: "medium",
    round_id: "round-root",
    summary: "需要人工确认",
    tool_input: { command: "echo permission-required" },
    tool_name: "Bash",
  };
  const questionPermission = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-1",
    interaction_mode: "question",
    request_id: "question-1",
    round_id: "round-root",
    summary: "请选择研究口径",
    tool_input: {
      questions: [{
        header: "研究口径",
        multiSelect: false,
        options: [
          { label: "保守", description: "优先采用可验证数据" },
          { label: "积极", description: "纳入前瞻性假设" },
        ],
        question: "这次分析采用哪种研究口径？",
      }],
    },
    tool_name: "AskUserQuestion",
    tool_use_id: "tool-question",
  };
  const planConfirmation = {
    ...permission,
    request_id: "plan-confirmation-1",
    summary: "确认按这份计划继续执行",
    tool_input: {
      plan: "先验证数据源，再生成最终报告。",
    },
    tool_name: "ExitPlanMode",
    tool_use_id: "tool-plan-confirmation",
  };
  const futureApproval = {
    ...permission,
    interaction_mode: "future_review",
    request_id: "future-approval-1",
    summary: "确认发布研究结果",
    tool_input: {
      description: "将报告发布到共享工作区。",
    },
    tool_name: "RequestHumanReview",
    tool_use_id: "tool-future-approval",
  };
  const provider = (child) => React.createElement(
    I18nProvider,
    null,
    React.createElement(
      ThreadControlContext.Provider,
      {
        value: {
          activeThread: null,
          closeThread: () => {},
          openThread: () => {},
        },
      },
      child,
    ),
  );

  const agentCardHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentStatusCard,
    {
      agentAvatar: null,
      agentId: "agent-1",
      agentName: "Dev",
      isThreadActive: false,
      messages: [],
      onClickThread: () => {},
      onPermissionResponse: () => true,
      pendingPermissions: [
        permission,
        questionPermission,
        planConfirmation,
        futureApproval,
      ],
      status: "pending",
      timestamp: 1,
    },
  )));
  assert.match(
    agentCardHtml,
    /echo permission-required/,
    "Agent 卡片必须直接展示待审批操作的具体内容",
  );
  assert.match(agentCardHtml, />允许</);
  assert.match(agentCardHtml, />拒绝</);
  assert.match(
    agentCardHtml,
    /这次分析采用哪种研究口径？/,
    "结构化问题必须直接在活动 Agent 卡片中作答",
  );
  assert.match(agentCardHtml, /保守/);
  assert.match(agentCardHtml, /继续协作/);
  assert.match(
    agentCardHtml,
    /先验证数据源，再生成最终报告/,
    "计划确认必须与普通工具审批一样直接出现在公区",
  );
  assert.match(
    agentCardHtml,
    /将报告发布到共享工作区/,
    "未知的新人工审批类型必须回退到公区通用控件",
  );
  assert.match(
    agentCardHtml,
    /data-human-interaction-surface/,
    "统一人工介入边界必须由公共渲染入口标记",
  );
  assert.doesNotMatch(
    agentCardHtml,
    />去回答</,
    "公区不得再用跳转 Thread 代替完整回答控件",
  );

  const permissionOnlyRoundHtml = renderToStaticMarkup(provider(
    React.createElement(GroupConversationRound, {
      renderer: {
        agentAvatarMap: {},
        agentNameMap: {},
        currentAgentAvatar: null,
        currentAgentName: "Dev",
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [permission],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        runtimePhase: null,
      },
      state: {
        index: 0,
        isLast: true,
        isLive: true,
        isLoaded: true,
        messages: [],
        pendingPermissions: [
          permission,
          questionPermission,
          planConfirmation,
          futureApproval,
        ],
        pendingSlots: [],
        rootRoundId: "round-root",
        roundId: "round-root",
      },
    }),
  ));
  assert.match(
    permissionOnlyRoundHtml,
    /echo permission-required/,
    "权限先于 Agent 消息到达时，主 Room 也不能丢失审批入口",
  );
  assert.match(
    permissionOnlyRoundHtml,
    /这次分析采用哪种研究口径？/,
    "问题先于 Agent 消息到达时，主 Room 也必须保留完整回答入口",
  );
  assert.match(permissionOnlyRoundHtml, /先验证数据源，再生成最终报告/);
  assert.match(permissionOnlyRoundHtml, /将报告发布到共享工作区/);

  const completedToolMessage = {
    ...assistantMessage({
      agentId: "agent-1",
      agentRoundId: "agent-round-1",
      isComplete: true,
      messageId: "assistant-tool-call",
      model: "glm-5.2",
      roundId: "round-root",
      status: "done",
      stopReason: "tool_use",
      text: "Goal 已设定，现在开始调研。",
      timestamp: 2,
    }),
    content: [
      { type: "text", text: "Goal 已设定，现在开始调研。" },
      {
        type: "tool_use",
        id: "tool-search",
        input: { query: "Apple M3 vs M4 vs M5 chip comparison specifications" },
        name: "WebSearch",
      },
      {
        type: "tool_use",
        id: "tool-question",
        input: questionPermission.tool_input,
        name: "AskUserQuestion",
      },
    ],
  };
  const completedPermission = {
    ...permission,
    message_id: "assistant-tool-call",
    request_id: "permission-search",
    summary: "Apple M3 vs M4 vs M5 chip comparison specifications",
    tool_input: {
      query: "Apple M3 vs M4 vs M5 chip comparison specifications",
    },
    tool_name: "WebSearch",
    tool_use_id: "tool-search",
  };
  const completedProjection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [completedToolMessage]]]),
    pendingPermissionGroups: new Map([
      ["round-root", [completedPermission, questionPermission]],
    ]),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root"],
  });
  const completedState = resolveGroupConversationRound({
    liveRoundIds: ["round-root"],
    messageGroups: completedProjection.messageGroups,
    pendingPermissionGroups: completedProjection.pendingPermissionGroups,
    pendingSlotGroups: completedProjection.pendingSlotGroups,
    rootRoundIds: completedProjection.rootRoundIds,
    roundIds: completedProjection.roundIds,
  }, 0);
  const completedRoundHtml = renderToStaticMarkup(provider(
    React.createElement(GroupConversationRound, {
      renderer: {
        agentAvatarMap: {},
        agentNameMap: { "agent-1": "Kevin" },
        currentAgentAvatar: null,
        currentAgentName: "Kevin",
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [
          completedPermission,
          questionPermission,
        ],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        runtimePhase: null,
      },
      state: completedState,
    }),
  ));
  assert.match(
    completedRoundHtml,
    /Apple M3 vs M4 vs M5 chip comparison specifications/,
    "工具消息进入完成态后，主 Room 仍必须展示待确认操作",
  );
  assert.match(completedRoundHtml, /这次分析采用哪种研究口径？/);
  assert.equal(
    completedRoundHtml.match(/这次分析采用哪种研究口径？/g)?.length,
    1,
    "a terminal Room card must not render the same pending question twice",
  );
  assert.match(completedRoundHtml, />允许</);
  assert.match(completedRoundHtml, />拒绝</);
  assert.match(completedRoundHtml, /继续协作/);
  assert.equal(
    completedRoundHtml.match(/data-human-interaction-surface/g)?.length,
    1,
    "the hidden terminal tool and its standalone interaction must share one surface",
  );
  assert.doesNotMatch(completedRoundHtml, /等待提问就绪/);

  const questionOnlyMessage = {
    ...completedToolMessage,
    message_id: "assistant-question-only",
    content: [{
      type: "tool_use",
      id: "tool-question",
      input: questionPermission.tool_input,
      name: "AskUserQuestion",
    }],
  };
  const questionOnlyPermission = {
    ...questionPermission,
    message_id: "assistant-question-only",
  };
  const questionOnlyProjection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [questionOnlyMessage]]]),
    pendingPermissionGroups: new Map([
      ["round-root", [questionOnlyPermission]],
    ]),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root"],
  });
  const questionOnlyState = resolveGroupConversationRound({
    liveRoundIds: ["round-root"],
    messageGroups: questionOnlyProjection.messageGroups,
    pendingPermissionGroups: questionOnlyProjection.pendingPermissionGroups,
    pendingSlotGroups: questionOnlyProjection.pendingSlotGroups,
    rootRoundIds: questionOnlyProjection.rootRoundIds,
    roundIds: questionOnlyProjection.roundIds,
  }, 0);
  const questionOnlyHtml = renderToStaticMarkup(provider(
    React.createElement(GroupConversationRound, {
      renderer: {
        agentAvatarMap: {},
        agentNameMap: { "agent-1": "Kevin" },
        currentAgentAvatar: null,
        currentAgentName: "Kevin",
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [questionOnlyPermission],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        runtimePhase: null,
      },
      state: questionOnlyState,
    }),
  ));
  assert.equal(
    questionOnlyHtml.match(/这次分析采用哪种研究口径？/g)?.length,
    1,
    "a visible terminal question tool must become the sole interaction surface",
  );
  assert.match(questionOnlyHtml, /保守/);
  assert.match(questionOnlyHtml, /继续协作/);
  assert.match(questionOnlyHtml, />拒绝</);
  assert.equal(
    questionOnlyHtml.match(/data-human-interaction-surface/g)?.length,
    1,
  );
  assert.doesNotMatch(questionOnlyHtml, /等待提问就绪/);
});

test("Room keeps tool-time summary and shows the terminal result in the same slot", async () => {
  const { GroupAgentReply } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-reply.tsx",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const tailMarker = "STREAM_TAIL_VISIBLE_AFTER_EIGHTY_CHARS";
  const text = `${"逐步输出的正文。".repeat(18)}${tailMarker}`;
  const message = assistantMessage({
    agentId: "agent-stream",
    agentRoundId: "agent-round-stream",
    messageId: "assistant-stream",
    status: "streaming",
    text,
    timestamp: 2,
  });
  const entry = {
    agentAvatar: null,
    agentName: "Stream Agent",
    agent_id: "agent-stream",
    agent_round_id: "agent-round-stream",
    assistant_messages: [message],
    display_order: 0,
    entry_id: "agent-stream:agent-round:agent-round-stream",
    guidedUserMessages: [],
    pendingPermissions: [],
    pending_slot: {
      agent_id: "agent-stream",
      agent_round_id: "agent-round-stream",
      index: 0,
      msg_id: "slot-stream",
      round_id: "round-root",
      status: "streaming",
      timestamp: 1,
    },
    status: "streaming",
    stopAgentRoundId: "agent-round-stream",
    timestamp: 1,
  };
  const renderReply = (nextEntry) => renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(GroupAgentReply, {
        entry: nextEntry,
        isThreadActive: false,
        onClickThread: () => {},
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        roundId: "round-root",
      }),
    ),
  );
  const activeHtml = renderReply(entry);
  const resultSummary = {
    duration_api_ms: 10,
    duration_ms: 20,
    is_error: false,
    message_id: "result-stream",
    num_turns: 1,
    result: text,
    subtype: "success",
    timestamp: 3,
  };
  const terminalHtml = renderReply({
    ...entry,
    assistant_messages: [{
      ...message,
      is_complete: true,
      result_summary: resultSummary,
      stop_reason: "end_turn",
      stream_status: "done",
      timestamp: 3,
    }],
    pending_slot: {
      ...entry.pending_slot,
      status: "done",
    },
    result_summary: resultSummary,
    status: "done",
    timestamp: 3,
  });

  assert.doesNotMatch(
    activeHtml,
    new RegExp(tailMarker),
    "tool-time Room content stays behind the bounded execution summary",
  );
  assert.match(activeHtml, /line-clamp-1/);
  assert.match(
    terminalHtml,
    new RegExp(tailMarker),
    "the public terminal result must be complete as soon as the backend snapshot arrives",
  );
  assert.doesNotMatch(
    terminalHtml,
    /line-clamp-1/,
    "the same Room slot switches from its bounded summary to the full result",
  );
  const statusBeforeResultHtml = renderReply({
    ...entry,
    pending_slot: {
      ...entry.pending_slot,
      status: "done",
    },
    status: "done",
  });
  assert.match(
    statusBeforeResultHtml,
    /line-clamp-1/,
    "terminal lifecycle status alone must keep the running card until its terminal message arrives",
  );
  assert.doesNotMatch(
    statusBeforeResultHtml,
    new RegExp(tailMarker),
    "the result view must wait for terminal message evidence",
  );
});

test("resolved history rounds remain only when visible content was projected", async () => {
  const {
    buildIndexedTimelineRoundIds,
    filterResolvedEmptyRoundIndexItems,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/timeline-model.ts",
  );
  const visible = roundIndexItem("round-visible");
  const internal = roundIndexItem("goal_continuation_private");

  const unresolvedItems = filterResolvedEmptyRoundIndexItems(
    [visible, internal],
    [visible.roundId],
    [],
  );
  assert.deepEqual(
    buildIndexedTimelineRoundIds(unresolvedItems, [visible.roundId]),
    [visible.roundId, internal.roundId],
    "an unresolved neighbor remains as an invisible history load anchor",
  );

  const resolvedEmptyItems = filterResolvedEmptyRoundIndexItems(
    [visible, internal],
    [visible.roundId],
    [internal.roundId],
  );
  assert.deepEqual(
    resolvedEmptyItems.map((item) => item.roundId),
    [visible.roundId],
    "a resolved round with no visible content must leave no placeholder",
  );

  const resolvedVisibleItems = filterResolvedEmptyRoundIndexItems(
    [visible, internal],
    [visible.roundId, internal.roundId],
    [internal.roundId],
  );
  assert.deepEqual(
    resolvedVisibleItems.map((item) => item.roundId),
    [visible.roundId, internal.roundId],
    "a resolved round with visible content stays for the real message card",
  );
});

test("deferred input ACK keeps queued user text out of the timeline", async () => {
  const { replaceOptimisticUserMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const optimistic = userMessage({
    content: "这条还没有被智能体消费",
    messageId: "local-message",
    roundId: "local-message",
    timestamp: 1,
  });

  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimistic],
      "local-message",
      "user-message",
      "round-message",
      false,
    ),
    [],
    "a queued ACK must remove the optimistic timeline message",
  );
  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimistic],
      "local-message",
      "user-message",
      "round-message",
      true,
    ).map(({ message_id, round_id }) => ({ message_id, round_id })),
    [{ message_id: "user-message", round_id: "round-message" }],
    "a committed ACK still canonicalizes normal user messages",
  );
});

test("deferred ACK cannot remove an already applied canonical user message", async () => {
  const { replaceOptimisticUserMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const optimistic = userMessage({
    content: "这条正在等待 ACK",
    messageId: "local-message",
    roundId: "local-message",
    timestamp: 1,
  });
  const canonical = userMessage({
    content: "这条已经被智能体消费",
    messageId: "user-message",
    roundId: "round-message",
    timestamp: 2,
  });

  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimistic, canonical],
      "local-message",
      "user-message",
      "round-message",
      false,
    ).map(({ message_id, round_id }) => ({ message_id, round_id })),
    [{ message_id: "user-message", round_id: "round-message" }],
    "a late deferred ACK must remove only the optimistic copy",
  );
});

test("Room pending slot keeps the backend display index", async () => {
  const {
    mergeChatAckPendingSlots,
    updatePendingAgentSlotStatus,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const slots = mergeChatAckPendingSlots([], {
    pending: [{
      agent_id: "agent-1",
      agent_round_id: "agent-round-1",
      index: 7,
      msg_id: "slot-1",
      round_id: "round-slot-root",
      status: "streaming",
      timestamp: 10,
    }],
    pending_snapshot: true,
    round_id: "round-root",
  });

  assert.equal(slots[0]?.index, 7);
  assert.equal(
    slots[0]?.round_id,
    "round-slot-root",
    "a per-slot root must win over the aggregate snapshot fallback",
  );

  const laterWake = mergeChatAckPendingSlots(slots, {
    pending: [{
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      index: 0,
      msg_id: "slot-2",
      status: "pending",
      timestamp: 20,
    }],
    pending_snapshot: false,
    round_id: "round-slot-root",
  });
  assert.deepEqual(
    laterWake.map(({ agent_round_id, round_id }) => ({
      agent_round_id,
      round_id,
    })),
    [
      { agent_round_id: "agent-round-1", round_id: "round-slot-root" },
      { agent_round_id: "agent-round-2", round_id: "round-slot-root" },
    ],
    "a later public wake in the same root must append without replacing the earlier slot",
  );
  assert.deepEqual(
    updatePendingAgentSlotStatus(
      laterWake,
      "slot-2",
      "streaming",
      "internal-wake-round",
    ).map(({ agent_round_id, round_id, status }) => ({
      agent_round_id,
      round_id,
      status,
    })),
    [
      {
        agent_round_id: "agent-round-1",
        round_id: "round-slot-root",
        status: "streaming",
      },
      {
        agent_round_id: "agent-round-2",
        round_id: "round-slot-root",
        status: "streaming",
      },
    ],
    "stream_start must advance status without moving the slot to another feed root",
  );
});

test("authoritative Room slot snapshots rebuild runtime trackers by root", async () => {
  const { AgentConversationRuntimeMachine } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/agent-conversation-runtime-machine.ts",
  );
  const machine = new AgentConversationRuntimeMachine("group");
  const baseAck = {
    ack_timeout_ms: 10_000,
    client_message_id: "",
    client_request_id: "",
    pending_snapshot: true,
    round_id: "",
    user_message_committed: false,
    user_message_id: "",
  };
  machine.trackChatAck({
    ...baseAck,
    pending: [
      {
        agent_id: "agent-a",
        agent_round_id: "agent-round-a",
        index: 0,
        msg_id: "slot-a",
        round_id: "root-a",
        status: "streaming",
        timestamp: 10,
      },
      {
        agent_id: "agent-b",
        agent_round_id: "agent-round-b",
        index: 0,
        msg_id: "slot-b",
        round_id: "root-b",
        status: "pending",
        timestamp: 20,
      },
    ],
  });
  machine.emit();
  assert.equal(machine.snapshot().phase, "streaming");
  assert.deepEqual(
    new Set(machine.snapshot().liveRoundIds),
    new Set(["root-a", "root-b"]),
  );

  machine.trackChatAck({
    ...baseAck,
    pending: [],
  });
  machine.emit();
  assert.equal(machine.snapshot().phase, "idle");
  assert.deepEqual(machine.snapshot().liveRoundIds, []);

  machine.trackChatAck({
    ...baseAck,
    pending_snapshot: false,
    round_id: "root-a",
    pending: [{
      agent_id: "agent-a",
      agent_round_id: "agent-round-a",
      index: 0,
      msg_id: "slot-a",
      status: "pending",
      timestamp: 30,
    }],
  });
  machine.trackChatAck({
    ...baseAck,
    pending_snapshot: false,
    round_id: "root-b",
    pending: [{
      agent_id: "agent-b",
      agent_round_id: "agent-round-b",
      index: 0,
      msg_id: "slot-b",
      status: "pending",
      timestamp: 40,
    }],
  });
  machine.emit();
  assert.deepEqual(
    new Set(machine.snapshot().liveRoundIds),
    new Set(["root-a", "root-b"]),
    "ordinary server ACKs must append without clearing earlier active slots",
  );
});

test("Room terminal agent status keeps its slot until a message or root takes over", async () => {
  const {
    cancelRunningAgentSlots,
    filterRoundPendingAgentSlots,
    reconcileAgentRoundPendingSlots,
    reconcilePendingSlotsWithAssistantMessage,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const runningSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stopped",
    msg_id: "slot-stopped",
    round_id: "round-root",
    status: "streaming",
    timestamp: 10,
  };
  const terminalCases = [
    ["finished", "done"],
    ["interrupted", "cancelled"],
    ["error", "error"],
  ];
  for (const [eventStatus, slotStatus] of terminalCases) {
    assert.deepEqual(
      reconcileAgentRoundPendingSlots(
        [runningSlot],
        "agent-round-stopped",
        eventStatus,
      ),
      [{ ...runningSlot, status: slotStatus }],
      `${eventStatus} must keep the same visible slot as ${slotStatus}`,
    );
  }

  const cancelledSlot = {
    ...runningSlot,
    status: "cancelled",
  };

  assert.deepEqual(
    reconcileAgentRoundPendingSlots(
      [cancelledSlot],
      "agent-round-stopped",
      "running",
    ),
    [cancelledSlot],
    "迟到的 non-terminal 事件不能把已停止槽位改回 streaming",
  );
  const doneSlot = {
    ...runningSlot,
    status: "done",
  };
  assert.deepEqual(
    cancelRunningAgentSlots([doneSlot]),
    [doneSlot],
    "session status settlement must not downgrade a finished slot to cancelled",
  );

  const terminalMessage = assistantMessage({
    agentRoundId: "agent-round-stopped",
    isComplete: true,
    messageId: "assistant-terminal",
    roundId: "round-root",
    status: "done",
    stopReason: "end_turn",
    text: "终态正文",
    timestamp: 11,
  });
  assert.deepEqual(
    reconcilePendingSlotsWithAssistantMessage([cancelledSlot], terminalMessage),
    [],
    "terminal message/result must atomically replace the retained slot",
  );
  assert.deepEqual(
    reconcilePendingSlotsWithAssistantMessage(
      [runningSlot],
      assistantMessage({
        agentRoundId: "agent-round-stopped",
        messageId: "assistant-streaming",
        roundId: "round-root",
        status: "streaming",
        text: "仍在流式输出",
        timestamp: 11,
      }),
    ),
    [runningSlot],
    "streaming assistant still needs the slot's stable index and start time",
  );
  assert.deepEqual(
    filterRoundPendingAgentSlots([cancelledSlot], "round-root"),
    [],
    "root round terminal status remains the final cleanup boundary",
  );
});

test("Room no-reply terminal status closes its published thinking snapshot", async () => {
  const {
    applyTerminalAgentRoundMessageStatus,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const {
    buildRoomThreadPanelModel,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/live/room-thread-panel-model.ts",
  );
  const thinkingSnapshot = {
    agent_id: "agent-lucy",
    agent_round_id: "agent-round-no-reply",
    content: [{ type: "thinking", thinking: "判断是否需要公开回复" }],
    is_complete: false,
    message_id: "assistant-no-reply",
    role: "assistant",
    round_id: "round-root",
    session_key: "room:group:conversation",
    stream_status: "streaming",
    timestamp: 10,
  };
  const unrelatedSnapshot = {
    ...thinkingSnapshot,
    agent_id: "agent-amy",
    agent_round_id: "agent-round-active",
    message_id: "assistant-active",
  };

  const reconciled = applyTerminalAgentRoundMessageStatus(
    [thinkingSnapshot, unrelatedSnapshot],
    "agent-round-no-reply",
    "finished",
  );

  assert.equal(
    reconciled[0]?.stream_status,
    "done",
    "no-reply 没有最终消息时也必须结束已经发布的 thinking 快照",
  );
  assert.equal(
    reconciled[1],
    unrelatedSnapshot,
    "slot 终态只能收口精确匹配的 agent_round_id",
  );
  const thread = buildRoomThreadPanelModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-lucy": "Lucy" },
    currentUserAvatar: null,
    messageGroups: new Map([["round-root", reconciled]]),
    onPermissionResponse: () => true,
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
  }, {
    agentId: "agent-lucy",
    agentRoundId: "agent-round-no-reply",
    roundId: "round-root",
  });
  assert.equal(
    thread?.isLoading,
    false,
    "Lucy Thread 不应在 no-reply 终态后继续显示正在思考",
  );
});

test("Room pending queue shows only user-authored guidance", async () => {
  const { projectRoomPendingInputQueueItems } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/panel/controller/group-chat-panel-projection.ts",
  );
  const items = [
    { id: "user", source: "user" },
    { id: "public-mention", source: "agent_public_mention" },
    { id: "directed-message", source: "agent_room_directed_message" },
  ];

  assert.deepEqual(
    projectRoomPendingInputQueueItems(items).map((item) => item.id),
    ["user"],
  );
});

test("blocked goals stay inline instead of opening a resume confirmation", async () => {
  const { buildGoalControllerProjection } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-model.ts",
  );
  const goal = {
    continuation_count: 1,
    created_at: "2026-07-14T00:00:00Z",
    empty_progress_count: 3,
    id: "goal-1",
    objective: "Replace this objective directly",
    session_key: "room:group:conversation-1",
    status: "blocked",
    updated_at: "2026-07-14T00:01:00Z",
    version: 2,
  };
  const projection = buildGoalControllerProjection({
    dialog: { goal, kind: "resume" },
    draft: null,
    goal,
    phase: null,
  });

  assert.equal(projection.canResume, true);
  assert.deepEqual(projection.dialog, { kind: "none" });
});

test("Room no-reply control markers never become visible assistant blocks", async () => {
  const { buildVisibleOrderedAssistantEntries } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts",
  );
  const entries = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds: new Set(),
    isLoading: false,
    mergedContent: [{ type: "text", text: "<nexus_room_no_reply/>" }],
    mergedContentSourceMessageIds: ["assistant-no-reply"],
    sourceMessageOrderById: new Map([["assistant-no-reply", 0]]),
    systemEventBlocks: [],
  });

  assert.deepEqual(entries, []);
});

test("recoverable malformed tool results stay out of the user timeline", async () => {
  const {
    buildVisibleOrderedAssistantEntries,
    collectHiddenToolUseIds,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts",
  );
  const content = [
    {
      type: "tool_use",
      id: "tool-malformed",
      name: "WebFetch",
      input: {},
      metadata: {
        _nexus_internal_kind: "malformed_tool_input",
      },
    },
    {
      type: "tool_result",
      tool_use_id: "tool-malformed",
      content: "Tool input was not valid JSON",
      is_error: true,
      metadata: {
        _nexus_internal_kind: "malformed_tool_input",
      },
    },
    { type: "text", text: "模型已自行修正并继续。" },
  ];
  const hiddenToolUseIds = collectHiddenToolUseIds(content, new Set());
  const entries = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds,
    isLoading: false,
    mergedContent: content,
    mergedContentSourceMessageIds: ["assistant-1", "assistant-2", "assistant-3"],
    sourceMessageOrderById: new Map([
      ["assistant-1", 0],
      ["assistant-2", 1],
      ["assistant-3", 2],
    ]),
    systemEventBlocks: [],
  });

  assert.deepEqual([...hiddenToolUseIds], ["tool-malformed"]);
  assert.deepEqual(
    entries.map(({ block }) => block),
    [{ type: "text", text: "模型已自行修正并继续。" }],
  );
});

test("recoverable malformed tool results stay out of process error counts", async () => {
  const { buildProcessSummary } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/message-process-summary.ts",
  );
  const summary = buildProcessSummary({
    pendingPermissionCount: 0,
    processContent: [
      {
        type: "tool_use",
        id: "tool-malformed",
        name: "WebFetch",
        input: {},
        metadata: {
          _nexus_internal_kind: "malformed_tool_input",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-malformed",
        content: "Tool input was not valid JSON",
        is_error: true,
        metadata: {
          _nexus_internal_kind: "malformed_tool_input",
        },
      },
    ],
  });

  assert.equal(summary, "查看过程");
});

test("recoverable malformed tool use does not keep the activity indicator busy", async () => {
  const { resolveContentActivityState } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/activity/message-content-activity.ts",
  );
  assert.equal(
    resolveContentActivityState({
      consumedBlockIndexes: new Set(),
      content: [{
        type: "tool_use",
        id: "tool-malformed",
        name: "WebFetch",
        input: {},
        metadata: {
          _nexus_internal_kind: "malformed_tool_input",
        },
      }],
      hiddenToolNames: new Set(),
      resolvedToolUseIds: new Set(),
    }),
    "thinking",
  );
});

test("history restores only the latest assistant round error", async () => {
  const {
    DEFAULT_ASSISTANT_ERROR_MESSAGE,
    latestAssistantResultErrorMessage,
    resolveAssistantResultErrorMessage,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/message/assistant-message-model.ts",
  );
  const failed = assistantMessage({
    messageId: "assistant-failed",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      errors: ["", "provider stream failed"],
      is_error: true,
      num_turns: 1,
      subtype: "error",
      timestamp: 2,
    },
    roundId: "round-failed",
    text: "",
    timestamp: 2,
  });

  assert.equal(
    latestAssistantResultErrorMessage([failed]),
    "provider stream failed",
  );
  assert.equal(
    latestAssistantResultErrorMessage([
      failed,
      assistantMessage({
        messageId: "assistant-retrying",
        roundId: "round-retrying",
        text: "正在重试",
        timestamp: 3,
      }),
    ]),
    null,
    "a newer active round must suppress the previous terminal error",
  );
  assert.equal(
    latestAssistantResultErrorMessage([
      assistantMessage({
        messageId: "assistant-room-failed",
        roundId: "room-round-1",
        resultSummary: {
          duration_api_ms: 0,
          duration_ms: 0,
          errors: ["slot provider failed"],
          is_error: true,
          num_turns: 1,
          subtype: "error",
          timestamp: 4,
        },
        text: "",
        timestamp: 4,
      }),
      assistantMessage({
        messageId: "assistant-room-success",
        roundId: "room-round-1",
        resultSummary: {
          duration_api_ms: 0,
          duration_ms: 0,
          is_error: false,
          num_turns: 1,
          subtype: "success",
          timestamp: 5,
        },
        text: "另一个 Agent 完成",
        timestamp: 5,
      }),
    ]),
    "slot provider failed",
    "same root round must retain a failing Room slot",
  );
  assert.equal(
    resolveAssistantResultErrorMessage({
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: true,
      num_turns: 0,
      subtype: "error",
    }),
    DEFAULT_ASSISTANT_ERROR_MESSAGE,
  );
});

test("terminal round status keeps its displayable error message", async () => {
  const { parseRoundStatusEventPayload } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );

  assert.deepEqual(
    parseRoundStatusEventPayload({
      is_terminal: true,
      message: "query: provider request failed",
      result_subtype: "error",
      round_id: "round-error",
      status: "error",
    }),
    {
      error_message: "query: provider request failed",
      is_terminal: true,
      result_subtype: "error",
      round_id: "round-error",
      status: "error",
    },
  );
});

test("Room no-reply control markers stay out of previews and result summaries", async () => {
  const { extractAgentPreviewText } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const { buildGroupAgentStatusModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const marker = "<nexus_room_no_reply/>";

  assert.equal(
    extractAgentPreviewText([assistantMessage({ text: marker, timestamp: 1 })]),
    "",
  );

  const status = buildGroupAgentStatusModel({
    labels: {
      failed: "Failed",
      stopped: "Stopped",
      waitingForUser: "Waiting",
    },
    messages: [],
    pendingPermissions: [],
    resultSummary: {
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: false,
      num_turns: 1,
      result: marker,
      subtype: "interrupted",
      timestamp: 1,
    },
    status: "cancelled",
  });
  assert.equal(status.summaryText, "Stopped");

  const noReplyEntry = {
    assistant_messages: [{
      ...assistantMessage({
        messageId: "assistant-no-reply",
        text: "",
        timestamp: 1,
      }),
      content: [{
        thinking: "仅供 Thread 查看",
        type: "thinking",
      }],
    }],
    result_summary: undefined,
    status: "done",
  };
  const { isNoPublicReplyAgentEntry } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  assert.equal(isNoPublicReplyAgentEntry(noReplyEntry), true);
});

test("Room public cards hide thinking while Thread keeps it available", async () => {
  const { extractAgentPreviewText } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    resolveMessageItemFinalProjection,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts",
  );
  const thinking = {
    thinking: "这段只应在 Lucy Thread 中显示",
    type: "thinking",
  };
  const assistant = assistantMessage({
    messageId: "assistant-thinking-only",
    text: "",
    timestamp: 1,
  });
  assistant.content = [thinking];

  assert.equal(
    extractAgentPreviewText([assistant]),
    "",
    "Room 公区状态卡不能把 thinking 当作回复摘要",
  );
  const projection = resolveMessageItemFinalProjection({
    assistantContentMode: "room_result",
    assistantMessages: [assistant],
    orderedProjection: {
      content: [thinking],
      streamingIndexes: new Set(),
    },
    resultSummary: undefined,
    roundId: "round-root",
    streamingBlockIndexes: new Set(),
    visibleAssistantTurns: [{
      content: [thinking],
      messageId: assistant.message_id,
      streamingIndexes: new Set(),
      textContent: [],
      textStreamingIndexes: new Set(),
    }],
    visibleOrderedAssistantEntries: [{
      block: thinking,
      mergedIndex: 0,
      sourceMessageId: assistant.message_id,
      sourceOrder: 0,
    }],
  });
  assert.equal(
    projection.finalAssistantContent,
    null,
    "Room 已完成卡片不能把 thinking 作为最终公区正文",
  );
});

test("Room no-reply keeps the completed MessageItem visual shell", async () => {
  const { GroupAgentReply } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-reply.tsx",
  );
  const { ThreadControlContext } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/group-thread-state.ts",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const entry = {
    agentAvatar: null,
    agent_id: "agent-lucy",
    agentName: "Lucy",
    agent_round_id: "agent-round-lucy",
    assistant_messages: [{
      ...assistantMessage({
        agentId: "agent-lucy",
        agentRoundId: "agent-round-lucy",
        messageId: "assistant-lucy",
        model: "glm-4.7",
        resultSummary: {
          duration_api_ms: 10,
          duration_ms: 100,
          is_error: false,
          num_turns: 1,
          result: "<nexus_room_no_reply/>",
          subtype: "success",
          timestamp: 2,
        },
        status: "done",
        timestamp: 2,
      }),
      content: [{
        thinking: "仅供 Thread 查看",
        type: "thinking",
      }],
    }],
    entry_id: "agent-lucy:agent-round-lucy",
    guidedUserMessages: [],
    pendingPermissions: [],
    pending_slot: null,
    result_summary: {
      duration_api_ms: 10,
      duration_ms: 100,
      is_error: false,
      num_turns: 1,
      result: "<nexus_room_no_reply/>",
      subtype: "success",
      timestamp: 2,
    },
    status: "done",
    stopAgentRoundId: null,
    timestamp: 2,
  };
  const html = renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(
        ThreadControlContext.Provider,
        {
          value: {
            activeThread: null,
            closeThread: () => {},
            openThread: () => {},
          },
        },
        React.createElement(GroupAgentReply, {
          agentMentionDirectory: { avatars: {}, names: {} },
          entry,
          isThreadActive: false,
          onClickThread: () => {},
          onPermissionResponse: () => true,
          roundId: "round-root",
        }),
      ),
    ),
  );

  assert.match(
    html,
    /nexus-chat-message-round-expanded/,
    "no-reply 必须沿用完成态 MessageItem 外壳",
  );
  assert.match(html, /nexus-chat-message-header/);
  assert.match(html, /本轮无需公开回复/);
  assert.match(html, /查看 Thread/);
  assert.match(html, /glm-4\.7/);
  assert.doesNotMatch(
    html,
    /bg-primary\/5/,
    "no-reply 不应退回活动状态卡的高亮背景",
  );
});

test("consumed Room guide update moves beside its running assistant", async () => {
  const { parseConversationMessage } = await server.ssrLoadModule(
    "/src/lib/conversation/message-protocol.ts",
  );
  const { mergeLoadedMessages, upsertMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );
  const {
    filterSupersededRoundIndexItems,
    groupMessagesByRound,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/timeline-model.ts",
  );
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );

  const rootUser = userMessage({
    content: "先分析",
    messageId: "user-root",
    roundId: "round-root",
    timestamp: 1,
  });
  const guideBeforeConsumption = userMessage({
    content: "然后给点建议",
    messageId: "user-guide",
    roundId: "round-guide",
    timestamp: 3,
  });
  const assistant = {
    agent_id: "agent-1",
    content: [{ type: "text", text: "最终建议" }],
    is_complete: false,
    message_id: "assistant-root",
    role: "assistant",
    round_id: "round-root",
    session_key: "room:group:conversation-1",
    stream_status: "streaming",
    timestamp: 2,
  };
  const consumedGuide = parseConversationMessage({
    ...guideBeforeConsumption,
    agent_id: "",
    delivery_policy: "guide",
    round_id: "round-root",
    source_round_id: "round-guide",
  });

  assert.ok(consumedGuide, "Room user updates allow an empty agent_id");
  const messages = upsertMessage(
    [rootUser, assistant, guideBeforeConsumption],
    consumedGuide,
  );
  const groups = groupMessagesByRound(messages);
  assert.equal(groups.has("round-guide"), false);

  const rootMessages = groups.get("round-root") ?? [];
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1" },
    messages: rootMessages,
    pendingPermissions: [],
    pendingSlots: [],
  });
  assert.deepEqual(
    model.userMessages.map(({ message }) => message.message_id),
    ["user-root"],
    "Room 主时间线不渲染已重挂的引导消息",
  );
  assert.equal(model.entries.length, 1);
  assert.equal(model.entries[0]?.agent_id, "agent-1");

  const sourceIndex = roundIndexItem("round-guide", {
    hasUserMessage: true,
    timestamp: 3,
  });
  const targetIndex = roundIndexItem("round-root", {
    agentIds: ["agent-1"],
    isLive: true,
    timestamp: 1,
  });
  assert.deepEqual(
    filterSupersededRoundIndexItems([targetIndex, sourceIndex], messages)
      .map((item) => item.roundId),
    ["round-root"],
    "the consumed source round must not remain as an unloaded navigator card",
  );
  assert.deepEqual(
    filterSupersededRoundIndexItems([
      targetIndex,
      { ...sourceIndex, agentIds: ["agent-2"], isLive: true },
    ], messages).map((item) => item.roundId),
    ["round-root", "round-guide"],
    "a source round with another live agent must remain visible",
  );

  const mergedAfterStaleHistory = mergeLoadedMessages(
    [rootUser, assistant, guideBeforeConsumption],
    messages,
  );
  const groupsAfterStaleHistory = groupMessagesByRound(mergedAfterStaleHistory);
  assert.equal(
    groupsAfterStaleHistory.has("round-guide"),
    false,
    "a stale history response must not undo durable guidance reparenting",
  );
  assert.deepEqual(
    (groupsAfterStaleHistory.get("round-root") ?? [])
      .filter((message) => message.role === "user")
      .map((message) => message.message_id),
    ["user-root", "user-guide"],
  );
  assert.equal(
    mergedAfterStaleHistory.find(
      (message) => message.message_id === "user-guide",
    )?.delivery_policy,
    "guide",
    "a stale history response must not undo fields persisted with reparenting",
  );

  const refreshedGuide = {
    ...consumedGuide,
    attachments: [{ id: "attachment-1", name: "detail.txt" }],
    content: "然后给点更完整的建议",
    timestamp: 4,
  };
  const mergedAfterCanonicalHistory = mergeLoadedMessages(
    [rootUser, assistant, refreshedGuide],
    mergedAfterStaleHistory,
  );
  const canonicalGuide = mergedAfterCanonicalHistory.find(
    (message) => message.message_id === "user-guide",
  );
  assert.equal(canonicalGuide?.round_id, "round-root");
  assert.equal(canonicalGuide?.source_round_id, "round-guide");
  assert.equal(canonicalGuide?.content, "然后给点更完整的建议");
  assert.equal(canonicalGuide?.attachments?.[0]?.name, "detail.txt");
  assert.equal(canonicalGuide?.timestamp, 4);
});

test("Room Composer hides the global stop action when no stop capability is supplied", async () => {
  const {
    projectComposerActions,
    projectComposerInput,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/controller/composer-view-projections.ts",
  );
  const base = {
    canCreateGoal: true,
    compact: false,
    goalCreateBlockedReason: null,
    input: "",
    inputState: projectComposerInput("", 0),
    isGoalCreating: false,
    isGoalMode: false,
    isPreparingAttachments: false,
    runtimeState: {
      activity: "replying",
      canStopGeneration: true,
      isAwaitingPermission: false,
      sessionBusy: true,
    },
  };

  assert.equal(
    projectComposerActions({ ...base, hasStopAction: false }).shouldShowStopButton,
    false,
  );
  assert.equal(
    projectComposerActions({ ...base, hasStopAction: true }).shouldShowStopButton,
    true,
  );
});

test("message protocol preserves CC rich blocks and contains unknown provider blocks", async () => {
  const {
    parseConversationMessage,
    parseStreamMessage,
  } = await server.ssrLoadModule(
    "/src/lib/conversation/message-protocol.ts",
  );

  const message = parseConversationMessage({
    agent_id: "agent-1",
    content: [
      { type: "redacted_thinking", data: "encrypted" },
      { type: "future_provider_block", value: 42 },
    ],
    message_id: "assistant-rich",
    role: "assistant",
    round_id: "round-rich",
    session_key: "agent:agent-1:ws:dm:test",
    timestamp: 1,
  });
  assert.equal(message?.content[0]?.type, "redacted_thinking");
  assert.deepEqual(message?.content[1], {
    type: "unsupported",
    original_type: "future_provider_block",
    payload: { type: "future_provider_block", value: 42 },
  });

  const stream = parseStreamMessage({
    agent_id: "agent-1",
    content_block: {
      type: "tool_use",
      id: "tool-1",
      input: { command: "pwd" },
      name: "Bash",
    },
    index: 0,
    message_id: "assistant-rich",
    parent_tool_use_id: "agent-call-1",
    round_id: "round-rich",
    session_key: "agent:agent-1:ws:dm:test",
    timestamp: 2,
    type: "content_block_start",
  });
  assert.equal(stream?.content_block?.type, "tool_use");
  assert.equal(stream?.parent_tool_use_id, "agent-call-1");

  const blockStop = parseStreamMessage({
    ...stream,
    content_block: undefined,
    index: 0,
    type: "content_block_stop",
  });
  assert.equal(blockStop?.type, "content_block_stop");
  assert.equal(blockStop?.index, 0);
});

test("stream reducer exposes tool calls and removes terminal empty assistants", async () => {
  const { applyStreamMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/stream-message-reducer.ts",
  );
  const base = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-room",
    message_id: "assistant-tool-stream",
    parent_tool_use_id: "agent-call-1",
    room_id: "room-1",
    round_id: "round-tool-stream",
    session_key: "agent:agent-1:ws:dm:test",
    timestamp: 1,
  };
  let messages = applyStreamMessage([], {
    ...base,
    message: { model: "glm-5.2" },
    type: "message_start",
  });
  assert.equal(messages[0]?.parent_id, "agent-call-1");
  assert.equal(
    messages[0]?.agent_round_id,
    "agent-round-room",
    "Room stream placeholder must keep the slot execution identity",
  );
  assert.equal(messages[0]?.room_id, "room-1");
  messages = applyStreamMessage(messages, {
    ...base,
    content_block: {
      type: "tool_use",
      id: "tool-1",
      input: { command: "pwd" },
      name: "Bash",
    },
    index: 0,
    type: "content_block_start",
  });
  assert.equal(messages[0]?.content[0]?.type, "tool_use");
  messages = applyStreamMessage(messages, {
    ...base,
    index: 0,
    type: "content_block_stop",
  });
  assert.equal(messages[0]?.content[0]?.type, "tool_use");

  let emptyMessages = applyStreamMessage([], {
    ...base,
    message_id: "assistant-empty",
    type: "message_start",
  });
  emptyMessages = applyStreamMessage(emptyMessages, {
    ...base,
    message_id: "assistant-empty",
    type: "message_stop",
  });
  assert.deepEqual(emptyMessages, []);
});

test("late history cannot roll an assistant snapshot backward", async () => {
  const { mergeLoadedMessages, upsertMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );

  const liveDone = upsertMessage(
    [assistantMessage({ text: "完整的模型", timestamp: 10 })],
    assistantMessage({
      isComplete: true,
      status: "done",
      stopReason: "end_turn",
      text: "完整的模型回复",
      timestamp: 20,
    }),
  );
  const afterStaleHistory = mergeLoadedMessages(
    [assistantMessage({
      isComplete: true,
      status: "done",
      stopReason: "end_turn",
      text: "完整的模型",
      timestamp: 99,
    })],
    liveDone,
  );
  assert.equal(afterStaleHistory[0]?.stream_status, "done");
  assert.equal(afterStaleHistory[0]?.content[0]?.text, "完整的模型回复");
  assert.equal(afterStaleHistory[0]?.timestamp, 20);

  const canonicalResult = {
    duration_api_ms: 20,
    duration_ms: 30,
    is_error: false,
    message_id: "assistant-root",
    num_turns: 2,
    result: "完整的模型回复，附上最终依据",
    subtype: "success",
    timestamp: 30,
  };
  const afterCanonicalHistory = mergeLoadedMessages(
    [assistantMessage({
      isComplete: true,
      resultSummary: canonicalResult,
      status: "done",
      stopReason: "end_turn",
      text: "完整的模型回复，附上最终依据",
      timestamp: 30,
    })],
    afterStaleHistory,
  );
  assert.equal(
    afterCanonicalHistory[0]?.content[0]?.text,
    "完整的模型回复，附上最终依据",
  );
  assert.equal(afterCanonicalHistory[0]?.result_summary?.timestamp, 30);
  assert.equal(afterCanonicalHistory[0]?.timestamp, 30);
});

test("Room keeps separate agent_round entries for the same agent", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const oldResult = assistantMessage({
    agentRoundId: "agent-round-old",
    isComplete: true,
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "旧回复",
      subtype: "success",
      timestamp: 10,
    },
    status: "done",
    stopReason: "end_turn",
    text: "旧回复",
    timestamp: 10,
  });
  const activeSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-new",
    msg_id: "slot-new",
    round_id: "round-root",
    status: "streaming",
    timestamp: 20,
  };

  let entries = buildRoomAgentRoundEntries([oldResult], [activeSlot]);
  assert.equal(entries.length, 2);
  assert.deepEqual(
    entries.map(({ agent_round_id, status }) => ({ agent_round_id, status })),
    [
      { agent_round_id: "agent-round-old", status: "done" },
      { agent_round_id: "agent-round-new", status: "streaming" },
    ],
  );
  assert.deepEqual(entries[1]?.assistant_messages, []);

  const currentStream = assistantMessage({
    agentRoundId: "agent-round-new",
    messageId: "assistant-new",
    status: "streaming",
    text: "正在处理新问题",
    timestamp: 21,
  });
  entries = buildRoomAgentRoundEntries(
    [oldResult, currentStream],
    [activeSlot],
  );
  assert.equal(entries[1]?.status, "streaming");
  assert.deepEqual(
    entries[1]?.assistant_messages.map((message) => message.message_id),
    ["assistant-new"],
  );

  const legacyStream = assistantMessage({
    messageId: "assistant-legacy-new",
    status: "streaming",
    text: "兼容旧协议流",
    timestamp: 22,
  });
  entries = buildRoomAgentRoundEntries(
    [
      { ...oldResult, agent_round_id: undefined },
      legacyStream,
    ],
    [activeSlot],
  );
  assert.equal(entries[1]?.status, "streaming");
  assert.equal(entries[1]?.result_summary, undefined);
  assert.deepEqual(
    entries[1]?.assistant_messages.map((message) => message.message_id),
    ["assistant-legacy-new"],
  );
});

test("Room Agent slot order survives live, terminal, and history projections", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const firstDone = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-round-1",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "assistant-agent-1-done",
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 已完成",
    timestamp: 20,
  });
  const secondStream = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-round-2",
    messageId: "assistant-agent-2-stream",
    text: "Agent2 正在处理",
    timestamp: 21,
  });
  const liveSlots = [
    {
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      index: 0,
      msg_id: "slot-agent-2",
      round_id: "round-root",
      status: "streaming",
      timestamp: 2,
    },
    {
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      index: 1,
      msg_id: "slot-agent-3",
      round_id: "round-root",
      status: "pending",
      timestamp: 2,
    },
  ];

  const mixed = buildRoomAgentRoundEntries(
    [secondStream, firstDone],
    liveSlots,
  );
  assert.deepEqual(
    mixed.map(({ agent_id, display_order, status }) => ({
      agent_id,
      display_order,
      status,
    })),
    [
      { agent_id: "agent-1", display_order: 1_000, status: "done" },
      { agent_id: "agent-2", display_order: 2_000, status: "streaming" },
      { agent_id: "agent-3", display_order: 2_001, status: "pending" },
    ],
    "a new live member must append after a terminal sibling instead of jumping above it",
  );

  const secondDone = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-round-2",
    displayOrder: 2_000,
    isComplete: true,
    messageId: "assistant-agent-2-done",
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 已完成",
    timestamp: 30,
  });
  const terminal = buildRoomAgentRoundEntries([secondDone, firstDone]);
  assert.deepEqual(
    terminal.map(({ agent_id, display_order }) => ({
      agent_id,
      display_order,
    })),
    [
      { agent_id: "agent-1", display_order: 1_000 },
      { agent_id: "agent-2", display_order: 2_000 },
    ],
    "pending -> terminal must retain the same canonical slot positions",
  );

  const firstFinishedLater = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "history-agent-round-1",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "history-assistant-agent-1",
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 后完成",
    timestamp: 40,
  });
  const secondFinishedEarlier = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "history-agent-round-2",
    displayOrder: 2_001,
    isComplete: true,
    messageId: "history-assistant-agent-2",
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 先完成",
    timestamp: 30,
  });
  const history = buildRoomAgentRoundEntries([
    secondFinishedEarlier,
    firstFinishedLater,
  ]);
  assert.deepEqual(
    history.map(({ agent_id, display_order }) => ({
      agent_id,
      display_order,
    })),
    [
      { agent_id: "agent-1", display_order: 1_000 },
      { agent_id: "agent-2", display_order: 2_001 },
    ],
    "history reload must restore slot order instead of completion order",
  );
});

test("Room interruption projection follows the slot identity without a ghost card", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const slot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stopped",
    msg_id: "slot-stopped",
    round_id: "round-root",
    status: "streaming",
    timestamp: 20,
  };
  const stream = assistantMessage({
    agentRoundId: "agent-round-stopped",
    messageId: "assistant-stopped-stream",
    status: "streaming",
    text: "",
    timestamp: 21,
  });
  const interrupted = {
    ...assistantMessage({
      agentId: "agent-1",
      isComplete: true,
      messageId: "assistant_result_round-root",
      resultSummary: {
        duration_api_ms: 0,
        duration_ms: 0,
        is_error: false,
        num_turns: 0,
        subtype: "interrupted",
        timestamp: 22,
      },
      status: "cancelled",
      text: "",
      timestamp: 22,
    }),
    // 兼容旧事件：结果没有 agent_round_id，但 parent_id 仍指向 slot。
    agent_round_id: undefined,
    parent_id: "slot-stopped",
  };

  const entries = buildRoomAgentRoundEntries([stream, interrupted], [slot]);
  assert.equal(entries.length, 1);
  assert.equal(entries[0]?.agent_round_id, "agent-round-stopped");
  assert.equal(entries[0]?.status, "cancelled");
  assert.deepEqual(
    entries[0]?.assistant_messages.map((message) => message.message_id),
    ["assistant-stopped-stream"],
  );
});

test("Room canonical assistant replaces its temporary synthetic result", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const canonical = assistantMessage({
    agentRoundId: "agent-round-1",
    messageId: "assistant-canonical",
    model: "canonical-model",
    status: "streaming",
    text: "已完成过程处理",
    timestamp: 10,
  });
  const synthetic = assistantMessage({
    agentRoundId: "agent-round-1",
    isComplete: true,
    messageId: "assistant_result-1",
    resultSummary: {
      duration_api_ms: 20,
      duration_ms: 30,
      is_error: false,
      message_id: "result-1",
      num_turns: 2,
      subtype: "success",
      timestamp: 30,
    },
    status: "done",
    stopReason: "end_turn",
    text: "最终模型回复",
    timestamp: 30,
  });

  const entries = buildRoomAgentRoundEntries([canonical, synthetic]);
  assert.equal(entries.length, 1);
  assert.equal(entries[0]?.status, "done");
  assert.equal(entries[0]?.timestamp, 30);
  assert.deepEqual(
    entries[0]?.assistant_messages.map((message) => message.message_id),
    ["assistant-canonical"],
  );
  assert.equal(
    entries[0]?.assistant_messages[0]?.result_summary?.result,
    "最终模型回复",
  );
  assert.equal(entries[0]?.assistant_messages[0]?.model, "canonical-model");
});

test("Room Agent replies keep their first display order through completion", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const rootUser = userMessage({
    content: "一起分析",
    messageId: "user-root-display-order",
    roundId: "round-root",
    timestamp: 1,
  });
  const agent1Partial = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-round",
    messageId: "assistant-agent-1-partial",
    text: "Agent1 正在处理",
    timestamp: 2,
  });
  const agent2Done = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-round",
    isComplete: true,
    messageId: "assistant-agent-2-done",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "Agent2 完成",
      subtype: "success",
      timestamp: 4,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 完成",
    timestamp: 4,
  });
  const guide = userMessage({
    content: "Agent1 再补充结论",
    deliveryPolicy: "guide",
    messageId: "user-guide-display-order",
    roundId: "round-root",
    sourceRoundId: "round-guide-display-order",
    targetAgentIds: ["agent-1"],
    timestamp: 5,
  });
  const agent1Done = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-round",
    isComplete: true,
    messageId: "assistant-agent-1-done",
    resultSummary: {
      duration_api_ms: 20,
      duration_ms: 30,
      is_error: false,
      num_turns: 2,
      result: "Agent1 补充完成",
      subtype: "success",
      timestamp: 6,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 补充完成",
    timestamp: 6,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1", "agent-2": "Agent2" },
    messages: [rootUser, agent1Partial, agent2Done, guide, agent1Done],
    pendingPermissions: [],
    pendingSlots: [],
  });

  assert.deepEqual(
    model.entries.map(({ agent_id, agent_round_id }) => ({
      agent_id,
      agent_round_id,
    })),
    [
      { agent_id: "agent-1", agent_round_id: "agent-1-round" },
      { agent_id: "agent-2", agent_round_id: "agent-2-round" },
    ],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-display-order",
      "user:user-guide-display-order",
      "agent:agent-1",
      "agent:agent-2",
    ],
  );
});

test("late Room guidance does not reorder completed Agent cards", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1", "agent-2": "Agent2" },
    messages: [
      userMessage({
        content: "一起分析",
        messageId: "user-root-stable-completed",
        roundId: "round-root",
        timestamp: 1,
      }),
      assistantMessage({
        agentId: "agent-1",
        agentRoundId: "agent-1-completed",
        isComplete: true,
        messageId: "assistant-agent-1-completed",
        status: "done",
        stopReason: "end_turn",
        text: "Agent1 先完成",
        timestamp: 2,
      }),
      assistantMessage({
        agentId: "agent-2",
        agentRoundId: "agent-2-completed",
        isComplete: true,
        messageId: "assistant-agent-2-completed",
        status: "done",
        stopReason: "end_turn",
        text: "Agent2 后完成",
        timestamp: 4,
      }),
      userMessage({
        agentRoundId: "agent-1-completed",
        content: "这是 Agent1 实际消费的补充",
        deliveryPolicy: "guide",
        messageId: "user-guide-stable-completed",
        roundId: "round-root",
        sourceRoundId: "round-guide-stable-completed",
        targetAgentIds: ["agent-1"],
        timestamp: 5,
      }),
    ],
    pendingPermissions: [],
    pendingSlots: [],
  });

  assert.deepEqual(
    model.entries.map(({ agent_id }) => agent_id),
    ["agent-1", "agent-2"],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-stable-completed",
      "user:user-guide-stable-completed",
      "agent:agent-1",
      "agent:agent-2",
    ],
  );
});

test("Room keeps Agent slot order independent from runtime status", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {
      "agent-1": "Agent1",
      "agent-2": "Agent2",
      "agent-3": "Agent3",
    },
    messages: [
      assistantMessage({
        agentId: "agent-1",
        agentRoundId: "agent-1-active",
        messageId: "assistant-agent-1-latest",
        text: "Agent1 流式内容更新得更晚",
        timestamp: 20,
      }),
      assistantMessage({
        agentId: "agent-2",
        agentRoundId: "agent-2-active",
        messageId: "assistant-agent-2-earlier",
        text: "Agent2 仍在运行",
        timestamp: 10,
      }),
      assistantMessage({
        agentId: "agent-3",
        agentRoundId: "agent-3-completed",
        displayOrder: 4_000,
        isComplete: true,
        messageId: "assistant-agent-3-completed",
        status: "done",
        stopReason: "end_turn",
        text: "Agent3 已完成",
        timestamp: 30,
      }),
      userMessage({
        agentRoundId: "agent-1-active",
        content: "Agent1 继续补充",
        deliveryPolicy: "guide",
        messageId: "user-guide-active-stable",
        roundId: "round-root",
        sourceRoundId: "round-guide-active-stable",
        targetAgentIds: ["agent-1"],
        timestamp: 40,
      }),
    ],
    pendingPermissions: [],
    pendingSlots: [
      {
        agent_id: "agent-1",
        agent_round_id: "agent-1-active",
        index: 0,
        msg_id: "slot-agent-1",
        round_id: "round-root",
        status: "streaming",
        timestamp: 2,
      },
      {
        agent_id: "agent-2",
        agent_round_id: "agent-2-active",
        index: 1,
        msg_id: "slot-agent-2",
        round_id: "round-root",
        status: "streaming",
        timestamp: 3,
      },
    ],
  });

  assert.deepEqual(
    model.entries.map(({ agent_id, status }) => ({ agent_id, status })),
    [
      { agent_id: "agent-1", status: "streaming" },
      { agent_id: "agent-2", status: "streaming" },
      { agent_id: "agent-3", status: "done" },
    ],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-guide-active-stable",
      "agent:agent-1",
      "agent:agent-2",
      "agent:agent-3",
    ],
  );
});

test("Room keeps backend Agent slot order while statuses advance", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const stream = assistantMessage({
    agentId: "agent-streaming",
    agentRoundId: "round-streaming",
    messageId: "assistant-streaming",
    text: "正在输出正文",
    timestamp: 5,
  });
  const slots = [
    {
      agent_id: "agent-streaming",
      agent_round_id: "round-streaming",
      index: 0,
      msg_id: "slot-streaming",
      round_id: "round-root",
      status: "streaming",
      timestamp: 1,
    },
    {
      agent_id: "agent-pending",
      agent_round_id: "round-pending",
      index: 1,
      msg_id: "slot-pending",
      round_id: "round-root",
      status: "pending",
      timestamp: 2,
    },
  ];

  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {},
    messages: [stream],
    pendingPermissions: [],
    pendingSlots: slots,
  });
  assert.deepEqual(
    model.entries.map(({ agent_id, status }) => ({ agent_id, status })),
    [
      { agent_id: "agent-streaming", status: "streaming" },
      { agent_id: "agent-pending", status: "pending" },
    ],
  );

  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [stream]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map([["round-root", slots]]),
    roundIds: ["round-root"],
  });
  assert.deepEqual(projection.roundIds, [
    buildGroupAgentTimelineNodeId(
      "round-root",
      "agent-streaming:agent-round:round-streaming",
    ),
    buildGroupAgentTimelineNodeId(
      "round-root",
      "agent-pending:agent-round:round-pending",
    ),
  ]);
});

test("Room Agent timestamp stays on start while active and switches to finish at terminal", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const { buildGroupAgentStatusModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const slot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stable-time",
    msg_id: "slot-stable-time",
    round_id: "round-root",
    status: "streaming",
    timestamp: 2,
  };
  const stream = assistantMessage({
    agentRoundId: "agent-round-stable-time",
    messageId: "assistant-stable-time",
    text: "流式快照更新时间不能改 header",
    timestamp: 20,
  });
  const active = buildRoomAgentRoundEntries([stream], [slot])[0];
  assert.equal(active?.timestamp, 2);
  assert.equal(buildGroupAgentStatusModel({
    labels: {
      failed: "Failed",
      stopped: "Stopped",
      waitingForUser: "Waiting",
    },
    messages: active.assistant_messages,
    pendingPermissions: [],
    status: active.status,
    timestamp: active.timestamp,
  }).timestamp, 2);

  const result = assistantMessage({
    agentRoundId: "agent-round-stable-time",
    isComplete: true,
    messageId: "assistant-stable-time",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "最终回复",
      subtype: "success",
      timestamp: 30,
    },
    status: "done",
    stopReason: "end_turn",
    text: "最终回复",
    timestamp: 25,
  });
  const terminal = buildRoomAgentRoundEntries([result])[0];
  assert.equal(terminal?.timestamp, 30);
});

test("Room projects every agent_round as a stable root-local feed node", async () => {
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const rootUser = userMessage({
    content: "一起分析",
    messageId: "user-agent-node-root",
    roundId: "round-root",
    timestamp: 1,
  });
  const completed = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-node",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "assistant-agent-2-node",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "Agent2 完成",
      subtype: "success",
      timestamp: 4,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 完成",
    timestamp: 4,
  });
  const activeStream = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-node",
    messageId: "assistant-agent-1-node",
    text: "Agent1 仍在继续",
    timestamp: 7,
  });
  const consumedGuide = userMessage({
    agentRoundId: "agent-1-node",
    content: "Agent1 再补充一个维度",
    deliveryPolicy: "guide",
    messageId: "user-guide-agent-node",
    roundId: "round-root",
    sourceRoundId: "round-guide-agent-node",
    targetAgentIds: ["agent-1"],
    timestamp: 6,
  });
  const laterUser = userMessage({
    content: "另一个后续问题",
    messageId: "user-later-root",
    roundId: "round-later",
    timestamp: 5,
  });
  const activeSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-1-node",
    index: 0,
    msg_id: "slot-agent-1-node",
    round_id: "round-root",
    status: "streaming",
    timestamp: 2,
  };
  const activeProjection = projectGroupAgentTimeline({
    messageGroups: new Map([
      ["round-root", [rootUser, completed, activeStream, consumedGuide]],
      ["round-later", [laterUser]],
    ]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map([["round-root", [activeSlot]]]),
    roundIds: ["round-root", "round-later"],
  });
  const agent1NodeId = buildGroupAgentTimelineNodeId(
    "round-root",
    "agent-1:agent-round:agent-1-node",
  );
  const agent2NodeId = buildGroupAgentTimelineNodeId(
    "round-root",
    "agent-2:agent-round:agent-2-node",
  );
  assert.deepEqual(activeProjection.roundIds, [
    "round-root",
    agent2NodeId,
    agent1NodeId,
    "round-later",
  ]);
  assert.deepEqual(
    activeProjection.messageGroups.get(agent1NodeId)?.map(
      (message) => message.message_id,
    ),
    ["user-guide-agent-node", "assistant-agent-1-node"],
  );
  assert.equal(activeProjection.rootRoundIds.get(agent1NodeId), "round-root");

  const terminal = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-node",
    displayOrder: 2_000,
    isComplete: true,
    messageId: "assistant-agent-1-node",
    resultSummary: {
      duration_api_ms: 20,
      duration_ms: 30,
      is_error: false,
      num_turns: 2,
      result: "Agent1 完成",
      subtype: "success",
      timestamp: 8,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 完成",
    timestamp: 8,
  });
  const terminalProjection = projectGroupAgentTimeline({
    messageGroups: new Map([
      ["round-root", [rootUser, completed, terminal, consumedGuide]],
      ["round-later", [laterUser]],
    ]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root", "round-later"],
  });
  assert.deepEqual(
    terminalProjection.roundIds,
    activeProjection.roundIds,
    "pending -> terminal must not move an already visible Agent reply",
  );
  assert.equal(
    terminalProjection.roundIds.includes(agent1NodeId),
    true,
    "pending -> terminal must not change the visual node identity",
  );
});

test("single-target Room guidance attaches only to its consuming agent", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const rootUser = userMessage({
    content: "先分别分析",
    messageId: "user-root-target-order",
    roundId: "round-root",
    timestamp: 1,
  });
  const legacyGuide = userMessage({
    content: "旧协议插话",
    deliveryPolicy: "guide",
    messageId: "user-guide-legacy",
    roundId: "round-root",
    sourceRoundId: "round-guide-legacy",
    timestamp: 2,
  });
  const multiTargetGuide = userMessage({
    content: "两位都补充",
    deliveryPolicy: "guide",
    messageId: "user-guide-multi",
    roundId: "round-root",
    sourceRoundId: "round-guide-multi",
    targetAgentIds: ["agent-1", "agent-2"],
    timestamp: 3,
  });
  const agent2Result = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-old-round",
    isComplete: true,
    messageId: "assistant-agent-2",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "Agent2 已完成",
      subtype: "success",
      timestamp: 4,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 已完成",
    timestamp: 4,
  });
  const agent1Stream = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-live-round",
    messageId: "assistant-agent-1",
    text: "Agent1 原输出",
    timestamp: 5,
  });
  const targetedGuide = userMessage({
    content: "Agent1 改成比较 M4 和 M5",
    deliveryPolicy: "guide",
    messageId: "user-guide-agent-1",
    roundId: "round-root",
    sourceRoundId: "round-guide-agent-1",
    targetAgentIds: ["agent-1"],
    timestamp: 6,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1", "agent-2": "Agent2" },
    messages: [
      rootUser,
      legacyGuide,
      multiTargetGuide,
      agent2Result,
      agent1Stream,
      targetedGuide,
    ],
    pendingPermissions: [],
    pendingSlots: [{
      agent_id: "agent-1",
      agent_round_id: "agent-1-live-round",
      msg_id: "slot-agent-1",
      round_id: "round-root",
      status: "streaming",
      timestamp: 5,
    }],
  });

  assert.deepEqual(
    model.userMessages.map(({ message }) => message.message_id),
    ["user-root-target-order", "user-guide-legacy", "user-guide-multi"],
  );
  assert.deepEqual(
    model.entries
      .filter((entry) => entry.status === "done")
      .map((entry) => entry.agent_id),
    ["agent-2"],
  );
  assert.deepEqual(model.entries[0]?.guidedUserMessages, []);
  assert.deepEqual(
    model.entries
      .filter((entry) => entry.status !== "done")
      .map((entry) => entry.agent_id),
    ["agent-1"],
  );
  assert.deepEqual(
    model.entries[1]?.guidedUserMessages.map(
      ({ message }) => message.message_id,
    ),
    ["user-guide-agent-1"],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-target-order",
      "user:user-guide-legacy",
      "user:user-guide-multi",
      "agent:agent-2",
      "user:user-guide-agent-1",
      "agent:agent-1",
    ],
  );
});

test("single-target Room guidance also attaches to a completed agent", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const completedGuide = userMessage({
    content: "完成前补充的约束",
    deliveryPolicy: "guide",
    messageId: "user-guide-completed",
    roundId: "round-root",
    sourceRoundId: "round-guide-completed",
    targetAgentIds: ["agent-2"],
    timestamp: 2,
  });
  const completedResult = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-completed-round",
    isComplete: true,
    messageId: "assistant-agent-2-completed",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "已按补充约束完成",
      subtype: "success",
      timestamp: 3,
    },
    status: "done",
    stopReason: "end_turn",
    text: "已按补充约束完成",
    timestamp: 3,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-2": "Agent2" },
    messages: [
      userMessage({
        content: "初始问题",
        messageId: "user-root-completed",
        roundId: "round-root",
        timestamp: 1,
      }),
      completedGuide,
      completedResult,
    ],
    pendingPermissions: [],
    pendingSlots: [],
  });

  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-completed",
      "user:user-guide-completed",
      "agent:agent-2",
    ],
  );
});

test("Room guidance stays on its exact consumed agent round", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const guide = userMessage({
    agentRoundId: "agent-1-old-round",
    content: "这是旧执行轮实际消费的插话",
    deliveryPolicy: "guide",
    messageId: "user-guide-exact-round",
    roundId: "round-root",
    sourceRoundId: "round-guide-exact",
    targetAgentIds: ["agent-1"],
    timestamp: 11,
  });
  const oldResult = assistantMessage({
    agentRoundId: "agent-1-old-round",
    isComplete: true,
    messageId: "assistant-agent-1-old",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "旧轮按插话完成",
      subtype: "success",
      timestamp: 12,
    },
    status: "done",
    stopReason: "end_turn",
    text: "旧轮按插话完成",
    timestamp: 12,
  });
  const newStream = assistantMessage({
    agentRoundId: "agent-1-new-round",
    messageId: "assistant-agent-1-new",
    text: "新轮正在处理",
    timestamp: 13,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1" },
    messages: [guide, oldResult, newStream],
    pendingPermissions: [],
    pendingSlots: [{
      agent_id: "agent-1",
      agent_round_id: "agent-1-new-round",
      msg_id: "slot-agent-1-new",
      round_id: "round-root",
      status: "streaming",
      timestamp: 13,
    }],
  });

  assert.deepEqual(
    model.entries.map((entry) => ({
      agentRoundId: entry.agent_round_id,
      guides: entry.guidedUserMessages.map(({ message }) => message.message_id),
    })),
    [
      {
        agentRoundId: "agent-1-old-round",
        guides: ["user-guide-exact-round"],
      },
      { agentRoundId: "agent-1-new-round", guides: [] },
    ],
  );
});

function userMessage({
  agentRoundId,
  content,
  deliveryPolicy,
  messageId,
  roundId,
  sourceRoundId,
  targetAgentIds,
  timestamp,
}) {
  return {
    agent_id: "",
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
    content,
    ...(deliveryPolicy ? { delivery_policy: deliveryPolicy } : {}),
    message_id: messageId,
    role: "user",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    ...(sourceRoundId ? { source_round_id: sourceRoundId } : {}),
    ...(targetAgentIds ? { target_agent_ids: targetAgentIds } : {}),
    timestamp,
  };
}

function assistantMessage({
  agentId = "agent-1",
  agentRoundId,
  displayOrder,
  isComplete = false,
  messageId = "assistant-root",
  model,
  resultSummary,
  roundId = "round-root",
  status = "streaming",
  stopReason,
  text,
  timestamp,
}) {
  return {
    agent_id: agentId,
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
    content: [{ type: "text", text }],
    ...(displayOrder === undefined ? {} : { display_order: displayOrder }),
    is_complete: isComplete,
    message_id: messageId,
    ...(model ? { model } : {}),
    ...(resultSummary ? { result_summary: resultSummary } : {}),
    role: "assistant",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    ...(stopReason ? { stop_reason: stopReason } : {}),
    stream_status: status,
    timestamp,
  };
}

function flattenGroupRoundRenderOrder(model) {
  const order = model.userMessages.map(
    ({ message }) => `user:${message.message_id}`,
  );
  for (const entry of model.entries) {
    order.push(...entry.guidedUserMessages.map(
      ({ message }) => `user:${message.message_id}`,
    ));
    order.push(`agent:${entry.agent_id}`);
  }
  return order;
}

function roundIndexItem(roundId, overrides = {}) {
  return {
    agentIds: [],
    durationMs: null,
    hasUserMessage: false,
    isLive: false,
    roundId,
    status: null,
    timestamp: null,
    title: "",
    ...overrides,
  };
}
