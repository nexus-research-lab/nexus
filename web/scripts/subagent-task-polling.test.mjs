import assert from "node:assert/strict";
import fs from "node:fs";
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

test("Chinese subagent empty state uses the product term consistently", async () => {
  const { zhConversationMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/conversation.ts",
  );

  assert.equal(zhConversationMessages["subagents.label"], "子智能体");
  assert.equal(
    zhConversationMessages["subagents.no_active"],
    "没有已开启的子智能体",
  );
});

test("subagent control dialogs have complete locale-specific copy", async () => {
  const { enConversationMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/en/conversation.ts",
  );
  const { zhConversationMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/conversation.ts",
  );

  assert.equal(enConversationMessages["subagents.stop_subtitle"], "Interrupt only this exact task.");
  assert.equal(enConversationMessages["subagents.message_send"], "Send");
  assert.equal(enConversationMessages["subagents.message_shortcut_hint"], "Press Cmd/Ctrl + Enter to send.");
  assert.equal(zhConversationMessages["subagents.stop_subtitle"], "只会中断这个精确任务。");
  assert.equal(zhConversationMessages["subagents.message_send"], "发送");
  assert.equal(zhConversationMessages["subagents.message_shortcut_hint"], "按 Cmd/Ctrl + Enter 发送。");
});

test("subagent task changes refresh only the exact source and task", async () => {
  const {
    isSubagentTaskChangeFor,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/subagent/subagent-task-model.ts",
  );
  const sessionSource = {
    kind: "session",
    session_key: "agent:dev:ws:dm:conversation-1",
  };
  const event = {
    agent_id: "agent-dev",
    data: { task_ids: ["task-market-research"] },
    delivery_mode: "ephemeral",
    event_type: "subagent_task_changed",
    protocol_version: 2,
    session_key: sessionSource.session_key,
    timestamp: Date.now(),
  };

  assert.equal(isSubagentTaskChangeFor(event, sessionSource), true);
  assert.equal(
    isSubagentTaskChangeFor(event, sessionSource, "task-market-research"),
    true,
  );
  assert.equal(isSubagentTaskChangeFor(event, sessionSource, "task-other"), false);
  assert.equal(
    isSubagentTaskChangeFor(event, { ...sessionSource, session_key: "other" }),
    false,
  );
  assert.equal(isSubagentTaskChangeFor(event, sessionSource, null, "agent-other"), false);

  const roomEvent = {
    ...event,
    conversation_id: "conversation-room",
    room_id: "room-1",
    session_key: "room-session",
  };
  assert.equal(isSubagentTaskChangeFor(roomEvent, {
    kind: "room",
    conversation_id: "conversation-room",
    room_id: "room-1",
  }, null, "agent-dev"), true);
  assert.equal(isSubagentTaskChangeFor(roomEvent, {
    kind: "room",
    conversation_id: "conversation-other",
    room_id: "room-1",
  }), false);

  const listResource = fs.readFileSync(
    path.join(webRoot, "src/features/conversation/shared/subagent/use-subagent-tasks.ts"),
    "utf8",
  );
  const threadResource = fs.readFileSync(
    path.join(webRoot, "src/features/conversation/shared/subagent/thread/use-subagent-task-thread-resource.ts"),
    "utf8",
  );
  assert.doesNotMatch(listResource, /setInterval|SUBAGENT_TASK_POLL_INTERVAL_MS/);
  assert.doesNotMatch(threadResource, /setInterval|SUBAGENT_TASK_POLL_INTERVAL_MS/);
  assert.match(listResource, /useSubagentTaskRealtimeRefresh/);
  assert.match(threadResource, /taskId: scope\.task\.task_id/);
});

test("subagent title uses the model-provided task description before its generic type", async () => {
  const {
    findSubagentTaskByToolUseId,
    preferFreshSubagentTask,
    subagentTaskAvatarSeed,
    subagentTaskTitle,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/subagent/subagent-task-model.ts",
  );

  assert.equal(
    subagentTaskTitle({
      agent_type: "Explore",
      description: "调研 iPhone Air 硬件规格",
    }),
    "调研 iPhone Air 硬件规格",
  );
  assert.equal(
    subagentTaskTitle({
      agent_type: "Explore",
      description: "调研 iPhone Air 硬件规格",
      name: "硬件规格研究员",
    }),
    "硬件规格研究员",
  );
  assert.equal(subagentTaskTitle({ agent_type: "Explore" }), "Explore");
  assert.equal(
    subagentTaskAvatarSeed({
      task_id: "task-one",
      tool_use_id: "tool-agent-one",
    }),
    "tool-agent-one",
  );
  assert.equal(
    subagentTaskAvatarSeed({ task_id: "task-legacy" }),
    "task-legacy",
  );

  const tasks = [
    { task_id: "task-one", tool_use_id: "tool-agent-one" },
    { task_id: "task-two", tool_use_id: "tool-agent-two" },
  ];
  assert.equal(
    findSubagentTaskByToolUseId(tasks, "tool-agent-two"),
    tasks[1],
  );
  assert.equal(findSubagentTaskByToolUseId(tasks, "task-one"), tasks[0]);
  assert.equal(findSubagentTaskByToolUseId(tasks, "missing"), null);

  const capabilities = {
    observe: true,
    transcript: true,
    stop: true,
    send_message: true,
    resume: true,
  };
  const staleRunningDetail = {
    capabilities,
    runtime_kind: "nxs",
    status: "running",
    task_id: "task-one",
    updated_at: 2_000,
  };
  const completedListTask = {
    ...staleRunningDetail,
    status: "completed",
  };
  assert.equal(
    preferFreshSubagentTask(completedListTask, staleRunningDetail),
    completedListTask,
    "an equally timestamped terminal list projection must beat a stale active detail",
  );
  const resumedDetail = {
    ...staleRunningDetail,
    updated_at: 3_000,
  };
  assert.equal(
    preferFreshSubagentTask(completedListTask, resumedDetail),
    resumedDetail,
    "a newer active detail must expose a resumed task immediately",
  );
});

test("room subagent tasks can be scoped to the Agent that launched them", async () => {
  const { filterSubagentTasksByHostAgent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/subagent/subagent-task-list-model.ts",
  );
  const tasks = [
    {
      host_agent_id: "agent-cindy",
      round_id: "round-previous",
      task_id: "task-hardware",
      updated_at: 1_000,
    },
    {
      host_agent_id: "agent-kevin",
      round_id: "round-current",
      task_id: "task-market",
      updated_at: 2_000,
    },
    {
      host_agent_id: "agent-kevin",
      round_id: "round-current",
      task_id: "task-release",
      updated_at: 2_000,
    },
    { task_id: "task-legacy" },
  ];

  assert.deepEqual(
    filterSubagentTasksByHostAgent(tasks, "agent-cindy"),
    [tasks[0]],
  );
  assert.deepEqual(
    filterSubagentTasksByHostAgent(tasks, "agent-kevin"),
    [tasks[1], tasks[2]],
  );
  assert.equal(filterSubagentTasksByHostAgent(tasks, null), tasks);
});
