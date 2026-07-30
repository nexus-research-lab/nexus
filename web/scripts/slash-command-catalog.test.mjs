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

test("slash query only opens at the beginning of a message", async () => {
  const { findSlashCommandTextMatch } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/slash-command-model.ts",
  );

  assert.deepEqual(findSlashCommandTextMatch("/rev", 4, true), {
    end: 4,
    query: "rev",
    start: 0,
  });
  assert.equal(findSlashCommandTextMatch("please /rev", 11, true), null);
  assert.equal(findSlashCommandTextMatch("/review now", 11, true), null);
  assert.equal(findSlashCommandTextMatch("/rev", 4, false), null);
});

test("slash selection inserts a normal host or runtime message", async () => {
  const {
    filterSlashCommands,
    insertSlashCommand,
    isSelectableSlashCommand,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/slash-command-model.ts",
  );
  const commands = [
    {
      description: "Review code",
      enabled: true,
      execution: "runtime",
      name: "review",
    },
    {
      description: "Compact context",
      enabled: true,
      execution: "runtime",
      name: "compact",
    },
    {
      description: "Open the GitHub review prompt",
      enabled: true,
      execution: "runtime",
      name: "github:review (MCP)",
    },
  ];

  assert.deepEqual(
    filterSlashCommands(commands, "code").map((command) => command.name),
    ["review"],
  );
  assert.equal(isSelectableSlashCommand(commands[0]), true);
  assert.equal(
    isSelectableSlashCommand({
      ...commands[0],
      execution: "host",
    }),
    true,
  );
  assert.deepEqual(
    insertSlashCommand("/rev", {
      end: 4,
      query: "rev",
      start: 0,
    }, commands[0]),
    {
      cursorPosition: 8,
      value: "/review ",
    },
  );
  assert.deepEqual(
    insertSlashCommand("/github", {
      end: 7,
      query: "github",
      start: 0,
    }, commands[2]),
    {
      cursorPosition: 21,
      value: "/github:review (MCP) ",
    },
  );
});

test("command catalog parser accepts the public browser contract", async () => {
  const { parseCommandCatalogData } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );
  const payload = {
    agent_id: "agent-a",
    commands: [{
      argument_hint: "<target>",
      description: "Review code",
      enabled: true,
      execution: "runtime",
      name: "review",
    }],
    generation: 3,
    revision: "commands-1",
    runtime_kind: "cc",
    status: "ready",
  };

  assert.deepEqual(parseCommandCatalogData(payload), payload);
  assert.equal(
    parseCommandCatalogData({
      ...payload,
      commands: [{ ...payload.commands[0], enabled: "yes" }],
    }),
    null,
  );
});

test("Room host command catalog events stay scoped to the selected Agent", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const received = [];
  let currentCatalog = { commands: [], status: "cold" };
  const context = {
    scope: {
      agentId: "agent-a",
      isCurrentSessionEvent: (sessionKey) => sessionKey === "room:shared",
    },
    state: {
      setCommandCatalog: (next) => {
        currentCatalog = typeof next === "function"
          ? next(currentCatalog)
          : next;
        received.push(currentCatalog);
      },
    },
  };
  const event = {
    agent_id: "agent-a",
    data: {
      agent_id: "agent-a",
      commands: [{
        enabled: true,
        execution: "host",
        name: "goal",
      }],
      status: "unavailable",
    },
    event_type: "command_catalog",
    session_key: "room:shared",
  };

  AGENT_SESSION_EVENT_HANDLERS.command_catalog(event, context);
  AGENT_SESSION_EVENT_HANDLERS.command_catalog({
    ...event,
    agent_id: "agent-b",
    data: { ...event.data, agent_id: "agent-b" },
  }, context);

  assert.equal(received.length, 1);
  assert.equal(received[0].agent_id, "agent-a");
});

test("authoritative snapshots ignore stale runtime generations", async () => {
  const { selectCommandCatalogSnapshot } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );
  const ready = {
    commands: [{
      enabled: true,
      execution: "runtime",
      name: "review",
    }],
    generation: 2,
    status: "ready",
  };
  assert.equal(selectCommandCatalogSnapshot(ready, {
    commands: [],
    generation: 1,
    status: "starting",
  }), ready);
  assert.equal(selectCommandCatalogSnapshot(ready, {
    commands: [],
    generation: 2,
    status: "starting",
  }), ready);
  assert.deepEqual(selectCommandCatalogSnapshot(ready, {
    commands: [],
    generation: 3,
    status: "starting",
  }), {
    commands: [],
    generation: 3,
    status: "starting",
  });
});
