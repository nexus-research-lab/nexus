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

test.after(async () => {
  await server.close();
});

test("real Room cancellation guidance is projected once into Amy Thread", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const { getRoomThreadMessages } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-thread-model.ts",
  );

  const guide = {
    agent_id: "",
    content: "@Amy 算了不用了",
    delivery_policy: "guide",
    message_id: "msg_user_1716c22bc29d6240762bcf11",
    role: "user",
    round_id: "goal_continuation_9263beccd6692dd24807",
    session_key: "room:group:91c68883cc96",
    source_round_id: "round_21eae091f80fa6a69b71ace2",
    target_agent_ids: ["367448a0264b"],
    timestamp: 1784083409342,
  };
  const amyReply = {
    agent_id: "367448a0264b",
    content: [{
      type: "text",
      text: "收到，这个任务取消了。有需要再找我。<nexus_room_no_reply/>",
    }],
    is_complete: true,
    message_id: "d71ae7953d4401554941272e",
    role: "assistant",
    round_id: "goal_continuation_9263beccd6692dd24807",
    session_key: "room:group:91c68883cc96",
    timestamp: 1784083437370,
  };
  const devinReply = {
    agent_id: "0ed5434a8c13",
    content: [{ type: "text", text: "不应进入 Amy Thread" }],
    is_complete: true,
    message_id: "devin-reply",
    role: "assistant",
    round_id: "goal_continuation_9263beccd6692dd24807",
    session_key: "room:group:91c68883cc96",
    timestamp: 1784083437371,
  };
  const messages = [guide, amyReply, devinReply];

  const mainTimeline = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {
      "0ed5434a8c13": "Devin",
      "367448a0264b": "Amy",
    },
    messages,
    pendingPermissions: [],
    pendingSlots: [],
  });
  assert.deepEqual(mainTimeline.userMessages, [], "引导不能再次出现在 Room 主时间线");
  assert.deepEqual(
    mainTimeline.completedEntries.map((entry) => entry.agent_id),
    [amyReply.agent_id],
    "本轮公区只应投影一次 Amy 回复",
  );

  const amyThread = getRoomThreadMessages(messages, "367448a0264b");
  assert.deepEqual(
    amyThread.map((message) => message.message_id),
    [guide.message_id, amyReply.message_id],
    "Amy Thread 只能接收这一条引导和 Amy 的执行链",
  );
  assert.equal(
    amyThread[1].content[0].text,
    "收到，这个任务取消了。有需要再找我。",
    "Thread 直接内容必须剥离 Room 控制标记",
  );
});

test("Room chat ACK with empty pending preserves the active slot", async () => {
  const { mergeChatAckPendingSlots } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const activeSlot = {
    agent_id: "367448a0264b",
    agent_round_id: "agent-round-active",
    msg_id: "slot-active",
    round_id: "round-active",
    status: "streaming",
    timestamp: 1784083409342,
  };
  const emptyAck = {
    client_message_id: "client-message-queued",
    client_request_id: "client-request-queued",
    pending: [],
    pending_snapshot: false,
    round_id: "round-active",
    user_message_id: "user-message-queued",
  };

  assert.deepEqual(
    mergeChatAckPendingSlots([activeSlot], emptyAck),
    [activeSlot],
    "普通 queue ACK 不能覆盖仍在运行的 Agent slot",
  );
  assert.deepEqual(
    mergeChatAckPendingSlots([activeSlot], {
      ...emptyAck,
      pending_snapshot: true,
    }),
    [],
    "权威 pending snapshot 才可以用空数组清除 slot",
  );
});
