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

test.after(async () => {
  await server.close();
});

async function reliabilityModel() {
  return server.ssrLoadModule(
    "/src/hooks/agent/reliability/conversation-reliability-model.ts",
  );
}

test("transport interruption stays recoverable until retries are exhausted", async () => {
  const {
    INITIAL_CONVERSATION_RELIABILITY_STATE,
    reduceConversationReliabilityState,
  } = await reliabilityModel();
  let state = reduceConversationReliabilityState(
    INITIAL_CONVERSATION_RELIABILITY_STATE,
    { type: "scope_changed", session_key: "dm:session-a" },
  );
  state = reduceConversationReliabilityState(state, {
    type: "transport_observed",
    state: "connected",
  });
  state = reduceConversationReliabilityState(state, {
    type: "transport_observed",
    state: "reconnecting",
  });
  assert.equal(state.transport_phase, "recovering");
  assert.equal(state.failure, null, "a reconnect attempt is not a terminal failure");

  state = reduceConversationReliabilityState(state, {
    type: "transport_observed",
    state: "failed",
  });
  assert.equal(state.transport_phase, "unavailable");
  state = reduceConversationReliabilityState(state, {
    type: "transport_observed",
    state: "reconnecting",
  });
  assert.equal(state.transport_phase, "recovering");
  state = reduceConversationReliabilityState(state, {
    type: "transport_observed",
    state: "connected",
  });
  assert.equal(state.transport_phase, "healthy");
});

test("shared transport uses five bounded exponential reconnect attempts", async () => {
  const { getReconnectDelay, resolveWebSocketConfig } = await server.ssrLoadModule(
    "/src/lib/websocket/socket-policy.ts",
  );
  const config = resolveWebSocketConfig({ url: "ws://nexus.test" });
  assert.equal(config.maxReconnectAttempts, 5);
  assert.deepEqual(
    [1, 2, 3, 4, 5].map((attempt) => getReconnectDelay(config, attempt)),
    [1_000, 2_000, 4_000, 8_000, 16_000],
  );
});

test("durable reconciliation runs after reconnect but not on first connect", async () => {
  const { shouldReconcileConversationAfterReconnect } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/use-agent-conversation-socket.ts",
  );
  assert.equal(
    shouldReconcileConversationAfterReconnect(false, "connecting", "connected"),
    false,
  );
  assert.equal(
    shouldReconcileConversationAfterReconnect(true, "reconnecting", "connected"),
    true,
  );
  assert.equal(
    shouldReconcileConversationAfterReconnect(true, "connected", "connected"),
    false,
  );
});

test("recovery evidence only clears the correlated request or Agent round", async () => {
  const {
    INITIAL_CONVERSATION_RELIABILITY_STATE,
    reduceConversationReliabilityState,
  } = await reliabilityModel();
  let state = reduceConversationReliabilityState(
    INITIAL_CONVERSATION_RELIABILITY_STATE,
    { type: "scope_changed", session_key: "room:group:a" },
  );
  state = reduceConversationReliabilityState(state, {
    type: "failure_reported",
    failure: {
      agent_round_id: "agent-round-a",
      code: "round_failed",
      round_id: "root-a",
      session_key: "room:group:a",
    },
  });
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      agent_round_id: "agent-round-b",
      kind: "round_progress",
      round_id: "root-a",
      session_key: "room:group:a",
    },
  });
  assert.equal(state.failure?.agent_round_id, "agent-round-a");
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      agent_round_id: "agent-round-a",
      kind: "round_progress",
      round_id: "root-a",
      session_key: "room:group:a",
    },
  });
  assert.equal(state.failure, null);

  state = reduceConversationReliabilityState(state, {
    type: "failure_reported",
    failure: {
      client_request_id: "request-a",
      code: "delivery_unknown",
      session_key: "room:group:a",
    },
  });
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      client_request_id: "request-b",
      kind: "request_accepted",
      session_key: "room:group:a",
    },
  });
  assert.equal(state.failure?.client_request_id, "request-a");
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      client_request_id: "request-a",
      kind: "request_accepted",
      session_key: "room:group:a",
    },
  });
  assert.equal(state.failure, null);

  state = reduceConversationReliabilityState(state, {
    type: "failure_reported",
    failure: {
      code: "session_load_failed",
      session_key: "room:group:a",
    },
  });
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      kind: "round_progress",
      round_id: "root-live",
      session_key: "room:group:a",
    },
  });
  assert.equal(state.failure, null, "live output disproves an earlier Session load failure");
});

test("Room member failure stays on the exact Agent round", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const rootStatuses = [];
  const agentStatuses = [];
  const failures = [];
  const context = {
    callbacks: { onRoomEvent: () => {} },
    runtime: {
      applyAgentRoundStatus: (payload) => agentStatuses.push(payload),
      applyRoundStatus: (...args) => rootStatuses.push(args),
      rejectPendingRequestAck: () => false,
      updateMessageStatus: () => {},
    },
    scope: {
      chatType: "group",
      isCurrentSessionEvent: (sessionKey) => sessionKey === "room:group:a",
      sessionKey: "room:group:a",
    },
    state: {
      reliability: { reportFailure: (failure) => failures.push(failure) },
    },
  };
  AGENT_SESSION_EVENT_HANDLERS.error({
    agent_id: "agent-a",
    agent_round_id: "agent-round-a",
    data: { error_type: "room_error", message: "internal detail" },
    event_type: "error",
    protocol_version: 2,
    round_id: "root-a",
    session_key: "room:group:a",
    timestamp: 1,
  }, context);
  assert.equal(rootStatuses.length, 0, "an ephemeral member error must not poison the Room root");
  assert.equal(agentStatuses.length, 1);
  assert.equal(agentStatuses[0].agent_round_id, "agent-round-a");
  assert.equal(failures.length, 0, "member failure belongs in its Agent shell, not the Room status bar");

  AGENT_SESSION_EVENT_HANDLERS.error({
    data: {
      error_type: "input_queue_error",
      failure_code: "request_rejected",
      message: "internal queue detail",
    },
    event_type: "error",
    protocol_version: 2,
    session_key: "room:group:a",
    timestamp: 2,
  }, context);
  assert.equal(failures.length, 1, "a Room-wide operation failure belongs in the shared status bar");
  assert.equal(failures[0].code, "request_rejected");
});

test("provider retry is transient and a durable Session reconciliation is authoritative", async () => {
  const {
    INITIAL_CONVERSATION_RELIABILITY_STATE,
    reduceConversationReliabilityState,
  } = await reliabilityModel();
  let state = reduceConversationReliabilityState(
    INITIAL_CONVERSATION_RELIABILITY_STATE,
    { type: "scope_changed", session_key: "dm:session-a" },
  );
  state = reduceConversationReliabilityState(state, {
    type: "provider_retry_observed",
    retry: {
      round_id: "round-a",
      session_key: "dm:session-a",
    },
  });
  assert.equal(state.provider_retry?.round_id, "round-a");
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      kind: "round_progress",
      round_id: "round-a",
      session_key: "dm:session-a",
    },
  });
  assert.equal(state.provider_retry, null);

  state = reduceConversationReliabilityState(state, {
    type: "failure_reported",
    failure: {
      code: "session_load_failed",
      session_key: "dm:session-a",
    },
  });
  state = reduceConversationReliabilityState(state, {
    type: "recovery_observed",
    evidence: {
      failure: null,
      kind: "session_reconciled",
      session_key: "dm:session-a",
    },
  });
  assert.equal(state.failure, null);
});

test("provider retry keeps raw error and timing in the process row", async () => {
  const { buildSystemEventBlocks } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-system-events.ts",
  );
  const { ContentSystemEvent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-system-event.tsx",
  );
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const rawError = "The engine is currently overloaded, please try again later";
  const [block] = buildSystemEventBlocks([{
    agent_id: "nexus",
    content: rawError,
    message_id: "retry-1",
    metadata: {
      attempt: 1,
      error: "rate_limit",
      max_retries: 10,
      retry_delay_ms: 0,
      subtype: "api_retry",
    },
    role: "system",
    round_id: "round-a",
    session_key: "dm:session-a",
    timestamp: Date.now(),
  }], true);
  assert.equal(block.attempt, 1);
  assert.equal(block.max_retries, 10);
  assert.equal(block.error, "rate_limit");

  const t = (key, params = {}) => (MESSAGES.zh[key] ?? key).replace(
    /\{(\w+)\}/g,
    (match, name) => params[name] ?? match,
  );
  const html = renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      { value: { locale: "zh", setLocale: () => {}, t } },
      React.createElement(ContentSystemEvent, { block }),
    ),
  );
  assert.match(html, /正在重试/);
  assert.match(html, /1\/10/);
  assert.match(html, /The engine is currently overloaded/);
  assert.match(html, /aria-expanded="true"/);
  assert.match(html, /data-system-event-subtype="api_retry"/);
});

test("nxs Provider terminal reasons map to stable recovery guidance", async () => {
  const { resolveAssistantFailureCode } = await server.ssrLoadModule(
    "/src/hooks/agent/message/assistant-message-model.ts",
  );
  const message = (terminalReason) => ({
    result_summary: {
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: true,
      num_turns: 0,
      subtype: "error",
      terminal_reason: terminalReason,
    },
  });
  assert.equal(resolveAssistantFailureCode(message("rate_limit")), "provider_unavailable");
  assert.equal(resolveAssistantFailureCode(message("server_error")), "provider_unavailable");
  assert.equal(resolveAssistantFailureCode(message("billing_error")), "usage_limited");
  assert.equal(resolveAssistantFailureCode(message("authentication_failed")), "provider_configuration");
  assert.equal(resolveAssistantFailureCode(message("invalid_request")), "validation_failed");
});

test("user notice stays concise and hides transport or request details", async () => {
  const { ConversationReliabilityNotice } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-reliability-notice.tsx",
  );
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const t = (key) => MESSAGES.zh[key] ?? key;
  const html = renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      { value: { locale: "zh", setLocale: () => {}, t } },
      React.createElement(ConversationReliabilityNotice, {
        compact: false,
        reliability: {
          failure: {
            client_request_id: "secret-request-id",
            code: "round_failed",
            round_id: "secret-round-id",
            session_key: "dm:secret-session",
          },
          provider_retry: null,
          transport_phase: "healthy",
        },
      }),
    ),
  );
  assert.match(html, /回复生成失败/);
  assert.match(html, /请重新发送/);
  assert.doesNotMatch(html, /secret-request-id|secret-round-id|secret-session|查看详情/);
  assert.match(html, /data-conversation-failure-code="round_failed"/);
  assert.match(html, /aria-live="polite"/);
  assert.match(html, /role="status"/);
});

test("unknown message receipt warns against duplicate submission", async () => {
  const { ConversationReliabilityNotice } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-reliability-notice.tsx",
  );
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const t = (key) => MESSAGES.zh[key] ?? key;
  const html = renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      { value: { locale: "zh", setLocale: () => {}, t } },
      React.createElement(ConversationReliabilityNotice, {
        compact: true,
        reliability: {
          failure: {
            client_request_id: "request-a",
            code: "delivery_unknown",
            session_key: "dm:session-a",
          },
          provider_retry: null,
          transport_phase: "healthy",
        },
      }),
    ),
  );
  assert.match(html, /消息状态未确认/);
  assert.match(html, /确认前不要重复发送/);
  assert.match(html, />刷新</);
  assert.doesNotMatch(html, /请稍后重试/);
});
