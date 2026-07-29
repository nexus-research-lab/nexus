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

test("request ACK registry handles ACK and error before waiter registration", async () => {
  const {
    createPendingRequestAckRegistry,
    rejectPendingRequestAck,
    resolvePendingRequestAck,
    waitForRequestAck,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/use-pending-request-acks.ts",
  );

  const acknowledged = createPendingRequestAckRegistry();
  assert.equal(resolvePendingRequestAck(acknowledged, "req-ack-first"), false);
  await waitForRequestAck(
    acknowledged,
    "req-ack-first",
    () => assert.fail("settled ACK must not time out"),
    10,
  );

  const rejected = createPendingRequestAckRegistry();
  assert.equal(
    rejectPendingRequestAck(rejected, "req-error-first", "后端拒绝"),
    false,
  );
  await assert.rejects(
    waitForRequestAck(
      rejected,
      "req-error-first",
      () => assert.fail("rejected ACK must not time out"),
      10,
    ),
    /后端拒绝/,
  );
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
    content: "检查流式体验",
    message_id: "local-message-1",
    role: "user",
    round_id: "local-message-1",
    session_key: "room-session",
    timestamp: 10,
  };
  const canonical = {
    ...optimistic,
    client_message_id: "local-message-1",
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
  });
});

test("delayed completion cannot clear a newer Composer draft capsule", async () => {
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

  updateDraft(scope, (current) => ({
    ...current,
    attachments: [
      ...current.attachments,
      {
        file: { name: "继续补充.pdf" },
        id: "attachment-after-submit",
        kind: "file",
      },
    ],
    input: "切换后继续输入",
  }));

  const clearDraft = useComposerDraftStore
    .getState()
    .clear_composer_draft_if_revision;
  assert.equal(clearDraft(scope, submittedRevision), false);
  const newerDraft = useComposerDraftStore.getState().drafts_by_scope[scope];
  assert.equal(newerDraft.input, "切换后继续输入");
  assert.equal(newerDraft.inputMode, "goal");
  assert.equal(newerDraft.goalLeadAgentId, "agent-kevin");
  assert.deepEqual(newerDraft.selectedTargetIDs, ["agent-kevin"]);
  assert.deepEqual(
    newerDraft.attachments.map((attachment) => attachment.file.name),
    ["提交前.png", "继续补充.pdf"],
  );
  assert.equal(clearDraft(scope, newerDraft.revision), true);
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope[scope],
    undefined,
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
