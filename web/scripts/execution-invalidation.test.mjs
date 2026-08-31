import assert from "node:assert/strict";
import fs from "node:fs/promises";
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

test("execution invalidation reaches only the matching conversation session", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
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
