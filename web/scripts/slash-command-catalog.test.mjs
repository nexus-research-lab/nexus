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

test("slash query only opens at the beginning of a message", async () => {
  const { findSlashCommandTextMatch } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/slash-command-model.ts",
  );

  assert.deepEqual(findSlashCommandTextMatch("/", 1, true), {
    end: 1,
    query: "",
    start: 0,
  });
  assert.deepEqual(findSlashCommandTextMatch("/rev", 4, true), {
    end: 4,
    query: "rev",
    start: 0,
  });
  assert.equal(findSlashCommandTextMatch("please /rev", 11, true), null);
  assert.equal(findSlashCommandTextMatch("/review now", 11, true), null);
  assert.equal(findSlashCommandTextMatch("/rev", 4, false), null);
});

test("slash commands match name prefixes and sort by name", async () => {
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
    [],
  );
  assert.deepEqual(
    filterSlashCommands([
      {
        argument_hint: "<objective>",
        description: "Set the Goal",
        enabled: true,
        execution: "host",
        name: "goal",
      },
      {
        description: "Generate project files",
        enabled: true,
        execution: "runtime",
        name: "scaffold",
      },
      {
        description: "Coordinate agents",
        enabled: true,
        execution: "host",
        name: "collaborate",
      },
    ], "c").map((command) => command.name),
    ["collaborate"],
  );
  assert.deepEqual(
    filterSlashCommands(commands, "").map((command) => command.name),
    ["compact", "github:review (MCP)", "review"],
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

test("slash model picker only exposes Nexus provider models", async () => {
  const {
    buildSlashModelOptions,
    formatSlashModelInsertText,
    insertSlashTextAtCursor,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/slash-command-model.ts",
  );
  const options = buildSlashModelOptions({
    items: [{
      display_name: "Anthropic",
      models: [
        {
          display_name: "Sonnet custom label",
          model_id: "sonnet",
        },
        {
          display_name: "Claude Sonnet 4.6",
          model_id: "claude-sonnet-4-6",
        },
      ],
      provider: "anthropic",
    }],
  });

  assert.equal(options[0].id, "sonnet");
  assert.equal(
    options.filter((option) => option.id === "sonnet").length,
    1,
  );
  assert.equal(options.every((option) => Boolean(option.provider)), true);
  assert.deepEqual(
    options.find((option) => (
      option.provider === "anthropic"
      && option.id === "claude-sonnet-4-6"
    )),
    {
      id: "claude-sonnet-4-6",
      label: "Claude Sonnet 4.6",
      provider: "anthropic",
      providerLabel: "Anthropic",
    },
  );
  assert.deepEqual(
    insertSlashTextAtCursor(
      "继续",
      2,
      formatSlashModelInsertText({
        id: "claude-sonnet-4-6",
        label: "Claude Sonnet 4.6",
        provider: "anthropic",
      }),
    ),
    {
      cursorPosition: 37,
      value: "继续/model anthropic/claude-sonnet-4-6 ",
    },
  );
});

test("Nexus model confirmation closes without runtime activity", async () => {
  const { AgentConversationRuntimeMachine } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/agent-conversation-runtime-machine.ts",
  );
  const {
    applyTerminalRoundMessageStatus,
    replaceOptimisticUserMessage,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const { parseConversationMessage } = await server.ssrLoadModule(
    "/src/lib/conversation/message-protocol.ts",
  );
  const { parseEventMessage } = await server.ssrLoadModule(
    "/src/lib/websocket/protocol/event-message.ts",
  );
  const { parseRoundStatusEventPayload } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );
  const machine = new AgentConversationRuntimeMachine("dm");
  machine.trackOutboundRequest("request-model");
  machine.trackChatAck({
    ack_timeout_ms: 10_000,
    client_message_id: "client-message-model",
    client_request_id: "request-model",
    pending: [],
    pending_snapshot: false,
    round_id: "round-model",
    user_message_committed: false,
    user_message_delivery_mode: "transient",
    user_message_id: "user-model",
  });
  const optimisticUser = {
    agent_id: "agent-model",
    content: "/model deepseek/deepseek-v4-flash",
    message_id: "client-message-model",
    role: "user",
    round_id: "client-message-model",
    session_key: "agent:agent-model:ws:dm:session-model",
    timestamp: 1,
  };
  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimisticUser],
      "client-message-model",
      "user-model",
      "round-model",
      false,
      "transient",
    ).map(({ delivery_mode, message_id, round_id }) => ({
      delivery_mode,
      message_id,
      round_id,
    })),
    [{
      delivery_mode: "transient",
      message_id: "user-model",
      round_id: "round-model",
    }],
    "host Slash ACK must retain the user command as a transient timeline item",
  );
  const noticeEvent = parseEventMessage({
    data: {
      agent_id: "agent-model",
      content: [{
        text: "Set model to DeepSeek / deepseek-v4-flash",
        type: "text",
      }],
      message_id: "assistant-model",
      role: "assistant",
      round_id: "round-model",
      stop_reason: "end_turn",
      timestamp: 1,
    },
    delivery_mode: "transient",
    event_type: "message",
    protocol_version: 2,
    session_key: "agent:agent-model:ws:dm:session-model",
    timestamp: 1,
  });
  assert.ok(noticeEvent);
  assert.equal(parseEventMessage({
    ...noticeEvent,
    delivery_mode: "unknown",
  }), null);
  const notice = parseConversationMessage(noticeEvent.data, {
    deliveryMode: noticeEvent.delivery_mode,
    sessionKey: noticeEvent.session_key,
  });
  assert.ok(notice);
  machine.trackAssistantMessage(notice);

  assert.equal(machine.snapshot().isLoading, false);
  assert.deepEqual(machine.snapshot().liveRoundIds, []);

  const terminal = parseRoundStatusEventPayload({
    is_terminal: true,
    result_subtype: "success",
    round_id: "round-model",
    status: "finished",
  });
  assert.deepEqual(terminal, {
    is_terminal: true,
    result_subtype: "success",
    round_id: "round-model",
    status: "finished",
  });
  machine.trackRoundStatus(terminal.round_id, terminal.status);
  machine.emit();
  assert.equal(machine.snapshot().isLoading, false);
  assert.equal(machine.snapshot().terminalRoundIds.includes("round-model"), true);
  const visibleNotices = applyTerminalRoundMessageStatus(
    [notice],
    terminal.round_id,
    terminal.status,
  );
  assert.equal(visibleNotices.length, 1);
  assert.equal(visibleNotices[0].delivery_mode, "transient");
  assert.equal(visibleNotices[0].stream_status, "done");
  assert.deepEqual(
    applyTerminalRoundMessageStatus(
      [{ ...notice, delivery_mode: "ephemeral" }],
      terminal.round_id,
      terminal.status,
    ),
    [],
  );
  assert.equal(parseRoundStatusEventPayload({
    is_terminal: true,
    round_id: "round-model",
    status: "completed",
  }), null);
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

test("authoritative snapshots ignore stale generations and status", async () => {
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
    status: "unavailable",
  }), ready);
  assert.equal(selectCommandCatalogSnapshot(ready, {
    commands: [],
    generation: 2,
    status: "unavailable",
  }), ready);
  assert.deepEqual(selectCommandCatalogSnapshot(ready, {
    commands: [],
    generation: 3,
    status: "unavailable",
  }), {
    commands: [],
    generation: 3,
    status: "unavailable",
  });
});
