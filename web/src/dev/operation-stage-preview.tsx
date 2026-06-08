import { useMemo, useState } from "react";
import { createRoot } from "react-dom/client";

import "@/app/globals.css";
import { OperationStageDesktop } from "@/features/conversation/operation/stage/operation-stage-desktop";
import { OperationStageMotionStyles } from "@/features/conversation/operation/operation-stage-motion-styles";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "@/features/conversation/operation/operation-types";
import { apply_theme, detect_initial_theme } from "@/shared/theme/theme-context";
import type { WorkspaceActivityItem } from "@/types/app/workspace-live";

const now = Date.now();
const round_id = "round-preview-gomoku";
const session_key = "room-session:stage-preview";
const agent_id = "stage-preview-agent";

const html_content = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Gomoku</title>
  <style>
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f8fafc; font-family: ui-sans-serif, system-ui; }
    main { width: min(860px, 92vw); }
    h1 { margin: 0 0 18px; color: #172033; }
    .board { display: grid; grid-template-columns: repeat(15, 1fr); aspect-ratio: 1; border: 2px solid #8b5e34; background: #d7a85f; box-shadow: 0 24px 60px #18284222; }
    .cell { border: 1px solid #9a6a3b; display: grid; place-items: center; }
    .stone { width: 62%; aspect-ratio: 1; border-radius: 999px; box-shadow: inset 0 2px 4px #ffffff55, 0 5px 10px #18284224; }
    .black { background: #172033; }
    .white { background: #f8fafc; }
  </style>
</head>
<body>
  <main>
    <h1>Gomoku</h1>
    <section class="board">
      ${Array.from({ length: 225 }).map((_, index) => {
        const is_black = [112, 113, 114, 128].includes(index);
        const is_white = [97, 98, 127].includes(index);
        return `<div class="cell">${is_black || is_white ? `<span class="stone ${is_black ? "black" : "white"}"></span>` : ""}</div>`;
      }).join("")}
    </section>
  </main>
</body>
</html>`;

const live_event: NexusOperationEvent = {
  agent_id,
  id: "live-round-preview",
  kind: "plan_update",
  message_id: "message-user",
  phase: "running",
  round_id,
  session_key,
  surface: "conversation",
  title: "Nexus 桌面",
  summary: "用户请求写一个五子棋小游戏，等待第一个工具调用。",
  updated_at: now - 12_000,
};

const write_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "file", label: "创建", value: "gomoku.html" },
    { type: "artifact", label: "HTML", value: "内嵌预览已准备" },
  ],
  id: "tool-write-gomoku",
  kind: "workspace_edit",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "editor",
  target: "gomoku.html",
  title: "创建五子棋页面",
  tool_name: "Write",
  tool_use_id: "tool-write",
  input_preview: {
    file_path: "gomoku.html",
    content: html_content,
  },
  result_preview: "created gomoku.html",
  summary: "写入一个可以直接打开的五子棋 HTML 页面。",
  updated_at: now - 8_000,
};

const generic_tool_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "skill", label: "工具", value: "Context7" },
    { type: "status", label: "结果", value: "3 条文档片段" },
  ],
  id: "tool-generic-docs",
  kind: "unknown",
  message_id: "message-assistant",
  phase: "running",
  round_id,
  session_key,
  surface: "fallback",
  target: "React useEffect cleanup",
  title: "查询文档",
  tool_name: "Context7",
  tool_use_id: "tool-context7",
  input_preview: {
    library: "react",
    query: "useEffect cleanup",
  },
  result_preview: {
    snippets: [
      "Effect cleanup runs before the next effect and during unmount.",
      "Return a cleanup function from useEffect when subscribing to external systems.",
      "Abort fetches or ignore stale responses to prevent updates after unmount.",
    ],
  },
  summary: "查询 React 文档，提取 useEffect 清理函数相关片段。",
  updated_at: now - 6_500,
};

const generic_tool_followup_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "status", label: "校验", value: "cleanup 规则已记录" },
    { type: "status", label: "结果", value: "2 条执行建议" },
  ],
  id: "tool-generic-cleanup",
  kind: "unknown",
  message_id: "message-assistant",
  phase: "running",
  round_id,
  session_key,
  surface: "fallback",
  target: "useEffect cleanup checklist",
  title: "整理规则",
  tool_name: "Rules",
  tool_use_id: "tool-rules",
  input_preview: {
    context: "React useEffect cleanup",
    mode: "checklist",
  },
  result_preview: {
    checklist: [
      "取消订阅或移除监听器",
      "清理计时器并忽略过期异步结果",
    ],
  },
  summary: "把文档片段整理成可执行检查清单。",
  updated_at: now - 5_900,
};

const web_search_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "url", label: "搜索", value: "nexus mac desktop stage" },
    { type: "status", label: "结果", value: "3 条网页摘要" },
  ],
  id: "tool-web-search",
  kind: "web_research",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "web",
  target: "nexus mac desktop stage",
  title: "搜索桌面交互参考",
  tool_name: "web_search",
  tool_use_id: "tool-web-search",
  input_preview: {
    query: "nexus mac desktop stage",
  },
  result_preview: [
    "https://developer.apple.com/design/human-interface-guidelines/windows",
    "macOS window layouts emphasize one focused task with persistent toolbar controls.",
    "Stage Manager keeps recent app windows as compact previews on the side.",
  ],
  summary: "搜索 macOS 窗口、Stage Manager 和应用工具栏的交互参考。",
  updated_at: now - 5_700,
};

const open_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "terminal", label: "运行", value: "open gomoku.html" },
    { type: "url", label: "预览", value: "gomoku.html" },
  ],
  id: "tool-open-gomoku",
  kind: "command_run",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "terminal",
  target: "open gomoku.html",
  title: "打开预览",
  tool_name: "Bash",
  tool_use_id: "tool-open",
  input_preview: {
    command: "open gomoku.html",
  },
  result_preview: {
    content: "Opening gomoku.html\nSafari preview launched\n",
    exit_code: 0,
    is_error: false,
  },
  summary: "在本地浏览器窗口打开生成的页面。",
  updated_at: now - 5_000,
};

const terminal_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "terminal", label: "运行", value: "printf \"1\\n2\\n\" && pwd" },
    { type: "status", label: "退出", value: "0" },
  ],
  id: "tool-terminal-output",
  kind: "command_run",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "terminal",
  target: "printf \"1\\n2\\n\" && pwd",
  title: "运行验证命令",
  tool_name: "Bash",
  tool_use_id: "tool-terminal",
  input_preview: {
    command: "printf \"1\\n2\\n\" && pwd",
    cwd: "/private/tmp/nexus-operation-stage",
  },
  result_preview: {
    stdout: "1\n2\n/private/tmp/nexus-operation-stage\n",
    stderr: "",
    exit_code: 0,
    is_error: false,
  },
  summary: "终端显示真实命令输出和退出码。",
  updated_at: now - 4_800,
};

const permission_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "permission", label: "需要确认", value: "允许终端打开本地 HTML 预览" },
    { type: "terminal", label: "命令", value: "open gomoku.html" },
  ],
  id: "permission-open-gomoku",
  kind: "command_run",
  message_id: "message-assistant",
  phase: "waiting",
  round_id,
  session_key,
  surface: "terminal",
  target: "open gomoku.html",
  title: "需要确认",
  tool_name: "Bash",
  tool_use_id: "tool-open",
  input_preview: {
    command: "open gomoku.html",
  },
  summary: "允许 Nexus 通过终端打开生成的五子棋 HTML 页面。",
  updated_at: now - 5_800,
};

const summary_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "file", label: "产物", value: "gomoku.html" },
    { type: "terminal", label: "验证", value: "open gomoku.html" },
  ],
  id: "round-summary-gomoku",
  kind: "round_summary",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "summary",
  target: "gomoku.html",
  title: "五子棋小游戏已完成",
  result_preview: "已创建 gomoku.html，并通过浏览器预览打开。",
  summary: "产物已落到工作区，可继续打开或修改规则与样式。",
  updated_at: now - 1_000,
};

const workspace_item: WorkspaceActivityItem = {
  agent_id,
  event_type: "file_write_end",
  id: "workspace-gomoku-html",
  live_content: html_content,
  path: "gomoku.html",
  session_key,
  source: "agent",
  status: "updated",
  tool_use_id: "tool-write",
  updated_at: now - 7_000,
  version: 1,
};

const PREVIEW_STEPS = [
  { id: "idle", label: "空桌面", event: live_event, events: [live_event] },
  { id: "write", label: "创建文件", event: write_event, events: [live_event, write_event] },
  { id: "tool", label: "工具窗口", event: generic_tool_followup_event, events: [live_event, generic_tool_event, generic_tool_followup_event] },
  { id: "search", label: "浏览搜索", event: web_search_event, events: [live_event, web_search_event] },
  { id: "permission", label: "权限确认", event: permission_event, events: [live_event, write_event, permission_event] },
  { id: "terminal", label: "终端输出", event: terminal_event, events: [live_event, terminal_event] },
  { id: "open", label: "打开预览", event: open_event, events: [live_event, write_event, open_event] },
  { id: "done", label: "完成收束", event: summary_event, events: [live_event, write_event, open_event, summary_event] },
] as const;

type PreviewStepId = (typeof PREVIEW_STEPS)[number]["id"];

function build_snapshot(events: NexusOperationEvent[], active_event: NexusOperationEvent): NexusOperationSnapshot {
  return {
    active_event,
    events,
    key: session_key,
    recent_evidence: events.flatMap((event) => event.evidence ?? []).slice(-8),
    session_key,
    updated_at: active_event.updated_at,
    workspace_events: events.some((event) => event.id === write_event.id || event.id === open_event.id || event.id === summary_event.id)
      ? [workspace_item]
      : [],
  };
}

export function OperationStagePreview() {
  const [step_id, set_step_id] = useState<PreviewStepId>(() => read_preview_step_id());
  const step = PREVIEW_STEPS.find((item) => item.id === step_id) ?? PREVIEW_STEPS[0];
  const snapshot = useMemo(() => build_snapshot([...step.events], step.event), [step]);
  const select_step = (next_step_id: PreviewStepId) => {
    set_step_id(next_step_id);
    const url = new URL(window.location.href);
    url.searchParams.set("step", next_step_id);
    window.history.replaceState(null, "", url);
  };

  return (
    <main className="flex h-screen min-h-[720px] flex-col overflow-hidden bg-[rgb(236,240,245)] p-4 text-(--text-strong)">
      <OperationStageMotionStyles />
      <div className="mb-3 flex shrink-0 items-center justify-between gap-3">
        <div>
          <p className="text-[12px] font-black uppercase tracking-[0.18em] text-(--text-muted)">Operation Stage Preview</p>
          <h1 className="text-[18px] font-black tracking-normal">Mac 桌面叙事检查</h1>
        </div>
        <div className="flex items-center gap-2 rounded-full border border-white/70 bg-white/70 p-1 shadow-[0_16px_42px_rgba(18,28,42,0.10)] backdrop-blur-xl">
          {PREVIEW_STEPS.map((item) => (
            <button
              className={`rounded-full px-3 py-1.5 text-[12px] font-bold transition ${item.id === step.id ? "bg-[rgba(91,114,255,0.16)] text-[color:var(--primary)]" : "text-(--text-soft) hover:bg-white"}`}
              key={item.id}
              onClick={() => select_step(item.id)}
              type="button"
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>
      <section className="flex min-h-[620px] flex-1 overflow-hidden rounded-[24px] border border-white/70 bg-white/46 p-2 shadow-[0_28px_90px_rgba(18,28,42,0.16)]">
        <OperationStageDesktop event={step.event} snapshot={snapshot} />
      </section>
    </main>
  );
}

function read_preview_step_id(): PreviewStepId {
  const requested_step = new URLSearchParams(window.location.search).get("step");
  return PREVIEW_STEPS.some((item) => item.id === requested_step)
    ? requested_step as PreviewStepId
    : "idle";
}

apply_theme(detect_initial_theme());

const root = document.getElementById("root");
if (!root) {
  throw new Error("Root container #root not found.");
}

createRoot(root).render(<OperationStagePreview />);
