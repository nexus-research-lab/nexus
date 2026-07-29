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

test("slash selection inserts a normal runtime prompt", async () => {
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
      execution: "runtime_prompt",
      name: "review",
    },
    {
      description: "Compact context",
      enabled: true,
      execution: "runtime_prompt",
      name: "compact",
    },
    {
      description: "Open the GitHub review prompt",
      enabled: true,
      execution: "runtime_prompt",
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
    false,
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
      execution: "runtime_prompt",
      name: "review",
    }],
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

test("Room command catalog events stay scoped to the selected Agent", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const received = [];
  const context = {
    scope: {
      agentId: "agent-a",
      isCurrentSessionEvent: (sessionKey) => sessionKey === "room:shared",
    },
    state: {
      setCommandCatalog: (catalog) => received.push(catalog),
    },
  };
  const event = {
    agent_id: "agent-a",
    data: {
      agent_id: "agent-a",
      commands: [{
        enabled: true,
        execution: "runtime_prompt",
        name: "review",
      }],
      status: "ready",
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

test("command catalog refresh carries the full Room address", async () => {
  const { buildCommandCatalogRequest } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/conversation-command-builders.ts",
  );

  assert.deepEqual(buildCommandCatalogRequest({
    agent_id: "agent-a",
    conversation_id: "conversation-a",
    initialize_runtime: true,
    room_id: "room-a",
    session_key: "room:group:conversation-a",
  }), {
    agent_id: "agent-a",
    conversation_id: "conversation-a",
    initialize_runtime: true,
    room_id: "room-a",
    session_key: "room:group:conversation-a",
    type: "get_command_catalog",
  });
  assert.deepEqual(buildCommandCatalogRequest({
    agent_id: "agent-a",
    session_key: "agent:agent-a:ws:dm:main",
  }), {
    agent_id: "agent-a",
    session_key: "agent:agent-a:ws:dm:main",
    type: "get_command_catalog",
  });
});
