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

test("FOLLOW keeps one scroll owner while parallel Room Agents grow", async () => {
  const { resolveConversationFollowCommitOwner } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );

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

test("READING preserves the first visible Room round during static growth", async () => {
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
  const { HistoryPrependAnchor } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/history-prepend-anchor.ts",
  );
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
  assert.doesNotMatch(
    pendingHtml,
    /room-agent-execution-enter/,
    "a pending shell must not add translated geometry during parallel growth",
  );
  assert.equal(
    pendingHtml.match(/message-activity-spinner-track/g)?.length,
    1,
    "a pending slot uses the shared activity surface exactly once",
  );
  assert.match(pendingHtml, /translate-y-\[2px\]/);

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
      {
        preceding_tool_use_ids: ["tool-public-search"],
        subtype: "tool_use_summary",
        text: "搜索产品线资料",
        type: "progress_update",
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
    /网络搜索 M3 product line/,
    "Room main feed mirrors the running tool group header",
  );
  assert.doesNotMatch(workingHtml, /搜索产品线资料/);
  assert.match(workingHtml, /data-room-tool-activity/);
  assert.match(workingHtml, /data-process-activity-icon="search"/);
  assert.match(
    workingHtml,
    /text-\(--icon-muted\)[^>]*data-process-activity-icon="search"/,
  );
  assert.match(workingHtml, /text-primary/);
  assert.doesNotMatch(workingHtml, /data-tool-run-list|data-tool-run-id/);
  assert.equal(
    workingHtml.match(/message-activity-spinner-track/g)?.length,
    undefined,
    "the running tool header does not add a second loading spinner",
  );

  const workingAfterReplyHtml = renderShell({
    messages: [
      toolContinuation,
      assistantMessage({
        agentId: "agent-public-activity",
        agentRoundId: "agent-round-public-activity",
        messageId: "assistant-public-reply-after-tool",
        roundId: "round-public-activity",
        status: "streaming",
        text: "先同步当前进度",
        timestamp: 3,
      }),
    ],
    status: "streaming",
  });
  assert.match(workingAfterReplyHtml, /data-room-tool-activity/);
  assert.match(workingAfterReplyHtml, /网络搜索 M3 product line/);
  assert.doesNotMatch(workingAfterReplyHtml, /正在回复/);
  assert.ok(
    workingAfterReplyHtml.indexOf("先同步当前进度")
      < workingAfterReplyHtml.indexOf("网络搜索 M3 product line"),
    "the current Room tool follows the newest visible reply",
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
  const progressSnapshot = {
    ...thinkingSnapshot,
    content: [{ type: "progress_update", text: "正在核对最后一项" }],
    delivery_mode: "ephemeral",
    message_id: "assistant-progress",
  };

  const reconciled = applyTerminalAgentRoundMessageStatus(
    [thinkingSnapshot, progressSnapshot, unrelatedSnapshot],
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
  assert.equal(
    reconciled.some((message) => message.message_id === "assistant-progress"),
    false,
    "slot 终态必须清除同 agent_round 的临时自然语言进度",
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

test("a visible ToolBlock exclusively owns its running state", async () => {
  const { ContentRenderer } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer.tsx",
  );
  const { resolveContentActivityState } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/activity/message-content-activity.ts",
  );
  const toolUse = {
    type: "tool_use",
    id: "tool-write-state-owner",
    name: "Write",
    input: { file_path: "PLAN.md" },
  };
  assert.equal(resolveContentActivityState({
    consumedBlockIndexes: new Set(),
    content: [toolUse],
    hiddenToolNames: new Set(),
    resolvedToolUseIds: new Set(),
  }), null);

  const html = await renderWithI18n(React.createElement(ContentRenderer, {
    content: [toolUse],
    fallbackActivityState: "executing",
    isStreaming: true,
  }));
  assert.equal(html.match(/>执行中</g)?.length, 1);
  assert.doesNotMatch(html, /正在执行/);
});

test("an active Room execution returns to Agent activity after tool completion", async () => {
  const { projectRoomAgentActivityState } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-execution-model.ts",
  );
  const message = {
    agent_id: "agent-state-owner",
    agent_round_id: "agent-round-state-owner",
    content: [
      { type: "text", text: "准备写入" },
      {
        type: "tool_use",
        id: "tool-write-complete",
        name: "Write",
        input: { file_path: "PLAN.md" },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-write-complete",
        content: "ok",
      },
    ],
    message_id: "assistant-state-owner",
    role: "assistant",
    round_id: "round-state-owner",
    session_key: "room:state-owner",
    stream_status: "streaming",
    timestamp: 1,
  };
  assert.equal(projectRoomAgentActivityState({
    messages: [message],
    pendingPermissions: [],
    status: "streaming",
  }), "thinking");

  const pendingToolMessage = {
    ...message,
    content: [{
      type: "tool_use",
      id: "tool-write-running",
      name: "Write",
      input: { file_path: "PLAN.md" },
    }],
    is_complete: true,
    message_id: "assistant-tool-running",
    stop_reason: "tool_use",
    stream_status: "done",
  };
  const laterReplyMessage = {
    ...message,
    content: [{ type: "text", text: "先同步当前进度" }],
    is_complete: false,
    message_id: "assistant-reply-after-tool",
    stop_reason: undefined,
    timestamp: 2,
  };
  assert.equal(projectRoomAgentActivityState({
    messages: [pendingToolMessage, laterReplyMessage],
    pendingPermissions: [],
    status: "streaming",
  }), "executing");
});

test("live conversations keep one stable tool segment across consecutive patches", async () => {
  const { projectToolRunSegments } = await server.ssrLoadModule(
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
  ) => projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live,
    projection: { content, streamingIndexes: new Set() },
    responseResumed,
    toolUseSummary: null,
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

test("tool segments absorb reasoning but preserve interactions and errors", async () => {
  const { projectToolRunSegments } = await server.ssrLoadModule(
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
  const [activeFailedSegment] = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: content.slice(0, 2),
      streamingIndexes: new Set(),
    },
    responseResumed: false,
    toolUseSummary: null,
  });
  assert.equal(activeFailedSegment.phase, "active");
  assert.equal(activeFailedSegment.errorCount, 1);
  const segments = projectToolRunSegments({
    interactiveToolUseIds: new Set([permissionTool.id]),
    live: true,
    projection: {
      content,
      streamingIndexes: new Set([5]),
    },
    responseResumed: false,
    toolUseSummary: null,
  });

  assert.deepEqual(
    segments.map(({ id, kind, phase }) => ({ id, kind, phase })),
    [
      {
        id: "tool-run:tool-failed",
        kind: "tool_run",
        phase: "error",
      },
      {
        id: "tool-run:tool-question-boundary",
        kind: "tool_run",
        phase: "complete",
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
      "thinking",
    ],
  );
  assert.deepEqual(
    [...failedSegment.projection.streamingIndexes],
    [5],
    "folded execution preserves the remapped streaming reasoning index",
  );
  assert.deepEqual(
    segments[1].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
    "a completed question starts a normal tool run without merging backward",
  );
  assert.deepEqual(
    segments[2].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
    "a pending permission tool stays outside the collapsible run",
  );
});

test("ToolUseSummary updates the matching uninterrupted tool row and text stays visible", async () => {
  const { projectToolRunSegments } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/dm-tool-run-segments.ts",
  );
  const toolA = {
    type: "tool_use",
    id: "tool-summary-a",
    name: "Read",
    input: { file_path: "a.ts" },
  };
  const toolB = {
    type: "tool_use",
    id: "tool-summary-b",
    name: "Grep",
    input: { pattern: "needle" },
  };
  const segments = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: [
        { type: "thinking", thinking: "先看现有实现" },
        toolA,
        { type: "tool_result", tool_use_id: toolA.id, content: "a" },
        { type: "thinking", thinking: "继续核对调用点" },
        toolB,
        { type: "tool_result", tool_use_id: toolB.id, content: "b" },
      ],
      streamingIndexes: new Set(),
    },
    responseResumed: false,
    toolUseSummary: {
      precedingToolUseIds: [toolB.id],
      text: "定位时间线折叠入口",
    },
  });

  assert.deepEqual(
    segments.map(({ phase, summaryText, toolUseIds }) => ({
      phase,
      summaryText,
      toolUseIds,
    })),
    [
      {
        phase: "active",
        summaryText: "定位时间线折叠入口",
        toolUseIds: [toolA.id, toolB.id],
      },
    ],
  );
  assert.deepEqual(
    segments[0].projection.content.map(({ type }) => type),
    [
      "thinking",
      "tool_use",
      "tool_result",
      "thinking",
      "tool_use",
      "tool_result",
    ],
  );

  const commentarySegments = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: [
        toolA,
        { type: "tool_result", tool_use_id: toolA.id, content: "a" },
        { type: "text", text: "这里是给用户看的阶段结论" },
        toolB,
        { type: "tool_result", tool_use_id: toolB.id, content: "b" },
      ],
      streamingIndexes: new Set(),
    },
    responseResumed: false,
    toolUseSummary: {
      precedingToolUseIds: [toolB.id],
      text: "定位时间线折叠入口",
    },
  });
  assert.deepEqual(
    commentarySegments.map((segment) => ({
      kind: segment.kind,
      summaryText: segment.kind === "tool_run" ? segment.summaryText : null,
      toolUseIds: segment.kind === "tool_run" ? segment.toolUseIds : [],
    })),
    [
      {
        kind: "tool_run",
        summaryText: null,
        toolUseIds: [toolA.id],
      },
      {
        kind: "content",
        summaryText: null,
        toolUseIds: [],
      },
      {
        kind: "tool_run",
        summaryText: "定位时间线折叠入口",
        toolUseIds: [toolB.id],
      },
    ],
  );
  assert.deepEqual(
    commentarySegments[0].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
  );
  assert.deepEqual(
    commentarySegments[1].projection.content.map(({ type }) => type),
    ["text"],
    "visible narration remains in the reading flow",
  );
  assert.deepEqual(
    commentarySegments[2].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
  );
});

test("live commentary separates completed and active tool groups", async () => {
  const { AssistantMessageContent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-message-content.tsx",
  );
  const { resolveMessageItemFinalProjection } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts",
  );
  const {
    projectionFromOrderedEntries,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/message-item-projection.ts",
  );
  const { projectToolRunSegments } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/dm-tool-run-segments.ts",
  );
  const initialThinking = { type: "thinking", thinking: "先查基础数据" };
  const initialText = { type: "text", text: "EARLIER_REPLY_MARKER" };
  const oldTool = {
    type: "tool_use",
    id: "tool-before-reply",
    name: "WebSearch",
    input: { url: "https://old.example.test" },
  };
  const oldResult = {
    type: "tool_result",
    tool_use_id: oldTool.id,
    content: "ok",
  };
  const nextThinking = { type: "thinking", thinking: "整理后补官方口径" };
  const commentary = { type: "text", text: "LATEST_REPLY_MARKER" };
  const activeTool = {
    type: "tool_use",
    id: "tool-after-reply",
    name: "WebFetch",
    input: { url: "https://latest.example.test" },
  };
  const firstMessage = {
    role: "assistant",
    message_id: "assistant-before-reply",
    parent_id: "user-boundary",
    content: [initialThinking, initialText, oldTool],
  };
  const activeMessage = {
    role: "assistant",
    message_id: "assistant-after-reply",
    parent_id: "user-boundary",
    content: [nextThinking, commentary, activeTool],
  };
  const orderedEntries = [
    ...firstMessage.content.map((block) => ({
      block,
      sourceMessageId: firstMessage.message_id,
      sourceOrder: 0,
    })),
    {
      block: oldResult,
      sourceMessageId: firstMessage.message_id,
      sourceOrder: 0,
    },
    ...activeMessage.content.map((block) => ({
      block,
      sourceMessageId: activeMessage.message_id,
      sourceOrder: 1,
    })),
  ].map((entry, mergedIndex) => ({ ...entry, mergedIndex }));
  const streamingBlockIndexes = new Set([orderedEntries.length - 1]);
  const projection = resolveMessageItemFinalProjection({
    assistantContentMode: "dm_live",
    assistantMessages: [firstMessage, activeMessage],
    orderedProjection: projectionFromOrderedEntries(
      orderedEntries,
      streamingBlockIndexes,
    ),
    resultSummary: undefined,
    roundId: "round-boundary",
    userMessageId: "user-boundary",
    streamingBlockIndexes,
    visibleAssistantTurns: [
      {
        content: [...firstMessage.content, oldResult],
        messageId: firstMessage.message_id,
        streamingIndexes: new Set(),
        textContent: [initialText],
        textStreamingIndexes: new Set(),
      },
      {
        content: activeMessage.content,
        messageId: activeMessage.message_id,
        streamingIndexes: new Set([2]),
        textContent: [commentary],
        textStreamingIndexes: new Set(),
      },
    ],
    visibleOrderedAssistantEntries: orderedEntries,
  });
  assert.equal(
    projection.finalAssistantContent,
    null,
    "commentary followed by a normal tool remains in the direct timeline",
  );
  const segments = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: projection.directOrderedProjection,
    responseResumed: false,
    toolUseSummary: null,
  });
  assert.deepEqual(
    segments.flatMap((segment) => (
      segment.kind === "tool_run" ? [segment.toolUseIds] : []
    )),
    [[oldTool.id], [activeTool.id]],
  );
  const html = await renderWithI18n(React.createElement(
    AssistantMessageContent,
    {
      activity: {
        emptyStreamStatus: null,
        label: null,
        showCursor: true,
        standalone: false,
        state: "browsing",
        toolUseSummary: null,
      },
      direct: {
        projection: projection.directOrderedProjection,
        visible: true,
      },
      environment: {
        canRespondToPermissions: true,
        hiddenToolNames: [],
        mode: "dm_live",
      },
      final: {
        content: projection.finalAssistantContent,
        mentions: [],
        isStreaming: false,
        streamingIndexes: projection.finalAssistantStreamingIndexes,
        visible: false,
      },
      permissions: {
        all: [],
        matchedByToolUseId: new Map(),
        owner: "composer",
        unmatched: [],
      },
      process: {
        anchorRef: { current: null },
        expanded: false,
        projection: { content: [], streamingIndexes: new Set() },
        summary: { kind: "details", latestDetail: null, metrics: [] },
        toggle: () => {},
        visible: false,
      },
      showMaxTokensWarning: false,
    },
  ));

  assert.ok(
    html.indexOf('data-tool-run-id="tool-run:tool-before-reply"')
      < html.indexOf("LATEST_REPLY_MARKER"),
  );
  assert.ok(html.indexOf("LATEST_REPLY_MARKER") < html.indexOf("latest.example.test"));
  assert.equal(html.match(/latest\.example\.test/g)?.length, 1);
  assert.match(html, /data-live-tool-text="true"/);
});

test("DM activity groups collapse while Room Thread groups expand", async () => {
  const { AssistantToolRuns } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx",
  );
  const content = Array.from({ length: 4 }, (_, index) => [
    { type: "thinking", thinking: `Thought ${index}` },
    {
      type: "tool_use",
      id: `tool-icon-${index}`,
      name: index % 2 === 0 ? "Bash" : "Read",
      input: {},
    },
    {
      type: "tool_result",
      tool_use_id: `tool-icon-${index}`,
      content: "ok",
    },
  ]).flat();
  const toolRunProps = {
    activity: {
      emptyStreamStatus: null,
      label: null,
      showCursor: false,
      standalone: false,
      state: "executing",
      toolUseSummary: null,
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
    projection: { content, streamingIndexes: new Set() },
    responseResumed: true,
  };
  const toolRunHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    toolRunProps,
  ));

  assert.match(toolRunHtml, /aria-expanded="false"/);
  assert.match(toolRunHtml, /data-process-activity-icon-count="8"/);
  assert.match(toolRunHtml, />\+2<\/span>/);
  assert.equal(
    toolRunHtml.match(/data-process-activity-icon=/g)?.length,
    6,
  );
  assert.match(toolRunHtml, /data-process-activity-icon="thinking"/);
  assert.match(toolRunHtml, /data-process-activity-icon="terminal"/);
  assert.match(toolRunHtml, /data-process-activity-icon="inspect"/);
  assert.ok(
    toolRunHtml.lastIndexOf("+2")
      > toolRunHtml.lastIndexOf("data-process-activity-icon="),
    "the overflow count follows the visible activity icons",
  );
  assert.match(toolRunHtml, /class="flex shrink-0 items-center/);
  assert.doesNotMatch(toolRunHtml, /Thought 0|data-tool-run-detail-list/);
  assert.match(toolRunHtml, /before:bottom-0/);
  assert.match(toolRunHtml, /data-timeline-dot/);
  assert.match(toolRunHtml, /nexus-chat-timeline-block/);

  const threadHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    {
      ...toolRunProps,
      environment: {
        ...toolRunProps.environment,
        mode: "room_thread_process",
      },
    },
  ));
  assert.match(threadHtml, /aria-expanded="true"/);
  assert.match(threadHtml, /data-tool-run-detail-list/);
  assert.match(threadHtml, /Thought 0/);
  assert.doesNotMatch(threadHtml, /before:bottom-0/);
  assert.doesNotMatch(threadHtml, /data-timeline-dot/);
  assert.doesNotMatch(threadHtml, /nexus-chat-timeline-block/);

  const liveThreadHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    {
      ...toolRunProps,
      activity: {
        ...toolRunProps.activity,
        showCursor: true,
      },
      environment: {
        ...toolRunProps.environment,
        mode: "room_thread_process",
      },
      responseResumed: false,
    },
  ));
  assert.match(liveThreadHtml, /data-message-detail-follow="true"/);

});

test("detail scroll fade follows the remaining scroll directions", async () => {
  const { MessageDetailScroll } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/ui/message-rail.tsx",
  );
  const { isAtScrollBottom, resolveScrollFade } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  const position = (scrollTop) => ({
    clientHeight: 280,
    scrollHeight: 560,
    scrollTop,
  });

  assert.equal(resolveScrollFade({
    clientHeight: 280,
    scrollHeight: 280,
    scrollTop: 0,
  }), "none");
  assert.equal(resolveScrollFade(position(0)), "bottom");
  assert.equal(resolveScrollFade(position(140)), "both");
  assert.equal(resolveScrollFade(position(280)), "top");
  assert.equal(isAtScrollBottom(position(140)), false);
  assert.equal(isAtScrollBottom(position(280)), true);

  const nestedHtml = renderToStaticMarkup(React.createElement(
    MessageDetailScroll,
    null,
    React.createElement(
      MessageDetailScroll,
      null,
      "Nested detail",
    ),
  ));
  assert.equal(
    nestedHtml.match(/data-message-detail-scroll/g)?.length,
    1,
    "nested details share the outer scroll owner",
  );
});

test("Thought detail uses compact tool-detail typography", async () => {
  const { ThinkingBlock } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/blocks/thinking-block.tsx",
  );
  const html = await renderWithI18n(React.createElement(ThinkingBlock, {
    isStreaming: true,
    thinking: "Compact detail",
  }));

  assert.match(html, /nexus-message-detail-markdown/);
  assert.match(html, /data-message-detail-sticky-header="true"/);
  assert.match(html, /data-message-detail-follow="true"/);
  assert.match(html, /data-markdown-streaming="true"/);
});

test("semantic tool rejection stays distinct from transport completion in DM and Room", async () => {
  const { AssistantToolRuns } = await server.ssrLoadModule(
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
  const { projectToolRunSegments } = await server.ssrLoadModule(
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
    name: "Bash",
    input: {
      command: '"${NEXUS_COMMAND_PATH}" --json execution invoke --operation prepare_plan_execution --request-id plan-rejected-1',
    },
  };
  const result = {
    type: "tool_result",
    tool_use_id: tool.id,
    is_error: false,
    content: JSON.stringify({
      domain: "execution",
      action: "invoke",
      operation: "prepare_plan_execution",
      result: { data: {
        message: "Plan Document items must contain at least one complete Work Item",
        next_actions: [{
          domain: "execution",
          operation: "prepare_plan_execution",
          reason: "submit one complete Nexus Plan Document with every intended Work Item",
        }],
        outcome: "rejected",
        reason_code: "plan_items_empty",
      } },
    }),
  };
  const projection = {
    content: [tool, result],
    streamingIndexes: new Set(),
  };
  const [segment] = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection,
    responseResumed: true,
    toolUseSummary: null,
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
    AssistantToolRuns,
    {
      activity: {
        emptyStreamStatus: null,
        label: null,
        showCursor: true,
        standalone: false,
        state: "executing",
        toolUseSummary: null,
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

test("superseded WorkGraph result is muted and does not count as failure", async () => {
  const { AssistantToolRuns } = await server.ssrLoadModule(
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
  const { projectToolRunSegments } = await server.ssrLoadModule(
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
    id: "tool-submit-superseded",
    name: "Bash",
    input: {
      command: '"${NEXUS_COMMAND_PATH}" --json execution invoke --operation submit_work --request-id submit-superseded-1',
    },
  };
  const result = {
    type: "tool_result",
    tool_use_id: tool.id,
    is_error: false,
    content: JSON.stringify({
      domain: "execution",
      action: "invoke",
      operation: "submit_work",
      result: { data: {
        message: "旧工作已被新目标替换；请停止当前轮次并等待新指派",
        outcome: "superseded",
        reason_code: "execution_terminal",
      } },
    }),
  };
  const projection = {
    content: [tool, result],
    streamingIndexes: new Set(),
  };
  const [segment] = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection,
    responseResumed: true,
    toolUseSummary: null,
  });
  assert.equal(segment.phase, "superseded");
  assert.equal(segment.supersededCount, 1);
  assert.equal(segment.rejectedCount, 0);
  assert.equal(segment.errorCount, 0);
  assert.equal(resolveToolBlockStatus({ result }, false), "superseded");
  assert.deepEqual(
    buildProcessSummary({
      pendingPermissionCount: 0,
      processContent: [tool, result],
    }).metrics,
    [{ count: 1, kind: "action" }],
  );

  const provider = (child) => React.createElement(I18nProvider, null, child);
  const dmHtml = renderToStaticMarkup(provider(React.createElement(
    AssistantToolRuns,
    {
      activity: {
        emptyStreamStatus: null,
        label: null,
        showCursor: true,
        standalone: false,
        state: "executing",
        toolUseSummary: null,
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
  assert.match(dmHtml, /已被替换/);
  assert.doesNotMatch(dmHtml, /执行失败|已拒绝/);

  const roomHtml = renderToStaticMarkup(provider(React.createElement(
    ContentRenderer,
    { content: [tool, result] },
  )));
  assert.match(roomHtml, /已被替换/);
  assert.match(roomHtml, /旧工作已被新目标替换/);
  assert.doesNotMatch(roomHtml, /失败|已拒绝/);

  const detailHtml = renderToStaticMarkup(provider(React.createElement(
    ToolBlockResult,
    { toolResult: result },
  )));
  assert.match(detailHtml, /data-tool-result-semantic-outcome="superseded"/);
  assert.match(detailHtml, /execution_terminal/);
});

test("Room conversation identity stays stable across physical member sessions", async () => {
  const { getAgentConversationIdentityKey } = await server.ssrLoadModule(
    "/src/lib/conversation/agent-conversation-identity.ts",
  );
  const firstMemberSession = {
    chat_type: "group",
    conversation_id: "conversation-shared",
    room_session_id: "room-session-agent-1",
    session_key: "room:group:conversation-shared",
  };
  const secondMemberSession = {
    ...firstMemberSession,
    room_session_id: "room-session-agent-2",
  };

  assert.equal(
    getAgentConversationIdentityKey(firstMemberSession),
    "room-conversation:conversation-shared",
  );
  assert.equal(
    getAgentConversationIdentityKey(secondMemberSession),
    "room-conversation:conversation-shared",
    "a member runtime/session reorder must not reset the shared Room timeline",
  );
  assert.equal(
    getAgentConversationIdentityKey({
      chat_type: "dm",
      conversation_id: "conversation-dm",
      room_session_id: "room-session-dm",
      session_key: "agent:agent-1:workspace:dm:conversation-dm",
    }),
    "room-session:room-session-dm",
    "non-Room conversations keep their physical session boundary",
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
  const { AssistantMessageContent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-message-content.tsx",
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

  const messageContentProps = {
    activity: {
      emptyStreamStatus: null,
      label: null,
      showCursor: true,
      standalone: false,
      state: "replying",
      toolUseSummary: null,
    },
    direct: {
      projection: live.directOrderedProjection,
      visible: true,
    },
    environment: {
      canRespondToPermissions: true,
      hiddenToolNames: [],
      mode: "dm_live",
    },
    final: {
      content: live.finalAssistantContent,
      mentions: [],
      isStreaming: true,
      streamingIndexes: live.finalAssistantStreamingIndexes,
      visible: true,
    },
    permissions: {
      all: [],
      matchedByToolUseId: new Map(),
      owner: "composer",
      unmatched: [],
    },
    process: {
      anchorRef: { current: null },
      expanded: false,
      projection: { content: [], streamingIndexes: new Set() },
      summary: { kind: "details", latestDetail: null, metrics: [] },
      toggle: () => {},
      visible: false,
    },
    showMaxTokensWarning: false,
  };
  const html = await renderWithI18n(React.createElement(
    AssistantMessageContent,
    messageContentProps,
  ));
  assert.match(
    html,
    /nexus-chat-final-content[^\"]*before:left-\[5\.5px\]/,
    "the final reply must keep the same timeline lane as the process text",
  );

  const roomHtml = await renderWithI18n(React.createElement(
    AssistantMessageContent,
    {
      ...messageContentProps,
      environment: {
        ...messageContentProps.environment,
        mode: "room_result",
      },
    },
  ));
  assert.doesNotMatch(roomHtml, /before:left-\[5\.5px\]|data-timeline-dot/);
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
  let failureWrites = 0;
  const context = {
    runtime: {
      applyAgentRoundStatus: (payload) => {
        applied.push(["agent", payload.status]);
      },
      applyRoundStatus: (_roundId, status) => {
        applied.push(["round", status]);
      },
    },
    scope: {
      chatType: "group",
      isCurrentSessionEvent: () => true,
      sessionKey: "room:group:conversation-1",
    },
    state: {
      reliability: {
        observeRecovery: () => {},
        reportFailure: () => {
          failureWrites += 1;
        },
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
  assert.equal(failureWrites, 0);
});

test("Room no-reply stays out of the public feed", async () => {
  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const roundId = "round-root";
  const rootUser = userMessage({
    content: "继续协作",
    messageId: "user-root",
    roundId,
    timestamp: 1,
  });
  const resultSummary = {
    duration_api_ms: 10,
    duration_ms: 100,
    is_error: false,
    num_turns: 1,
    result: "<nexus_room_no_reply/>",
    subtype: "success",
    timestamp: 2,
  };
  const assistant = {
    ...assistantMessage({
      agentId: "agent-lucy",
      agentRoundId: "agent-round-lucy",
      messageId: "assistant-lucy",
      model: "glm-4.7",
      resultSummary,
      roundId,
      status: "done",
      timestamp: 2,
    }),
    content: [{
      thinking: "仅供 Thread 查看",
      type: "thinking",
    }],
  };
  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([[roundId, [rootUser, assistant]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: [roundId],
  });

  assert.deepEqual(projection.roundIds, [roundId]);
  assert.deepEqual(
    projection.messageGroups.get(roundId)?.map((message) => message.message_id),
    [rootUser.message_id],
    "no-reply 是内部控制结果，不应生成公区 Agent 节点或空壳",
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

  const roomNotice = parseConversationMessage({
    agent_id: "",
    content: [{ type: "text", text: "请使用 @AgentName 指定要对话的成员" }],
    conversation_id: "conversation-1",
    message_id: "assistant-room-notice",
    role: "assistant",
    room_id: "room-1",
    round_id: "round-room-notice",
    session_key: "room:group:conversation-1",
    timestamp: 3,
  });
  assert.equal(roomNotice?.message_id, "assistant-room-notice");
  assert.equal(parseConversationMessage({
    ...roomNotice,
    room_id: undefined,
    session_key: "agent:agent-1:ws:dm:test",
  }), null, "agentless assistant remains invalid outside a Room");
  assert.equal(parseStreamMessage({
    ...stream,
    agent_id: "",
  }), null, "Room stream events still require an agent identity");
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

test("canonical timeline hides private Room execution evidence", async () => {
  const {
    buildRoomAgentRoundEntries,
    hasRoomAgentRoundEntries,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const { syncRoomAgentExecutionsFromSlots } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const {
    filterVisibleRoomLiveRoundIds,
    filterVisibleTimelineMessages,
    groupMessagesByRound,
    groupPendingPermissionsByRound,
    groupPendingSlotsByRound,
    groupRoomAgentExecutionStatesByRound,
    projectVisibleRoomTimelineEvidence,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/timeline-model.ts",
  );
  const privateRoundId = "root-round";
  const privateSlot = {
    agent_id: "agent-witch",
    agent_round_id: "agent-round-witch",
    hidden_from_user: true,
    index: 0,
    msg_id: "slot-witch",
    round_id: privateRoundId,
    status: "streaming",
    timestamp: 1,
  };
  const [privateExecution] = syncRoomAgentExecutionsFromSlots([], [privateSlot]);
  assert.equal(privateExecution.hidden_from_user, true);
  const privatePermission = {
    agent_id: privateSlot.agent_id,
    agent_round_id: privateSlot.agent_round_id,
    request_id: "permission-witch",
    round_id: privateRoundId,
    tool_input: {},
    tool_name: "Read",
  };
  const privateEvidence = projectVisibleRoomTimelineEvidence(
    [privateSlot],
    [privatePermission],
    [privateExecution],
  );

  assert.deepEqual(privateEvidence.pendingSlots, []);
  assert.deepEqual(privateEvidence.pendingPermissions, []);
  assert.deepEqual(privateEvidence.executionStates, []);
  assert.deepEqual(
    filterVisibleRoomLiveRoundIds(
      [privateRoundId],
      groupMessagesByRound([]),
      groupPendingPermissionsByRound(privateEvidence.pendingPermissions),
      groupPendingSlotsByRound(privateEvidence.pendingSlots),
      groupRoomAgentExecutionStatesByRound(privateEvidence.executionStates),
    ),
    [],
    "只有私域证据的物理 round 不应进入公区时间线",
  );
  assert.equal(hasRoomAgentRoundEntries([], [], [], []), false);
  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [],
      privateEvidence.pendingSlots,
      privateEvidence.pendingPermissions,
      privateEvidence.executionStates,
    ),
    [],
  );
  assert.deepEqual(
    filterVisibleTimelineMessages([
      { message_id: "private", hidden_from_user: true },
      { message_id: "public" },
    ]).map((message) => message.message_id),
    ["public"],
  );

  const publicSlot = {
    ...privateSlot,
    hidden_from_user: false,
  };
  const publicEvidence = projectVisibleRoomTimelineEvidence(
    [publicSlot],
    [],
    syncRoomAgentExecutionsFromSlots([], [publicSlot]),
  );
  assert.equal(
    buildRoomAgentRoundEntries(
      [],
      publicEvidence.pendingSlots,
      publicEvidence.pendingPermissions,
      publicEvidence.executionStates,
    ).length,
    1,
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
      reliability: {
        observeRecovery: () => {
          error = null;
        },
        reportFailure: (failure) => {
          error = failure.code;
        },
      },
      sessionKey: "room:group:conversation-stop",
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
  assert.equal(dropped.read().error, "connection_unavailable");
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
  assert.equal(
    shellHtml.match(/data-room-agent-execution-actions/g)?.length,
    1,
    "Thread and exact stop must share one Agent-header action group",
  );
  assert.match(
    shellHtml,
    /data-room-agent-action="stop"[\s\S]*data-room-agent-action="thread"/,
    "the stable Thread entry stays at the right edge when stop appears",
  );

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
