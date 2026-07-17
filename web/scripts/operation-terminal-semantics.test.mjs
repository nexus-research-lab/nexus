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

const { parseTerminalResult } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/terminal-result-model.ts",
);
const { buildTerminalSession } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/terminal-session-model.ts",
);
const { TerminalSession } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/terminal-session.tsx",
);
const { collectTerminalSessionEvents } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-terminal-session-events.ts",
);

const now = Date.now();

function bashEvent({
  id,
  command,
  phase = "done",
  result,
  shellId = null,
  timestamp,
}) {
  return {
    id,
    session_key: "session:terminal-test",
    round_id: "round:terminal-test",
    agent_id: "agent:test",
    message_id: `message:${id}`,
    tool_use_id: `tool:${id}`,
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    phase,
    title: "运行命令",
    target: command,
    input_preview: { command },
    result_preview: result ?? (shellId
      ? { content: { task_id: shellId }, is_error: false }
      : null),
    duration_ms: null,
    started_at: timestamp,
    updated_at: timestamp,
  };
}

function killShellEvent({ id, shellId = null, timestamp }) {
  return {
    id,
    session_key: "session:terminal-test",
    round_id: "round:terminal-test",
    agent_id: "agent:test",
    message_id: `message:${id}`,
    tool_use_id: `tool:${id}`,
    tool_name: "KillShell",
    kind: "command_stop",
    surface: "terminal",
    phase: "done",
    title: "终止命令",
    target: shellId ?? "KillShell",
    input_preview: shellId ? { shell_id: shellId } : {},
    result_preview: shellId
      ? { content: { task_id: shellId, message: `Stopped ${shellId}` }, is_error: false }
      : { content: "Stop request completed", is_error: false },
    duration_ms: null,
    started_at: timestamp,
    updated_at: timestamp,
  };
}

test("Bash output parser preserves stdout, stderr, exit code, and wrapping", () => {
  const parsed = parseTerminalResult({
    content: {
      stdout: "first\nsecond\n",
      stderr: "warning\n",
      exit_code: 7,
    },
    is_error: true,
  });

  assert.deepEqual(parsed.stdout, ["first", "second"]);
  assert.deepEqual(parsed.stderr, ["warning"]);
  assert.equal(parsed.exitCode, 7);
  assert.deepEqual(
    parsed.rows.map(({ stream, text }) => [stream, text]),
    [["stdout", "first"], ["stdout", "second"], ["stderr", "warning"]],
  );
});

test("failed and incomplete Bash metadata stays truthful without unknown placeholders", () => {
  const failed = bashEvent({
    id: "failed",
    command: "cat missing.txt",
    phase: "error",
    result: { content: { stderr: "missing.txt", exit_code: 23 }, is_error: true },
    timestamp: now,
  });
  const incomplete = bashEvent({
    id: "incomplete",
    command: "echo ok",
    result: { content: "ok", is_error: false },
    timestamp: now + 1,
  });
  const [failedEntry] = buildTerminalSession({ event: failed, relatedEvents: [] }).entries;
  const [incompleteEntry] = buildTerminalSession({ event: incomplete, relatedEvents: [] }).entries;

  assert.equal(failedEntry.statusLabel, "退出 23");
  assert.equal(failedEntry.statusTone, "error");
  assert.equal(failedEntry.durationLabel, null);
  assert.equal(failedEntry.cwdLabel, null);
  assert.equal(incompleteEntry.statusLabel, "已完成");
  assert.equal(incompleteEntry.durationLabel, null);
});

test("background Bash remains active from its real shell id", () => {
  const background = bashEvent({
    id: "background",
    command: "pnpm dev",
    shellId: "shell-background",
    timestamp: now,
  });
  const session = buildTerminalSession({ event: background, relatedEvents: [] });

  assert.equal(session.hasActiveProcess, true);
  assert.equal(session.entries[0].shellId, "shell-background");
  assert.equal(session.entries[0].statusLabel, "后台运行中");
  assert.equal(session.entries[0].durationLabel, null);
});

test("KillShell attaches only to the command with the exact shell id", () => {
  const first = bashEvent({
    id: "background-first",
    command: "pnpm dev:first",
    shellId: "shell-first",
    timestamp: now,
  });
  const second = bashEvent({
    id: "background-second",
    command: "pnpm dev:second",
    shellId: "shell-second",
    timestamp: now + 1,
  });
  const stopSecond = killShellEvent({
    id: "stop-second",
    shellId: "shell-second",
    timestamp: now + 2,
  });
  const session = buildTerminalSession({
    event: stopSecond,
    relatedEvents: [first, second, stopSecond],
  });

  assert.equal(session.entries.length, 2);
  assert.equal(session.entries[0].statusLabel, "后台运行中");
  assert.equal(session.entries[0].controls.length, 0);
  assert.equal(session.entries[1].statusLabel, "已终止");
  assert.equal(session.entries[1].controls[0].targetLabel, "shell-second");
});

test("KillShell without a shell id stays independent and never claims the previous command", () => {
  const background = bashEvent({
    id: "background-unrelated",
    command: "pnpm dev",
    shellId: "shell-unrelated",
    timestamp: now,
  });
  const untargetedStop = killShellEvent({
    id: "stop-untargeted",
    timestamp: now + 1,
  });
  const session = buildTerminalSession({
    event: untargetedStop,
    relatedEvents: [background, untargetedStop],
  });

  assert.equal(session.entries.length, 2);
  assert.equal(session.entries[0].command, "pnpm dev");
  assert.equal(session.entries[0].statusLabel, "后台运行中");
  assert.equal(session.entries[0].controls.length, 0);
  assert.equal(session.entries[1].command, null);
  assert.equal(session.entries[1].controls.length, 1);
  assert.equal(session.entries[1].controls[0].targetLabel, null);

  const collected = collectTerminalSessionEvents(
    untargetedStop,
    {
      key: "terminal-test",
      session_key: "session:terminal-test",
      active_event: untargetedStop,
      events: [background, untargetedStop],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now + 1,
    },
    [untargetedStop],
  );
  assert.deepEqual(collected.map((event) => event.id), ["stop-untargeted"]);
});

test("terminal view omits unavailable cwd, exit code, duration, and KillShell target", () => {
  const incomplete = bashEvent({
    id: "render-incomplete",
    command: "echo ok",
    result: { content: "ok", is_error: false },
    timestamp: now,
  });
  const untargetedStop = killShellEvent({
    id: "render-stop-untargeted",
    timestamp: now + 1,
  });
  const markup = [
    renderToStaticMarkup(createElement(TerminalSession, { event: incomplete, relatedEvents: [] })),
    renderToStaticMarkup(createElement(TerminalSession, { event: untargetedStop, relatedEvents: [] })),
  ].join("\n");

  assert.doesNotMatch(markup, /cwd \?/);
  assert.doesNotMatch(markup, /退出码未知/);
  assert.doesNotMatch(markup, /耗时未知/);
  assert.doesNotMatch(markup, /目标未知/);
  assert.match(markup, /已完成/);
  assert.match(markup, /KillShell · 终止请求已完成/);
});
