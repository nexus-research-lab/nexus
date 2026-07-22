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

test.after(async () => server.close());

const {
  deriveStageDesktopIntents,
  stageAppSessionIdForIntent,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-desktop-intents.ts",
);
const { planOperationDesktop } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-scene-planner.ts",
);
const { projectOperationSnapshot } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-projector.ts",
);
const { mergeOperationStageSnapshotsForRestore } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-stage-snapshot-merge.ts",
);
const { resolveOperationToolVisualContract } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-tool-visual-contract.ts",
);
const {
  buildLibrarySessionView,
  filterLibraryEntries,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/library-session-model.ts",
);
const { HandoffSurface } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/handoff-surface.tsx",
);
const { collectNarrativeEvents } = await server.ssrLoadModule(
  "/src/features/conversation/operation/stage/operation-stage-narrative.ts",
);

const now = Date.now();
const sessionKey = "room:group:lifecycle";
const roundId = "round:lifecycle";

function operationEvent(overrides = {}) {
  return {
    id: "event:lifecycle",
    session_key: sessionKey,
    round_id: roundId,
    agent_id: "agent:lifecycle",
    message_id: "message:lifecycle",
    tool_use_id: "tool:lifecycle",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "workspace",
    phase: "done",
    title: "读取文件",
    target: "src/app.ts",
    summary: "读取源文件",
    updated_at: now,
    ...overrides,
  };
}

function operationSnapshot(activeEvent, events, workspaceEvents = []) {
  return {
    key: sessionKey,
    session_key: sessionKey,
    active_event: activeEvent,
    events,
    recent_evidence: events.flatMap((event) => event.evidence ?? []),
    runtime_events: [],
    workspace_events: workspaceEvents,
    updated_at: activeEvent.updated_at,
  };
}

test("Skill events use one persistent Library app session", () => {
  const first = operationEvent({
    id: "skill:frontend",
    tool_use_id: "tool:skill:frontend",
    tool_name: "Skill",
    kind: "context_read",
    surface: "knowledge",
    target: "frontend-design",
    input_preview: { skill_name: "frontend-design" },
    result_preview: "# Frontend design\n\nBuild the real application surface.",
    summary: "载入前端设计规范",
    updated_at: now - 20,
  });
  const second = operationEvent({
    id: "skill:webapp",
    tool_use_id: "tool:skill:webapp",
    tool_name: "skill.use",
    kind: "context_read",
    surface: "knowledge",
    target: "webapp-testing",
    input_preview: { skill_name: "webapp-testing" },
    result_preview: { content: "# Web testing\n\nVerify the rendered interface." },
    summary: "载入浏览器测试规范",
    updated_at: now - 10,
  });
  const snapshot = operationSnapshot(second, [first, second]);
  const desktop = planOperationDesktop({ event: second, snapshot });
  const libraryWindows = desktop.windows.filter((window) => window.kind === "library");
  const view = buildLibrarySessionView({ event: second, relatedEvents: [first, second] });

  assert.equal(resolveOperationToolVisualContract(second).component, "library");
  assert.equal(deriveStageDesktopIntents(second)[0]?.app, "library");
  assert.equal(
    stageAppSessionIdForIntent(roundId, deriveStageDesktopIntents(second)[0], (value) => value),
    `${roundId}:library`,
  );
  assert.equal(libraryWindows.length, 1);
  assert.equal(libraryWindows[0].id, `${roundId}:library`);
  assert.equal(libraryWindows[0].phase, "focused");
  assert.deepEqual(view.entries.map((entry) => entry.name), ["webapp-testing", "frontend-design"]);
  assert.match(view.entries[0].content, /Verify the rendered interface/);
  assert.deepEqual(filterLibraryEntries(view.entries, "frontend").map((entry) => entry.name), ["frontend-design"]);
});

test("terminal round completion keeps real apps and adds one compact handoff", () => {
  const write = operationEvent({
    id: "write:gomoku",
    tool_use_id: "tool:write:gomoku",
    tool_name: "Write",
    kind: "workspace_edit",
    surface: "editor",
    target: "gomoku.html",
    input_preview: { file_path: "gomoku.html", content: "<!doctype html><main>Gomoku</main>" },
    evidence: [{ type: "file", label: "创建", value: "gomoku.html" }],
    summary: "创建五子棋页面",
    updated_at: now - 30,
  });
  const run = operationEvent({
    id: "run:gomoku",
    tool_use_id: "tool:run:gomoku",
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    target: "open gomoku.html",
    input_preview: { command: "open gomoku.html" },
    result_preview: { content: "Opened gomoku.html", exit_code: 0, is_error: false },
    summary: "打开五子棋页面",
    updated_at: now - 20,
  });
  const summary = operationEvent({
    id: "summary:gomoku",
    tool_use_id: null,
    tool_name: null,
    kind: "round_summary",
    surface: "summary",
    target: "gomoku.html",
    result_preview: "五子棋页面已完成",
    summary: "页面已经创建并完成预览。",
    updated_at: now - 10,
  });
  const workspaceItem = {
    id: "workspace:gomoku",
    agent_id: write.agent_id,
    path: "gomoku.html",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: sessionKey,
    tool_use_id: write.tool_use_id,
    event_type: "file_write_end",
    live_content: write.input_preview.content,
    updated_at: now - 25,
  };
  const snapshot = operationSnapshot(summary, [write, run, summary], [workspaceItem]);
  const desktop = planOperationDesktop({ event: summary, snapshot });
  const handoffs = desktop.windows.filter((window) => window.kind === "handoff");

  assert.equal(handoffs.length, 1);
  assert.equal(handoffs[0].layout, "compact");
  assert.equal(handoffs[0].phase, "focused");
  assert.equal(handoffs[0].payload.handoff_summary.status_label, "可继续");
  assert.equal(desktop.active_window_id, `${roundId}:handoff`);
  assert.equal(
    desktop.windows.filter((window) => window.phase === "focused").length,
    1,
  );
  assert.equal(
    desktop.windows.find((window) => window.kind === "browser")?.phase,
    "background",
  );
  assert.ok(desktop.windows.some((window) => window.kind === "terminal"));
  assert.ok(desktop.windows.some((window) => window.kind === "code_editor"));

  const markup = renderToStaticMarkup(createElement(HandoffSurface, {
    event: summary,
    evidence: summary.evidence ?? [],
    handoffSummary: handoffs[0].payload.handoff_summary,
    relatedEvents: [write, run, summary],
    snapshot,
  }));
  assert.match(markup, /本轮已完成/);
  assert.match(markup, /打开产物/);
  assert.doesNotMatch(markup, /看板|缩略片|执行清单|执行中|未收束/);
});

test("completed NXS rounds without result_summary still enter handoff", () => {
  const messages = [
    {
      message_id: "message:user",
      session_key: sessionKey,
      round_id: roundId,
      agent_id: "agent:lifecycle",
      role: "user",
      content: "调研 M5 芯片",
      timestamp: now - 30,
    },
    {
      message_id: "message:tool",
      session_key: sessionKey,
      round_id: roundId,
      agent_id: "agent:lifecycle",
      role: "assistant",
      content: [
        {
          type: "tool_use",
          id: "tool:web-search",
          name: "WebSearch",
          input: { query: "Apple M5" },
        },
        {
          type: "tool_result",
          tool_use_id: "tool:web-search",
          content: "Apple M5 search results",
          is_error: false,
        },
      ],
      is_complete: true,
      stop_reason: "tool_use",
      timestamp: now - 20,
    },
    {
      message_id: "message:answer",
      session_key: sessionKey,
      round_id: roundId,
      agent_id: "agent:lifecycle",
      role: "assistant",
      content: [{ type: "text", text: "M5 调研已经完成。" }],
      is_complete: true,
      stop_reason: "end_turn",
      timestamp: now - 10,
    },
  ];
  const snapshot = projectOperationSnapshot({
    key: sessionKey,
    session_key: sessionKey,
    agent_id: "agent:lifecycle",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events: [],
  });
  const summary = snapshot.active_event;
  const desktop = summary ? planOperationDesktop({ event: summary, snapshot }) : null;

  assert.equal(summary?.kind, "round_summary");
  assert.equal(summary?.phase, "done");
  assert.equal(summary?.summary, "M5 调研已经完成。");
  assert.equal(snapshot.runtime_events.at(-1)?.event_type, "round_handoff");
  assert.equal(desktop?.active_window_id, `${roundId}:handoff`);
  assert.equal(desktop?.windows.filter((window) => window.phase === "focused").length, 1);
  assert.ok(desktop?.windows.some((window) => window.kind === "browser"));

  const liveSnapshot = projectOperationSnapshot({
    key: sessionKey,
    session_key: sessionKey,
    agent_id: "agent:lifecycle",
    messages,
    pending_permissions: [],
    live_round_ids: [roundId],
    workspace_events: [],
  });
  assert.notEqual(liveSnapshot.active_event?.kind, "round_summary");
  assert.ok(liveSnapshot.runtime_events.every((item) => item.event_type !== "round_handoff"));
});

test("execution paths and handoff metrics retain the complete projected round", () => {
  const toolEvents = Array.from({ length: 13 }, (_, index) => operationEvent({
    id: `search:${index + 1}`,
    tool_use_id: `tool:search:${index + 1}`,
    tool_name: "WebSearch",
    kind: "web_research",
    surface: "web",
    target: `query ${index + 1}`,
    updated_at: now - 20 + index,
  }));
  const summary = operationEvent({
    id: "summary:long-round",
    tool_use_id: null,
    tool_name: null,
    kind: "round_summary",
    surface: "summary",
    summary: "调研完成。",
    updated_at: now,
  });
  const mirroredWorkspaceEvent = operationEvent({
    id: "workspace:search:1",
    tool_use_id: toolEvents[0].tool_use_id,
    tool_name: "workspace_event",
    kind: "artifact_update",
    surface: "workspace",
    target: "research-notes.md",
    updated_at: now - 1,
  });
  const snapshot = operationSnapshot(summary, [...toolEvents, mirroredWorkspaceEvent, summary]);
  const desktop = planOperationDesktop({ event: summary, snapshot });
  const handoff = desktop.windows.find((window) => window.kind === "handoff");
  const markup = renderToStaticMarkup(createElement(HandoffSurface, {
    event: summary,
    evidence: [],
    handoffSummary: handoff?.payload.handoff_summary,
    relatedEvents: handoff?.payload.related_events ?? [],
    snapshot,
  }));

  assert.equal(collectNarrativeEvents(summary, snapshot).length, 15);
  assert.equal(handoff?.payload.related_events?.length, 15);
  assert.match(markup, /13 个工具步骤/);
});

test("a new round and a new Session never inherit prior desktop events", () => {
  const oldSummary = operationEvent({
    id: "summary:old",
    kind: "round_summary",
    surface: "summary",
    target: "old.html",
    updated_at: now - 20,
  });
  const restored = operationSnapshot(oldSummary, [oldSummary]);
  const nextRound = operationEvent({
    id: "round:new:idle",
    kind: "plan_update",
    surface: "conversation",
    phase: "running",
    round_id: "round:new",
    tool_use_id: null,
    tool_name: null,
    target: null,
    updated_at: now,
  });
  const projected = operationSnapshot(nextRound, [nextRound]);
  const mergedRound = mergeOperationStageSnapshotsForRestore(restored, projected);

  assert.deepEqual(mergedRound.events.map((event) => event.round_id), ["round:new"]);
  assert.equal(planOperationDesktop({ event: nextRound, snapshot: mergedRound }).windows.length, 0);

  const newSession = {
    ...projected,
    key: "session:new",
    session_key: "session:new",
  };
  const mergedSession = mergeOperationStageSnapshotsForRestore(restored, newSession);
  assert.equal(mergedSession.key, "session:new");
  assert.deepEqual(mergedSession.events.map((event) => event.id), [nextRound.id]);
});

test("failed summaries use the same compact handoff with truthful failure copy", () => {
  const failed = operationEvent({
    id: "summary:failed",
    kind: "round_summary",
    surface: "summary",
    phase: "error",
    summary: "构建命令退出，页面没有完成。",
    updated_at: now,
  });
  const snapshot = operationSnapshot(failed, [failed]);
  const desktop = planOperationDesktop({ event: failed, snapshot });
  const handoff = desktop.windows.find((window) => window.kind === "handoff");
  const markup = renderToStaticMarkup(createElement(HandoffSurface, {
    event: failed,
    evidence: [],
    handoffSummary: handoff?.payload.handoff_summary,
    relatedEvents: [failed],
    snapshot,
  }));

  assert.ok(handoff);
  assert.equal(handoff.phase, "focused");
  assert.match(markup, /执行未完成/);
  assert.match(markup, /需要回看/);
});
