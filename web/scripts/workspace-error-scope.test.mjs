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

test("ResizeObserver 浏览器告警不上报为 Web 崩溃", async () => {
  const { isBenignResizeObserverError } = await server.ssrLoadModule(
    "/src/bootstrap/recovery/chunk-error-recovery.ts",
  );

  assert.equal(isBenignResizeObserverError(
    "ResizeObserver loop completed with undelivered notifications.",
  ), true);
  assert.equal(isBenignResizeObserverError(
    new Error("ResizeObserver loop limit exceeded"),
  ), true);
  assert.equal(isBenignResizeObserverError("Request timed out"), false);
});

test("workspace subscription errors do not mutate conversation state", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const calls = [];
  const context = {
    runtime: {
      applyRoundStatus: (...args) => calls.push(["round", ...args]),
      rejectPendingRequestAck: (...args) => calls.push(["ack", ...args]),
      updateMessageStatus: (...args) => calls.push(["message", ...args]),
    },
    scope: {
      isCurrentSessionEvent: () => true,
    },
    state: {
      setError: (...args) => calls.push(["error", ...args]),
    },
  };

  AGENT_SESSION_EVENT_HANDLERS.error({
    data: {
      agent_id: "agent-1",
      error_type: "workspace_subscription_error",
      message: "服务内部错误",
      type: "subscribe_workspace",
    },
    event_type: "error",
    protocol_version: 2,
    timestamp: 1,
  }, context);

  assert.deepEqual(calls, []);
});

test("session-scoped chat errors retain their existing behavior", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const calls = [];
  const context = {
    runtime: {
      applyRoundStatus: (...args) => calls.push(["round", ...args]),
      rejectPendingRequestAck: (...args) => calls.push(["ack", ...args]),
      updateMessageStatus: (...args) => calls.push(["message", ...args]),
    },
    scope: {
      isCurrentSessionEvent: (sessionKey) => sessionKey === "session-1",
    },
    state: {
      setError: (...args) => calls.push(["error", ...args]),
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
    ["error", "模型暂时不可用"],
  ]);
});
