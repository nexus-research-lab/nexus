// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("execution invalidation reaches only the matching conversation session", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await import("@/hooks/agent/transport/handlers/session-event-handlers.ts");
  const received = [];
  const context = {
    callbacks: {
      onRoomEvent: (eventType, data) => received.push({ data, eventType }),
    },
    scope: {
      isCurrentSessionEvent: (sessionKey) => sessionKey === "session-current",
    },
  };
  const handler = AGENT_SESSION_EVENT_HANDLERS.execution_invalidated;
  handler({
    data: { execution_id: "execution-old", version: 2 },
    event_type: "execution_invalidated",
    session_key: "session-other",
  }, context);
  handler({
    data: { execution_id: "execution-current", version: 3 },
    event_type: "execution_invalidated",
    session_key: "session-current",
  }, context);

  assert.deepEqual(received, [{
    data: { execution_id: "execution-current", version: 3 },
    eventType: "execution_invalidated",
  }]);
});
