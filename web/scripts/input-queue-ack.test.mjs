import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
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

function websocketEvent(eventType, data, sessionKey = "dm:session-a") {
  return {
    data,
    event_type: eventType,
    protocol_version: 2,
    session_key: sessionKey,
    timestamp: 1,
  };
}

test("request transport lease keeps exact ownership across navigation", async () => {
  const { RequestTransportLeaseRegistry } = await server.ssrLoadModule(
    "/src/lib/websocket/request-transport-leases.ts",
  );
  const retainedBindings = [];
  const releasedBindings = [];
  const settlements = [];
  let idleCount = 0;
  const registry = new RequestTransportLeaseRegistry(
    (lease, binding) => {
      retainedBindings.push({ binding, lease });
      return () => releasedBindings.push(lease);
    },
    () => {
      idleCount += 1;
    },
  );
  const binding = {
    conversation_id: "conversation-a",
    room_id: "room-a",
    session_key: "room:group:conversation-a",
    type: "bind_session",
  };
  const releaseOwner = registry.acquire({
    clientRequestId: "req-goal-a",
    onAccepted: () => settlements.push("owner:accepted"),
    onRejected: (reason) => settlements.push(`owner:${reason}`),
    sessionBinding: binding,
  });
  const releaseDuplicate = registry.acquire({
    clientRequestId: "req-goal-a",
    onAccepted: () => settlements.push("duplicate:accepted"),
    onRejected: () => settlements.push("duplicate:rejected"),
    sessionBinding: { ...binding, session_key: "room:group:wrong" },
  });
  assert.equal(registry.hasLeases(), true);
  assert.equal(retainedBindings.length, 1);
  assert.deepEqual(retainedBindings[0].binding, binding);

  releaseDuplicate();
  assert.equal(
    registry.hasLeases(),
    true,
    "a duplicate caller must not gain release authority over the owner lease",
  );
  assert.equal(
    registry.handleMessage(websocketEvent(
      "chat_ack",
      { client_request_id: "req-foreign" },
      "dm:session-b",
    )),
    false,
    "a foreign ACK must not release the original request",
  );
  assert.equal(releasedBindings.length, 0);
  assert.equal(
    registry.handleMessage(websocketEvent(
      "chat_ack",
      { client_request_id: "req-goal-a" },
      "dm:session-b",
    )),
    true,
    "request identity, not the currently visible route, owns the ACK",
  );
  assert.deepEqual(settlements, ["owner:accepted"]);
  assert.equal(registry.hasLeases(), false);
  assert.equal(releasedBindings.length, 1);
  assert.equal(idleCount, 1);
  releaseOwner();
  assert.equal(releasedBindings.length, 1, "terminal release is idempotent");
});

test("new Session preserves only durable Goal ACK owners while reset cancels all", async () => {
  const { runAgentSessionTransition } = await server.ssrLoadModule(
    "/src/hooks/agent/session/use-agent-conversation-session.ts",
  );
  const calls = [];
  const effects = {
    cancelPendingRequestAcks: (_reason, keepPreserved) => (
      calls.push(`cancel:${keepPreserved}`)
    ),
    clearLiveSessionState: () => calls.push("clear-live"),
    resetHistoryPagination: () => calls.push("reset-history"),
    resetRuntimeMachine: () => calls.push("reset-runtime"),
  };
  runAgentSessionTransition(
    "start",
    "new Session",
    () => calls.push("start-b"),
    {},
    effects,
  );
  assert.deepEqual(calls, [
    "cancel:true",
    "clear-live",
    "start-b",
    "reset-history",
    "reset-runtime",
  ], "submit A then immediately create B must not cancel A's ACK owner");

  calls.length = 0;
  runAgentSessionTransition(
    "reset",
    "explicit reset",
    () => calls.push("reset-session"),
    {},
    effects,
  );
  assert.deepEqual(calls, [
    "cancel:false",
    "clear-live",
    "reset-session",
    "reset-history",
    "reset-runtime",
  ]);
});

test("Session navigation cancels ordinary ACKs without cancelling Goal owners", async () => {
  const {
    cancelPendingRequestAcks,
    createPendingRequestAckRegistry,
    resolvePendingRequestAck,
    trackPendingRequestAck,
    waitForRequestAck,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-pending-request-acks.ts",
  );
  const registry = createPendingRequestAckRegistry();
  trackPendingRequestAck(registry, "req-chat");
  trackPendingRequestAck(registry, "req-goal", true);
  const chat = waitForRequestAck(
    registry,
    "req-chat",
    () => assert.fail("navigation cancellation should precede timeout"),
    50,
  );
  const goal = waitForRequestAck(
    registry,
    "req-goal",
    () => assert.fail("the durable Goal owner should remain live"),
    50,
  );
  cancelPendingRequestAcks(registry, "切换会话", true);
  await assert.rejects(chat, /切换会话/);
  assert.equal(registry.pending.has("req-goal"), true);
  assert.equal(resolvePendingRequestAck(registry, "req-goal"), true);
  await goal;
  assert.equal(registry.preserved.size, 0);
});

test("request transport hard timeout releases the retained Session exactly once", async () => {
  const { RequestTransportLeaseRegistry } = await server.ssrLoadModule(
    "/src/lib/websocket/request-transport-leases.ts",
  );
  let releaseCount = 0;
  let timeoutCount = 0;
  const registry = new RequestTransportLeaseRegistry(
    () => () => {
      releaseCount += 1;
    },
    () => {},
  );
  const release = registry.acquire({
    clientRequestId: "req-timeout",
    onAccepted: () => assert.fail("timed out request must not be accepted"),
    onRejected: () => assert.fail("hard timeout uses its typed owner callback"),
    onTimeout: () => {
      timeoutCount += 1;
    },
    sessionBinding: { session_key: "dm:session-a", type: "bind_session" },
    timeoutMs: 5,
  });
  await new Promise((resolve) => setTimeout(resolve, 15));
  assert.equal(registry.hasLeases(), false);
  assert.equal(releaseCount, 1);
  assert.equal(timeoutCount, 1);
  release();
  assert.equal(releaseCount, 1);
});

test("Goal acceptance window outlives the detached backend deadline", async () => {
  const {
    getGoalRequestAcceptanceTimeoutMs,
    getMessageSendAckTimeoutMs,
  } = await server.ssrLoadModule(
    "/src/config/conversation-policy.ts",
  );
  assert.equal(getMessageSendAckTimeoutMs(), 10_000);
  assert.equal(getGoalRequestAcceptanceTimeoutMs(), 20_000);
  assert.ok(getGoalRequestAcceptanceTimeoutMs() > 15_000);
});

test("request ACK registry handles ACK and error before waiter registration", async () => {
  const {
    createPendingRequestAckRegistry,
    discardPendingRequestAck,
    rejectPendingRequestAck,
    resolvePendingRequestAck,
    trackPendingRequestAck,
    waitForRequestAck,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-pending-request-acks.ts",
  );
  const { RequestAcceptanceUnknownError } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-request-ack-failure.ts",
  );

  const acknowledged = createPendingRequestAckRegistry();
  trackPendingRequestAck(acknowledged, "req-ack-first", true);
  assert.equal(resolvePendingRequestAck(acknowledged, "req-ack-first"), false);
  trackPendingRequestAck(acknowledged, "req-ack-first", true);
  await waitForRequestAck(
    acknowledged,
    "req-ack-first",
    () => assert.fail("settled ACK must not time out"),
    10,
  );
  assert.equal(acknowledged.preserved.size, 0);

  const rejected = createPendingRequestAckRegistry();
  trackPendingRequestAck(rejected, "req-error-first", true);
  assert.equal(
    rejectPendingRequestAck(rejected, "req-error-first", "后端拒绝"),
    false,
  );
  trackPendingRequestAck(rejected, "req-error-first", true);
  await assert.rejects(
    waitForRequestAck(
      rejected,
      "req-error-first",
      () => assert.fail("rejected ACK must not time out"),
      10,
    ),
    /后端拒绝/,
  );
  assert.equal(rejected.preserved.size, 0);

  const abandoned = createPendingRequestAckRegistry();
  trackPendingRequestAck(abandoned, "req-send-failed", true);
  resolvePendingRequestAck(abandoned, "req-send-failed");
  trackPendingRequestAck(abandoned, "req-send-failed", true);
  discardPendingRequestAck(abandoned, "req-send-failed");
  assert.equal(abandoned.preserved.size, 0);
  assert.equal(abandoned.settled.size, 0);
  assert.equal(abandoned.tracked.size, 0);

  const unknown = createPendingRequestAckRegistry();
  const unknownError = new RequestAcceptanceUnknownError("受理状态未知");
  trackPendingRequestAck(unknown, "req-unknown-first");
  assert.equal(
    rejectPendingRequestAck(unknown, "req-unknown-first", unknownError),
    false,
  );
  await assert.rejects(
    waitForRequestAck(
      unknown,
      "req-unknown-first",
      () => assert.fail("unknown ACK must not time out"),
      10,
    ),
    (error) => error === unknownError,
    "early rejection must preserve its typed recovery outcome",
  );

  const foreign = createPendingRequestAckRegistry();
  assert.equal(
    resolvePendingRequestAck(foreign, "req-owned-by-another-hook"),
    false,
  );
  assert.equal(foreign.settled.size, 0);
  assert.equal(
    rejectPendingRequestAck(
      foreign,
      "req-owned-by-another-hook",
      "后端拒绝",
    ),
    false,
  );
  assert.equal(foreign.rejected.size, 0);
});

test("request ACK settles its original request after the view switches sessions", async () => {
  const {
    createPendingRequestAckRegistry,
    rejectPendingRequestAck,
    resolvePendingRequestAck,
    trackPendingRequestAck,
    waitForRequestAck,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-pending-request-acks.ts",
  );
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const registry = createPendingRequestAckRegistry();
  const oldRequestID = "req-old-session";
  trackPendingRequestAck(registry, oldRequestID);
  const accepted = waitForRequestAck(
    registry,
    oldRequestID,
    () => assert.fail("the old-session ACK must settle by request identity"),
    25,
  );
  const calls = [];
  const context = {
    runtime: {
      rejectPendingRequestAck: (requestID, reason) => (
        rejectPendingRequestAck(registry, requestID, reason)
      ),
      resolvePendingRequestAck: (requestID) => (
        resolvePendingRequestAck(registry, requestID)
      ),
      trackChatAck: () => calls.push("project-current-feed"),
    },
    scope: {
      isCurrentSessionEvent: (sessionKey) => sessionKey === "session-new",
    },
    state: {
      setError: () => calls.push("show-current-error"),
    },
  };
  AGENT_SESSION_EVENT_HANDLERS.chat_ack({
    data: {
      ack_timeout_ms: 10_000,
      client_message_id: "local-msg-old",
      client_request_id: oldRequestID,
      pending: [],
      pending_snapshot: false,
      round_id: "round-old",
      user_message_committed: true,
      user_message_id: "message-old",
    },
    event_type: "chat_ack",
    session_key: "session-old",
  }, context);
  await accepted;
  assert.deepEqual(
    calls,
    [],
    "an old ACK settles transport state without projecting into the new feed",
  );

  const rejectedRequestID = "req-rejected-old-session";
  trackPendingRequestAck(registry, rejectedRequestID);
  const rejected = waitForRequestAck(
    registry,
    rejectedRequestID,
    () => assert.fail("the old-session rejection must settle by request identity"),
    25,
  );
  AGENT_SESSION_EVENT_HANDLERS.error({
    data: {
      client_request_id: rejectedRequestID,
      message: "Goal 已存在",
      type: "chat",
    },
    event_type: "error",
    session_key: "session-old",
  }, context);
  await assert.rejects(rejected, /Goal 已存在/);
  assert.deepEqual(
    calls,
    [],
    "an old rejection must not display an error in the newly selected session",
  );
});

test("chat ACK timeout recovery recognizes a durable client message identity", async () => {
  const {
    hasAcceptedClientMessage,
    recoverRequestAckTimeout,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-request-ack-failure.ts",
  );
  const messages = [{
    agent_id: "nexus",
    client_message_id: "local_msg_accepted",
    content: "已持久化",
    message_id: "msg_user_server",
    role: "user",
    round_id: "round_server",
    session_key: "agent:nexus:ws:dm:session-1",
    timestamp: 1,
  }];

  assert.equal(
    hasAcceptedClientMessage(messages, "local_msg_accepted"),
    true,
  );
  assert.equal(
    hasAcceptedClientMessage(messages, "local_msg_missing"),
    false,
  );

  const calls = [];
  const accepted = await recoverRequestAckTimeout({
    clientMessageId: "local_msg_accepted",
    inputQueueItems: () => [],
    reconnect: () => calls.push("reconnect"),
    reload: async () => messages,
    websocketState: () => "connected",
  });
  assert.equal(accepted, "accepted");
  assert.deepEqual(calls, ["reconnect"]);

  const failedTransportCalls = [];
  const failedTransport = await recoverRequestAckTimeout({
    clientMessageId: "local_msg_accepted",
    inputQueueItems: () => [],
    reconnect: () => failedTransportCalls.push("reconnect"),
    reload: async () => messages,
    websocketState: () => "failed",
  });
  assert.equal(failedTransport, "accepted");
  assert.deepEqual(
    failedTransportCalls,
    ["reconnect"],
    "ACK recovery must restart a transport that exhausted automatic retries",
  );
  const unknown = await recoverRequestAckTimeout({
    clientMessageId: "local_msg_unknown",
    inputQueueItems: () => [],
    reconnect: () => {},
    reload: async () => null,
    websocketState: () => "reconnecting",
  });
  assert.equal(
    unknown,
    "unknown",
    "a failed history load must not be treated as a rejected request",
  );
  const reloadError = await recoverRequestAckTimeout({
    clientMessageId: "local_msg_unknown",
    inputQueueItems: () => [],
    reconnect: () => {},
    reload: async () => {
      throw new Error("history unavailable");
    },
    websocketState: () => "reconnecting",
  });
  assert.equal(
    reloadError,
    "unknown",
    "a thrown history reload must preserve the optimistic message",
  );
  const queueEvidenceAfterReloadError = await recoverRequestAckTimeout({
    clientMessageId: "local_msg_queued",
    inputQueueItems: () => [{
      client_message_id: "local_msg_queued",
      content: "排队中",
      created_at: 1,
      delivery_policy: "queue",
      id: "queue-1",
      scope: "dm",
      session_key: "agent:nexus:ws:dm:session-1",
      source: "user",
      updated_at: 1,
    }],
    reconnect: () => {},
    reload: async () => {
      throw new Error("history unavailable");
    },
    websocketState: () => "reconnecting",
  });
  assert.equal(
    queueEvidenceAfterReloadError,
    "accepted",
    "durable queue evidence must remain authoritative when history reload fails",
  );
  const unconfirmed = await recoverRequestAckTimeout({
    clientMessageId: "local_msg_missing",
    inputQueueItems: () => [],
    reconnect: () => {},
    reload: async () => messages,
    websocketState: () => "reconnecting",
  });
  assert.equal(
    unconfirmed,
    "unknown",
    "absence from history and a possibly stale queue snapshot is not rejection evidence",
  );
  assert.equal(
    hasAcceptedClientMessage([], "local_msg_queued", [{
      client_message_id: "local_msg_queued",
      content: "排队中",
      created_at: 1,
      delivery_policy: "queue",
      id: "queue-1",
      scope: "dm",
      session_key: "agent:nexus:ws:dm:session-1",
      source: "user",
      updated_at: 1,
    }]),
    true,
    "durable queue acceptance must survive a lost ACK before history projection",
  );
});

test("Goal ACK recovery reads the captured Session without projecting the new route", async () => {
  const { buildRequestAckRecoveryReader } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-request-ack-failure.ts",
  );
  const reads = [];
  const originalIdentity = {
    agent_id: "lead-a",
    chat_type: "group",
    conversation_id: "conversation-a",
    room_id: "room-a",
    session_key: "room:group:conversation-a",
  };
  const reader = buildRequestAckRecoveryReader(
    {
      identity: originalIdentity,
      sessionKey: "room:group:conversation-a",
    },
    async (identity, sessionKey) => {
      reads.push({ identity, sessionKey });
      return [];
    },
    async () => {
      assert.fail("the currently visible Session B must not be reloaded");
    },
  );
  await reader();
  assert.deepEqual(reads, [{
    identity: originalIdentity,
    sessionKey: "room:group:conversation-a",
  }]);
});

test("input queue retry keeps message identity and rotates request identity", async () => {
  const {
    createInputQueueDraftFingerprint,
    resolveInputQueueClientMessageId,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/input-queue-actions.ts",
  );
  const { createOutboundRequestDescriptor } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/outbound-request.ts",
  );

  const fingerprint = createInputQueueDraftFingerprint(
    "还有 M5",
    "queue",
    [{
      file_name: "notes.md",
      kind: "text",
      workspace_path: "notes.md",
    }],
    ["researcher"],
  );
  const identities = new Map();
  const firstMessageID = resolveInputQueueClientMessageId(
    identities,
    fingerprint,
  );
  const retryMessageID = resolveInputQueueClientMessageId(
    identities,
    fingerprint,
  );
  const first = createOutboundRequestDescriptor(firstMessageID);
  const retry = createOutboundRequestDescriptor(retryMessageID);

  assert.equal(retry.client_message_id, first.client_message_id);
  assert.notEqual(retry.client_request_id, first.client_request_id);
});

test("input queue enqueue command carries ACK correlation IDs", async () => {
  const { enqueueInputQueueMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/input-queue-actions.ts",
  );
  const sent = [];
  const request = enqueueInputQueueMessage(
    "还有 M5",
    {
      activeSessionKeyRef: { current: "room:group:conversation-1" },
      identity: {
        agent_id: "planner",
        chat_type: "group",
        conversation_id: "conversation-1",
        room_id: "room-1",
      },
      messages: [],
      pendingPermissions: [],
      sessionKey: "room:group:conversation-1",
      setError: () => {},
      setMessages: () => {},
      setPendingPermissions: () => {},
      wsSend: (message) => {
        sent.push(message);
        return { disposition: "sent" };
      },
      wsState: "connected",
    },
    "queue",
    [],
    ["researcher"],
    "local_msg_stable",
  );

  assert.equal(request.client_message_id, "local_msg_stable");
  assert.equal(sent[0].client_message_id, request.client_message_id);
  assert.equal(sent[0].client_request_id, request.client_request_id);
  assert.equal(sent[0].type, "input_queue");
});

test("input queue ACK parser validates accepted and duplicate flags", async () => {
  const {
    parseChatAckData,
    parseInputQueueAckData,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );
  const ack = {
    accepted: true,
    ack_timeout_ms: 10_000,
    action: "enqueue",
    client_message_id: "local_msg_1",
    client_request_id: "req_1",
    duplicate: false,
    item_id: "queue_1",
  };

  assert.deepEqual(parseInputQueueAckData(ack), ack);
  assert.equal(
    parseInputQueueAckData({ ...ack, accepted: "yes" }),
    null,
  );
  assert.equal(
    parseInputQueueAckData({ ...ack, duplicate: undefined }),
    null,
  );
  const serverPendingAck = {
    ack_timeout_ms: 10_000,
    client_message_id: "",
    client_request_id: "",
    pending: [{
      agent_id: "agent-2",
      agent_round_id: "agent-round-public-wake",
      handoff_id: "handoff-public-wake",
      index: 0,
      msg_id: "slot-public-wake",
      status: "pending",
      timestamp: 20,
    }],
    pending_snapshot: false,
    round_id: "round-root",
    user_message_committed: false,
    user_message_id: "",
  };
  assert.deepEqual(
    parseChatAckData(serverPendingAck),
    serverPendingAck,
    "a server-initiated public wake must create its stable Room slot before streaming starts",
  );
  assert.equal(
    parseChatAckData({ ...serverPendingAck, pending: [] }),
    null,
    "an uncorrelated empty ACK has no state to apply",
  );
  const emptyPendingSnapshot = {
    ...serverPendingAck,
    pending: [],
    pending_snapshot: true,
    round_id: "",
  };
  assert.deepEqual(
    parseChatAckData(emptyPendingSnapshot),
    emptyPendingSnapshot,
    "an empty authoritative reconnect snapshot must clear stale Room slots",
  );
  const multiRootSnapshot = {
    ...emptyPendingSnapshot,
    pending: [
      {
        ...serverPendingAck.pending[0],
        round_id: "round-root-a",
      },
      {
        ...serverPendingAck.pending[0],
        agent_id: "agent-3",
        agent_round_id: "agent-round-public-wake-b",
        msg_id: "slot-public-wake-b",
        round_id: "round-root-b",
      },
    ],
  };
  assert.deepEqual(
    parseChatAckData(multiRootSnapshot),
    multiRootSnapshot,
    "an authoritative reconnect snapshot must preserve every slot root",
  );
  assert.equal(
    parseChatAckData({
      ...multiRootSnapshot,
      pending: multiRootSnapshot.pending.map(({ round_id: _roundId, ...slot }) => slot),
    }),
    null,
    "a multi-root snapshot cannot attach rootless slots to an empty aggregate root",
  );
  assert.equal(
    parseChatAckData({ ...serverPendingAck, pending_snapshot: "true" }),
    null,
  );
  assert.equal(
    parseChatAckData({
      ...serverPendingAck,
      pending: [{
        ...serverPendingAck.pending[0],
        handoff_id: 42,
      }],
    }),
    null,
    "handoff correlation must be a non-empty string when present",
  );
  assert.equal(
    parseChatAckData({
      ...serverPendingAck,
      client_message_id: "client-message-1",
      client_request_id: "request-1",
      pending_snapshot: true,
      user_message_committed: true,
      user_message_id: "user-message-1",
    }),
    null,
    "a correlated request ACK cannot masquerade as an authoritative snapshot",
  );
});

test("public handoff correlation survives ACK, active execution, and terminal lifecycle", async () => {
  const { mergeChatAckPendingSlots } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const {
    applyRoomAgentExecutionStatus,
    syncRoomAgentExecutionsFromSlots,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/room-agent-execution-state.ts",
  );
  const handoffId = "handoff-public-wake";
  const slots = mergeChatAckPendingSlots([], {
    ack_timeout_ms: 10_000,
    client_message_id: "",
    client_request_id: "",
    pending: [{
      agent_id: "agent-2",
      agent_round_id: "agent-round-public-wake",
      handoff_id: handoffId,
      index: 0,
      msg_id: "slot-public-wake",
      round_id: "round-root",
      status: "pending",
      timestamp: 20,
    }],
    pending_snapshot: false,
    round_id: "round-root",
    user_message_committed: false,
    user_message_id: "",
  });
  assert.equal(slots[0].handoff_id, handoffId);

  const active = syncRoomAgentExecutionsFromSlots([], slots);
  assert.equal(active[0].handoff_id, handoffId);
  const terminal = applyRoomAgentExecutionStatus(active, {
    agent_id: "agent-2",
    agent_round_id: "agent-round-public-wake",
    is_terminal: true,
    round_id: "round-root",
    status: "finished",
  });
  assert.equal(
    terminal[0].handoff_id,
    handoffId,
    "terminal lifecycle evidence must not erase the exact handoff identity",
  );
  assert.equal(terminal[0].phase, "terminal");
});

test("Room handoff mention phases are realtime-only, monotonic, and reconnect-safe", async () => {
  const { projectRoomAgentHandoffStatuses } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/panel/controller/room-handoff-status-model.ts",
  );
  const handoffId = "handoff-room-1";
  const liveFinalMessage = {
    agent_id: "agent-source",
    agent_mentions: [{
      agent_id: "agent-target",
      content_block_index: 0,
      end_rune: 13,
      handoff_id: handoffId,
      label: "@Target",
      start_rune: 6,
    }],
    content: [{ text: "交给 @Target", type: "text" }],
    delivery_mode: "durable",
    is_complete: true,
    message_id: "source-message",
    role: "assistant",
    round_id: "root-round",
    session_key: "room:group:conversation",
    stream_status: "done",
    timestamp: 10,
  };

  assert.deepEqual(
    projectRoomAgentHandoffStatuses({
      executionStates: [],
      inputQueueItems: [],
      messages: [{ ...liveFinalMessage, delivery_mode: undefined }],
      pendingSlots: [],
    }),
    {},
    "history reload must not resurrect a completed handoff from mention metadata alone",
  );
  assert.equal(
    projectRoomAgentHandoffStatuses({
      executionStates: [],
      inputQueueItems: [],
      messages: [liveFinalMessage],
      pendingSlots: [],
    })[handoffId],
    "preparing",
  );
  assert.equal(
    projectRoomAgentHandoffStatuses({
      executionStates: [],
      inputQueueItems: [{
        content: "交给 Target",
        created_at: 11,
        delivery_policy: "queue",
        handoff_id: handoffId,
        id: "queue-handoff",
        scope: "room",
        session_key: "room:group:conversation",
        source: "agent_public_mention",
        updated_at: 11,
      }],
      messages: [liveFinalMessage],
      pendingSlots: [],
    })[handoffId],
    "queued",
  );
  assert.equal(
    projectRoomAgentHandoffStatuses({
      executionStates: [{
        agent_id: "agent-target",
        agent_round_id: "agent-round-target",
        display_order: 1,
        first_seen_at: 12,
        handoff_id: handoffId,
        phase: "active",
        round_id: "root-round",
        status: "streaming",
      }],
      inputQueueItems: [{
        content: "late queue snapshot",
        created_at: 11,
        delivery_policy: "queue",
        handoff_id: handoffId,
        id: "queue-handoff",
        scope: "room",
        session_key: "room:group:conversation",
        source: "agent_public_mention",
        updated_at: 11,
      }],
      messages: [liveFinalMessage],
      pendingSlots: [],
    })[handoffId],
    "active",
    "late queue/message evidence cannot regress an active handoff",
  );
  assert.equal(
    projectRoomAgentHandoffStatuses({
      executionStates: [],
      inputQueueItems: [],
      messages: [{ ...liveFinalMessage, delivery_mode: undefined }],
      pendingSlots: [{
        agent_id: "agent-target",
        agent_round_id: "agent-round-target",
        handoff_id: handoffId,
        index: 0,
        msg_id: "slot-target",
        round_id: "root-round",
        status: "streaming",
        timestamp: 12,
      }],
    })[handoffId],
    "active",
    "a reconnect pending snapshot must restore the handoff without realtime message flags",
  );
});

test("Agent mention chip updates one inline handoff surface without adding a reply card", async () => {
  const { AgentHandoffStatusProvider } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/agent-handoff-status-context.tsx",
  );
  const { AgentMentionChip } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/agent-mention-chip.tsx",
  );
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const translations = {
    "room.agent_contact_open": "打开 Target 的联络",
    "room.agent_handoff_active": "已交接",
    "room.agent_handoff_preparing": "交接中",
    "room.agent_handoff_queued": "排队中",
  };
  const html = renderToStaticMarkup(createElement(
    I18N_CONTEXT.Provider,
    {
      value: {
        locale: "zh",
        setLocale: () => {},
        t: (key) => translations[key] ?? key,
      },
    },
    createElement(
      AgentHandoffStatusProvider,
      { statuses: { "handoff-room-1": "queued" } },
      createElement(
        AgentMentionChip,
        {
          agentId: "agent-target",
          directory: { names: { "agent-target": "Target" } },
          handoffId: "handoff-room-1",
        },
        "@Target",
      ),
    ),
  ));

  assert.match(html, /@Target/);
  assert.match(html, /排队中/);
  assert.equal(
    html.match(/role="status"/g)?.length,
    1,
    "handoff feedback must stay inside the single mention chip",
  );
  assert.doesNotMatch(html, /data-room-agent-execution-shell/);
});

test("input queue ACK resolves only accepted requests", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const resolved = [];
  const handler = AGENT_SESSION_EVENT_HANDLERS.input_queue_ack;
  const context = {
    runtime: {
      resolvePendingRequestAck: (requestID) => {
        resolved.push(requestID);
        return true;
      },
    },
    scope: {
      isCurrentSessionEvent: () => true,
    },
  };
  const data = {
    accepted: true,
    ack_timeout_ms: 10_000,
    action: "enqueue",
    client_message_id: "local_msg_1",
    client_request_id: "req_1",
    duplicate: false,
    item_id: "queue_1",
  };

  handler({ data, event_type: "input_queue_ack" }, context);
  handler({
    data: {
      ...data,
      accepted: false,
      client_request_id: "req_rejected",
    },
    event_type: "input_queue_ack",
  }, context);

  assert.deepEqual(resolved, ["req_1"]);
});

test("Room durable user atomically replaces its optimistic feed node", async () => {
  const {
    mergeLoadedMessages,
    upsertRealtimeMessage,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );
  const { replaceOptimisticUserMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const optimistic = {
    agent_id: "",
    client_message_id: "local-message-1",
    content: "检查流式体验",
    message_id: "local-message-1",
    role: "user",
    round_id: "local-message-1",
    session_key: "room-session",
    timestamp: 10,
  };
  const canonical = {
    ...optimistic,
    message_id: "msg-user-1",
    round_id: "round-1",
    timestamp: 11,
  };
  const before = [
    {
      ...optimistic,
      content: "更早消息",
      message_id: "older",
      round_id: "older",
      timestamp: 1,
    },
    optimistic,
    {
      ...optimistic,
      content: "更晚消息",
      message_id: "newer",
      round_id: "newer",
      timestamp: 20,
    },
  ];
  const reconciled = upsertRealtimeMessage(before, canonical);

  assert.deepEqual(
    reconciled.map((message) => message.message_id),
    ["older", "msg-user-1", "newer"],
    "the canonical event must reuse the optimistic visual position",
  );
  assert.equal(
    reconciled.filter((message) => message.content === "检查流式体验").length,
    1,
    "the durable broadcast must never create a one-frame duplicate user card",
  );
  assert.equal(
    replaceOptimisticUserMessage(
      reconciled,
      "local-message-1",
      "msg-user-1",
      "round-1",
      true,
    ).length,
    reconciled.length,
    "the later ACK remains idempotent after realtime reconciliation",
  );
  const ackFirst = replaceOptimisticUserMessage(
    [optimistic],
    "local-message-1",
    "msg-user-1",
    "round-1",
    true,
  );
  assert.equal(
    ackFirst[0]?.client_message_id,
    "local-message-1",
    "ACK-first delivery must retain the optimistic visual identity",
  );
  const canonicalWithoutClientIdentity = {
    ...canonical,
  };
  delete canonicalWithoutClientIdentity.client_message_id;
  const snapshotMerged = mergeLoadedMessages(
    [canonicalWithoutClientIdentity],
    reconciled,
  );
  assert.equal(
    snapshotMerged.find(
      (message) => message.message_id === "msg-user-1",
    )?.client_message_id,
    "local-message-1",
    "a later history refresh must not remount the acknowledged user bubble",
  );
  const timeoutRecoveryMerged = mergeLoadedMessages(
    [canonical],
    [optimistic],
  );
  assert.deepEqual(
    timeoutRecoveryMerged.map((message) => message.message_id),
    ["msg-user-1"],
    "durable client identity must reconcile an optimistic user after ACK loss",
  );
  const broadcastBeforeAck = replaceOptimisticUserMessage(
    [optimistic, canonicalWithoutClientIdentity],
    "local-message-1",
    "msg-user-1",
    "round-1",
    true,
  );
  assert.deepEqual(
    broadcastBeforeAck.map((message) => ({
      client_message_id: message.client_message_id,
      message_id: message.message_id,
    })),
    [{
      client_message_id: "local-message-1",
      message_id: "msg-user-1",
    }],
    "ACK must annotate an already received canonical user before removing the optimistic copy",
  );

  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const optimisticProjection = projectGroupAgentTimeline({
    messageGroups: new Map([["local-message-1", [optimistic]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["local-message-1"],
  });
  const canonicalProjection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-1", [canonical]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["round-1"],
  });
  assert.deepEqual(
    canonicalProjection.roundIds,
    optimisticProjection.roundIds,
    "durable acknowledgement must retain the optimistic React and virtual item identity",
  );
  assert.equal(
    canonicalProjection.rootRoundIds.get("local-message-1"),
    "round-1",
    "the stable visual identity must still resolve to the canonical root round",
  );
});

test("Safari composition guard only consumes Enter after composition end", async () => {
  const { isWithinCompositionEndEnterGuard } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-model.ts",
  );

  assert.equal(isWithinCompositionEndEnterGuard(1_050, 1_000), true);
  assert.equal(isWithinCompositionEndEnterGuard(999, 1_000), false);
  assert.equal(isWithinCompositionEndEnterGuard(1_081, 1_000), false);
});

test("Composer drafts stay isolated by Session while history follows the chat", async () => {
  const {
    buildComposerDraftScopeKey,
    buildComposerHistoryScopeKey,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-draft-scope.ts",
  );
  const { useComposerDraftStore } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-draft-store.ts",
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
  });

  const firstSessionScope = buildComposerDraftScopeKey({
    agentId: "lead-agent",
    roomId: "room-1",
    sessionKey: "session-1",
  });
  const sameSessionScope = buildComposerDraftScopeKey({
    agentId: "other-agent",
    roomId: "room-1",
    sessionKey: "session-1",
  });
  const secondSessionScope = buildComposerDraftScopeKey({
    roomId: "room-1",
    sessionKey: "session-2",
  });
  const otherRoomScope = buildComposerDraftScopeKey({
    roomId: "room-2",
    sessionKey: "session-1",
  });
  assert.equal(firstSessionScope, sameSessionScope);
  assert.notEqual(firstSessionScope, secondSessionScope);
  assert.notEqual(firstSessionScope, otherRoomScope);
  assert.equal(
    buildComposerHistoryScopeKey({ roomId: "room-1" }),
    buildComposerHistoryScopeKey({
      agentId: "other-agent",
      roomId: "room-1",
    }),
  );

  const updateDraft = useComposerDraftStore.getState().update_composer_draft;
  const diagramAttachment = {
    file: { name: "芯片对比.png" },
    id: "attachment-diagram",
    kind: "image",
  };
  updateDraft(firstSessionScope, (current) => ({
    ...current,
    attachments: [diagramAttachment],
    goalLeadAgentId: "agent-cindy",
    input: "对比 M3、M4 和 M5",
    inputMode: "goal",
    selectedTargetIDs: ["agent-cindy"],
  }));
  updateDraft(secondSessionScope, (current) => ({
    ...current,
    input: "第二个 Session 的待发送内容",
  }));
  updateDraft(otherRoomScope, (current) => ({
    ...current,
    input: "另一个 Room 的草稿",
  }));

  const restoredFirstSessionDraft = useComposerDraftStore
    .getState()
    .drafts_by_scope[sameSessionScope];
  assert.equal(restoredFirstSessionDraft.input, "对比 M3、M4 和 M5");
  assert.equal(restoredFirstSessionDraft.inputMode, "goal");
  assert.equal(restoredFirstSessionDraft.goalLeadAgentId, "agent-cindy");
  assert.deepEqual(
    restoredFirstSessionDraft.selectedTargetIDs,
    ["agent-cindy"],
  );
  assert.deepEqual(restoredFirstSessionDraft.attachments, [diagramAttachment]);
  const secondSessionDraft = useComposerDraftStore
    .getState()
    .drafts_by_scope[secondSessionScope];
  assert.equal(secondSessionDraft.input, "第二个 Session 的待发送内容");
  assert.equal(secondSessionDraft.inputMode, "message");
  assert.deepEqual(secondSessionDraft.attachments, []);
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[otherRoomScope].input,
    "另一个 Room 的草稿",
  );

  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
  });
});

test("Goal submission claims its original Session and restores only without newer input", async () => {
  const { useComposerDraftStore } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-draft-store.ts",
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
  });
  const firstScope = "room:goal-session-a";
  const secondScope = "room:goal-session-b";
  const updateDraft = useComposerDraftStore.getState().update_composer_draft;
  updateDraft(firstScope, (current) => ({
    ...current,
    goalLeadAgentId: "agent-lead",
    input: "完成第一条 Goal",
    inputMode: "goal",
  }));
  updateDraft(secondScope, (current) => ({
    ...current,
    input: "另一个 Session 的消息",
  }));

  const firstRevision = useComposerDraftStore
    .getState()
    .drafts_by_scope[firstScope].revision;
  const beginGoal = useComposerDraftStore.getState().begin_goal_submission;
  const submission = beginGoal(firstScope, firstRevision);
  assert.ok(submission);
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[firstScope],
    undefined,
    "the dispatched Goal must leave its original Composer immediately",
  );
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[secondScope].input,
    "另一个 Session 的消息",
  );
  assert.equal(
    useComposerDraftStore.getState()
      .goal_submission_by_scope[firstScope].submissionId,
    submission.submissionId,
  );

  const failGoal = useComposerDraftStore.getState().fail_goal_submission;
  assert.equal(failGoal(submission, "后端拒绝 Goal"), true);
  const restored = useComposerDraftStore.getState().drafts_by_scope[firstScope];
  assert.equal(restored.input, "完成第一条 Goal");
  assert.equal(restored.inputMode, "goal");
  assert.equal(restored.goalLeadAgentId, "agent-lead");
  assert.equal(
    useComposerDraftStore.getState().goal_error_by_scope[firstScope],
    "后端拒绝 Goal",
  );
  assert.equal(
    useComposerDraftStore.getState().goal_error_by_scope[secondScope],
    undefined,
  );

  const retry = beginGoal(firstScope, restored.revision);
  assert.ok(retry);
  updateDraft(firstScope, (current) => ({
    ...current,
    input: "请求期间的新输入",
  }));
  assert.equal(failGoal(retry, "迟到失败"), false);
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[firstScope].input,
    "请求期间的新输入",
    "a late failure must not overwrite newer input",
  );
  assert.equal(
    useComposerDraftStore.getState().goal_error_by_scope[firstScope],
    undefined,
    "a stale failure must not attach its error to newer input",
  );

  const latest = useComposerDraftStore.getState().drafts_by_scope[firstScope];
  const success = beginGoal(firstScope, latest.revision);
  assert.ok(success);
  assert.equal(
    useComposerDraftStore.getState().complete_goal_submission(success),
    true,
  );
  assert.equal(
    useComposerDraftStore.getState().goal_submission_by_scope[firstScope],
    undefined,
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
  });
});

test("unknown Goal remains confirming until a newer durable Goal matches", async () => {
  const { useComposerDraftStore } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-draft-store.ts",
  );
  const {
    observeComposerGoal,
    readObservedComposerGoal,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-goal-observation.ts",
  );
  const { reconcileComposerGoalSubmission } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-goal-submission-reconciliation.ts",
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
  });
  const scope = "room:room-a:session:room:group:conversation-a";
  const foreignScope = "room:room-b:session:room:group:conversation-b";
  const oldGoal = {
    id: "goal-existing",
    metadata: {},
    objective: "生成发布说明",
    version: 4,
  };
  observeComposerGoal(scope, oldGoal);
  useComposerDraftStore.getState().update_composer_draft(scope, (current) => ({
    ...current,
    input: "生成发布说明",
    inputMode: "goal",
  }));
  const draft = useComposerDraftStore.getState().drafts_by_scope[scope];
  const submission = useComposerDraftStore.getState().begin_goal_submission(
    scope,
    draft.revision,
    readObservedComposerGoal(scope),
  );
  assert.ok(submission);
  assert.equal(
    reconcileComposerGoalSubmission(scope, { ...oldGoal, version: 5 }),
    false,
    "durable evidence must not settle before the transport becomes unknown",
  );
  assert.equal(
    useComposerDraftStore.getState().mark_goal_submission_confirming(submission),
    true,
  );
  assert.equal(
    useComposerDraftStore.getState().goal_submission_by_scope[scope].phase,
    "confirming",
  );
  assert.equal(
    useComposerDraftStore.getState().begin_goal_submission(scope, 0),
    null,
    "the same scope stays fail-closed while acceptance is being confirmed",
  );
  assert.equal(
    reconcileComposerGoalSubmission(foreignScope, {
      ...oldGoal,
      id: "goal-foreign",
      version: 9,
    }),
    false,
  );
  assert.equal(
    reconcileComposerGoalSubmission(scope, oldGoal),
    false,
    "an unchanged pre-submit Goal with the same objective is not evidence",
  );
  assert.equal(
    reconcileComposerGoalSubmission(scope, {
      ...oldGoal,
      metadata: { source_objective: "生成发布说明" },
      objective: "整理并生成一份可发布的说明",
      version: 5,
    }),
    true,
    "a newer normalized Goal may reconcile through source_objective",
  );
  assert.equal(
    useComposerDraftStore.getState().goal_submission_by_scope[scope],
    undefined,
  );
});

test("unobserved Goal baseline reconciles only from its exact durable control record", async () => {
  const { useComposerDraftStore } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-draft-store.ts",
  );
  const {
    reconcileComposerGoalSubmission,
    reconcileComposerGoalSubmissionFromMessages,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-goal-submission-reconciliation.ts",
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
  });
  const scope = "dm:agent-a:session:dm:session-a";
  useComposerDraftStore.getState().update_composer_draft(scope, (current) => ({
    ...current,
    input: "首次加载前提交 Goal",
    inputMode: "goal",
  }));
  const draft = useComposerDraftStore.getState().drafts_by_scope[scope];
  const submission = useComposerDraftStore.getState().begin_goal_submission(
    scope,
    draft.revision,
    null,
  );
  assert.ok(submission);
  const confirmationIdentity = {
    clientMessageId: "local_msg_goal-a",
    clientRequestId: "req-goal-a",
    sessionKey: "dm:session-a",
  };
  assert.equal(
    useComposerDraftStore.getState().mark_goal_submission_confirming(
      submission,
      confirmationIdentity,
    ),
    true,
  );
  assert.equal(
    reconcileComposerGoalSubmission(scope, {
      id: "goal-existing-or-new",
      metadata: { source_objective: "首次加载前提交 Goal" },
      objective: "首次加载前提交 Goal",
      version: 9,
    }),
    false,
    "an unknown baseline cannot use same-objective Goal state as acceptance proof",
  );
  const optimistic = {
    agent_id: "agent-a",
    client_message_id: confirmationIdentity.clientMessageId,
    content: "/goal 首次加载前提交 Goal",
    message_id: confirmationIdentity.clientMessageId,
    metadata: { subtype: "goal_set" },
    role: "user",
    round_id: confirmationIdentity.clientMessageId,
    session_key: confirmationIdentity.sessionKey,
    timestamp: 1,
  };
  assert.equal(
    reconcileComposerGoalSubmissionFromMessages(scope, [optimistic]),
    false,
    "the local optimistic card is not durable acceptance evidence",
  );
  assert.equal(
    reconcileComposerGoalSubmissionFromMessages(scope, [{
      ...optimistic,
      message_id: "message-server-a",
      session_key: "dm:foreign-session",
    }]),
    false,
    "a foreign Session control record cannot settle the original scope",
  );
  assert.equal(
    reconcileComposerGoalSubmissionFromMessages(scope, [{
      ...optimistic,
      message_id: "message-server-a",
    }]),
    true,
    "returning to A settles from its exact durable client_message_id even if Goal is terminal",
  );
  assert.equal(
    useComposerDraftStore.getState().goal_submission_by_scope[scope],
    undefined,
  );
});

test("Composer submission clears immediately and failure recovery preserves newer input", async () => {
  const { useComposerDraftStore } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-draft-store.ts",
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
  });
  const scope = "room:revision-guard";
  const updateDraft = useComposerDraftStore.getState().update_composer_draft;
  updateDraft(scope, (current) => ({
    ...current,
    attachments: [{
      file: { name: "提交前.png" },
      id: "attachment-before-submit",
      kind: "image",
    }],
    goalLeadAgentId: "agent-kevin",
    input: "原始草稿",
    inputMode: "goal",
    selectedTargetIDs: ["agent-kevin"],
  }));
  const submittedRevision = useComposerDraftStore
    .getState()
    .drafts_by_scope[scope].revision;
  const claimDraft = useComposerDraftStore
    .getState()
    .claim_composer_draft_for_submission;
  const submittedDraft = claimDraft(scope, submittedRevision);
  assert.equal(submittedDraft.input, "原始草稿");
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[scope],
    undefined,
    "locally dispatched content must leave the Composer before ACK",
  );

  updateDraft(scope, (current) => ({
    ...current,
    attachments: [{
      file: { name: "继续补充.pdf" },
      id: "attachment-after-submit",
      kind: "file",
    }],
    input: "切换后继续输入",
  }));

  const restoreDraft = useComposerDraftStore
    .getState()
    .restore_composer_draft_after_failed_submission;
  assert.equal(restoreDraft(scope, submittedDraft), false);
  const newerDraft = useComposerDraftStore.getState().drafts_by_scope[scope];
  assert.equal(newerDraft.input, "切换后继续输入");
  assert.equal(newerDraft.inputMode, "message");
  assert.equal(newerDraft.goalLeadAgentId, null);
  assert.deepEqual(newerDraft.selectedTargetIDs, []);
  assert.deepEqual(
    newerDraft.attachments.map((attachment) => attachment.file.name),
    ["继续补充.pdf"],
  );
  const newerSubmittedDraft = claimDraft(scope, newerDraft.revision);
  assert.equal(newerSubmittedDraft.input, "切换后继续输入");
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[scope],
    undefined,
  );
  assert.equal(restoreDraft(scope, submittedDraft), true);
  const restoredDraft = useComposerDraftStore.getState().drafts_by_scope[scope];
  assert.equal(restoredDraft.input, "原始草稿");
  assert.equal(restoredDraft.inputMode, "goal");
  assert.equal(restoredDraft.goalLeadAgentId, "agent-kevin");
  assert.deepEqual(restoredDraft.selectedTargetIDs, ["agent-kevin"]);
  assert.deepEqual(
    restoredDraft.attachments.map((attachment) => attachment.file.name),
    ["提交前.png"],
  );
  updateDraft(scope, (current) => ({
    ...current,
    input: "派发前继续输入",
  }));
  assert.equal(
    claimDraft(scope, restoredDraft.revision),
    null,
    "attachment preparation must not clear a draft edited before dispatch",
  );
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[scope].input,
    "派发前继续输入",
  );
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
  });
});

test("restored Composer draft places the caret after the final character", async () => {
  const { focusComposerInputAtEnd } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-model.ts",
  );
  let focusOptions = null;
  let selection = null;
  const textarea = {
    focus(options) {
      focusOptions = options;
    },
    scrollHeight: 96,
    scrollTop: 0,
    setSelectionRange(start, end) {
      selection = [start, end];
    },
    value: "设定一个 goal",
  };

  focusComposerInputAtEnd(textarea);

  assert.deepEqual(focusOptions, { preventScroll: true });
  assert.deepEqual(selection, [textarea.value.length, textarea.value.length]);
  assert.equal(textarea.scrollTop, textarea.scrollHeight);
});

test("Composer input history persists locally and stays isolated by chat", async () => {
  const {
    MAX_COMPOSER_HISTORY_ITEMS,
    useComposerHistoryStore,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-history-store.ts",
  );
  await useComposerHistoryStore.persist.clearStorage();
  useComposerHistoryStore.setState({ items_by_scope: {} });

  const recordHistory = useComposerHistoryStore
    .getState()
    .record_composer_history;
  recordHistory("room:alpha", "  第一条消息  ");
  recordHistory("room:alpha", "第二条消息");
  recordHistory("room:beta", "另一个聊天");

  assert.deepEqual(
    useComposerHistoryStore.getState().items_by_scope["room:alpha"],
    ["第二条消息", "第一条消息"],
  );
  assert.deepEqual(
    useComposerHistoryStore.getState().items_by_scope["room:beta"],
    ["另一个聊天"],
  );

  for (let index = 0; index < MAX_COMPOSER_HISTORY_ITEMS + 5; index += 1) {
    recordHistory("room:bounded", `历史-${index}`);
  }
  const boundedHistory = useComposerHistoryStore
    .getState()
    .items_by_scope["room:bounded"];
  assert.equal(boundedHistory.length, MAX_COMPOSER_HISTORY_ITEMS);
  assert.equal(boundedHistory[0], `历史-${MAX_COMPOSER_HISTORY_ITEMS + 4}`);
  assert.equal(boundedHistory.at(-1), "历史-5");

  const storage = useComposerHistoryStore.persist.getOptions().storage;
  const persisted = await storage.getItem("nexus-composer-history");
  assert.deepEqual(
    persisted.state.items_by_scope["room:alpha"],
    ["第二条消息", "第一条消息"],
  );

  await useComposerHistoryStore.persist.clearStorage();
  useComposerHistoryStore.setState({ items_by_scope: {} });
});
