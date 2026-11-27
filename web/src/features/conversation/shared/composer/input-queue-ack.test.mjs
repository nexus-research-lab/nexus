// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const NOOP_RELIABILITY = {
  changeScope: () => {},
  observeProviderRetry: () => {},
  observeRecovery: () => {},
  observeTransport: () => {},
  reconcileSession: () => {},
  reportFailure: () => {},
};


test("new Session preserves durable submission owners while reset cancels all", async () => {
  const { runAgentSessionTransition } = await import("@/hooks/agent/session/use-agent-conversation-session.ts");
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

test("Session navigation preserves message and Goal owners while cancelling page requests", async () => {
  const {
    cancelPendingRequestAcks,
    createPendingRequestAckRegistry,
    resolvePendingRequestAck,
    trackPendingRequestAck,
    waitForRequestAck,
  } = await import("@/hooks/agent/actions/use-pending-request-acks.ts");
  const registry = createPendingRequestAckRegistry();
  trackPendingRequestAck(registry, "req-page");
  trackPendingRequestAck(registry, "req-chat", true);
  trackPendingRequestAck(registry, "req-goal", true);
  const pageRequest = waitForRequestAck(
    registry,
    "req-page",
    () => assert.fail("navigation cancellation should precede timeout"),
    50,
  );
  const chat = waitForRequestAck(
    registry,
    "req-chat",
    () => assert.fail("the durable message owner should remain live"),
    50,
  );
  const goal = waitForRequestAck(
    registry,
    "req-goal",
    () => assert.fail("the durable Goal owner should remain live"),
    50,
  );
  cancelPendingRequestAcks(registry, "切换会话", true);
  await assert.rejects(pageRequest, /切换会话/);
  assert.equal(registry.pending.has("req-chat"), true);
  assert.equal(registry.pending.has("req-goal"), true);
  assert.equal(resolvePendingRequestAck(registry, "req-chat"), true);
  assert.equal(resolvePendingRequestAck(registry, "req-goal"), true);
  await chat;
  await goal;
  assert.equal(registry.preserved.size, 0);
});

test("request transport hard timeout releases the retained Session exactly once", async () => {
  const { RequestTransportLeaseRegistry } = await import("@/lib/websocket/request-transport-leases.ts");
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

test("ordinary requests keep a 10-second ACK window and Goal covers its detached deadline", async () => {
  const {
    getGoalRequestAcceptanceTimeoutMs,
    getMessageSendAckTimeoutMs,
  } = await import("@/config/conversation-policy.ts");
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
  } = await import("@/hooks/agent/actions/use-pending-request-acks.ts");
  const { RequestAcceptanceUnknownError } = await import("@/hooks/agent/actions/use-request-ack-failure.ts");
  const { RequestAcceptanceRejectedError } = await import("@/hooks/agent/actions/use-pending-request-acks.ts");

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
  const rejectedError = new RequestAcceptanceRejectedError(
    "后端拒绝",
    {
      clientMessageId: "local-goal-rejected",
      clientRequestId: "req-error-first",
      sessionKey: "dm:session-a",
    },
  );
  trackPendingRequestAck(rejected, "req-error-first", true);
  assert.equal(
    rejectPendingRequestAck(rejected, "req-error-first", rejectedError),
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
    (error) => (
      error === rejectedError
      && error.correlation.clientMessageId === "local-goal-rejected"
    ),
    "an early explicit rejection must preserve its exact recovery identity",
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

test("ACK recovery failures settle the original waiter as unknown", async () => {
  const { useRequestAckFailure, RequestAcceptanceUnknownError } = await import("@/hooks/agent/actions/use-request-ack-failure.ts");
  let actions;
  let rejectCause;
  const settled = new Promise((resolve) => { rejectCause = resolve; });
  function Probe() {
    actions = useRequestAckFailure({
      activeSessionKeyRef: { current: "session-a" },
      clearOutboundRequest: () => {},
      getInputQueueItems: () => { throw new Error("queue snapshot unavailable"); },
      hasPendingRequestAck: () => true,
      rejectPendingRequestAck: (requestId, cause) => {
        assert.equal(requestId, "req-a");
        rejectCause(cause);
        return true;
      },
      readSessionMessages: async () => [],
      reloadCurrentSession: async () => [],
      resolvePendingRequestAck: () => assert.fail("no positive acceptance evidence"),
      reliability: NOOP_RELIABILITY,
      setMessages: () => assert.fail("uncertain input must be preserved"),
      wsReconnectRef: { current: () => {} },
      wsStateRef: { current: "reconnecting" },
    });
    return null;
  }
  renderToStaticMarkup(createElement(Probe));
  actions.handleRequestAckTimeout("req-a", "msg-a");
  assert.ok(await settled instanceof RequestAcceptanceUnknownError);
});

test("Goal ACK recovery reads the captured Session without projecting the new route", async () => {
  const { buildRequestAckRecoveryReader } = await import("@/hooks/agent/actions/use-request-ack-failure.ts");
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
  } = await import("@/hooks/agent/actions/input-queue-actions.ts");
  const { createOutboundRequestDescriptor } = await import("@/hooks/agent/actions/outbound-request.ts");

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
  const { enqueueInputQueueMessage } = await import("@/hooks/agent/actions/input-queue-actions.ts");
  const { createOutboundRequestDescriptor } = await import("@/hooks/agent/actions/outbound-request.ts");
  const sent = [];
  const preparedRequest = createOutboundRequestDescriptor("local_msg_stable");
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
      reliability: NOOP_RELIABILITY,
      sessionKey: "room:group:conversation-1",
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
    preparedRequest,
  );

  assert.equal(request, preparedRequest);
  assert.equal(request.client_message_id, "local_msg_stable");
  assert.equal(sent[0].client_message_id, request.client_message_id);
  assert.equal(sent[0].client_request_id, request.client_request_id);
  assert.equal(sent[0].type, "input_queue");
});

test("chat dispatch preserves its caller-owned request identity", async () => {
  const { sendSessionMessage } = await import("@/hooks/agent/actions/conversation-chat-actions.ts");
  const { createOutboundRequestDescriptor } = await import("@/hooks/agent/actions/outbound-request.ts");
  const sent = [];
  let messages = [];
  const preparedRequest = createOutboundRequestDescriptor("local_msg_chat_a");
  const request = await sendSessionMessage(
    "发送后立即切换 Session",
    {
      activeSessionKeyRef: { current: "agent:nexus:ws:dm:session-a" },
      identity: {
        agent_id: "nexus",
        chat_type: "dm",
      },
      messages,
      pendingPermissions: [],
      reliability: NOOP_RELIABILITY,
      sessionKey: "agent:nexus:ws:dm:session-a",
      setMessages: (updater) => {
        messages = updater(messages);
      },
      setPendingPermissions: () => {},
      wsSend: (message) => {
        sent.push(message);
        return { disposition: "sent" };
      },
      wsState: "connected",
    },
    {},
    preparedRequest,
  );

  assert.equal(request, preparedRequest);
  assert.equal(sent[0].client_message_id, preparedRequest.client_message_id);
  assert.equal(sent[0].client_request_id, preparedRequest.client_request_id);
  assert.equal(messages[0].client_message_id, preparedRequest.client_message_id);
});

test("input queue ACK parser validates accepted and duplicate flags", async () => {
  const {
    parseChatAckData,
    parseInputQueueAckData,
  } = await import("@/hooks/agent/transport/handlers/session-event-data.ts");
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
  const sequencedPendingSnapshot = {
    ...emptyPendingSnapshot,
    snapshot_room_seq: 42,
  };
  assert.deepEqual(
    parseChatAckData(sequencedPendingSnapshot),
    sequencedPendingSnapshot,
    "the authoritative slot snapshot must carry its durable Room replay fence",
  );
  assert.equal(
    parseChatAckData({ ...sequencedPendingSnapshot, snapshot_room_seq: -1 }),
    null,
  );
  assert.equal(
    parseChatAckData({ ...sequencedPendingSnapshot, snapshot_room_seq: 1.5 }),
    null,
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

test("Room snapshot cursor rejects replay events that could revive stale execution", async () => {
  const { routeAgentConversationEvent } = await import("@/hooks/agent/transport/agent-event-router.ts");
  const context = {
    scope: {
      roomId: "room-1",
      sessionKey: null,
    },
    transport: {
      roomSeqCursorRef: { current: 42 },
      sessionSeqCursorRef: { current: 0 },
    },
  };
  const event = (roomSeq) => ({
    data: {},
    event_type: "future_room_event",
    protocol_version: 1,
    room_id: "room-1",
    room_seq: roomSeq,
    timestamp: Date.now(),
  });

  routeAgentConversationEvent(event(42), context);
  assert.equal(context.transport.roomSeqCursorRef.current, 42);
  routeAgentConversationEvent(event(43), context);
  assert.equal(context.transport.roomSeqCursorRef.current, 43);
});

test("Room snapshot fence resets an obsolete cursor after a backend restart", async () => {
  const { setRoomSnapshotReplayFence } = await import("@/hooks/agent/transport/handlers/session-event-handlers.ts");
  const cursor = { current: 42 };
  setRoomSnapshotReplayFence(cursor, 0);
  assert.equal(
    cursor.current,
    0,
    "a new server generation must not inherit an unreachable old Room sequence",
  );
});

test("public handoff correlation survives ACK, active execution, and terminal lifecycle", async () => {
  const { mergeChatAckPendingSlots } = await import("@/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts");
  const {
    applyRoomAgentExecutionStatus,
    syncRoomAgentExecutionsFromSlots,
  } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
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

test("Room handoff reply header uses a non-action @ identity chip", async () => {
  const { AssistantMessageHeader } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-message-header.tsx");
  const { I18N_CONTEXT } = await import("@/shared/i18n/i18n-context.ts");
  const translations = {
    "composer.stop_generation": "停止",
    "message.assistant_fallback": "协作成员",
    "room.agent_contact_open": "打开 Lead 的联络",
    "room.agent_handoff_reply": "回应 @Lead",
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
    createElement(AssistantMessageHeader, {
      agentMentionDirectory: { names: { "agent-lead": "Lead" } },
      canStop: false,
      compact: false,
      handoffReplySourceAgentId: "agent-lead",
      name: "Researcher",
      onStop: () => {},
      showMetadata: false,
    }),
  ));

  assert.match(html, /回应 @Lead/);
  assert.match(html, /data-handoff-reply="true"/);
  assert.doesNotMatch(html, /<a\b|href=|agent:\/\//);
  assert.doesNotMatch(
    html,
    /<button\b/,
    "the host receipt must not become a contact or wake action",
  );
});

test("Safari composition guard only consumes Enter after composition end", async () => {
  const { isWithinCompositionEndEnterGuard } = await import("@/features/conversation/shared/composer/composer-model.ts");

  assert.equal(isWithinCompositionEndEnterGuard(1_050, 1_000), true);
  assert.equal(isWithinCompositionEndEnterGuard(999, 1_000), false);
  assert.equal(isWithinCompositionEndEnterGuard(1_081, 1_000), false);
});

test("unknown Goal remains confirming until a newer durable Goal matches", async () => {
  const { useComposerDraftStore } = await import("@/features/conversation/shared/composer/composer-draft-store.ts");
  const {
    observeComposerGoal,
    readObservedComposerGoal,
  } = await import("@/features/conversation/shared/composer/composer-goal-observation.ts");
  const { reconcileComposerGoalSubmission } = await import("@/features/conversation/shared/composer/composer-goal-submission-reconciliation.ts");
  useComposerDraftStore.setState({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_recovery_by_scope: {},
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
