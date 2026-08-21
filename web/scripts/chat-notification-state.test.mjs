import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

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

test("sidebar orders Room notifications and clears the exact opened target", async () => {
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
    "WebSocket replay cannot duplicate a sidebar notification",
  );
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], 2);
  assert.deepEqual(
    useSidebarStore.getState().chat_unread_anchors[target.key].messages,
    [first, second],
    "room_seq keeps sidebar Conversation routing deterministic",
  );

  useSidebarStore.getState().acknowledge_chat_tab();
  assert.equal(useSidebarStore.getState().chat_badge_count, 0);
  assert.equal(
    useSidebarStore.getState().chat_unread_counts[target.key],
    2,
    "opening Chat alone does not acknowledge unopened Conversations",
  );

  useSidebarStore.getState().clear_chat_notifications_for_target(target.key);
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], undefined);
  assert.equal(useSidebarStore.getState().chat_unread_anchors[target.key], undefined);

  useSidebarStore.getState().record_chat_notification(target, {
    ...first,
    message_id: "message-cleared",
    room_seq: 13,
  });
  useSidebarStore.getState().clear_chat_notifications_for_room("room-1");
  assert.equal(useSidebarStore.getState().chat_unread_counts[target.key], undefined);
  assert.equal(useSidebarStore.getState().chat_unread_anchors[target.key], undefined);

  useSidebarStore.getState().record_chat_notification(target, {
    ...first,
    message_id: "message-after-read",
    room_seq: 14,
  });
  useSidebarStore.getState().discard_chat_state_for_room("room-1");
  assert.equal(
    useSidebarStore.getState().notified_chat_message_ids.some(
      (identity) => identity.startsWith(`${target.key}\u001f`),
    ),
    false,
    "Room deletion prunes notification replay state",
  );
});

test("Room control-only completions cannot become sidebar notifications", async () => {
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

test("Room tool-use segments wait for the terminal reply before notification", async () => {
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
  assert.equal(isGroupRoomNotificationTarget(target, "dm"), false);
});

test("Room sidebar preserves other Conversations and opens the earliest sequence", async () => {
  const { projectSidebarUnreadItems } = await server.ssrLoadModule(
    "/src/features/home/sidebar/sidebar-unread-model.ts",
  );
  const {
    getActiveChatTargetFromPath,
  } = await server.ssrLoadModule(
    "/src/features/home/notifications/chat-notification-target.ts",
  );
  const activeTarget = getActiveChatTargetFromPath(
    "/rooms/room-1/conversations/conversation-active",
  );
  const firstKey = "room:room-1:conversation:conversation-first";
  const activeKey = "room:room-1:conversation:conversation-active";
  const laterKey = "room:room-1:conversation:conversation-later";
  const [item] = projectSidebarUnreadItems({
    activeTarget,
    chatUnreadAnchors: {
      [activeKey]: {
        conversation_id: "conversation-active",
        key: activeKey,
        messages: [{ message_id: "active", room_seq: 8, timestamp: 800 }],
        room_id: "room-1",
      },
      [firstKey]: {
        conversation_id: "conversation-first",
        key: firstKey,
        messages: [{ message_id: "first", room_seq: 9, timestamp: 900 }],
        room_id: "room-1",
      },
      [laterKey]: {
        conversation_id: "conversation-later",
        key: laterKey,
        messages: [{ message_id: "later", room_seq: 10, timestamp: 1_000 }],
        room_id: "room-1",
      },
    },
    chatUnreadCounts: { [activeKey]: 1, [firstKey]: 1, [laterKey]: 1 },
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
      [laterKey]: {
        conversation_id: "conversation-later",
        key: laterKey,
        room_id: "room-1",
      },
    },
    chatUnreadTimestamps: {
      [activeKey]: 800,
      [firstKey]: 900,
      [laterKey]: 1_000,
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
