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

const { projectOperationSnapshot } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-projector.ts",
);
const {
  mergeOperationStageSnapshotsForRestore,
  sanitizeOperationStageSnapshotForRestore,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-stage-snapshot-merge.ts",
);
const { findWaitingPermissionEvent } = await server.ssrLoadModule(
  "/src/features/conversation/operation/stage/operation-stage-permission-model.ts",
);
const { decodePermissionRequest } = await server.ssrLoadModule(
  "/src/hooks/agent/transport/handlers/permission/permission-event-data.ts",
);
const { sendSessionPermissionResponse } = await server.ssrLoadModule(
  "/src/hooks/agent/actions/conversation-control-actions.ts",
);

const now = Date.now();
const sessionKey = "room:group:permission-test";
const roundId = "round-permission-test";
const messageId = "message-permission-test";
const toolUseId = "tool-permission-test";
const command = "printf 'ready\\n'";

function assistantMessage() {
  return {
    role: "assistant",
    message_id: messageId,
    session_key: sessionKey,
    agent_id: "agent-permission-test",
    round_id: roundId,
    timestamp: now,
    is_complete: false,
    content: [{
      type: "tool_use",
      id: toolUseId,
      name: "Bash",
      input: { command },
    }],
  };
}

function pendingPermission(requestId = "permission-current", requestedAt = now + 5) {
  return {
    request_id: requestId,
    tool_name: "Bash",
    tool_input: { command },
    session_key: sessionKey,
    agent_id: "agent-permission-test",
    message_id: messageId,
    round_id: roundId,
    tool_use_id: toolUseId,
    interaction_mode: "permission",
    requested_at: requestedAt,
    expires_at: new Date(requestedAt + 120_000).toISOString(),
  };
}

function project(pendingPermissions) {
  return projectOperationSnapshot({
    key: sessionKey,
    session_key: sessionKey,
    agent_id: "agent-permission-test",
    messages: [assistantMessage()],
    pending_permissions: pendingPermissions,
    live_round_ids: [roundId],
    workspace_events: [],
  });
}

test("permission resolution removes stale Stage event and runtime projections immediately", () => {
  const waiting = project([pendingPermission()]);
  const resolved = project([]);
  const merged = mergeOperationStageSnapshotsForRestore(waiting, resolved);

  assert.equal(waiting.active_event?.phase, "waiting");
  assert.equal(resolved.active_event?.phase, "running");
  assert.equal(merged.active_event?.phase, "running");
  assert.equal(
    merged.events.some((event) => event.permission_request_id === "permission-current"),
    false,
  );
  assert.equal(
    merged.runtime_events.some((event) => event.event_type === "permission_request"),
    false,
  );
  assert.equal(findWaitingPermissionEvent(merged.active_event, merged.events), null);
});

test("persisted snapshots never restore an actionable permission prompt", () => {
  const waiting = project([pendingPermission()]);
  const restored = sanitizeOperationStageSnapshotForRestore(waiting);

  assert.equal(restored.events.some((event) => event.permission_request_id), false);
  assert.equal(
    restored.runtime_events.some((event) => event.event_type === "permission_request"),
    false,
  );
  assert.equal(restored.active_event?.phase === "waiting", false);
});

test("permission runtime ordering uses receipt time instead of expiration time", () => {
  const requestedAt = now + 17;
  const waiting = project([{
    ...pendingPermission("permission-timestamp", requestedAt),
    message_id: null,
    tool_use_id: null,
  }]);
  const runtimePermission = waiting.runtime_events.find(
    (event) => event.event_type === "permission_request",
  );

  assert.equal(runtimePermission?.timestamp, requestedAt);

  const decoded = decodePermissionRequest({
    protocol_version: 1,
    event_type: "permission_request",
    session_key: sessionKey,
    timestamp: requestedAt,
    data: {
      request_id: "permission-decoded",
      tool_name: "Bash",
      tool_input: { command },
    },
  });
  assert.equal(decoded?.requested_at, requestedAt);
});

test("consecutive permission notifications advance without reviving resolved requests", () => {
  const first = permissionEvent("permission-first", "tool-first", now + 10);
  const second = permissionEvent("permission-second", "tool-second", now + 20);
  assert.equal(
    findWaitingPermissionEvent(second, [first, second])?.permission_request_id,
    "permission-second",
  );

  const secondResolved = {
    ...second,
    id: "permission-second-resolved",
    phase: "done",
    permission_decision: "allow",
    updated_at: now + 21,
  };
  assert.equal(
    findWaitingPermissionEvent(secondResolved, [first, second, secondResolved])?.permission_request_id,
    "permission-first",
  );

  const firstResolved = {
    ...first,
    id: "permission-first-resolved",
    phase: "done",
    permission_decision: "deny",
    updated_at: now + 22,
  };
  assert.equal(
    findWaitingPermissionEvent(firstResolved, [first, second, secondResolved, firstResolved]),
    null,
  );
});

test("two permission surfaces cannot send the same request twice in one render", () => {
  let pending = [pendingPermission()];
  const sent = [];
  const errors = [];
  const context = {
    activeSessionKeyRef: { current: sessionKey },
    identity: {
      agent_id: "agent-permission-test",
      chat_type: "group",
      conversation_id: "permission-test",
      room_id: "room-permission-test",
      session_key: sessionKey,
    },
    messages: [assistantMessage()],
    readPendingPermissions: () => pending,
    sessionKey,
    setError: (error) => errors.push(error),
    setMessages: () => {},
    setPendingPermissions: (next) => {
      pending = typeof next === "function" ? next(pending) : next;
    },
    wsSend: (message) => {
      sent.push(message);
      return { disposition: "sent" };
    },
    wsState: "connected",
  };

  assert.equal(sendSessionPermissionResponse({
    request_id: "permission-current",
    decision: "allow",
  }, context), true);
  assert.equal(sendSessionPermissionResponse({
    request_id: "permission-current",
    decision: "allow",
  }, context), false);
  assert.equal(sent.length, 1);
  assert.equal(pending.length, 0);
  assert.equal(errors.at(-1), null);
});

function permissionEvent(requestId, toolId, timestamp) {
  return {
    id: `event-${requestId}`,
    session_key: sessionKey,
    round_id: roundId,
    agent_id: "agent-permission-test",
    tool_use_id: toolId,
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    phase: "waiting",
    title: "运行命令",
    target: command,
    input_preview: { command },
    permission_request_id: requestId,
    permission_interaction_mode: "permission",
    updated_at: timestamp,
  };
}
