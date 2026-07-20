import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createElement } from "react";
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

const { buildTaskAppSession } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/task-app-model.ts",
);
const { TaskAppSurface } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/task-app-surface.tsx",
);
const { projectOperationSnapshot } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-projector.ts",
);
const { resolveOperationToolProfile } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-tool-catalog.ts",
);

const now = Date.now();

function taskEvent({
  id,
  input,
  kind = "task_progress",
  phase = "done",
  result = null,
  toolName,
  timestamp,
}) {
  return {
    agent_id: "agent:task-test",
    id,
    input_preview: input,
    kind,
    message_id: `message:${id}`,
    phase,
    result_preview: result,
    round_id: "round:task-test",
    session_key: "session:task-test",
    surface: "task",
    target: input?.task_id ?? input?.taskId ?? input?.subject ?? toolName,
    title: toolName,
    tool_name: toolName,
    tool_use_id: `tool:${id}`,
    updated_at: timestamp,
  };
}

test("Claude Code and nxs task tools use exact task mappings", () => {
  for (const toolName of ["Task", "Agent", "TaskCreate", "task.create", "task.run", "task.background"]) {
    assert.equal(resolveOperationToolProfile(toolName).surface, "task", toolName);
    assert.equal(resolveOperationToolProfile(toolName).action, "task", toolName);
  }
  for (const toolName of [
    "TaskOutput", "AgentOutputTool", "TaskList", "TaskGet", "TaskUpdate", "TaskStop",
    "task.list", "task.get", "task.update", "task.stop", "task.output", "task.backgrounds",
  ]) {
    assert.equal(resolveOperationToolProfile(toolName).surface, "task", toolName);
    assert.equal(resolveOperationToolProfile(toolName).action, "task_progress", toolName);
  }
  for (const toolName of ["TodoWrite", "todo.write", "todo.read", "plan.enter", "plan.status", "plan.exit"]) {
    assert.equal(resolveOperationToolProfile(toolName).surface, "task", toolName);
    assert.equal(resolveOperationToolProfile(toolName).action, "plan", toolName);
  }
  assert.equal(resolveOperationToolProfile("KillShell").surface, "terminal");
  assert.equal(resolveOperationToolProfile("task_manager.search").action, "generic");
});

test("TodoWrite uses the latest real plan snapshot and active labels", () => {
  const first = taskEvent({
    id: "todo:first",
    input: { todos: [{ activeForm: "正在梳理", content: "梳理需求", status: "in_progress" }] },
    kind: "plan_update",
    phase: "running",
    toolName: "TodoWrite",
    timestamp: now,
  });
  const latest = taskEvent({
    id: "todo:latest",
    input: { todos: [
      { activeForm: "正在梳理", content: "梳理需求", status: "completed" },
      { activeForm: "正在验证", content: "运行验证", status: "in_progress" },
    ] },
    kind: "plan_update",
    phase: "running",
    toolName: "TodoWrite",
    timestamp: now + 1,
  });
  const session = buildTaskAppSession(latest, [first, latest]);

  assert.equal(session.active_section, "plan");
  assert.deepEqual(session.plan_items.map((item) => item.title), ["梳理需求", "运行验证"]);
  assert.deepEqual(session.plan_items.map((item) => item.state), ["completed", "running"]);
  assert.equal(session.plan_items[1].active_label, "正在验证");
});

test("todo.read restores a returned plan snapshot", () => {
  const event = taskEvent({
    id: "todo:read",
    input: {},
    kind: "plan_update",
    result: {
      content: "Current todos",
      structured_output: {
        todos: [{ activeForm: "正在验证", content: "运行验证", status: "in_progress" }],
      },
    },
    toolName: "todo.read",
    timestamp: now,
  });
  const session = buildTaskAppSession(event, [event]);

  assert.equal(session.plan_items.length, 1);
  assert.equal(session.plan_items[0].title, "运行验证");
  assert.equal(session.plan_items[0].active_label, "正在验证");
  assert.equal(session.plan_items[0].state, "running");
});

test("one task lifecycle merges by the real task id", () => {
  const started = taskEvent({
    id: "task:started",
    input: { description: "检查舞台窗口", prompt: "检查任务 App 的真实状态", task_id: "task-real-42" },
    kind: "task_delegate",
    phase: "running",
    toolName: "Task",
    timestamp: now,
  });
  const progress = taskEvent({
    id: "task:progress",
    input: {
      description: "检查舞台窗口",
      last_tool_name: "Read",
      status: "running",
      task_id: "task-real-42",
      usage: { duration_ms: 2400, tool_uses: 3, total_tokens: 680 },
    },
    phase: "running",
    toolName: "TaskOutput",
    timestamp: now + 1,
  });
  const finished = taskEvent({
    id: "task:finished",
    input: { status: "completed", task_id: "task-real-42" },
    result: { summary: "窗口检查完成", status: "completed" },
    toolName: "TaskOutput",
    timestamp: now + 2,
  });
  const session = buildTaskAppSession(finished, [started, progress, finished]);
  const [item] = session.task_items;

  assert.equal(session.task_items.length, 1);
  assert.equal(item.task_id, "task-real-42");
  assert.equal(item.title, "检查舞台窗口");
  assert.equal(item.prompt, "检查任务 App 的真实状态");
  assert.equal(item.last_tool_name, "Read");
  assert.equal(item.output, "窗口检查完成");
  assert.equal(item.state, "completed");
  assert.equal(item.events.length, 3);
  assert.ok(item.usage.some((usage) => usage.label === "Tokens" && usage.value === "680"));
});

test("structured TaskList results become real task rows", () => {
  const listed = taskEvent({
    id: "task:list",
    input: {},
    result: {
      content: "2 tasks",
      structured_output: {
        tasks: [
          { id: "one", subject: "读取代码", status: "in_progress" },
          { id: "two", subject: "运行测试", status: "pending" },
        ],
      },
    },
    toolName: "TaskList",
    timestamp: now,
  });
  const session = buildTaskAppSession(listed, [listed]);

  assert.deepEqual(session.task_items.map((item) => item.task_id), ["one", "two"]);
  assert.deepEqual(session.task_items.map((item) => item.state), ["running", "pending"]);
});

test("system task events are projected and preserve terminal status", () => {
  const messages = [
    {
      agent_id: "agent:task-test",
      content: "检查舞台窗口",
      message_id: "system:started",
      metadata: {
        description: "检查舞台窗口",
        prompt: "验证真实任务状态",
        subtype: "task_started",
        task_id: "task-system-7",
        tool_use_id: "tool:system-task",
      },
      role: "system",
      round_id: "round:system-task",
      session_key: "session:task-test",
      timestamp: now,
    },
    {
      agent_id: "agent:task-test",
      content: "检查失败",
      message_id: "system:finished",
      metadata: {
        status: "failed",
        subtype: "task_notification",
        summary: "检查失败",
        task_id: "task-system-7",
        usage: { total_tokens: 99 },
      },
      role: "system",
      round_id: "round:system-task",
      session_key: "session:task-test",
      timestamp: now + 1,
    },
  ];
  const snapshot = projectOperationSnapshot({
    agent_id: "agent:task-test",
    key: "task-system",
    live_round_ids: [],
    messages,
    pending_permissions: [],
    session_key: "session:task-test",
    workspace_events: [],
  });
  const taskEvents = snapshot.events.filter((event) => event.surface === "task");
  const session = buildTaskAppSession(taskEvents.at(-1), taskEvents);

  assert.equal(taskEvents.length, 2);
  assert.deepEqual(taskEvents.map((event) => event.phase), ["running", "error"]);
  assert.equal(session.task_items.length, 1);
  assert.equal(session.task_items[0].state, "failed");
  assert.equal(session.task_items[0].task_id, "task-system-7");
});

test("Tasks surface contains no fabricated process telemetry", () => {
  const event = taskEvent({
    id: "task:render",
    input: { description: "执行真实任务", status: "running", task_id: "task-ui-1" },
    phase: "running",
    toolName: "TaskOutput",
    timestamp: now,
  });
  const markup = renderToStaticMarkup(createElement(TaskAppSurface, {
    event,
    relatedEvents: [event],
  }));

  assert.match(markup, /执行真实任务/);
  assert.match(markup, /进行中/);
  assert.doesNotMatch(markup, /PID|%CPU|CPU 负载|Memory|进程 ID/);
});
