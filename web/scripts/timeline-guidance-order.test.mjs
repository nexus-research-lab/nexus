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

async function loadI18nValue(locale = "zh") {
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  return {
    locale,
    setLocale: () => {},
    t: (key, params = {}) => Object.entries(params).reduce(
      (message, [name, value]) => message.replaceAll(
        `{${name}}`,
        String(value),
      ),
      MESSAGES[locale][key] ?? key,
    ),
  };
}

async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  return renderToStaticMarkup(React.createElement(
    I18N_CONTEXT.Provider,
    { value: await loadI18nValue(locale) },
    element,
  ));
}

test("conversation viewport suppresses the browser scroll-region outline", async () => {
  const { ConversationPanelViewport } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const html = await renderWithI18n(React.createElement(
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

test("scroll-to-latest is a local floating hit target without a layout band", async () => {
  const { ScrollToLatestButton } = await server.ssrLoadModule(
    "/src/features/conversation/shared/scroll-to-latest-button.tsx",
  );
  const visibleHtml = await renderWithI18n(React.createElement(
    ScrollToLatestButton,
    {
      isLoading: true,
      onClick: () => {},
      visible: true,
    },
  ));
  const hiddenHtml = await renderWithI18n(React.createElement(
    ScrollToLatestButton,
    {
      isLoading: false,
      onClick: () => {},
      visible: false,
    },
  ));

  assert.match(
    visibleHtml,
    /data-scroll-to-latest="true"/,
    "the visible action exposes one explicit floating target",
  );
  assert.match(
    visibleHtml,
    /class="[^"]*\bh-11\b[^"]*\bw-11\b[^"]*"/,
    "the floating target keeps a comfortable local hit area",
  );
  assert.doesNotMatch(
    visibleHtml,
    /\stitle=/,
    "the icon-only action must not expose a native tooltip over the feed",
  );
  assert.doesNotMatch(
    visibleHtml,
    /animate-bounce/,
    "live output must not animate the user-controlled return action",
  );
  assert.equal(
    hiddenHtml,
    "",
    "the hidden action must contribute zero layout or pointer surface",
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
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
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

test("Room streaming revisions keep scroll coordination fresh for non-last Agent output", async () => {
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
    "任意并行 Agent 的流式正文增长都必须唤醒滚动协调，但不能等同于共享贴底写入",
  );
});

test("upper Room Agent streaming delegates virtual height changes without pulling the bottom", async () => {
  const { resolveConversationFollowCommitOwner } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );

  assert.equal(
    resolveConversationFollowCommitOwner({
      bottomScrollActive: false,
      isNewSession: false,
      isVirtualFeed: true,
      topologyChanged: false,
      viewportAnchorRestored: false,
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
      viewportAnchorRestored: false,
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
      viewportAnchorRestored: false,
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
      viewportAnchorRestored: true,
    }),
    "viewport-anchor",
    "static content growing above the viewport must preserve its visible round",
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
    assert.equal(animator.isActive(), true);
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
    assert.equal(
      animator.isActive(),
      true,
      "auto settlement must remain active until virtual measurements stay quiet",
    );
    for (let frame = 1; frame <= 4 && frames.length > 0; frame += 1) {
      frames.shift()(frame * (1_000 / 60));
    }
    assert.equal(
      animator.isActive(),
      false,
      "two stable frames release initialization ownership to Virtualizer",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("FOLLOW synchronously locks every committed growth before paint", async () => {
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

    container.scrollHeight = 1_040;
    animator.follow();
    assert.equal(container.scrollTop, 540);
    assert.equal(frames.length, 0, "FOLLOW must not queue a visible spring");

    container.scrollHeight = 1_080;
    animator.follow();
    assert.equal(container.scrollTop, 580);
    assert.deepEqual(
      positions,
      [540, 580],
      "each committed stream height reaches the true bottom in the same turn",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("FOLLOW always resolves the current real bottom without a high-water target", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
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

test("stream growth takes over an explicit return-to-latest transaction", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const cancelledFrames = [];
  const originalWindow = globalThis.window;
  let nextFrameId = 0;
  globalThis.window = {
    cancelAnimationFrame: (frameId) => {
      cancelledFrames.push(frameId);
    },
    requestAnimationFrame: (callback) => {
      nextFrameId += 1;
      frames.push(callback);
      return nextFrameId;
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
    assert.equal(frames.length, 1);
    assert.equal(animator.isActive(), true);

    container.scrollHeight = 1_120;
    animator.follow();
    assert.equal(animator.isActive(), false);
    assert.equal(
      container.scrollTop,
      620,
      "the first committed stream growth must hand control back to synchronous FOLLOW",
    );
    assert.deepEqual(
      cancelledFrames,
      [1],
      "the obsolete smooth frame must be cancelled before FOLLOW lands",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("synchronous FOLLOW closes subpixel scrollTop quantization", async () => {
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
      "a rounded scrollTop must close without starting RAF",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("FOLLOW has no suspended App frame to replay on resume", async () => {
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
    assert.equal(container.scrollTop, 1_000);
    assert.equal(
      frames.length,
      0,
      "resume cannot replay hidden FOLLOW motion because no RAF is retained",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("FOLLOW has no active animation across a Composer or App viewport resize", async () => {
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
    assert.equal(container.scrollTop, 580);
    assert.deepEqual(observedPositions, [580]);
    assert.equal(frames.length, 0);

    container.clientHeight = 420;
    animator.follow();
    assert.equal(container.scrollTop, 660);
    assert.deepEqual(observedPositions, [580, 660]);
    assert.equal(
      frames.length,
      0,
      "viewport ownership changes must not leave a FOLLOW RAF behind",
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
  assert.equal(
    shouldAdjustConversationVirtualScrollPosition(
      { end: 980 },
      28,
      {
        getTotalSize: () => 1_000,
        scrollOffset: 500,
        scrollRect: { height: 500 },
      },
    ),
    true,
    "a measured tail growing from the real bottom must keep FOLLOW inside Virtualizer",
  );
  assert.equal(
    shouldAdjustConversationVirtualScrollPosition(
      { end: 980 },
      28,
      {
        getTotalSize: () => 1_000,
        scrollOffset: 450,
        scrollRect: { height: 500 },
      },
    ),
    false,
    "the same tail growth must not move a reader who is away from the bottom",
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
  const permissionFirstTopology = buildConversationScrollTopologyKey(
    "room:group:conversation-1",
    [streamingMessage],
    [],
    [{
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      request_id: "permission-agent-3",
      round_id: "historical-root",
      tool_input: { command: "echo three" },
      tool_name: "Bash",
    }],
  );
  assert.notEqual(
    permissionFirstTopology,
    topologyBefore,
    "a permission-first Room execution must change the topology before its slot arrives",
  );
  assert.equal(
    buildConversationScrollTopologyKey(
      "room:group:conversation-1",
      [streamingMessage],
      [{
        agent_id: "agent-3",
        agent_round_id: "agent-round-3",
        msg_id: "slot-3",
        round_id: "historical-root",
        status: "pending",
        timestamp: 3,
      }],
      [{
        agent_id: "agent-3",
        agent_round_id: "agent-round-3",
        request_id: "permission-agent-3",
        round_id: "historical-root",
        tool_input: { command: "echo three" },
        tool_name: "Bash",
      }],
    ),
    permissionFirstTopology,
    "the later slot must reuse the permission-first node instead of moving the reading anchor",
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
    "the terminal state transition must remain an explicit atomic revision",
  );
});

test("pending interactions keep first position and latest request snapshot", async () => {
  const { coalescePendingPermissions } = await server.ssrLoadModule(
    "/src/lib/conversation/pending-permission-match.ts",
  );
  const { resolveMessageItemPermissions } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-permissions.ts",
  );
  const { resolvePendingInteractionOwner } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/message-item-projection.ts",
  );
  const first = {
    message_id: "assistant-permission-owner",
    request_id: "request-stable",
    summary: "旧快照",
    tool_input: { command: "echo stable" },
    tool_name: "Bash",
    tool_use_id: "tool-stable",
  };
  const other = {
    request_id: "request-other",
    tool_input: { path: "/tmp/report" },
    tool_name: "Read",
  };
  const latest = {
    ...first,
    summary: "最新快照",
  };
  assert.deepEqual(
    coalescePendingPermissions([first, other, latest]),
    [latest, other],
    "a repeated request updates in place instead of creating a second surface",
  );

  const assistant = {
    agent_id: "agent-1",
    content: [{
      id: "tool-stable",
      input: { command: "echo stable" },
      name: "Bash",
      type: "tool_use",
    }],
    message_id: "assistant-permission-owner",
    role: "assistant",
    round_id: "round-permission-owner",
    session_key: "room:group:conversation-1",
    stream_status: "streaming",
    timestamp: 1,
  };
  const projection = resolveMessageItemPermissions(
    [assistant],
    [first, other, latest],
  );
  assert.deepEqual(projection.pendingInteractionPermissions, [latest, other]);
  assert.equal(
    projection.matchedPendingPermissionsByToolUseId.get("tool-stable"),
    latest,
  );
  assert.deepEqual(projection.unmatchedPendingPermissions, [other]);
  assert.equal(resolvePendingInteractionOwner("room_result"), "composer");
  assert.equal(resolvePendingInteractionOwner("room_thread"), "composer");
  assert.equal(
    resolvePendingInteractionOwner("room_thread_process"),
    "composer",
  );
  assert.equal(resolvePendingInteractionOwner("dm_live"), "composer");
  assert.equal(resolvePendingInteractionOwner("dm_archived"), "composer");
});

test("Room keeps every pending runtime human interaction in the Composer", async () => {
  const { ComposerInteractionSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-surface.tsx",
  );
  const { GroupAgentExecutionShell } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-execution-shell.tsx",
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

  const composerHtml = renderToStaticMarkup(provider(React.createElement(
    ComposerInteractionSurface,
    {
      agentAvatarMap: { "agent-1": null },
      agentNameMap: { "agent-1": "Dev" },
      onResponse: () => true,
      permissions: [
        permission,
        questionPermission,
        planConfirmation,
        futureApproval,
      ],
    },
  )));
  assert.match(composerHtml, /data-composer-interaction-surface="true"/);
  assert.match(composerHtml, /Dev/);
  assert.match(composerHtml, /echo permission-required/);
  assert.match(composerHtml, /1 \/ 4/);
  assert.match(composerHtml, />允许本次</);
  assert.match(composerHtml, />拒绝</);
  assert.doesNotMatch(
    composerHtml,
    /这次分析采用哪种研究口径？/,
    "Composer must show only the first request in the stable queue",
  );
  const nextComposerHtml = renderToStaticMarkup(provider(React.createElement(
    ComposerInteractionSurface,
    {
      onResponse: () => true,
      permissions: [
        questionPermission,
        planConfirmation,
        futureApproval,
      ],
    },
  )));
  assert.match(nextComposerHtml, /这次分析采用哪种研究口径？/);
  assert.match(nextComposerHtml, /1 \/ 3/);
  assert.match(nextComposerHtml, /继续协作/);

  const agentCardHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentExecutionShell,
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
      roundId: "round-root:agent-1:agent-round-1",
      status: "pending",
      timestamp: 1,
    },
  )));
  assert.doesNotMatch(agentCardHtml, /data-human-interaction-surface/);
  assert.doesNotMatch(agentCardHtml, />允许</);
  assert.doesNotMatch(agentCardHtml, />拒绝</);
  assert.doesNotMatch(agentCardHtml, /继续协作/);
  const adjacentAgentHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentExecutionShell,
    {
      agentAvatar: null,
      agentId: "agent-2",
      agentName: "Review",
      isThreadActive: false,
      messages: [],
      onClickThread: () => {},
      onPermissionResponse: () => true,
      pendingPermissions: [],
      roundId: "round-root:agent-2:agent-round-2",
      showAgentBoundary: true,
      status: "pending",
      timestamp: 2,
    },
  )));
  assert.match(
    adjacentAgentHtml,
    /data-conversation-agent-boundary/,
    "相邻 Agent 只用局部身份提示建立边界",
  );
  assert.doesNotMatch(
    adjacentAgentHtml,
    /conversation-round-divider/,
    "Room 主 Feed 的 Agent 边界不能再伪装成 Markdown 全宽分隔线",
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
        roomAgentExecutionStates: [],
        rootRoundId: "round-root",
        roundId: "round-root",
      },
    }),
  ));
  assert.doesNotMatch(permissionOnlyRoundHtml, /data-human-interaction-surface/);
  assert.doesNotMatch(permissionOnlyRoundHtml, />允许</);
  assert.doesNotMatch(permissionOnlyRoundHtml, />拒绝</);
  assert.doesNotMatch(permissionOnlyRoundHtml, /继续协作/);

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
    roomAgentExecutionStateGroups:
      completedProjection.roomAgentExecutionStateGroups,
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
    /Goal[\s\S]*已设定，现在开始调研/,
    "the Room timeline keeps its public reply while Composer owns approval",
  );
  assert.doesNotMatch(completedRoundHtml, />允许</);
  assert.doesNotMatch(completedRoundHtml, />拒绝</);
  assert.doesNotMatch(completedRoundHtml, /继续协作/);
  assert.doesNotMatch(completedRoundHtml, /data-human-interaction-surface/);

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
    roomAgentExecutionStateGroups:
      questionOnlyProjection.roomAgentExecutionStateGroups,
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
  assert.doesNotMatch(questionOnlyHtml, /继续协作/);
  assert.doesNotMatch(questionOnlyHtml, />允许</);
  assert.doesNotMatch(questionOnlyHtml, />拒绝</);
  assert.doesNotMatch(questionOnlyHtml, /data-human-interaction-surface/);
});

test("Room streams and completes inside one stable Agent execution shell", async () => {
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

  assert.match(
    activeHtml,
    new RegExp(tailMarker),
    "the public Room reply must grow in place while the Agent is streaming",
  );
  assert.match(
    activeHtml,
    /正在回复/,
    "the shared activity indicator must remain visible in the public reply",
  );
  assert.doesNotMatch(
    activeHtml,
    /line-clamp-1/,
    "the live reply must not collapse into a one-line status preview",
  );
  assert.match(
    terminalHtml,
    new RegExp(tailMarker),
    "the public terminal result must be complete as soon as the backend snapshot arrives",
  );
  assert.doesNotMatch(
    terminalHtml,
    /正在回复/,
    "terminal Room replies must remove the transient activity indicator",
  );
  assert.equal(
    activeHtml.match(/data-room-agent-execution-shell/g)?.length,
    1,
  );
  assert.equal(
    terminalHtml.match(/data-room-agent-execution-shell/g)?.length,
    1,
  );
  assert.match(
    activeHtml,
    /data-room-agent-execution-shell="round-root:agent-stream:agent-round:agent-round-stream"/,
  );
  assert.match(
    terminalHtml,
    /data-room-agent-execution-shell="round-root:agent-stream:agent-round:agent-round-stream"/,
    "pending and terminal snapshots must retain the same outer execution identity",
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
    new RegExp(tailMarker),
    "a terminal lifecycle event must not replace or hide the already visible stream",
  );
});

test("Room public activity survives the pause between reply text and tool work", async () => {
  const { GroupAgentExecutionShell } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-execution-shell.tsx",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const renderShell = (props) => renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(GroupAgentExecutionShell, {
        agentAvatar: null,
        agentId: "agent-public-activity",
        agentName: "Researcher",
        isThreadActive: false,
        onClickThread: () => {},
        onPermissionResponse: () => true,
        pendingPermissions: [],
        roundId: "round-public-activity:agent-public-activity",
        timestamp: 1,
        ...props,
      }),
    ),
  );

  const pendingHtml = renderShell({
    messages: [],
    status: "pending",
  });
  assert.match(pendingHtml, /正在思考/);
  assert.match(
    pendingHtml,
    /room-agent-execution-enter/,
    "the first pending shell receives one bounded compositor-only entrance animation",
  );
  assert.equal(
    pendingHtml.match(/message-activity-spinner-track/g)?.length,
    1,
    "a pending slot uses the shared activity surface exactly once",
  );

  const completedPublicTurn = assistantMessage({
    agentId: "agent-public-activity",
    agentRoundId: "agent-round-public-activity",
    isComplete: true,
    messageId: "assistant-public-turn",
    roundId: "round-public-activity",
    status: "done",
    stopReason: "end_turn",
    text: "我先说明计划，随后继续在 Thread 中执行。",
    timestamp: 2,
  });
  const continuedHtml = renderShell({
    messages: [completedPublicTurn],
    status: "streaming",
  });
  assert.match(continuedHtml, /我先说明计划，随后继续在/);
  assert.match(continuedHtml, /中执行。/);
  assert.match(
    continuedHtml,
    /正在思考/,
    "an active Agent Thread keeps a public activity row after an intermediate text turn completes",
  );
  assert.equal(
    continuedHtml.match(/message-activity-spinner-track/g)?.length,
    1,
    "the continued Thread activity stays inside the existing Agent card",
  );

  const toolContinuation = {
    ...assistantMessage({
      agentId: "agent-public-activity",
      agentRoundId: "agent-round-public-activity",
      isComplete: true,
      messageId: "assistant-public-tool-turn",
      roundId: "round-public-activity",
      status: "done",
      stopReason: "tool_use",
      text: "我先搜索产品线信息。",
      timestamp: 2,
    }),
    content: [
      { type: "text", text: "我先搜索产品线信息。" },
      {
        id: "tool-public-search",
        input: { query: "M3 product line" },
        name: "WebSearch",
        type: "tool_use",
      },
    ],
  };
  const workingHtml = renderShell({
    messages: [toolContinuation],
    status: "streaming",
  });
  assert.match(workingHtml, /我先搜索产品线信息。/);
  assert.match(
    workingHtml,
    /正在浏览/,
    "tool continuation remains visibly active after its preceding text stops streaming",
  );
  assert.equal(
    workingHtml.match(/message-activity-spinner-track/g)?.length,
    1,
    "the public tool activity appends to the same MessageItem instead of adding a second card",
  );

  const terminalHtml = renderShell({
    messages: [{
      ...toolContinuation,
      result_summary: {
        duration_api_ms: 10,
        duration_ms: 20,
        is_error: false,
        message_id: "result-public-activity",
        num_turns: 1,
        result: "研究完成。",
        subtype: "success",
        timestamp: 3,
      },
      stop_reason: "end_turn",
      timestamp: 3,
    }],
    status: "done",
    timestamp: 3,
  });
  assert.doesNotMatch(terminalHtml, /正在浏览/);
  assert.equal(
    terminalHtml.match(/data-room-agent-execution-shell/g)?.length,
    1,
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

test("partial DM round indexes preserve loaded transcript chronology after remount", async () => {
  const {
    buildIndexedTimelineRoundIds,
    groupMessagesByRound,
    mergeLoadedRoundIndexItems,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/timeline-model.ts",
  );
  const { buildSessionNavigationItems } = await server.ssrLoadModule(
    "/src/features/conversation/shared/session-navigator/session-navigator-model.ts",
  );
  const loadedRoundIds = [
    "round-legacy-1",
    "round-legacy-2",
    "round-live-1",
    "round-live-2",
  ];
  const partialIndex = [
    roundIndexItem("round-live-1", { timestamp: 3 }),
    roundIndexItem("round-live-2", { timestamp: 4 }),
  ];

  assert.deepEqual(
    buildIndexedTimelineRoundIds(partialIndex, loadedRoundIds),
    loadedRoundIds,
    "legacy transcript rounds must stay before their shared durable index anchors",
  );

  const mergedIndex = mergeLoadedRoundIndexItems(partialIndex, loadedRoundIds);
  assert.deepEqual(
    mergedIndex.map((item) => item.roundId),
    loadedRoundIds,
    "feed and navigator must consume the same merged order",
  );

  const messages = loadedRoundIds.map((roundId, index) => userMessage({
    content: `第 ${index + 1} 轮`,
    messageId: `message-${index + 1}`,
    roundId,
    timestamp: index + 1,
  }));
  const navigationItems = buildSessionNavigationItems(
    {
      feed_round_ids: loadedRoundIds,
      live_round_ids: [],
      loaded_round_ids: loadedRoundIds,
      message_groups: groupMessagesByRound(messages),
      pending_permission_groups: new Map(),
      pending_slot_groups: new Map(),
      room_agent_execution_state_groups: new Map(),
      round_index_items: mergedIndex,
    },
    await loadI18nValue(),
  );
  assert.deepEqual(
    navigationItems.map((item) => item.roundId),
    loadedRoundIds,
    "responsive remounts must not move freshly generated rounds ahead of old history",
  );
});

test("conversation navigation fallbacks follow the interface language", async () => {
  const { buildSessionNavigationItems } = await server.ssrLoadModule(
    "/src/features/conversation/shared/session-navigator/session-navigator-model.ts",
  );
  const { formatSpeakerSummary } = await server.ssrLoadModule(
    "/src/features/conversation/shared/session-navigator/session-navigator-ruler-model.ts",
  );
  const localization = await loadI18nValue("en");
  const navigationItems = buildSessionNavigationItems(
    {
      feed_round_ids: ["round-unloaded"],
      live_round_ids: [],
      loaded_round_ids: [],
      message_groups: new Map(),
      pending_permission_groups: new Map(),
      pending_slot_groups: new Map(),
      room_agent_execution_state_groups: new Map(),
      round_index_items: [roundIndexItem("round-unloaded", {
        hasUserMessage: true,
        status: "error",
      })],
    },
    localization,
  );

  assert.equal(navigationItems[0].title, "Round 1");
  assert.equal(navigationItems[0].summary, "Scroll to load details");
  assert.equal(navigationItems[0].meta, "Failed");
  assert.equal(
    formatSpeakerSummary(navigationItems[0], localization.t),
    "User",
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
    roomAgentExecutionStateGroups: new Map(),
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

test("Room orchestration control markers never become visible assistant blocks", async () => {
  const {
    buildVisibleOrderedAssistantEntries,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts",
  );
  const { getResultSummaryDisplayText } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-stats.ts",
  );
  for (const marker of [
    "<nexus_room_no_reply/>",
    "<nexus_room_fanout/>",
    "<NEXUS_ROOM_FANOUT />",
  ]) {
    const entries = buildVisibleOrderedAssistantEntries({
      hiddenToolNames: new Set(),
      hiddenToolUseIds: new Set(),
      isLoading: false,
      mergedContent: [{ type: "text", text: marker }],
      mergedContentSourceMessageIds: ["assistant-control-marker"],
      sourceMessageOrderById: new Map([["assistant-control-marker", 0]]),
      systemEventBlocks: [],
    });
    assert.deepEqual(entries, []);
    assert.equal(
      getResultSummaryDisplayText({ result: marker }),
      null,
      "结果与复制投影也不能恢复内部控制标记",
    );
  }
  assert.deepEqual(
    buildVisibleOrderedAssistantEntries({
      hiddenToolNames: new Set(),
      hiddenToolUseIds: new Set(),
      isLoading: false,
      mergedContent: [{
        type: "thinking",
        thinking: "<nexus_room_fanout/>",
      }],
      mergedContentSourceMessageIds: ["assistant-thinking-marker"],
      sourceMessageOrderById: new Map([["assistant-thinking-marker", 0]]),
      systemEventBlocks: [],
    }),
    [],
    "内部标记即使误入 thinking 也不能占据过程高度",
  );

  const [visibleEntry] = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds: new Set(),
    isLoading: false,
    mergedContent: [{
      type: "text",
      text: "请 Ban 和 Kevin 并行处理。<nexus_room_fanout/>",
    }],
    mergedContentSourceMessageIds: ["assistant-fanout"],
    sourceMessageOrderById: new Map([["assistant-fanout", 0]]),
    systemEventBlocks: [],
  });
  assert.equal(visibleEntry.block.text, "请 Ban 和 Kevin 并行处理。");
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

  assert.deepEqual(summary, {
    kind: "details",
    latestDetail: null,
    metrics: [],
  });
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

test("DM live keeps one stable open segment across consecutive tool patches", async () => {
  const { projectDmToolRunSegments } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/dm-tool-run-segments.ts",
  );
  const toolA = {
    type: "tool_use",
    id: "tool-run-a",
    name: "Read",
    input: { file_path: "a.ts" },
  };
  const resultA = {
    type: "tool_result",
    tool_use_id: toolA.id,
    content: "a",
  };
  const toolB = {
    type: "tool_use",
    id: "tool-run-b",
    name: "Grep",
    input: { pattern: "needle" },
  };
  const resultB = {
    type: "tool_result",
    tool_use_id: toolB.id,
    content: "b",
  };
  const project = (
    content,
    { live = true, responseResumed = false } = {},
  ) => projectDmToolRunSegments({
    interactiveToolUseIds: new Set(),
    live,
    projection: { content, streamingIndexes: new Set() },
    responseResumed,
  });

  for (const content of [
    [toolA],
    [toolA, resultA],
    [toolA, resultA, toolB],
    [toolA, resultA, toolB, resultB],
  ]) {
    const [segment] = project(content);
    assert.equal(segment.kind, "tool_run");
    assert.equal(segment.id, "tool-run:tool-run-a");
    assert.equal(segment.phase, "active");
  }

  const [completed] = project(
    [toolA, resultA, toolB, resultB],
    { responseResumed: true },
  );
  assert.equal(completed.id, "tool-run:tool-run-a");
  assert.equal(completed.phase, "complete");
  assert.equal(completed.toolUseIds.length, 2);

  const [unresolvedDuringResponse] = project(
    [toolA],
    { responseResumed: true },
  );
  assert.equal(
    unresolvedDuringResponse.phase,
    "active",
    "a stale response boundary cannot collapse an unresolved newer tool",
  );

  const [terminal] = project(
    [toolA, resultA, toolB, resultB],
    { live: false },
  );
  assert.equal(terminal.phase, "complete");
});

test("DM tool segments split on narrative and preserve interactions and errors", async () => {
  const { projectDmToolRunSegments } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/dm-tool-run-segments.ts",
  );
  const failedTool = {
    type: "tool_use",
    id: "tool-failed",
    name: "Bash",
    input: { command: "false" },
  };
  const questionTool = {
    type: "tool_use",
    id: "tool-question-boundary",
    name: "AskUserQuestion",
    input: { questions: [] },
  };
  const permissionTool = {
    type: "tool_use",
    id: "tool-permission-boundary",
    name: "Write",
    input: { file_path: "answer.md" },
  };
  const trailingTool = {
    type: "tool_use",
    id: "tool-after-boundaries",
    name: "Read",
    input: { file_path: "answer.md" },
  };
  const content = [
    failedTool,
    {
      type: "tool_result",
      tool_use_id: failedTool.id,
      content: "exit 1",
      is_error: true,
    },
    {
      type: "task_progress",
      task_id: "task-failed",
      description: "command finished",
      tool_use_id: failedTool.id,
    },
    {
      type: "system_event",
      content: "retry exhausted",
      icon: "retry",
      label: "Retry",
      source_message_id: "system-failed",
      subtype: "api_retry",
      timestamp: 3,
      tone: "warning",
      tool_use_id: failedTool.id,
    },
    {
      type: "workspace_file_artifact",
      path: "failure.log",
      source_tool_use_id: failedTool.id,
    },
    { type: "thinking", thinking: "调整方案" },
    questionTool,
    {
      type: "tool_result",
      tool_use_id: questionTool.id,
      content: "answered",
    },
    permissionTool,
    {
      type: "tool_result",
      tool_use_id: permissionTool.id,
      content: "allowed",
    },
    trailingTool,
  ];
  const [activeFailedSegment] = projectDmToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: content.slice(0, 2),
      streamingIndexes: new Set(),
    },
    responseResumed: false,
  });
  assert.equal(activeFailedSegment.phase, "active");
  assert.equal(activeFailedSegment.errorCount, 1);
  const segments = projectDmToolRunSegments({
    interactiveToolUseIds: new Set([permissionTool.id]),
    live: true,
    projection: {
      content,
      streamingIndexes: new Set([5]),
    },
    responseResumed: false,
  });

  assert.deepEqual(
    segments.map(({ id, kind, phase }) => ({ id, kind, phase })),
    [
      {
        id: "tool-run:tool-failed",
        kind: "tool_run",
        phase: "error",
      },
      { id: "content:thinking:5", kind: "content", phase: undefined },
      {
        id: "interactive-tool:tool-question-boundary",
        kind: "content",
        phase: undefined,
      },
      {
        id: "interactive-tool:tool-permission-boundary",
        kind: "content",
        phase: undefined,
      },
      {
        id: "tool-run:tool-after-boundaries",
        kind: "tool_run",
        phase: "active",
      },
    ],
  );
  const failedSegment = segments[0];
  assert.equal(failedSegment.errorCount, 1);
  assert.deepEqual(
    failedSegment.projection.content.map(({ type }) => type),
    [
      "tool_use",
      "tool_result",
      "task_progress",
      "system_event",
      "workspace_file_artifact",
    ],
  );
  assert.deepEqual(
    [...segments[1].projection.streamingIndexes],
    [0],
    "segment slicing remaps the original streaming block index",
  );
  assert.deepEqual(
    segments[2].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
    "AskUserQuestion keeps its own stable interaction segment",
  );
  assert.deepEqual(
    segments[3].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
    "a pending permission tool stays outside the collapsible run",
  );
});

test("DM tool run view expands only the active segment and leaves Room direct content unchanged", async () => {
  const { AssistantDmToolRuns } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx",
  );
  const { AssistantMessageContent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-message-content.tsx",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const tool = {
    type: "tool_use",
    id: "tool-view",
    name: "Read",
    input: { file_path: "view.ts" },
  };
  const unresolvedProjection = {
    content: [tool],
    streamingIndexes: new Set(),
  };
  const resolvedProjection = {
    content: [
      tool,
      {
        type: "tool_result",
        tool_use_id: tool.id,
        content: "view",
      },
    ],
    streamingIndexes: new Set(),
  };
  const roomProjection = {
    content: [{
      type: "tool_use",
      id: "tool-room-view",
      name: "Read",
      input: { file_path: "view.ts" },
    }],
    streamingIndexes: new Set(),
  };
  const activity = {
    emptyStreamStatus: null,
    showCursor: true,
    standalone: false,
    state: "executing",
  };
  const permissions = {
    all: [],
    matchedByToolUseId: new Map(),
    owner: "content",
    unmatched: [],
  };
  const environment = {
    canRespondToPermissions: true,
    hiddenToolNames: [],
    mode: "dm_live",
  };
  const renderDm = (
    responseResumed,
    projection = resolvedProjection,
  ) => renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(AssistantDmToolRuns, {
        activity,
        environment,
        generatedFilesLabel: "生成文件",
        permissions,
        projection,
        responseResumed,
      }),
    ),
  );

  const activeHtml = renderDm(false);
  assert.match(
    activeHtml,
    /data-conversation-process-group-id="tool-run:tool-view"/,
  );
  assert.match(activeHtml, /data-dm-tool-run-phase="active"/);
  assert.match(activeHtml, /aria-expanded="true"/);
  assert.match(activeHtml, /读取内容/);

  const completedHtml = renderDm(true);
  assert.match(completedHtml, /data-dm-tool-run-phase="complete"/);
  assert.match(completedHtml, /aria-expanded="false"/);
  assert.doesNotMatch(
    completedHtml,
    /读取内容/,
    "completed tool details stay out of the live reading path until expanded",
  );

  const unresolvedResponseHtml = renderDm(true, unresolvedProjection);
  assert.match(unresolvedResponseHtml, /data-dm-tool-run-phase="active"/);
  assert.match(unresolvedResponseHtml, /aria-expanded="true"/);

  const staleFinalHtml = renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(AssistantMessageContent, {
        activity,
        direct: { projection: unresolvedProjection, visible: true },
        environment,
        final: {
          content: "先前已经显示的正文",
          isStreaming: false,
          mentions: [],
          streamingIndexes: new Set(),
          visible: true,
        },
        permissions: { ...permissions, owner: "composer" },
        process: {
          anchorRef: { current: null },
          expanded: false,
          projection: { content: [], streamingIndexes: new Set() },
          summary: "",
          toggle: () => {},
          visible: false,
        },
        showMaxTokensWarning: false,
      }),
    ),
  );
  assert.match(
    staleFinalHtml,
    /data-dm-tool-run-phase="active"/,
    "old final text already visible must not close a newer live tool",
  );
  assert.match(staleFinalHtml, /aria-expanded="true"/);

  const roomHtml = renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(AssistantMessageContent, {
        activity,
        direct: { projection: roomProjection, visible: true },
        environment: { ...environment, mode: "room_thread" },
        final: {
          content: null,
          isStreaming: false,
          mentions: [],
          streamingIndexes: new Set(),
          visible: false,
        },
        permissions: { ...permissions, owner: "composer" },
        process: {
          anchorRef: { current: null },
          expanded: false,
          projection: { content: [], streamingIndexes: new Set() },
          summary: "",
          toggle: () => {},
          visible: false,
        },
        showMaxTokensWarning: false,
      }),
    ),
  );
  assert.doesNotMatch(roomHtml, /data-dm-tool-run-list/);
  assert.match(roomHtml, /读取内容/);
});

test("semantic tool rejection stays distinct from transport completion in DM and Room", async () => {
  const { AssistantDmToolRuns } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx",
  );
  const { ContentRenderer } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer.tsx",
  );
  const { resolveToolBlockStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer-model.ts",
  );
  const { ToolBlockResult } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/blocks/tool/tool-block-detail.tsx",
  );
  const { projectDmToolRunSegments } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/dm-tool-run-segments.ts",
  );
  const { buildProcessSummary } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/message-process-summary.ts",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const tool = {
    type: "tool_use",
    id: "tool-plan-rejected",
    name: "mcp__nexus_execution__prepare_plan_execution",
    input: {
      plan_document: [
        "nexus_plan: 1",
        "operation: create",
        "objective: 产出 LPL 本周看点简报",
        "completion_criteria:",
        "  - 简报可供发布",
        "items: []",
      ].join("\n"),
    },
  };
  const result = {
    type: "tool_result",
    tool_use_id: tool.id,
    is_error: false,
    content: JSON.stringify({
      message: "Plan Document items must contain at least one complete Work Item",
      next_actions: [{
        reason: "submit one complete Nexus Plan Document with every intended Work Item",
        tool: "prepare_plan_execution",
      }],
      outcome: "rejected",
      reason_code: "plan_items_empty",
    }),
  };
  const projection = {
    content: [tool, result],
    streamingIndexes: new Set(),
  };
  const [segment] = projectDmToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection,
    responseResumed: true,
  });
  assert.equal(segment.phase, "rejected");
  assert.equal(segment.rejectedCount, 1);
  assert.equal(segment.errorCount, 0);
  assert.equal(
    resolveToolBlockStatus({ result }, false),
    "rejected",
    "a completed transport must not turn a rejected mutation green",
  );
  assert.deepEqual(
    buildProcessSummary({
      pendingPermissionCount: 0,
      processContent: [tool, result],
    }).metrics,
    [
      { count: 1, kind: "action" },
      { count: 1, kind: "error" },
    ],
  );

  const provider = (child) => React.createElement(I18nProvider, null, child);
  const dmHtml = renderToStaticMarkup(provider(React.createElement(
    AssistantDmToolRuns,
    {
      activity: {
        emptyStreamStatus: null,
        showCursor: true,
        standalone: false,
        state: "executing",
      },
      environment: {
        canRespondToPermissions: true,
        hiddenToolNames: [],
        mode: "dm_live",
      },
      generatedFilesLabel: "生成文件",
      permissions: {
        all: [],
        matchedByToolUseId: new Map(),
        owner: "content",
        unmatched: [],
      },
      projection,
      responseResumed: true,
    },
  )));
  assert.match(dmHtml, /data-dm-tool-run-phase="rejected"/);
  assert.match(dmHtml, /已拒绝/);
  assert.doesNotMatch(dmHtml, />完成</);

  const roomHtml = renderToStaticMarkup(provider(React.createElement(
    ContentRenderer,
    { content: [tool, result] },
  )));
  assert.match(roomHtml, /已拒绝/);
  assert.match(roomHtml, /Plan Document items/);
  assert.doesNotMatch(roomHtml, /next_actions/);

  const detailHtml = renderToStaticMarkup(provider(React.createElement(
    ToolBlockResult,
    { toolResult: result },
  )));
  assert.match(detailHtml, /data-tool-result-semantic-outcome="rejected"/);
  assert.match(detailHtml, /Plan Document items/);
  assert.match(detailHtml, /plan_items_empty/);
  assert.doesNotMatch(detailHtml, /next_actions/);
});

test("thinking and replying indicators render a real stepped frame track", async () => {
  const { MessageActivityStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/message-activity-status.tsx",
  );

  for (const [state, label] of [
    ["thinking", "正在思考"],
    ["replying", "正在回复"],
  ]) {
    const html = renderToStaticMarkup(
      React.createElement(MessageActivityStatus, { label, state }),
    );
    assert.match(html, new RegExp(label));
    assert.match(html, /message-activity-spinner-track/);
    assert.match(
      html,
      /animation:nexus-message-activity-frames [^;]+ steps\((?:[2-9]|[1-9][0-9]+), end\) infinite/,
      `${state} must animate through multiple frames instead of freezing one glyph`,
    );
  }
});

test("live empty text mounts before the first stream batch while history stays sparse", async () => {
  const { shouldMountTextContentBlock } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer-model.ts",
  );

  assert.equal(
    shouldMountTextContentBlock("", true),
    true,
    "a new live block must establish its Markdown identity before the first text batch",
  );
  assert.equal(
    shouldMountTextContentBlock("   ", false),
    false,
    "an empty historical block must not create layout height",
  );
  assert.equal(
    shouldMountTextContentBlock("历史正文", false),
    true,
    "historical text remains immediately renderable on first mount",
  );
});

test("same-RAF live text starts empty while history and recovery snapshots stay immediate", async () => {
  const { MarkdownRenderer } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/markdown-renderer.tsx",
  );
  const {
    clearLiveStreamRevealMarkers,
    hasLiveStreamRevealMarker,
  } = await server.ssrLoadModule(
    "/src/lib/conversation/live-stream-reveal.ts",
  );
  const { applyStreamPayloadBatchForActiveSession } =
    await server.ssrLoadModule(
      "/src/hooks/agent/transport/use-conversation-stream-buffer.ts",
    );
  const { upsertMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );

  const sessionKey = "room:group:first-frame";
  const base = {
    agent_id: "agent-1",
    message_id: "assistant-first-frame",
    round_id: "round-first-frame",
    session_key: sessionKey,
    timestamp: 1,
  };
  const liveMessages = applyStreamPayloadBatchForActiveSession(
    [],
    [
      { ...base, type: "message_start" },
      {
        ...base,
        content_block: { type: "text", text: "" },
        index: 0,
        type: "content_block_start",
      },
      {
        ...base,
        content_block: { type: "text", text: "首批完整正文" },
        index: 0,
        type: "content_block_delta",
      },
    ],
    sessionKey,
    sessionKey,
  );
  const liveBlock = liveMessages[0]?.content[0];
  assert.equal(hasLiveStreamRevealMarker(liveBlock), true);

  const liveHtml = renderToStaticMarkup(React.createElement(
    MarkdownRenderer,
    {
      content: liveBlock.text,
      initialRevealFromEmpty: hasLiveStreamRevealMarker(liveBlock),
      isStreaming: true,
    },
  ));
  assert.doesNotMatch(
    liveHtml,
    /首批完整正文/,
    "a non-empty first transport batch must enter the local backlog before any text is painted",
  );

  const historicalHtml = renderToStaticMarkup(React.createElement(
    MarkdownRenderer,
    {
      content: "恢复中的历史正文",
      isStreaming: true,
    },
  ));
  assert.match(
    historicalHtml,
    /恢复中的历史正文/,
    "an active snapshot without the local live marker must not replay from the beginning",
  );

  const placeholder = applyStreamPayloadBatchForActiveSession(
    [],
    [{ ...base, message_id: "assistant-snapshot-first", type: "message_start" }],
    sessionKey,
    sessionKey,
  );
  const snapshotFirst = upsertMessage(placeholder, {
    ...base,
    content: [{ type: "text", text: "终态快照首批正文" }],
    is_complete: true,
    message_id: "assistant-snapshot-first",
    role: "assistant",
    stop_reason: "end_turn",
    stream_status: "done",
  });
  assert.equal(
    hasLiveStreamRevealMarker(snapshotFirst[0]?.content[0]),
    true,
    "a canonical snapshot in the message_start task must preserve the local first-frame contract",
  );

  const cleared = clearLiveStreamRevealMarkers(
    liveMessages,
    new Set([base.message_id]),
  );
  assert.equal(hasLiveStreamRevealMarker(cleared[0]?.content[0]), false);
});

test("active MessageItem streaming height resets between Assistant turns", async () => {
  const { resolveMessageItemStreamingLayoutState } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/message-item-streaming-layout.ts",
  );
  const tallTurn = {
    active: true,
    assistantTurnKey: "assistant-long-response",
    minHeight: 960,
  };

  assert.strictEqual(
    resolveMessageItemStreamingLayoutState(
      tallTurn,
      "assistant-long-response",
      true,
    ),
    tallTurn,
    "streaming revisions within one Assistant turn retain the monotonic height",
  );
  assert.deepEqual(
    resolveMessageItemStreamingLayoutState(
      tallTurn,
      "assistant-tool-continuation",
      true,
    ),
    {
      active: true,
      assistantTurnKey: "assistant-tool-continuation",
      minHeight: 60,
    },
    "a later tool or response turn cannot inherit the preceding long response height",
  );
  assert.deepEqual(
    resolveMessageItemStreamingLayoutState(
      tallTurn,
      "assistant-long-response",
      false,
    ),
    {
      active: false,
      assistantTurnKey: "assistant-long-response",
      minHeight: 60,
    },
    "terminal layout clears the streaming height before the same turn can resume",
  );
});

test("DM live and terminal keep the final response on one content surface", async () => {
  const {
    projectionFromOrderedEntries,
    resolveAssistantResponseSurface,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/message-item-projection.ts",
  );
  const { resolveMessageItemFinalProjection } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts",
  );
  const message = assistantMessage({
    messageId: "assistant-dm-surface",
    roundId: "round-dm-surface",
    status: "streaming",
    text: "稳定逐字正文",
    timestamp: 2,
  });
  const thinking = { type: "thinking", thinking: "整理答案" };
  const textBlock = message.content[0];
  const orderedEntries = [
    {
      block: thinking,
      mergedIndex: 0,
      sourceMessageId: message.message_id,
      sourceOrder: 0,
    },
    {
      block: textBlock,
      mergedIndex: 1,
      sourceMessageId: message.message_id,
      sourceOrder: 0,
    },
  ];
  const visibleTurns = [{
    content: [thinking, textBlock],
    messageId: message.message_id,
    streamingIndexes: new Set([1]),
    textContent: [textBlock],
    textStreamingIndexes: new Set([0]),
  }];
  const project = (assistantContentMode, streamingBlockIndexes) => (
    resolveMessageItemFinalProjection({
      assistantContentMode,
      assistantMessages: [message],
      orderedProjection: projectionFromOrderedEntries(
        orderedEntries,
        streamingBlockIndexes,
      ),
      resultSummary: undefined,
      roundId: message.round_id,
      streamingBlockIndexes,
      visibleAssistantTurns: visibleTurns,
      visibleOrderedAssistantEntries: orderedEntries,
    })
  );

  assert.equal(resolveAssistantResponseSurface("dm_live"), "final");
  assert.equal(resolveAssistantResponseSurface("dm_archived"), "final");
  const live = project("dm_live", new Set([1]));
  const terminal = project("dm_archived", new Set());
  assert.deepEqual(live.finalAssistantContent, terminal.finalAssistantContent);
  assert.deepEqual(live.directOrderedProjection.content, [thinking]);
  assert.deepEqual(terminal.processProjection.content, [thinking]);
  assert.equal(
    live.directOrderedProjection.content.some((block) => block.type === "text"),
    false,
    "the live process track must not duplicate or own the final response text",
  );
});

test("Goal 完成收据只在 assistant 真正终态后打开 footer", async () => {
  const { resolveAssistantDisplayState } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/display/message-item-display-model.ts",
  );
  const projection = (streamStatus) => ({
    assistantMessages: [{}],
    directOrderedProjection: { content: [] },
    finalAssistantContent: [{ type: "text", text: "最终交付" }],
    finalAssistantStreamingIndexes: new Set(),
    finalAssistantText: "最终交付",
    goalCompletionReceipt: { goal_id: "goal-hidden", round_id: "round-hidden" },
    liveActivityState: null,
    mergedContent: [{ type: "text", text: "最终交付" }],
    pendingInteractionPermissions: [],
    processProjection: { content: [] },
    resultSummary: null,
    stats: null,
    streamStatus,
    streamingBlockIndexes: new Set(),
  });
  const resolve = (streamStatus) => resolveAssistantDisplayState({
    assistantContentMode: "dm_archived",
    hasStopHandler: false,
    isLastRound: true,
    isLoading: streamStatus !== "done",
    pendingPermissionCount: 0,
    projection: projection(streamStatus),
  });

  assert.equal(resolve("streaming").footerVisible, false);
  assert.equal(resolve("done").footerVisible, true);
});

test("迟到历史用 Goal 完成收据推进同一 assistant 快照", async () => {
  const { mergeLoadedMessages } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );
  const base = {
    agent_id: "agent-1",
    content: [{ type: "text", text: "最终交付" }],
    is_complete: true,
    message_id: "assistant-receipt-history",
    role: "assistant",
    round_id: "round-receipt-history",
    session_key: "agent:agent-1:ws:dm:receipt-history",
    stop_reason: "end_turn",
    timestamp: 1000,
  };
  const merged = mergeLoadedMessages(
    [{
      ...base,
      goal_completion_receipt: {
        actual_tokens: 42,
        goal_id: "goal-hidden",
        round_id: base.round_id,
      },
    }],
    [base],
  );

  assert.equal(merged.length, 1);
  assert.equal(merged[0].goal_completion_receipt.actual_tokens, 42);
  assert.equal(merged[0].content[0].text, "最终交付");
});

test("Room terminal result keeps public structure, hides thinking, and preserves monotonic text", async () => {
  const { resolveRoomResultFinalAssistantContent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts",
  );
  const fallback = [{ type: "text", text: "逐字输出" }];
  const extended = resolveRoomResultFinalAssistantContent({
    fallbackFinalAssistantContent: fallback,
    resultText: "逐字输出完成",
  });

  assert.ok(Array.isArray(extended));
  assert.equal(extended[0]?.type, "text");
  assert.equal(extended[0]?.text, "逐字输出完成");
  assert.equal(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: fallback,
      resultText: "逐字",
    }),
    fallback,
    "a shorter terminal summary must not shrink already visible Room text",
  );
  assert.deepEqual(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: null,
      resultText: "仅有终态结果",
    }),
    [{ type: "text", text: "仅有终态结果" }],
  );

  const thinking = { type: "thinking", thinking: "内部过程" };
  const artifact = {
    type: "workspace_file_artifact",
    path: "report.md",
  };
  const corrected = resolveRoomResultFinalAssistantContent({
    fallbackFinalAssistantContent: [
      thinking,
      { type: "text", text: "旧正文" },
      artifact,
    ],
    resultText: "修订后的正文",
  });
  assert.deepEqual(corrected[0], { type: "text", text: "修订后的正文" });
  assert.equal(corrected[1], artifact);
  assert.deepEqual(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: [thinking, artifact],
      resultText: "附件后的正文",
    }),
    [artifact, { type: "text", text: "附件后的正文" }],
  );
  assert.deepEqual(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: [{
        type: "text",
        text: "逐字输出 \n",
      }],
      resultText: "逐字输出\n完成",
    }),
    [{ type: "text", text: "逐字输出\n完成" }],
    "terminal suffixes must use the visible text boundary without duplicating whitespace",
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
  const runtimeExitMessage =
    "Agent runtime 的响应流意外结束，本轮未完成。会话会在下一条消息自动恢复，请重试。";
  assert.equal(
    latestAssistantResultErrorMessage([assistantMessage({
      messageId: "assistant-runtime-exit",
      resultSummary: {
        duration_api_ms: 0,
        duration_ms: 0,
        is_error: true,
        num_turns: 0,
        result: runtimeExitMessage,
        subtype: "error",
        timestamp: 2,
      },
      roundId: "round-runtime-exit",
      text: "",
      timestamp: 2,
    })]),
    null,
    "result-only failure is already visible as the final assistant reply",
  );
  assert.equal(
    latestAssistantResultErrorMessage([assistantMessage({
      messageId: "assistant-partial-runtime-exit",
      resultSummary: {
        duration_api_ms: 0,
        duration_ms: 0,
        is_error: true,
        num_turns: 0,
        result: runtimeExitMessage,
        subtype: "error",
        timestamp: 2,
      },
      roundId: "round-partial-runtime-exit",
      text: "已完成一部分输出",
      timestamp: 2,
    })]),
    runtimeExitMessage,
    "partial assistant output still needs a separate terminal error banner",
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

test("round status updates lifecycle without duplicating durable error copy", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const applied = [];
  let errorWrites = 0;
  const context = {
    runtime: {
      applyAgentRoundStatus: (payload) => {
        applied.push(["agent", payload.status]);
      },
      applyRoundStatus: (_roundId, status) => {
        applied.push(["round", status]);
      },
    },
    scope: { isCurrentSessionEvent: () => true },
    state: {
      setError: () => {
        errorWrites += 1;
      },
    },
  };

  AGENT_SESSION_EVENT_HANDLERS.agent_round_status({
    data: {
      agent_id: "agent-1",
      agent_round_id: "agent-round-1",
      is_terminal: true,
      round_id: "round-1",
      status: "error",
    },
    event_type: "agent_round_status",
    protocol_version: 2,
    session_key: "room:group:conversation-1",
    timestamp: 1,
  }, context);
  AGENT_SESSION_EVENT_HANDLERS.round_status({
    data: {
      is_terminal: true,
      message: "already projected by durable result",
      result_subtype: "error",
      round_id: "round-1",
      status: "error",
    },
    event_type: "round_status",
    protocol_version: 2,
    session_key: "room:group:conversation-1",
    timestamp: 2,
  }, context);

  assert.deepEqual(applied, [["agent", "error"], ["round", "error"]]);
  assert.equal(errorWrites, 0);
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

test("Room no-message terminal projection replaces hidden control markers", async () => {
  const { projectRoomAgentExecutionMessages } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-execution-model.ts",
  );
  const marker = "<nexus_room_no_reply/>";
  const [terminal] = projectRoomAgentExecutionMessages({
    agentId: "agent-1",
    labels: {
      failed: "Failed",
      stopped: "Stopped",
    },
    messages: [],
    resultSummary: {
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: false,
      num_turns: 1,
      result: marker,
      subtype: "interrupted",
      timestamp: 1,
    },
    roundId: "round-stopped",
    status: "cancelled",
    timestamp: 1,
  });
  assert.equal(terminal.result_summary?.result, "Stopped");
});

test("Room public cards hide thinking while Thread keeps it available", async () => {
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

test("Room Thread inspector keeps process without repeating the public reply", async () => {
  const {
    projectionFromOrderedEntries,
    shouldShowAssistantTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/message-item-projection.ts",
  );
  const { resolveMessageItemFinalProjection } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts",
  );
  const thinking = { thinking: "合并同类项", type: "thinking" };
  const finalText = { text: "合并同类项，每点扩到约 20 字。", type: "text" };
  const assistant = assistantMessage({
    messageId: "assistant-room-thread-process",
    text: finalText.text,
    timestamp: 1,
  });
  assistant.content = [thinking, finalText];
  const orderedEntries = [thinking, finalText].map((block, mergedIndex) => ({
    block,
    mergedIndex,
    sourceMessageId: assistant.message_id,
    sourceOrder: 0,
  }));
  const visibleTurns = [{
    content: [thinking, finalText],
    messageId: assistant.message_id,
    streamingIndexes: new Set(),
    textContent: [finalText],
    textStreamingIndexes: new Set(),
  }];
  const project = (assistantContentMode) => resolveMessageItemFinalProjection({
    assistantContentMode,
    assistantMessages: [assistant],
    orderedProjection: projectionFromOrderedEntries(
      orderedEntries,
      new Set(),
    ),
    resultSummary: undefined,
    roundId: assistant.round_id,
    streamingBlockIndexes: new Set(),
    visibleAssistantTurns: visibleTurns,
    visibleOrderedAssistantEntries: orderedEntries,
  });

  assert.deepEqual(
    project("room_thread_process").directOrderedProjection.content,
    [thinking],
    "Room inspector 应只保留思考、工具和系统过程",
  );
  assert.deepEqual(
    project("room_thread").directOrderedProjection.content,
    [thinking, finalText],
    "通用 transcript 仍需保留完整输出",
  );
  assert.equal(
    shouldShowAssistantTimeline("room_thread_process"),
    false,
    "Room inspector 不应重复绘制内部时间轴线和圆点",
  );
  assert.equal(
    shouldShowAssistantTimeline("room_thread"),
    true,
    "通用 transcript 仍需保留过程时间轴",
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
    isSessionSettingsSaving: false,
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
  const ready = {
    ...base,
    hasStopAction: false,
    inputState: projectComposerInput("next turn", 0),
    runtimeState: {
      activity: null,
      canStopGeneration: false,
      isAwaitingPermission: false,
      sessionBusy: false,
    },
  };
  assert.equal(projectComposerActions(ready).isSendDisabled, false);
  assert.equal(
    projectComposerActions({
      ...ready,
      isSessionSettingsSaving: true,
    }).isSendDisabled,
    true,
    "model and permission changes must persist before the next turn starts",
  );
});

test("Room Composer stop-all freezes exact active multi-Agent targets at click time", async () => {
  const {
    collectActiveRoomAgentRoundIds,
    stopRoomAgentOutputs,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/panel/controller/use-group-chat-composer-model.ts",
  );
  const conversation = {
    room_agent_execution_states: [
      { agent_round_id: "round-agent-a", phase: "active" },
      { agent_round_id: "round-agent-b", phase: "pending_permission" },
      { agent_round_id: "round-agent-finished", phase: "terminal" },
    ],
    pending_agent_slots: [
      { agent_round_id: "round-agent-a", status: "streaming" },
      { agent_round_id: "round-agent-c", status: "pending" },
      { agent_round_id: "round-agent-finished", status: "completed" },
    ],
    stopping_agent_round_ids: ["round-agent-b"],
  };
  const targets = collectActiveRoomAgentRoundIds(conversation);

  assert.deepEqual(
    targets,
    ["round-agent-a", "round-agent-c"],
    "terminal, duplicate, and already-stopping rounds must not enter the batch",
  );

  const stopped = [];
  stopRoomAgentOutputs(targets, (agentRoundId) => {
    stopped.push(agentRoundId);
    if (agentRoundId === "round-agent-a") {
      targets.push("round-agent-late");
    }
  });
  assert.deepEqual(
    stopped,
    ["round-agent-a", "round-agent-c"],
    "the first synchronous stop response must not mutate the click-time batch",
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

test("queued stream patches cannot shorten a newer terminal snapshot", async () => {
  const { applyStreamMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/stream-message-reducer.ts",
  );
  const terminal = assistantMessage({
    isComplete: true,
    messageId: "assistant-terminal-race",
    status: "done",
    stopReason: "end_turn",
    text: "完整正文abcdef",
    timestamp: 20,
  });
  const terminalMessages = [terminal];

  const afterStalePatch = applyStreamMessage(terminalMessages, {
    agent_id: "agent-1",
    content_block: { type: "text", text: "完整正文abc" },
    index: 0,
    message_id: "assistant-terminal-race",
    round_id: "round-root",
    session_key: "room:group:conversation-1",
    timestamp: 10,
    type: "content_block_delta",
  });

  assert.equal(afterStalePatch[0]?.content[0]?.text, "完整正文abcdef");
  assert.equal(afterStalePatch[0]?.stream_status, "done");
  assert.equal(
    afterStalePatch,
    terminalMessages,
    "the delayed RAF patch must leave the terminal snapshot unchanged",
  );

  for (const status of ["cancelled", "error"]) {
    const stopped = assistantMessage({
      messageId: `assistant-${status}-race`,
      status,
      text: `${status} 完整正文`,
      timestamp: 20,
    });
    const stoppedMessages = [stopped];
    const afterStoppedPatch = applyStreamMessage(stoppedMessages, {
      agent_id: "agent-1",
      content_block: { type: "text", text: `${status} 旧正文` },
      index: 0,
      message_id: `assistant-${status}-race`,
      round_id: "round-root",
      session_key: "room:group:conversation-1",
      timestamp: 10,
      type: "content_block_delta",
    });

    assert.equal(afterStoppedPatch, stoppedMessages);
    assert.equal(afterStoppedPatch[0]?.stream_status, status);
    assert.equal(afterStoppedPatch[0]?.content[0]?.text, `${status} 完整正文`);
  }
});

test("queued stream patches cannot shorten a newer active snapshot", async () => {
  const { applyStreamMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/stream-message-reducer.ts",
  );
  const active = assistantMessage({
    messageId: "assistant-active-race",
    status: "streaming",
    text: "正在输出的较新完整正文",
    timestamp: 20,
  });
  const activeMessages = [active];

  const afterStalePatch = applyStreamMessage(activeMessages, {
    agent_id: "agent-1",
    content_block: { type: "text", text: "正在输出的较新" },
    index: 0,
    message_id: "assistant-active-race",
    round_id: "round-root",
    session_key: "room:group:conversation-1",
    timestamp: 10,
    type: "content_block_delta",
  });

  assert.equal(afterStalePatch, activeMessages);
  assert.equal(
    afterStalePatch[0]?.content[0]?.text,
    "正在输出的较新完整正文",
  );
  assert.equal(afterStalePatch[0]?.stream_status, "streaming");
});

test("RAF stream batches stay isolated to the latest exact session", async () => {
  const { applyStreamPayloadBatchForActiveSession } =
    await server.ssrLoadModule(
      "/src/hooks/agent/transport/use-conversation-stream-buffer.ts",
    );
  const payload = (sessionKey, messageId, timestamp) => ({
    agent_id: "agent-1",
    message_id: messageId,
    round_id: "round-stream-buffer",
    session_key: sessionKey,
    timestamp,
    type: "message_start",
  });
  const currentSession = "room:group:conversation-current";
  const messages = applyStreamPayloadBatchForActiveSession(
    [],
    [
      payload(currentSession, "assistant-current-1", 1),
      payload("room:group:conversation-old", "assistant-old", 2),
      payload(currentSession, "assistant-current-2", 3),
    ],
    currentSession,
    currentSession,
  );
  assert.deepEqual(
    messages.map((message) => message.message_id),
    ["assistant-current-1", "assistant-current-2"],
    "the current session keeps batch arrival order while old payloads are discarded",
  );
  assert.equal(
    applyStreamPayloadBatchForActiveSession(
      messages,
      [payload(currentSession, "assistant-stale-after-switch", 4)],
      currentSession,
      "room:group:conversation-newer",
    ),
    messages,
    "a second session switch before React commits must reject the whole captured batch",
  );
  assert.equal(
    applyStreamPayloadBatchForActiveSession(
      messages,
      [payload("agent:default:workspace:group:c1", "assistant-alias", 5)],
      "room:group:c1",
      "room:group:c1",
    ),
    messages,
    "stream isolation uses exact protocol session keys instead of cross-scope aliases",
  );
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

test("Room consumes a legacy synthetic result by its parent slot with repeated agents", async () => {
  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const rootUser = userMessage({
    content: "同一个 Agent 连续执行两次",
    messageId: "user-repeated-agent",
    roundId: "round-repeated-agent",
    timestamp: 1,
  });
  const firstCanonical = assistantMessage({
    agentId: "agent-repeat",
    agentRoundId: "agent-round-first",
    messageId: "assistant-repeat-first",
    roundId: "round-repeated-agent",
    text: "第一次执行过程",
    timestamp: 2,
  });
  const secondCanonical = assistantMessage({
    agentId: "agent-repeat",
    agentRoundId: "agent-round-second",
    messageId: "assistant-repeat-second",
    roundId: "round-repeated-agent",
    text: "第二次执行过程",
    timestamp: 3,
  });
  const legacyResult = {
    ...assistantMessage({
      agentId: "agent-repeat",
      isComplete: true,
      messageId: "assistant_result_round-repeated-agent",
      resultSummary: {
        duration_api_ms: 10,
        duration_ms: 20,
        is_error: false,
        num_turns: 1,
        result: "第一次完成",
        subtype: "success",
        timestamp: 4,
      },
      roundId: "round-repeated-agent",
      status: "done",
      stopReason: "end_turn",
      text: "第一次完成",
      timestamp: 4,
    }),
    agent_round_id: undefined,
    parent_id: "slot-repeat-first",
  };
  const slots = [
    {
      agent_id: "agent-repeat",
      agent_round_id: "agent-round-first",
      index: 0,
      msg_id: "slot-repeat-first",
      round_id: "round-repeated-agent",
      status: "streaming",
      timestamp: 2,
    },
    {
      agent_id: "agent-repeat",
      agent_round_id: "agent-round-second",
      index: 1,
      msg_id: "slot-repeat-second",
      round_id: "round-repeated-agent",
      status: "streaming",
      timestamp: 3,
    },
  ];
  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([[
      "round-repeated-agent",
      [rootUser, firstCanonical, secondCanonical, legacyResult],
    ]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map([["round-repeated-agent", slots]]),
    roundIds: ["round-repeated-agent"],
  });
  const projectedIds = Array.from(projection.messageGroups.values()).flatMap(
    (messages) => messages.map((message) => message.message_id),
  );
  assert.equal(
    projectedIds.includes("assistant_result_round-repeated-agent"),
    false,
    "the merged legacy terminal must not remain as a root-level ghost card",
  );
  assert.equal(
    projection.roundIds.filter((roundId) => roundId.startsWith("room-agent-round:")).length,
    2,
    "the two executions still keep separate stable nodes",
  );
  assert.deepEqual(
    projection.messageGroups.get("round-repeated-agent")?.map(
      (message) => message.message_id,
    ),
    ["user-repeated-agent"],
  );
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

test("Room keeps a permission-first execution on its agent_round node", async () => {
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const rootUser = userMessage({
    content: "需要 Agent 执行",
    messageId: "user-permission-first",
    roundId: "round-permission-first",
    timestamp: 1,
  });
  const permission = {
    agent_id: "agent-permission-first",
    agent_round_id: "agent-round-permission-first",
    interaction_mode: "question",
    request_id: "permission-first",
    round_id: "round-permission-first",
    tool_input: {
      questions: [{
        options: [{ label: "继续" }],
        question: "是否继续？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const slot = {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    index: 0,
    msg_id: "slot-permission-first",
    round_id: permission.round_id,
    status: "streaming",
    timestamp: 2,
  };
  const message = assistantMessage({
    agentId: permission.agent_id,
    agentRoundId: permission.agent_round_id,
    messageId: "assistant-permission-first",
    roundId: permission.round_id,
    status: "streaming",
    text: "继续执行中",
    timestamp: 3,
  });
  const agentNodeId = buildGroupAgentTimelineNodeId(
    permission.round_id,
    `${permission.agent_id}:agent-round:${permission.agent_round_id}`,
  );
  const project = ({ messages, permissions, slots }) => (
    projectGroupAgentTimeline({
      messageGroups: new Map([[permission.round_id, messages]]),
      pendingPermissionGroups: new Map([
        [permission.round_id, permissions],
      ]),
      pendingSlotGroups: new Map([[permission.round_id, slots]]),
      roundIds: [permission.round_id],
    })
  );

  const permissionFirst = project({
    messages: [rootUser],
    permissions: [permission],
    slots: [],
  });
  const withSlot = project({
    messages: [rootUser],
    permissions: [permission],
    slots: [slot],
  });
  const withMessage = project({
    messages: [rootUser, message],
    permissions: [permission],
    slots: [slot],
  });

  assert.deepEqual(permissionFirst.roundIds, [
    "round-permission-first",
    agentNodeId,
  ]);
  assert.deepEqual(withSlot.roundIds, permissionFirst.roundIds);
  assert.deepEqual(withMessage.roundIds, permissionFirst.roundIds);
  assert.equal(
    permissionFirst.pendingPermissionGroups.get(agentNodeId)?.[0]?.request_id,
    permission.request_id,
  );
  assert.equal(
    permissionFirst.pendingPermissionGroups.get("round-permission-first")?.length,
    0,
    "the permission must never render on the generic root before its slot arrives",
  );
});

test("Room permission-first order survives reverse slot arrival", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const permissions = [
    {
      agent_id: "agent-a",
      agent_round_id: "agent-round-a",
      request_id: "permission-a",
      round_id: "round-permission-order",
      tool_input: { command: "echo a" },
      tool_name: "Bash",
    },
    {
      agent_id: "agent-b",
      agent_round_id: "agent-round-b",
      request_id: "permission-b",
      round_id: "round-permission-order",
      tool_input: { command: "echo b" },
      tool_name: "Bash",
    },
  ];
  const reverseSlots = [
    {
      agent_id: "agent-b",
      agent_round_id: "agent-round-b",
      index: 0,
      msg_id: "slot-b",
      round_id: "round-permission-order",
      status: "streaming",
      timestamp: 2,
    },
    {
      agent_id: "agent-a",
      agent_round_id: "agent-round-a",
      index: 1,
      msg_id: "slot-a",
      round_id: "round-permission-order",
      status: "streaming",
      timestamp: 3,
    },
  ];
  assert.deepEqual(
    buildRoomAgentRoundEntries([], reverseSlots, permissions).map(
      (entry) => entry.agent_round_id,
    ),
    ["agent-round-a", "agent-round-b"],
    "later slot metadata must enrich existing executions without moving their cards",
  );
});

test("Room permission-first children append after an existing reply and never move", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    acknowledgeRoomAgentExecutionPermission,
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromMessages,
    syncRoomAgentExecutionsFromPermissions,
    syncRoomAgentExecutionsFromSlots,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const roundId = "round-parent-before-permissions";
  const parent = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-round-1",
    displayOrder: 10_000,
    isComplete: true,
    messageId: "assistant-agent-1",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "@Agent2 调研 M1/M2，@Agent3 调研 M3/M4",
    timestamp: 10,
  });
  const permissions = [
    {
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      request_id: "permission-agent-2",
      round_id: roundId,
      tool_input: { query: "M1 M2" },
      tool_name: "WebSearch",
    },
    {
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      request_id: "permission-agent-3",
      round_id: roundId,
      tool_input: { query: "M3 M4" },
      tool_name: "WebSearch",
    },
  ];
  const entryOrder = (messages, slots, pendingPermissions, states) => (
    buildRoomAgentRoundEntries(
      messages,
      slots,
      pendingPermissions,
      states,
    ).map((entry) => entry.agent_round_id)
  );

  // Agent Session permission events may beat the shared pending-slot snapshot.
  // The already visible parent reply still owns the first canonical position.
  const permissionFirst = syncRoomAgentExecutionsFromPermissions(
    [],
    permissions,
    20,
  );
  const expectedOrder = [
    "agent-round-1",
    "agent-round-2",
    "agent-round-3",
  ];
  assert.deepEqual(
    entryOrder([parent], [], permissions, permissionFirst),
    expectedOrder,
    "permission-only children must append after the existing parent reply",
  );

  const acknowledged = permissions.reduce(
    (states, permission) => acknowledgeRoomAgentExecutionPermission(
      states,
      permission,
      21,
    ),
    permissionFirst,
  );
  const afterPermissionRemoval = syncRoomAgentExecutionsFromPermissions(
    acknowledged,
    [],
    22,
  );
  assert.deepEqual(
    entryOrder([parent], [], [], afterPermissionRemoval),
    expectedOrder,
    "acknowledging the last permission must retain the same execution shells",
  );

  const reverseSlots = [
    {
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      index: 1,
      msg_id: "slot-agent-3",
      round_id: roundId,
      status: "streaming",
      timestamp: 30,
    },
    {
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      index: 0,
      msg_id: "slot-agent-2",
      round_id: roundId,
      status: "streaming",
      timestamp: 30,
    },
  ];
  const active = syncRoomAgentExecutionsFromSlots(
    afterPermissionRemoval,
    reverseSlots,
  );
  assert.deepEqual(
    entryOrder([parent], reverseSlots, [], active),
    expectedOrder,
    "reverse slot arrival must enrich rather than reorder permission-first nodes",
  );

  const agent2Stream = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-round-2",
    messageId: "assistant-agent-2",
    roundId,
    status: "streaming",
    text: "Agent2 正在回复",
    timestamp: 31,
  });
  const agent3Done = assistantMessage({
    agentId: "agent-3",
    agentRoundId: "agent-round-3",
    isComplete: true,
    messageId: "assistant-agent-3",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "Agent3 已回复",
    timestamp: 32,
  });
  const afterStream = syncRoomAgentExecutionFromStream(active, {
    agent_id: "agent-2",
    agent_round_id: "agent-round-2",
    message_id: "assistant-agent-2",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    timestamp: 31,
    type: "message_start",
  });
  const withMessages = syncRoomAgentExecutionsFromMessages(
    afterStream,
    [agent3Done, agent2Stream],
  );
  assert.deepEqual(
    entryOrder(
      [parent, agent3Done, agent2Stream],
      reverseSlots,
      [],
      withMessages,
    ),
    expectedOrder,
    "stream and terminal message evidence must keep the first visible order",
  );
});

test("Room stream-first children append after a visible legacy Lead reply", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    syncRoomAgentExecutionFromStream,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const roundId = "round-visible-lead-before-streams";
  const messages = [
    userMessage({
      content: "协作调研",
      messageId: "user-visible-lead-root",
      roundId,
      timestamp: 1,
    }),
    {
      ...userMessage({
        content: "已创建 goal",
        messageId: "user-visible-lead-goal",
        roundId,
        timestamp: 2,
      }),
      hidden_from_user: true,
    },
    assistantMessage({
      agentId: "agent-lead",
      isComplete: true,
      messageId: "assistant-visible-lead",
      roundId,
      status: "done",
      stopReason: "end_turn",
      text: "我先完成分工，Analyst 和 Researcher 继续执行。",
      timestamp: 10,
    }),
  ];
  const analystStream = {
    agent_id: "agent-analyst",
    agent_round_id: "agent-round-analyst",
    message_id: "assistant-analyst",
    round_id: roundId,
    session_key: "room:group:conversation-stream-first",
    timestamp: 20,
    type: "message_start",
  };
  const researcherStream = {
    agent_id: "agent-researcher",
    agent_round_id: "agent-round-researcher",
    message_id: "assistant-researcher",
    round_id: roundId,
    session_key: "room:group:conversation-stream-first",
    timestamp: 21,
    type: "message_start",
  };
  const afterAnalyst = syncRoomAgentExecutionFromStream([], analystStream);
  const afterResearcher = syncRoomAgentExecutionFromStream(
    afterAnalyst,
    researcherStream,
  );

  assert.deepEqual(
    buildRoomAgentRoundEntries(
      messages,
      [],
      [],
      afterResearcher,
    ).map((entry) => entry.agent_id),
    ["agent-lead", "agent-analyst", "agent-researcher"],
    "stream evidence must append after the Lead card that is already visible",
  );
  assert.deepEqual(
    afterResearcher.map((state) => state.display_order),
    [20_000, 21_000],
    "stream-first execution anchors must use the same timestamp scale as durable Room ordering",
  );
});

test("Room durable snapshot backfills an earlier Lead after a live child", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromMessages,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const roundId = "round-live-child-before-history";
  const lead = assistantMessage({
    agentId: "agent-lead",
    agentRoundId: "agent-round-lead",
    displayOrder: 10_000,
    isComplete: true,
    messageId: "assistant-history-lead",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "我先完成分工，Researcher 继续执行。",
    timestamp: 30,
  });
  const researcher = assistantMessage({
    agentId: "agent-researcher",
    agentRoundId: "agent-round-researcher",
    displayOrder: 20_000,
    messageId: "assistant-live-researcher",
    roundId,
    status: "streaming",
    text: "Researcher 正在调研",
    timestamp: 31,
  });
  const streamFirst = syncRoomAgentExecutionFromStream([], {
    agent_id: "agent-researcher",
    agent_round_id: "agent-round-researcher",
    message_id: "assistant-live-researcher",
    round_id: roundId,
    session_key: "room:group:conversation-live-before-history",
    timestamp: 21,
    type: "message_start",
  });
  const reconciled = syncRoomAgentExecutionsFromMessages(
    streamFirst,
    [lead, researcher],
  );
  const statesByAgent = new Map(
    reconciled.map((state) => [state.agent_id, state]),
  );

  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [lead, researcher],
      [],
      [],
      reconciled,
    ).map((entry) => entry.agent_id),
    ["agent-lead", "agent-researcher"],
    "a live child observed during history loading must not stay above its earlier durable Lead",
  );
  assert.equal(statesByAgent.get("agent-lead")?.display_order, 10_000);
  assert.equal(statesByAgent.get("agent-researcher")?.display_order, 20_000);
  assert.equal(
    statesByAgent.get("agent-researcher")?.first_seen_at,
    21,
    "canonical order reconciliation must preserve the original live first-seen timestamp",
  );
});

test("Room legacy snapshot fallback cannot speculate ahead of a live execution", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromMessages,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const roundId = "round-legacy-history-after-live";
  const legacyLead = assistantMessage({
    agentId: "agent-lead",
    agentRoundId: "agent-round-legacy-lead",
    isComplete: true,
    messageId: "assistant-legacy-lead",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "缺少持久展示顺序的旧 Lead 回复",
    timestamp: 10,
  });
  const liveResearcher = syncRoomAgentExecutionFromStream([], {
    agent_id: "agent-researcher",
    agent_round_id: "agent-round-live-researcher",
    message_id: "assistant-live-researcher",
    round_id: roundId,
    session_key: "room:group:conversation-legacy-history",
    timestamp: 20,
    type: "message_start",
  });
  const reconciled = syncRoomAgentExecutionsFromMessages(
    liveResearcher,
    [legacyLead],
  );

  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [legacyLead],
      [],
      [],
      reconciled,
    ).map((entry) => entry.agent_id),
    ["agent-researcher", "agent-lead"],
    "a legacy completion timestamp is not authoritative enough to move an unseen reply above a visible execution",
  );
});

test("Room late permission enriches an observed slot without moving its Agent", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    syncRoomAgentExecutionsFromPermissions,
    syncRoomAgentExecutionsFromSlots,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const slots = [
    {
      agent_id: "agent-a",
      agent_round_id: "agent-round-a",
      index: 0,
      msg_id: "slot-a",
      round_id: "round-late-permission",
      status: "streaming",
      timestamp: 1,
    },
    {
      agent_id: "agent-b",
      agent_round_id: "agent-round-b",
      index: 1,
      msg_id: "slot-b",
      round_id: "round-late-permission",
      status: "streaming",
      timestamp: 2,
    },
  ];
  const permissionForSecond = {
    agent_id: "agent-b",
    agent_round_id: "agent-round-b",
    request_id: "permission-b-late",
    round_id: "round-late-permission",
    tool_input: { command: "echo b" },
    tool_name: "Bash",
  };
  const observed = syncRoomAgentExecutionsFromSlots([], slots);
  const enriched = syncRoomAgentExecutionsFromPermissions(
    observed,
    [permissionForSecond],
    3,
  );

  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [],
      slots,
      [permissionForSecond],
      enriched,
    ).map((entry) => entry.agent_round_id),
    ["agent-round-a", "agent-round-b"],
    "a question or permission arriving for B must not move B above the visible A card",
  );
  assert.deepEqual(
    enriched.map((state) => state.display_order),
    observed.map((state) => state.display_order),
  );
});

test("Room execution anchors seed canonical history and slot order before live appends", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    syncRoomAgentExecutionsFromMessages,
    syncRoomAgentExecutionsFromSlots,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const firstFinishedLater = assistantMessage({
    agentId: "agent-a",
    agentRoundId: "agent-round-a-history",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "assistant-a-history",
    roundId: "round-canonical-seed",
    status: "done",
    stopReason: "end_turn",
    text: "A 后完成",
    timestamp: 40,
  });
  const secondFinishedEarlier = assistantMessage({
    agentId: "agent-b",
    agentRoundId: "agent-round-b-history",
    displayOrder: 2_000,
    isComplete: true,
    messageId: "assistant-b-history",
    roundId: "round-canonical-seed",
    status: "done",
    stopReason: "end_turn",
    text: "B 先完成",
    timestamp: 30,
  });
  const completionOrderedMessages = [
    secondFinishedEarlier,
    firstFinishedLater,
  ];
  const historyStates = syncRoomAgentExecutionsFromMessages(
    [],
    completionOrderedMessages,
  );
  assert.deepEqual(
    buildRoomAgentRoundEntries(
      completionOrderedMessages,
      [],
      [],
      historyStates,
    ).map((entry) => entry.agent_id),
    ["agent-a", "agent-b"],
    "completion timestamps must not replace the backend Agent start order",
  );
  assert.deepEqual(
    historyStates.map((state) => state.display_order),
    [1_000, 2_000],
  );

  const reverseSlots = [
    {
      agent_id: "agent-b",
      agent_round_id: "agent-round-b-slot",
      index: 1,
      msg_id: "slot-b-canonical",
      round_id: "round-slot-seed",
      status: "streaming",
      timestamp: 2,
    },
    {
      agent_id: "agent-a",
      agent_round_id: "agent-round-a-slot",
      index: 0,
      msg_id: "slot-a-canonical",
      round_id: "round-slot-seed",
      status: "streaming",
      timestamp: 2,
    },
  ];
  const slotStates = syncRoomAgentExecutionsFromSlots([], reverseSlots);
  assert.deepEqual(
    buildRoomAgentRoundEntries([], reverseSlots, [], slotStates).map(
      (entry) => entry.agent_id,
    ),
    ["agent-a", "agent-b"],
  );
});

test("Room acknowledged permission keeps one non-interactive node until evidence takes over", async () => {
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const {
    acknowledgeRoomAgentExecutionPermission,
    applyRoomAgentExecutionStatus,
    isVisibleRoomAgentExecutionState,
    stopRoomAgentExecutions,
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromPermissions,
    syncRoomAgentExecutionsFromSlots,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const roundId = "round-acknowledged-permission";
  const permission = {
    agent_id: "agent-ack",
    agent_round_id: "agent-round-ack",
    interaction_mode: "question",
    request_id: "permission-ack",
    round_id: roundId,
    tool_input: {
      questions: [{
        options: [{ label: "继续" }],
        question: "是否继续？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const rootUser = userMessage({
    content: "执行并询问",
    messageId: "user-ack",
    roundId,
    timestamp: 1,
  });
  const nodeId = buildGroupAgentTimelineNodeId(
    roundId,
    "agent-ack:agent-round:agent-round-ack",
  );
  const project = (permissions, slots, states) => projectGroupAgentTimeline({
    messageGroups: new Map([[roundId, [rootUser]]]),
    pendingPermissionGroups: new Map([[roundId, permissions]]),
    pendingSlotGroups: new Map([[roundId, slots]]),
    roomAgentExecutionStateGroups: new Map([[roundId, states]]),
    roundIds: [roundId],
  });

  const pending = syncRoomAgentExecutionsFromPermissions([], [permission], 2);
  const beforeResponse = project([permission], [], pending);
  const acknowledged = acknowledgeRoomAgentExecutionPermission(
    pending,
    permission,
    3,
  );
  const afterPermissionRemoval = syncRoomAgentExecutionsFromPermissions(
    acknowledged,
    [],
    4,
  );
  const afterResponse = project([], [], afterPermissionRemoval);
  assert.deepEqual(beforeResponse.roundIds, [roundId, nodeId]);
  assert.deepEqual(afterResponse.roundIds, beforeResponse.roundIds);
  assert.equal(afterResponse.pendingPermissionGroups.get(nodeId)?.length, 0);
  assert.equal(afterPermissionRemoval[0]?.phase, "acknowledged");
  assert.equal(isVisibleRoomAgentExecutionState(afterPermissionRemoval[0]), true);

  const slot = {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    index: 0,
    msg_id: "slot-ack",
    round_id: roundId,
    status: "streaming",
    timestamp: 5,
  };
  const active = syncRoomAgentExecutionsFromSlots(
    afterPermissionRemoval,
    [slot],
  );
  assert.deepEqual(project([], [slot], active).roundIds, beforeResponse.roundIds);
  assert.equal(active[0]?.display_order, afterPermissionRemoval[0]?.display_order);
  assert.equal(active[0]?.phase, "active");

  const stopped = stopRoomAgentExecutions(afterPermissionRemoval);
  assert.equal(stopped[0]?.phase, "terminal");
  assert.equal(stopped[0]?.status, "cancelled");

  const turnStopped = syncRoomAgentExecutionFromStream([], {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    message_id: "assistant-ack",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    timestamp: 6,
    type: "message_stop",
  });
  assert.equal(turnStopped[0]?.phase, "active");
  assert.equal(turnStopped[0]?.status, "streaming");
  const authoritativeError = applyRoomAgentExecutionStatus(turnStopped, {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    is_terminal: true,
    round_id: roundId,
    status: "error",
  }, 7);
  assert.equal(authoritativeError[0]?.status, "error");
  const staleRunning = applyRoomAgentExecutionStatus(authoritativeError, {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    is_terminal: false,
    round_id: roundId,
    status: "running",
  }, 8);
  assert.equal(staleRunning[0]?.phase, "terminal");
  assert.equal(staleRunning[0]?.status, "error");
});

test("Room Assistant turn completion keeps its Agent execution active", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const {
    syncRoomAgentExecutionFromLiveMessage,
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromMessages,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const roundId = "round-tool-continuation";
  const agentRoundId = "agent-round-tool-continuation";
  const turnStop = {
    agent_id: "agent-tool",
    agent_round_id: agentRoundId,
    message: { stop_reason: "tool_use" },
    message_id: "assistant-tool-turn",
    round_id: roundId,
    session_key: "room:group:conversation-tool",
    timestamp: 2,
    type: "message_stop",
  };
  const fromStream = syncRoomAgentExecutionFromStream([], turnStop);
  assert.equal(fromStream[0]?.phase, "active");
  assert.equal(fromStream[0]?.status, "streaming");

  const toolTurnMessage = assistantMessage({
    agentId: "agent-tool",
    agentRoundId,
    isComplete: true,
    messageId: "assistant-tool-turn",
    roundId,
    status: "done",
    stopReason: "tool_use",
    text: "先调用工具",
    timestamp: 2,
  });
  const fromDurableTurn = syncRoomAgentExecutionsFromMessages(
    fromStream,
    [toolTurnMessage],
  );
  assert.equal(fromDurableTurn[0]?.phase, "active");
  assert.equal(fromDurableTurn[0]?.status, "streaming");

  const completedPublicTurn = assistantMessage({
    agentId: "agent-tool",
    agentRoundId,
    isComplete: true,
    messageId: "assistant-public-turn",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "我先同步计划，Thread 继续执行。",
    timestamp: 3,
  });
  const afterCompletedLiveTurn = syncRoomAgentExecutionFromLiveMessage(
    fromStream,
    completedPublicTurn,
  );
  assert.equal(
    afterCompletedLiveTurn[0]?.phase,
    "active",
    "a live Assistant turn cannot close its enclosing Agent execution",
  );
  assert.equal(afterCompletedLiveTurn[0]?.status, "streaming");
  const afterActiveSnapshot = syncRoomAgentExecutionsFromMessages(
    fromStream,
    [completedPublicTurn],
  );
  assert.equal(
    afterActiveSnapshot[0]?.phase,
    "active",
    "a reconnect snapshot cannot close an already observed live execution",
  );
  assert.equal(
    buildRoomAgentRoundEntries(
      [completedPublicTurn],
      [],
      [],
      afterCompletedLiveTurn,
    )[0]?.status,
    "streaming",
    "the public card must follow the active Agent lifecycle while its Thread continues",
  );

  const activeSlot = {
    agent_id: "agent-tool",
    agent_round_id: agentRoundId,
    index: 0,
    msg_id: "slot-tool-turn",
    round_id: roundId,
    status: "streaming",
    timestamp: 1,
  };
  assert.equal(
    buildRoomAgentRoundEntries(
      [toolTurnMessage],
      [activeSlot],
      [],
      fromDurableTurn,
    )[0]?.status,
    "streaming",
    "a completed tool-use message is not the terminal state of its agent_round",
  );
});

test("Room terminal execution rejects stale active evidence and late interaction", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const {
    filterPendingPermissionsForTerminalRoomExecutions,
  } = await server.ssrLoadModule(
    "/src/lib/conversation/pending-permission-match.ts",
  );
  const { AgentConversationRuntimeMachine } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/agent-conversation-runtime-machine.ts",
  );
  const roundId = "round-terminal-monotonic";
  const agentRoundId = "agent-round-terminal-monotonic";
  const staleMessage = assistantMessage({
    agentId: "agent-terminal",
    agentRoundId,
    messageId: "assistant-terminal-stale-stream",
    roundId,
    status: "streaming",
    text: "迟到的流式快照",
    timestamp: 4,
  });
  const staleSlot = {
    agent_id: "agent-terminal",
    agent_round_id: agentRoundId,
    index: 0,
    msg_id: "slot-terminal-stale",
    round_id: roundId,
    status: "streaming",
    timestamp: 2,
  };
  const terminalState = {
    agent_id: "agent-terminal",
    agent_round_id: agentRoundId,
    display_order: 0,
    first_seen_at: 1,
    phase: "terminal",
    round_id: roundId,
    status: "error",
  };
  const lateQuestion = {
    agent_id: "agent-terminal",
    agent_round_id: agentRoundId,
    interaction_mode: "question",
    request_id: "question-after-terminal",
    round_id: roundId,
    tool_input: {
      questions: [{
        options: [{ label: "重试" }],
        question: "是否继续？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {},
    executionStates: [terminalState],
    messages: [staleMessage],
    pendingPermissions: [lateQuestion],
    pendingSlots: [staleSlot],
  });

  assert.equal(
    model.entries[0]?.status,
    "error",
    "an authoritative terminal error must bound stale slot/message activity",
  );
  assert.deepEqual(
    model.entries[0]?.pendingPermissions,
    [],
    "a late exact permission cannot revive an interactive terminal shell",
  );

  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([[roundId, [staleMessage]]]),
    pendingPermissionGroups: new Map([[roundId, [lateQuestion]]]),
    pendingSlotGroups: new Map([[roundId, [staleSlot]]]),
    roomAgentExecutionStateGroups: new Map([[roundId, [terminalState]]]),
    roundIds: [roundId],
  });
  assert.deepEqual(
    Array.from(projection.pendingPermissionGroups.values()).flat(),
    [],
    "the filtered permission must not fall back to a generic root card",
  );

  const rawPermissions =
    filterPendingPermissionsForTerminalRoomExecutions(
      [lateQuestion],
      [terminalState],
    );
  assert.deepEqual(
    rawPermissions,
    [],
    "the volatile source must reject the request before it contributes to runtime count",
  );
  const runtime = new AgentConversationRuntimeMachine("group");
  runtime.trackRoundStatus(roundId, "running");
  runtime.setPendingPermissionCount(rawPermissions.length);
  assert.notEqual(
    runtime.snapshot().phase,
    "awaiting_permission",
    "an invisible late request must not lock the composer",
  );

  const legacyQuestion = {
    ...lateQuestion,
    agent_round_id: null,
    request_id: "legacy-question-after-terminal",
  };
  assert.deepEqual(
    filterPendingPermissionsForTerminalRoomExecutions(
      [legacyQuestion],
      [terminalState],
    ),
    [legacyQuestion],
    "legacy interaction without an exact execution identity cannot be guessed away",
  );
});

test("targeted stop mutates only its execution after the interrupt is sent", async () => {
  const { stopSessionGeneration } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/conversation-control-actions.ts",
  );
  const roundId = "round-targeted-stop";
  const permissionA = {
    agent_id: "agent-a",
    agent_round_id: "agent-round-a-stop",
    request_id: "permission-a-stop",
    round_id: roundId,
    tool_input: { command: "echo a" },
    tool_name: "Bash",
  };
  const permissionB = {
    agent_id: "agent-b",
    agent_round_id: "agent-round-b-question",
    interaction_mode: "question",
    request_id: "permission-b-question",
    round_id: roundId,
    tool_input: {
      questions: [{
        options: [{ label: "继续" }],
        question: "B 是否继续？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const legacyPermission = {
    agent_id: "agent-b",
    request_id: "permission-b-legacy",
    round_id: roundId,
    tool_input: { command: "echo legacy" },
    tool_name: "Bash",
  };
  const createContext = (disposition) => {
    let error = "old error";
    let permissions = [permissionA, permissionB, legacyPermission];
    let sentCommand = null;
    const context = {
      acknowledgePermissionRequest: () => {},
      activeSessionKeyRef: { current: "room:group:conversation-stop" },
      identity: {
        agent_id: "agent-a",
        chat_type: "group",
        conversation_id: "conversation-stop",
        room_id: "room-stop",
      },
      messages: [userMessage({
        content: "并行执行",
        messageId: "user-targeted-stop",
        roundId,
        timestamp: 1,
      })],
      pendingPermissions: permissions,
      sessionKey: "room:group:conversation-stop",
      setError: (next) => {
        error = typeof next === "function" ? next(error) : next;
      },
      setMessages: () => {},
      setPendingPermissions: (next) => {
        permissions = typeof next === "function" ? next(permissions) : next;
      },
      wsSend: (command) => {
        sentCommand = command;
        return { disposition };
      },
      wsState: "connected",
    };
    return {
      context,
      read: () => ({ error, permissions, sentCommand }),
    };
  };

  const sent = createContext("sent");
  const request = stopSessionGeneration(
    sent.context,
    permissionA.agent_round_id,
  );
  assert.ok(request);
  assert.equal(
    sent.read().sentCommand.agent_round_id,
    permissionA.agent_round_id,
  );
  assert.equal(
    sent.read().sentCommand.client_request_id,
    request.client_request_id,
  );
  assert.deepEqual(
    sent.read().permissions.map((permission) => permission.request_id),
    [permissionB.request_id, legacyPermission.request_id],
    "stopping A must preserve B's exact and legacy interactions",
  );
  assert.equal(sent.read().error, null);

  const dropped = createContext("dropped");
  assert.equal(
    stopSessionGeneration(dropped.context, permissionA.agent_round_id),
    null,
  );
  assert.deepEqual(
    dropped.read().permissions.map((permission) => permission.request_id),
    [
      permissionA.request_id,
      permissionB.request_id,
      legacyPermission.request_id,
    ],
    "a failed interrupt must leave runtime-facing interaction state retryable",
  );
  assert.equal(dropped.read().error, "中断请求发送失败，请稍后重试");
});

test("Room exact stop survives slot cleanup and settles ACK/terminal races per Agent", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const { confirmRoomAgentExecutionStop } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const {
    addStoppingAgentRoundId,
    removeStoppingAgentRoundId,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/state/use-conversation-volatile-state.ts",
  );
  const { buildRoomExecutionActivityKey } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/panel/controller/use-group-chat-panel-model.ts",
  );
  const { parseInterruptAckData } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );
  const roundId = "round-stop-race";
  const stateA = {
    agent_id: "agent-a",
    agent_round_id: "agent-round-a",
    display_order: 1,
    first_seen_at: 1,
    phase: "active",
    round_id: roundId,
    status: "streaming",
  };
  const stateB = {
    ...stateA,
    agent_id: "agent-b",
    agent_round_id: "agent-round-b",
    display_order: 2,
  };
  const completedTurn = assistantMessage({
    agentId: stateA.agent_id,
    agentRoundId: stateA.agent_round_id,
    isComplete: true,
    messageId: "assistant-a-tool-boundary",
    roundId,
    status: "done",
    stopReason: "tool_use",
    text: "先完成一段输出",
    timestamp: 2,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {},
    executionStates: [stateA, stateB],
    messages: [completedTurn],
    pendingPermissions: [],
    pendingSlots: [],
  });
  assert.equal(
    model.entries.find((entry) => entry.agent_id === stateA.agent_id)
      ?.stopAgentRoundId,
    stateA.agent_round_id,
    "the exact stop target must come from execution identity after pending slot cleanup",
  );

  let stopping = addStoppingAgentRoundId([], stateA.agent_round_id);
  assert.strictEqual(
    addStoppingAgentRoundId(stopping, stateA.agent_round_id),
    stopping,
    "a double click must not register the same exact target twice",
  );
  stopping = addStoppingAgentRoundId(stopping, stateB.agent_round_id);
  stopping = removeStoppingAgentRoundId(stopping, stateA.agent_round_id);
  const terminalBeforeAck = removeStoppingAgentRoundId(
    stopping,
    stateA.agent_round_id,
  );
  assert.deepEqual(
    terminalBeforeAck,
    [stateB.agent_round_id],
    "terminal-before-ACK settlement must be idempotent and preserve Agent B",
  );

  const stoppedStates = confirmRoomAgentExecutionStop(
    [stateA, stateB],
    stateA.agent_round_id,
  );
  assert.equal(stoppedStates[0].phase, "terminal");
  assert.equal(stoppedStates[0].status, "cancelled");
  assert.strictEqual(stoppedStates[1], stateB);
  assert.strictEqual(
    confirmRoomAgentExecutionStop(stoppedStates, stateA.agent_round_id),
    stoppedStates,
    "ACK-before-terminal and terminal-before-ACK must converge idempotently",
  );
  assert.notEqual(
    buildRoomExecutionActivityKey(1, true, [stateA, stateB]),
    buildRoomExecutionActivityKey(1, true, stoppedStates),
    "the WorkGraph resource must refresh when one Agent reaches interrupted terminal",
  );
  assert.deepEqual(
    parseInterruptAckData({
      accepted: true,
      ack_timeout_ms: 10_000,
      agent_round_id: stateA.agent_round_id,
      client_request_id: "request-stop-a",
      round_id: roundId,
    }),
    {
      accepted: true,
      ack_timeout_ms: 10_000,
      agent_round_id: stateA.agent_round_id,
      client_request_id: "request-stop-a",
      round_id: roundId,
    },
  );
});

test("Room stopping controls and unresolved tools share the interrupted terminal state", async () => {
  const { ContentRenderer } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer.tsx",
  );
  const { GroupAgentExecutionShell } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-execution-shell.tsx",
  );
  const { resolveToolBlockStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer-model.ts",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const provider = (child) => React.createElement(I18nProvider, null, child);
  const shellHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentExecutionShell,
    {
      agentAvatar: null,
      agentId: "agent-stopping",
      agentName: "Researcher",
      isStopping: true,
      isThreadActive: false,
      messages: [assistantMessage({
        agentId: "agent-stopping",
        agentRoundId: "agent-round-stopping",
        messageId: "assistant-stopping",
        status: "done",
        stopReason: "tool_use",
        text: "准备调用工具",
        timestamp: 1,
      })],
      onClickThread: () => {},
      onPermissionResponse: () => true,
      onStopAgentRound: () => {},
      pendingPermissions: [],
      roundId: "round-stopping:agent-stopping",
      status: "streaming",
      timestamp: 1,
    },
  )));
  assert.match(shellHtml, /停止中…/);
  assert.match(shellHtml, /disabled=""/);

  const toolUse = {
    id: "tool-interrupted",
    input: { file_path: "report.md" },
    name: "Write",
    type: "tool_use",
  };
  const toolHtml = renderToStaticMarkup(provider(React.createElement(
    ContentRenderer,
    {
      content: [toolUse],
      unresolvedToolStatus: "stopped",
    },
  )));
  assert.match(toolHtml, /已停止/);
  assert.doesNotMatch(toolHtml, />执行中</);
  assert.doesNotMatch(toolHtml, /处理中…/);
  assert.equal(resolveToolBlockStatus(undefined, false, "stopped"), "stopped");
  assert.equal(
    resolveToolBlockStatus({ result: { content: "ok", is_error: false } }, false, "stopped"),
    "success",
    "a real provider result must outrank the terminal fallback",
  );
});

test("Room virtual height keeps Composer interactions out of the feed estimate", async () => {
  const { projectGroupRoundHeights } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-height-model.ts",
  );
  const roundId = "room-agent-round:height";
  const slot = {
    agent_id: "agent-height",
    agent_round_id: "agent-round-height",
    msg_id: "slot-height",
    round_id: "round-height",
    status: "pending",
    timestamp: 1,
  };
  const question = {
    agent_id: slot.agent_id,
    agent_round_id: slot.agent_round_id,
    interaction_mode: "question",
    request_id: "question-height",
    round_id: slot.round_id,
    tool_input: {
      questions: [{
        options: [
          { label: "方案 A", description: "按当前配置继续" },
          { label: "方案 B", description: "调整配置后继续" },
        ],
        question: "请选择下一步执行方案",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const baseHeights = new Map([[roundId, 96]]);
  const slotOnly = projectGroupRoundHeights({
    baseHeights,
    containerWidth: 640,
    messageGroups: new Map([[roundId, []]]),
    pendingPermissionGroups: new Map([[roundId, []]]),
    pendingSlotGroups: new Map([[roundId, [slot]]]),
    roundIds: [roundId],
  });
  const withQuestion = projectGroupRoundHeights({
    baseHeights,
    containerWidth: 640,
    messageGroups: new Map([[roundId, []]]),
    pendingPermissionGroups: new Map([[roundId, [question]]]),
    pendingSlotGroups: new Map([[roundId, [slot]]]),
    roundIds: [roundId],
  });

  assert.ok(slotOnly.get(roundId) > baseHeights.get(roundId));
  assert.equal(
    withQuestion.get(roundId),
    slotOnly.get(roundId),
    "a Composer-owned question must not reserve a second form inside the feed",
  );
  assert.equal(baseHeights.get(roundId), 96, "the shared estimate stays immutable");
});

test("Room virtual height preserves matched tool evidence while Composer owns approval", async () => {
  const { projectGroupRoundHeights } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-height-model.ts",
  );
  const roundId = "room-agent-round:matched-height";
  const toolUseId = "tool-matched-height";
  const permission = {
    agent_id: "agent-height",
    agent_round_id: "agent-round-height",
    interaction_mode: "permission",
    message_id: "assistant-height",
    request_id: "permission-height",
    round_id: "round-height",
    tool_input: { command: "echo height" },
    tool_name: "Bash",
    tool_use_id: toolUseId,
  };
  const assistant = {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    content: [{
      id: toolUseId,
      input: permission.tool_input,
      name: permission.tool_name,
      type: "tool_use",
    }],
    message_id: permission.message_id,
    role: "assistant",
    round_id: permission.round_id,
    session_key: "room:group:conversation-1",
    stream_status: "streaming",
    timestamp: 1,
  };
  const baseHeights = new Map([[roundId, 300]]);
  const project = (messages, permissions) => projectGroupRoundHeights({
    baseHeights,
    containerWidth: 640,
    messageGroups: new Map([[roundId, messages]]),
    pendingPermissionGroups: new Map([[roundId, permissions]]),
    pendingSlotGroups: new Map([[roundId, []]]),
    roundIds: [roundId],
  }).get(roundId);
  const unmatchedHeight = project(
    [assistant],
    [{ ...permission, tool_use_id: "tool-other" }],
  );
  const matchedHeight = project([assistant], [permission]);
  assert.equal(
    unmatchedHeight,
    matchedHeight,
    "permission matching must not remove the read-only tool evidence",
  );
  assert.equal(
    project(
      [assistant],
      [{ ...permission, summary: "旧快照" }, permission],
    ),
    matchedHeight,
    "duplicate snapshots of one request must not inflate virtual height",
  );
  assert.equal(
    project([
      assistant,
      {
        ...assistant,
        content: [{
          content: "ok",
          tool_use_id: toolUseId,
          type: "tool_result",
        }],
        message_id: "assistant-height-result",
        timestamp: 2,
      },
    ], [permission]),
    unmatchedHeight,
    "a resolved tool row is no longer deducted as an unresolved match",
  );
});

test("DM and Room follow real content growth with one compact tail", async () => {
  const { getScrollBottomTop } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  const { resolveConversationRound } = await server.ssrLoadModule(
    "/src/features/conversation/shared/feed/conversation-feed-model.ts",
  );
  const { ConversationFeed } = await server.ssrLoadModule(
    "/src/features/conversation/shared/feed/conversation-feed.tsx",
  );
  const { GroupConversationFeed } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-feed.tsx",
  );
  const roomRoundIds = ["old-root", "root", "root-agent-1", "root-agent-2"];
  const roomRootIds = new Map([
    ["root-agent-1", "root"],
    ["root-agent-2", "root"],
  ]);
  const initialBottomTop = getScrollBottomTop({
    clientHeight: 600,
    scrollHeight: 1_000,
    scrollTop: 400,
  });
  const grownBottomTop = getScrollBottomTop({
    clientHeight: 600,
    scrollHeight: 1_180,
    scrollTop: 400,
  });
  assert.equal(
    grownBottomTop - initialBottomTop,
    180,
    "each content-height delta immediately moves FOLLOW upward by the same delta",
  );
  const clientMessageId = "client-message-stable";
  const optimisticState = resolveConversationRound({
    liveRoundIds: [],
    messageGroups: new Map([[clientMessageId, [{
      content: "hello",
      message_id: clientMessageId,
      role: "user",
      round_id: clientMessageId,
      timestamp: 1,
    }]]]),
    pendingPermissions: [],
    roundIds: [clientMessageId],
  }, 0);
  const canonicalRoundId = "canonical-round";
  const canonicalState = resolveConversationRound({
    liveRoundIds: [],
    messageGroups: new Map([[canonicalRoundId, [{
      client_message_id: clientMessageId,
      content: "hello",
      message_id: "canonical-message",
      role: "user",
      round_id: canonicalRoundId,
      timestamp: 1,
    }]]]),
    pendingPermissions: [],
    roundIds: [canonicalRoundId],
  }, 0);
  assert.equal(
    canonicalState.nodeId,
    optimisticState.nodeId,
    "ACK preserves the same React and Virtualizer node identity",
  );

  const refs = {
    bottomAnchorRef: { current: null },
    scrollRef: { current: null },
  };
  const dmHtml = renderToStaticMarkup(React.createElement(ConversationFeed, {
    isMobileLayout: false,
    refs,
    renderer: {
      currentAgentName: "Agent",
      onPermissionResponse: () => true,
    },
    source: {
      liveRoundIds: [],
      messageGroups: new Map(),
      pendingPermissions: [],
      roundIds: ["dm-old", "dm-last"],
    },
  }));
  assert.equal(dmHtml.match(/data-conversation-feed-tail/g)?.length, 1);
  assert.equal(dmHtml.includes("data-conversation-runway"), false);
  assert.equal(
    dmHtml.match(/\bpb-1\b/g)?.length,
    2,
    "each measurable desktop DM round owns its spacing",
  );
  assert.doesNotMatch(dmHtml, /\b(?:gap-1|space-y-4)\b/);

  const roomHtml = renderToStaticMarkup(React.createElement(
    GroupConversationFeed,
    {
      isMobileLayout: false,
      refs,
      renderer: {
        agentAvatarMap: {},
        agentNameMap: {},
        currentAgentAvatar: null,
        currentAgentName: null,
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        runtimePhase: null,
      },
      source: {
        liveRoundIds: [],
        messageGroups: new Map(),
        pendingPermissionGroups: new Map(),
        pendingSlotGroups: new Map(),
        roomAgentExecutionStateGroups: new Map(),
        rootRoundIds: roomRootIds,
        roundIds: roomRoundIds,
      },
    },
  ));
  assert.equal(roomHtml.match(/data-conversation-feed-tail/g)?.length, 1);
  assert.equal(roomHtml.includes("data-conversation-runway"), false);
  assert.equal(
    roomHtml.match(/\bpb-1\b/g)?.length,
    roomRoundIds.length,
    "each measurable desktop Room node owns its spacing",
  );
  assert.doesNotMatch(roomHtml, /\b(?:gap-1|space-y-4)\b/);
});

test("Room Agent timestamp stays on start while active and switches to finish at terminal", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
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

test("Room timeline conserves user messages while optimistic roots become canonical", async () => {
  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const clientMessageId = "client-room-conservation";
  const optimistic = {
    ...userMessage({
      content: "先分析这件事",
      messageId: clientMessageId,
      roundId: "optimistic-room-round",
      timestamp: 1,
    }),
    client_message_id: clientMessageId,
  };
  const canonical = {
    ...optimistic,
    content: "先分析这件事（已确认）",
    message_id: "canonical-room-message",
    round_id: "canonical-room-round",
    timestamp: 2,
  };
  const followUp = userMessage({
    content: "再补充可靠性维度",
    messageId: "room-follow-up",
    roundId: "canonical-room-round",
    timestamp: 3,
  });
  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([
      ["optimistic-room-round", [optimistic]],
      ["canonical-room-round", [canonical, followUp]],
    ]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["optimistic-room-round", "canonical-room-round"],
  });

  assert.deepEqual(projection.roundIds, [clientMessageId]);
  assert.equal(
    projection.rootRoundIds.get(clientMessageId),
    "canonical-room-round",
  );
  assert.deepEqual(
    projection.messageGroups.get(clientMessageId)?.map(
      (message) => message.message_id,
    ),
    ["canonical-room-message", "room-follow-up"],
    "canonical ACK replacement must not overwrite another visible user message",
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
