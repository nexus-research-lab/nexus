// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("ResizeObserver 浏览器告警不上报为 Web 崩溃", async () => {
  const { isBenignResizeObserverError } = await import("@/bootstrap/recovery/chunk-error-recovery.ts");

  assert.equal(isBenignResizeObserverError(
    "ResizeObserver loop completed with undelivered notifications.",
  ), true);
  assert.equal(isBenignResizeObserverError(
    new Error("ResizeObserver loop limit exceeded"),
  ), true);
  assert.equal(isBenignResizeObserverError("Request timed out"), false);
});

test("session-scoped chat errors expose a stable failure code", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await import("@/hooks/agent/transport/handlers/session-event-handlers.ts");
  const calls = [];
  const context = {
    runtime: {
      applyRoundStatus: (...args) => calls.push(["round", ...args]),
      rejectPendingRequestAck: (...args) => calls.push(["ack", ...args]),
      updateMessageStatus: (...args) => calls.push(["message", ...args]),
    },
    scope: {
      chatType: "dm",
      isCurrentSessionEvent: (sessionKey) => sessionKey === "session-1",
      sessionKey: "session-1",
    },
    state: {
      reliability: {
        reportFailure: (...args) => calls.push(["failure", ...args]),
      },
    },
  };

  AGENT_SESSION_EVENT_HANDLERS.error({
    data: {
      error_type: "chat_error",
      message: "模型暂时不可用",
      round_id: "round-1",
      type: "chat",
    },
    event_type: "error",
    message_id: "message-1",
    protocol_version: 2,
    session_key: "session-1",
    timestamp: 2,
  }, context);

  assert.deepEqual(calls, [
    ["round", "round-1", "error"],
    ["message", "message-1", "error", "round-1"],
    ["failure", {
      agent_round_id: null,
      client_request_id: null,
      code: "provider_unavailable",
      round_id: "round-1",
      session_key: "session-1",
    }],
  ]);
});
