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
const localStorageValues = new Map();
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: {
    clear: () => localStorageValues.clear(),
    getItem: (key) => localStorageValues.get(key) ?? null,
    removeItem: (key) => localStorageValues.delete(key),
    setItem: (key, value) => localStorageValues.set(key, String(value)),
  },
});

test.after(async () => {
  await server.close();
});

async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const value = {
    locale,
    setLocale: () => {},
    t: (key, params = {}) => Object.entries(params).reduce(
      (message, [name, parameter]) => message.replaceAll(
        `{${name}}`,
        String(parameter),
      ),
      MESSAGES[locale][key] ?? key,
    ),
  };
  return renderToStaticMarkup(React.createElement(
    I18N_CONTEXT.Provider,
    { value },
    element,
  ));
}

test("sidebar orders exact Room anchors and consumes them message by message", async () => {
  const { useSidebarStore } = await server.ssrLoadModule(
    "/src/store/sidebar.ts",
  );
  useSidebarStore.setState(useSidebarStore.getInitialState(), true);
  const target = {
    conversation_id: "conversation-1",
    key: "room:room-1:conversation:conversation-1",
    room_id: "room-1",
  };
  const first = {
    agent_id: "agent-a",
    message_id: "message-a",
    room_seq: 11,
    timestamp: 1_000,
  };
  const second = {
    agent_id: "agent-b",
    message_id: "message-b",
    room_seq: 12,
    timestamp: 2_000,
  };

  assert.equal(
    useSidebarStore.getState().record_chat_notification(target, second),
    true,
  );
  assert.equal(
    useSidebarStore.getState().record_chat_notification(target, first),
    true,
  );
  assert.equal(
    useSidebarStore.getState().record_chat_notification(target, second),
    false,
    "replayed completions do not duplicate the unread queue",
  );
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], 2);
  assert.equal(useSidebarStore.getState().chat_badge_count, 2);
  assert.deepEqual(
    useSidebarStore.getState().chat_unread_anchors[target.key].messages,
    [first, second],
    "room_seq restores durable order even when WebSocket delivery is reversed",
  );

  const activeMessage = {
    agent_id: "agent-c",
    message_id: "message-c",
    room_seq: 13,
    timestamp: 3_000,
  };
  assert.equal(
    useSidebarStore.getState().record_chat_notification(
      target,
      activeMessage,
      { preserve_anchor: true },
    ),
    true,
  );
  useSidebarStore.getState().acknowledge_chat_tab();
  assert.equal(useSidebarStore.getState().chat_badge_count, 0);
  assert.equal(
    useSidebarStore.getState().chat_unread_counts[target.key],
    3,
    "entering Chat acknowledges the navigation badge without discarding unread anchors",
  );

  useSidebarStore.getState().record_chat_notification(target, {
    agent_id: "agent-d",
    message_id: "message-d",
    room_seq: 14,
    timestamp: 4_000,
  });
  assert.equal(useSidebarStore.getState().chat_badge_count, 1);
  assert.equal(
    useSidebarStore.getState().chat_unread_counts[target.key],
    4,
    "an active Room completion remains unread until the Feed proves it visible",
  );

  useSidebarStore.getState().consume_chat_unread_messages(
    target.key,
    ["message-a"],
  );
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], 3);
  assert.deepEqual(
    useSidebarStore.getState().chat_unread_anchors[target.key].messages,
    [second, activeMessage, {
      agent_id: "agent-d",
      message_id: "message-d",
      room_seq: 14,
      timestamp: 4_000,
    }],
    "reading one materialized node preserves every other unread anchor",
  );

  useSidebarStore.getState().consume_chat_unread_messages(
    target.key,
    ["message-b"],
  );
  assert.deepEqual(
    useSidebarStore.getState().chat_unread_anchors[target.key].messages,
    [activeMessage, {
      agent_id: "agent-d",
      message_id: "message-d",
      room_seq: 14,
      timestamp: 4_000,
    }],
  );
  useSidebarStore.getState().consume_chat_unread_messages(
    target.key,
    ["message-c", "message-d"],
  );
  assert.equal(useSidebarStore.getState().chat_unread_anchors[target.key], undefined);

  useSidebarStore.getState().record_chat_notification(target, {
    ...first,
    message_id: "message-cleared",
    room_seq: 14,
  });
  useSidebarStore.getState().clear_chat_notifications_for_room("room-1");
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], undefined);
  assert.equal(useSidebarStore.getState().chat_unread_anchors[target.key], undefined);

  useSidebarStore.getState().record_chat_notification(target, {
    ...first,
    message_id: "message-after-read",
    room_seq: 15,
  });
  useSidebarStore.getState().discard_chat_state_for_room("room-1");
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], undefined);
  assert.equal(useSidebarStore.getState().chat_unread_anchors[target.key], undefined);
  assert.equal(
    useSidebarStore.getState().notified_chat_message_ids.some(
      (identity) => identity.startsWith(`${target.key}\u001f`),
    ),
    false,
    "local or remote Room deletion prunes anchor and replay-dedupe state",
  );
});

test("Room unread model keeps unresolved oldest messages ahead of loaded Agent nodes", async () => {
  const {
    countUnreadAgentNodes,
    resolveStoredUnreadMessages,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-unread-model.ts",
  );
  const messageGroups = new Map([
    ["node-a", [{
      agent_id: "agent-a",
      agent_round_id: "agent-round-a",
      content: [],
      message_id: "message-a",
      role: "assistant",
      round_id: "root-old",
      session_key: "session-1",
      timestamp: 3,
    }]],
    ["node-b", [{
      agent_id: "agent-b",
      agent_round_id: "agent-round-b",
      content: [],
      message_id: "message-b",
      result_summary: {
        message_id: "result-b",
      },
      role: "assistant",
      round_id: "root-new",
      session_key: "session-1",
      timestamp: 2,
    }]],
  ]);
  const source = {
    messageGroups,
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roomAgentExecutionStateGroups: new Map(),
    rootRoundIds: new Map([
      ["node-a", "root-old"],
      ["node-b", "root-new"],
    ]),
    roundIds: ["node-a", "node-b"],
  };
  const anchor = {
    conversation_id: "conversation-1",
    key: "room:room-1:conversation:conversation-1",
    messages: [
      {
        message_id: "assistant_result-b",
        room_seq: 12,
        timestamp: 2,
      },
      {
        message_id: "message-a",
        room_seq: 13,
        timestamp: 3,
      },
      {
        message_id: "message-unloaded",
        room_seq: 11,
        round_id: "root-unloaded",
        timestamp: 1,
      },
      {
        agent_round_id: "agent-round-a",
        message_id: "canonicalized-wrapper-a",
        room_seq: 14,
        timestamp: 4,
      },
    ],
    room_id: "room-1",
  };

  const messages = resolveStoredUnreadMessages(source, [anchor]);
  assert.deepEqual(
    messages.map(({ messageId, nodeId, rootRoundId }) => ({
      messageId,
      nodeId,
      rootRoundId,
    })),
    [
      {
        messageId: "message-unloaded",
        nodeId: null,
        rootRoundId: "root-unloaded",
      },
      {
        messageId: "assistant_result-b",
        nodeId: "node-b",
        rootRoundId: "root-new",
      },
      {
        messageId: "message-a",
        nodeId: "node-a",
        rootRoundId: "root-old",
      },
      {
        messageId: "canonicalized-wrapper-a",
        nodeId: "node-a",
        rootRoundId: "root-old",
      },
    ],
    "the first unread stays ordered while synthetic and canonicalized completion IDs resolve to stable Agent nodes",
  );
  assert.equal(countUnreadAgentNodes(messages), 3);
});

test("Room control-only completions cannot become unread navigation anchors", async () => {
  const { hasVisibleAssistantOutput } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/message-content-model.ts",
  );
  const message = {
    agent_id: "agent-a",
    content: [{ type: "text", text: "<nexus_room_fanout/>" }],
    message_id: "control-message",
    result_summary: {
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: false,
      num_turns: 1,
      result: "<nexus_room_no_reply/>",
      subtype: "success",
    },
    role: "assistant",
    round_id: "root-1",
    session_key: "session-1",
    timestamp: 1,
  };
  assert.equal(hasVisibleAssistantOutput(message), false);
  assert.equal(
    hasVisibleAssistantOutput({
      ...message,
      content: [{ type: "text", text: "真实回复" }],
    }),
    true,
  );
});

test("Room tool-use segments wait for the Agent terminal reply before becoming unread", async () => {
  const { isCompletedAssistantMessage } = await server.ssrLoadModule(
    "/src/features/home/notifications/chat-notification-model.ts",
  );
  const message = {
    agent_id: "agent-a",
    content: [{
      id: "tool-1",
      input: { query: "Nexus" },
      name: "WebSearch",
      type: "tool_use",
    }],
    is_complete: true,
    message_id: "assistant-tool-use",
    role: "assistant",
    round_id: "root-1",
    session_key: "room:group:conversation-1",
    stop_reason: "tool_use",
    stream_status: "done",
    timestamp: 1,
  };
  assert.equal(isCompletedAssistantMessage(message), false);
  assert.equal(
    isCompletedAssistantMessage({
      ...message,
      content: [{ type: "text", text: "最终回复" }],
      message_id: "assistant-final",
      stop_reason: "end_turn",
    }),
    true,
  );
  assert.equal(
    isCompletedAssistantMessage({
      ...message,
      message_id: "assistant-result",
      result_summary: {
        duration_api_ms: 1,
        duration_ms: 1,
        is_error: false,
        num_turns: 2,
        subtype: "success",
      },
    }),
    true,
  );
});

test("Room shared session identity survives an initially unloaded directory", async () => {
  const { isGroupRoomNotificationTarget } = await server.ssrLoadModule(
    "/src/features/home/notifications/chat-notification-target.ts",
  );
  const target = {
    conversation_id: "conversation-1",
    room_id: "room-1",
    session_key: "room:group:conversation-1",
  };
  assert.equal(isGroupRoomNotificationTarget(target, undefined), true);
  assert.equal(
    isGroupRoomNotificationTarget(target, "dm"),
    false,
    "an authoritative directory entry wins over the protocol fallback",
  );
  assert.equal(
    isGroupRoomNotificationTarget({
      ...target,
      session_key: "agent:agent-1:ws:dm:room-1",
    }, undefined),
    false,
  );
});

test("Room unread target direction works for mounted and virtualized Agent nodes", async () => {
  const { resolveGroupUnreadNodePosition } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-unread-model.ts",
  );
  const target = {
    dataset: {
      conversationRoundId: "node-b",
      conversationRoundIndex: "1",
    },
    getBoundingClientRect: () => ({ bottom: -12, top: -80 }),
  };
  const container = {
    getBoundingClientRect: () => ({ bottom: 500, top: 0 }),
    querySelectorAll: () => [target],
  };
  assert.equal(
    resolveGroupUnreadNodePosition(container, ["node-a", "node-b"], "node-b"),
    "above",
  );

  target.getBoundingClientRect = () => ({ bottom: 180, top: 100 });
  assert.equal(
    resolveGroupUnreadNodePosition(container, ["node-a", "node-b"], "node-b"),
    "visible",
  );

  const mountedVirtualNode = {
    dataset: {
      conversationRoundId: "node-c",
      conversationRoundIndex: "2",
    },
  };
  container.querySelectorAll = () => [mountedVirtualNode];
  assert.equal(
    resolveGroupUnreadNodePosition(
      container,
      ["node-a", "node-b", "node-c"],
      "node-a",
    ),
    "above",
  );
});

test("Room virtual unread navigation falls back by index before mounting the exact node", async () => {
  const { createConversationRoundScrollHandle } = await server.ssrLoadModule(
    "/src/features/conversation/shared/feed/use-conversation-round-navigation.ts",
  );
  const fallbackCalls = [];
  const scrollCalls = [];
  let mountedRounds = [];
  const scrollElement = {
    clientHeight: 500,
    dataset: {},
    getBoundingClientRect: () => ({ bottom: 500, top: 0 }),
    querySelectorAll: () => mountedRounds,
    scrollHeight: 4_000,
    scrollTo: (options) => scrollCalls.push(options),
    scrollTop: 1_200,
  };
  const roundIds = Array.from({ length: 20 }, (_, index) => `node-${index}`);
  const handle = createConversationRoundScrollHandle({
    fallbackScrollToIndex: (index, options) => {
      fallbackCalls.push([index, options]);
    },
    getScrollElement: () => scrollElement,
    roundIds,
  });
  const options = {
    align: "start",
    behavior: "auto",
    target: "round",
  };

  assert.equal(handle.scrollToRoundId("node-17", options), true);
  assert.deepEqual(fallbackCalls, [[17, options]]);
  assert.deepEqual(scrollCalls, []);

  mountedRounds = [{
    dataset: {
      conversationRoundId: "node-17",
      conversationRoundIndex: "17",
    },
    getBoundingClientRect: () => ({ bottom: 300, top: 180 }),
  }];
  assert.equal(handle.scrollToRoundId("node-17", options), true);
  assert.equal(fallbackCalls.length, 1);
  assert.deepEqual(scrollCalls, [{
    behavior: "auto",
    top: 1_356,
  }]);
});

test("Room sidebar keeps other conversations unread and opens the earliest sequence", async () => {
  const { projectSidebarUnreadItems } = await server.ssrLoadModule(
    "/src/features/home/sidebar/sidebar-unread-model.ts",
  );
  const {
    getActiveChatTargetFromPath,
    isChatNotificationTargetActive,
  } = await server.ssrLoadModule(
    "/src/features/home/notifications/chat-notification-target.ts",
  );
  const activeTarget = getActiveChatTargetFromPath(
    "/rooms/room-1/conversations/conversation-active",
  );
  const firstKey = "room:room-1:conversation:conversation-first";
  const activeKey = "room:room-1:conversation:conversation-active";
  assert.equal(
    isChatNotificationTargetActive(activeTarget, {
      key: firstKey,
      room_id: "room-1",
    }),
    false,
    "opening one Room conversation cannot swallow another conversation",
  );

  const [item] = projectSidebarUnreadItems({
    activeTarget,
    chatUnreadAnchors: {
      [activeKey]: {
        conversation_id: "conversation-active",
        key: activeKey,
        messages: [{
          message_id: "message-active",
          room_seq: 8,
          timestamp: 800,
        }],
        room_id: "room-1",
      },
      [firstKey]: {
        conversation_id: "conversation-first",
        key: firstKey,
        messages: [{
          message_id: "message-first",
          room_seq: 9,
          timestamp: 900,
        }],
        room_id: "room-1",
      },
      "room:room-1:conversation:conversation-later": {
        conversation_id: "conversation-later",
        key: "room:room-1:conversation:conversation-later",
        messages: [{
          message_id: "message-later",
          room_seq: 10,
          timestamp: 1_000,
        }],
        room_id: "room-1",
      },
    },
    chatUnreadCounts: {
      [activeKey]: 1,
      [firstKey]: 1,
      "room:room-1:conversation:conversation-later": 1,
    },
    chatUnreadTargets: {
      [activeKey]: {
        conversation_id: "conversation-active",
        key: activeKey,
        room_id: "room-1",
      },
      [firstKey]: {
        conversation_id: "conversation-first",
        key: firstKey,
        room_id: "room-1",
      },
      "room:room-1:conversation:conversation-later": {
        conversation_id: "conversation-later",
        key: "room:room-1:conversation:conversation-later",
        room_id: "room-1",
      },
    },
    chatUnreadTimestamps: {
      [activeKey]: 800,
      [firstKey]: 900,
      "room:room-1:conversation:conversation-later": 1_000,
    },
    items: [{
      id: "room-1",
      kind: "room",
      roomId: "room-1",
      title: "Room",
    }],
  });
  assert.equal(item.unreadCount, 2);
  assert.equal(item.unreadConversationId, "conversation-first");
  assert.equal(item.unreadTargetKey, firstKey);
});

test("Room Feed renders one passive node-local unread divider without changing virtual height", async () => {
  const { GroupConversationFeed } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-feed.tsx",
  );
  const { projectGroupRoundHeights } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-height-model.ts",
  );
  const roundIds = ["node-a", "node-b", "node-c"];
  const html = renderToStaticMarkup(React.createElement(GroupConversationFeed, {
    isMobileLayout: false,
    refs: {
      bottomAnchorRef: { current: null },
      scrollRef: { current: null },
    },
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
      roundIds,
      unreadMarkerRoundId: "node-b",
    },
  }));
  assert.equal(html.match(/data-room-unread-marker=/g)?.length, 1);
  assert.equal(html.match(/data-room-unread-marker-line/g)?.length, 2);
  assert.match(html, /未读消息从这里开始/);
  assert.match(html, />新消息</);
  assert.doesNotMatch(html, /role="separator"/);
  assert.match(html, /\babsolute\b/);
  assert.match(html, /\bflex-1\b/);
  assert.doesNotMatch(html, /\bshadow(?:-|")/);
  assert.doesNotMatch(html, /\bborder(?:-|")/);
  const virtualHtml = renderToStaticMarkup(React.createElement(
    GroupConversationFeed,
    {
      isMobileLayout: false,
      refs: {
        bottomAnchorRef: { current: null },
        scrollRef: { current: null },
      },
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
        roundIds: Array.from(
          { length: 20 },
          (_, index) => `virtual-node-${index}`,
        ),
      },
    },
  ));
  assert.match(virtualHtml, /data-conversation-virtual-feed="true"/);

  const heights = projectGroupRoundHeights({
    baseHeights: new Map(roundIds.map((roundId) => [roundId, 100])),
    messageGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds,
  });
  assert.equal(heights.get("node-a"), 100);
  assert.equal(heights.get("node-b"), 100);
  assert.equal(heights.get("node-c"), 100);
});

test("Room unread navigation uses a compact directional chip while DM keeps the round button", async () => {
  const { ScrollToLatestButton } = await server.ssrLoadModule(
    "/src/features/conversation/shared/scroll-to-latest-button.tsx",
  );
  const roomElement = React.createElement(
    ScrollToLatestButton,
    {
      direction: "above",
      isLoading: false,
      onClick: () => {},
      unreadCount: 3,
      visible: true,
    },
  );
  const dmElement = React.createElement(
    ScrollToLatestButton,
    {
      isLoading: false,
      onClick: () => {},
      visible: true,
    },
  );
  const roomHtml = await renderWithI18n(roomElement);
  const dmHtml = await renderWithI18n(dmElement);
  const englishRoomHtml = await renderWithI18n(roomElement, "en");
  const englishDmHtml = await renderWithI18n(dmElement, "en");

  assert.match(roomHtml, /data-room-unread-jump="true"/);
  assert.match(roomHtml, /3 条新消息/);
  assert.doesNotMatch(roomHtml, /\bw-11\b/);
  assert.match(dmHtml, /data-scroll-to-latest="true"/);
  assert.match(dmHtml, /\bw-11\b/);
  assert.match(englishRoomHtml, /3 new messages/);
  assert.match(englishDmHtml, /aria-label="Back to latest"/);
  assert.doesNotMatch(englishRoomHtml, /条新消息|定位到第一条/);
});
