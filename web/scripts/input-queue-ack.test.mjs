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
  const { parseInputQueueAckData } = await server.ssrLoadModule(
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
    buildComposerDraftRestoreKey,
    buildComposerDraftScopeKey,
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

  const roomScope = buildComposerDraftScopeKey({
    agentId: "lead-agent",
    roomId: "room-1",
  });
  const sameRoomScope = buildComposerDraftScopeKey({
    agentId: "other-agent",
    roomId: "room-1",
  });
  const otherRoomScope = buildComposerDraftScopeKey({
    roomId: "room-2",
  });
  assert.equal(roomScope, sameRoomScope);
  assert.notEqual(roomScope, otherRoomScope);
  const firstSessionScope = buildComposerDraftRestoreKey({
    draftScopeKey: roomScope,
    sessionKey: "session-1",
  });
  const sameSessionScope = buildComposerDraftRestoreKey({
    draftScopeKey: sameRoomScope,
    sessionKey: "session-1",
  });
  const secondSessionScope = buildComposerDraftRestoreKey({
    draftScopeKey: sameRoomScope,
    sessionKey: "session-2",
  });
  const otherRoomSessionScope = buildComposerDraftRestoreKey({
    draftScopeKey: otherRoomScope,
    sessionKey: "session-1",
  });
  assert.equal(firstSessionScope, sameSessionScope);
  assert.notEqual(firstSessionScope, secondSessionScope);
  assert.notEqual(firstSessionScope, otherRoomSessionScope);

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
  updateDraft(otherRoomSessionScope, (current) => ({
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
    useComposerDraftStore
      .getState()
      .drafts_by_scope[otherRoomSessionScope].input,
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
