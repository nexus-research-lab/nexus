import { useMemo, useState } from "react";
import { createRoot } from "react-dom/client";

import "@/app/globals.css";
import { OperationStageDesktop } from "@/features/conversation/operation/stage/operation-stage-desktop";
import { OperationStageMotionStyles } from "@/features/conversation/operation/operation-stage-motion-styles";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "@/features/conversation/operation/operation-types";
import type { OperationRuntimeEvent } from "@/features/conversation/operation/operation-runtime-types";
import { applyTheme, detectInitialTheme } from "@/shared/theme/theme-context";
import type { WorkspaceActivityItem } from "@/types/app/workspace-live";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

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

const edited_html_content = html_content.replace(
  "<h1>Gomoku</h1>",
  "<h1>Gomoku · Nexus Stage</h1>",
);

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

const edit_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "diff", label: "修改", value: "gomoku.html" },
    { type: "status", label: "保存", value: "写入中" },
  ],
  id: "tool-edit-gomoku",
  kind: "workspace_edit",
  message_id: "message-assistant",
  phase: "running",
  round_id,
  session_key,
  surface: "editor",
  target: "gomoku.html",
  title: "修改五子棋标题",
  tool_name: "Edit",
  tool_use_id: "tool-edit",
  input_preview: {
    file_path: "gomoku.html",
    old_string: "<h1>Gomoku</h1>",
    new_string: "<h1>Gomoku · Nexus Stage</h1>",
  },
  summary: "打开 gomoku.html，将页面标题更新为 Nexus Stage 版本。",
  updated_at: now - 6_600,
};

const read_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "file", label: "读取", value: "gomoku.html" },
  ],
  id: "tool-read-gomoku",
  kind: "workspace_read",
  message_id: "message-assistant",
  phase: "running",
  round_id,
  session_key,
  surface: "editor",
  target: "gomoku.html",
  title: "读取五子棋页面",
  tool_name: "Read",
  tool_use_id: "tool-read",
  input_preview: {
    file_path: "gomoku.html",
  },
  result_preview: html_content,
  summary: "打开文件并读取当前内容。",
  updated_at: now - 7_300,
};

const finder_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "file", label: "搜索", value: "web/src/**/*.tsx" },
    { type: "status", label: "命中", value: "operation-stage-preview.tsx" },
  ],
  id: "tool-grep-stage",
  kind: "workspace_search",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "workspace",
  target: "web/src/**/*.tsx",
  title: "搜索舞台入口",
  tool_name: "Grep",
  tool_use_id: "tool-grep",
  input_preview: {
    pattern: "OperationStageDesktop",
    path: "web/src",
    glob: "**/*.tsx",
  },
  result_preview: [
    "web/src/dev/operation-stage-preview.tsx",
    "web/src/features/conversation/operation/stage/operation-stage-desktop.tsx",
  ],
  summary: "在工作区搜索舞台桌面入口。",
  updated_at: now - 7_900,
};

const task_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "task", label: "计划", value: "3/5" },
    { type: "status", label: "当前", value: "运行验证命令" },
  ],
  id: "tool-todo-pomodoro",
  kind: "plan_update",
  message_id: "message-assistant",
  phase: "running",
  round_id,
  session_key,
  surface: "task",
  target: "番茄钟实现计划",
  title: "更新执行计划",
  tool_name: "TodoWrite",
  tool_use_id: "tool-todo",
  input_preview: {
    todos: [
      { activeForm: "正在梳理需求", content: "梳理需求", status: "completed" },
      { activeForm: "正在创建页面", content: "创建 HTML/CSS/JS", status: "completed" },
      { activeForm: "正在运行验证", content: "运行验证命令", status: "in_progress" },
      { activeForm: "正在打开预览", content: "打开浏览器预览", status: "pending" },
      { activeForm: "正在整理交付", content: "交付结果", status: "pending" },
    ],
  },
  result_preview: "计划已更新",
  summary: "任务 App 展示计划和当前执行项。",
  updated_at: now - 7_600,
};

const subtask_started_event: NexusOperationEvent = {
  agent_id,
  evidence: [{ type: "task", label: "task", value: "task-stage-a12" }],
  id: "system-task-started-stage-a12",
  kind: "task_delegate",
  message_id: "system-task-started-stage-a12",
  phase: "running",
  round_id,
  session_key,
  surface: "task",
  target: "task-stage-a12",
  title: "检查番茄钟交互",
  tool_name: "Task",
  tool_use_id: "tool-task-stage-a12",
  input_preview: {
    description: "检查番茄钟交互",
    prompt: "检查开始、暂停、重置和倒计时归零后的交互状态。",
    task_id: "task-stage-a12",
  },
  summary: "子任务已开始。",
  updated_at: now - 7_300,
};

const subtask_progress_event: NexusOperationEvent = {
  ...subtask_started_event,
  id: "task-progress-stage-a12",
  kind: "task_progress",
  message_id: "message-task-progress-stage-a12",
  title: "检查番茄钟交互",
  tool_name: "TaskOutput",
  input_preview: {
    description: "检查番茄钟交互",
    last_tool_name: "Read",
    status: "running",
    task_id: "task-stage-a12",
    usage: { duration_ms: 8_400, tool_uses: 3, total_tokens: 1_286 },
  },
  result_preview: {
    description: "检查番茄钟交互",
    last_tool_name: "Read",
    task_id: "task-stage-a12",
    usage: { duration_ms: 8_400, tool_uses: 3, total_tokens: 1_286 },
  },
  summary: "正在检查重置逻辑。",
  updated_at: now - 7_000,
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

const knowledge_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "skill", label: "技能", value: "frontend-design" },
    { type: "status", label: "上下文", value: "组件交互规则" },
  ],
  id: "tool-skill-frontend",
  kind: "context_read",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "knowledge",
  target: "frontend-design",
  title: "读取技能上下文",
  tool_name: "Skill",
  tool_use_id: "tool-skill",
  input_preview: {
    skill_name: "frontend-design",
  },
  result_preview: [
    "Use actual app UI as the first screen.",
    "Controls must be interactive and fit their containers.",
  ],
  summary: "Nexus 知识窗口展示技能上下文，而不是 JSON 卡片。",
  updated_at: now - 5_950,
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
  tool_name: "WebSearch",
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

const web_fetch_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "url", label: "抓取", value: "https://example.com/pomodoro" },
    { type: "status", label: "内容", value: "页面正文片段" },
  ],
  id: "tool-web-fetch",
  kind: "web_research",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "web",
  target: "https://example.com/pomodoro",
  title: "抓取网页资料",
  tool_name: "WebFetch",
  tool_use_id: "tool-web-fetch",
  input_preview: {
    url: "https://example.com/pomodoro",
    prompt: "提取番茄钟交互要点",
  },
  result_preview: [
    "Pomodoro timers alternate focus intervals and short breaks.",
    "Users expect start, pause, reset, and session counters.",
  ],
  summary: "Navi 显示真实 URL 和抓取结果片段。",
  updated_at: now - 5_500,
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
    content: "Opening gomoku.html\nNavi preview launched\n",
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
  duration_ms: 1840,
  summary: "终端显示真实命令输出和退出码。",
  updated_at: now - 4_800,
};

const background_terminal_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "terminal", label: "运行", value: "pnpm dev" },
    { type: "status", label: "进程", value: "shell-42" },
  ],
  id: "tool-terminal-background",
  kind: "command_run",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "terminal",
  target: "pnpm dev",
  title: "运行开发服务",
  tool_name: "Bash",
  tool_use_id: "tool-terminal-background",
  input_preview: {
    command: "pnpm dev",
    cwd: "/private/tmp/nexus-operation-stage",
    run_in_background: true,
  },
  result_preview: {
    content: {
      message: "Command started in background",
      task_id: "shell-42",
    },
    is_error: false,
  },
  duration_ms: 920,
  updated_at: now - 4_700,
};

const kill_shell_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "terminal", label: "终止", value: "shell-42" },
    { type: "status", label: "进程", value: "pnpm dev" },
  ],
  id: "tool-kill-shell",
  kind: "command_stop",
  message_id: "message-assistant",
  phase: "done",
  round_id,
  session_key,
  surface: "terminal",
  target: "shell-42",
  title: "终止运行中的命令",
  tool_name: "KillShell",
  tool_use_id: "tool-kill-shell",
  input_preview: {
    shell_id: "shell-42",
  },
  result_preview: {
    content: {
      command: "pnpm dev",
      message: "Successfully stopped task: shell-42",
      task_id: "shell-42",
      task_type: "local_bash",
    },
    is_error: false,
  },
  duration_ms: 130,
  summary: "终止仍在运行的 shell 进程。",
  updated_at: now - 4_500,
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
  permission_interaction_mode: "permission",
  permission_request_id: "permission-open-gomoku",
  summary: "允许 Nexus 通过终端打开生成的五子棋 HTML 页面。",
  updated_at: now - 5_800,
};

const question_event: NexusOperationEvent = {
  agent_id,
  evidence: [
    { type: "permission", label: "需要回答", value: "选择番茄钟默认节奏" },
    { type: "task", label: "协作点", value: "等待用户偏好后继续生成界面" },
  ],
  id: "question-pomodoro-preference",
  kind: "plan_update",
  message_id: "message-assistant",
  phase: "waiting",
  round_id,
  session_key,
  surface: "conversation",
  target: "番茄钟默认节奏",
  title: "需要用户回答",
  tool_name: "AskUserQuestion",
  tool_use_id: "tool-question",
  input_preview: {
    questions: [
      {
        header: "节奏",
        question: "番茄钟默认时长用哪一组？",
        options: [
          { label: "25/5", description: "标准番茄钟，适合多数任务。" },
          { label: "50/10", description: "长专注块，适合开发或写作。" },
        ],
      },
      {
        header: "提醒",
        multi_select: true,
        question: "需要哪些提醒方式？",
        options: [
          { label: "声音提示" },
          { label: "页面闪烁" },
          { label: "桌面通知" },
        ],
      },
    ],
  },
  permission_interaction_mode: "question",
  permission_request_id: "question-pomodoro-preference",
  summary: "智能体需要确认番茄钟默认节奏和提醒方式。",
  updated_at: now - 5_400,
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

const edit_workspace_item: WorkspaceActivityItem = {
  agent_id,
  diff_stats: {
    additions: 1,
    changed_lines: 1,
    deletions: 1,
  },
  event_type: "file_write_delta",
  id: "workspace-gomoku-html-edit",
  live_content: edited_html_content,
  path: "gomoku.html",
  session_key,
  source: "agent",
  status: "writing",
  tool_use_id: "tool-edit",
  updated_at: now - 6_400,
  version: 2,
};

const PREVIEW_STEPS = [
  { id: "idle", label: "空桌面", event: live_event, events: [live_event] },
  { id: "tasks-plan", label: "任务计划", event: task_event, events: [live_event, task_event] },
  { id: "tasks-run", label: "子任务", event: subtask_progress_event, events: [live_event, task_event, subtask_started_event, subtask_progress_event] },
  { id: "finder", label: "文件", event: finder_event, events: [live_event, task_event, finder_event] },
  { id: "write", label: "创建文件", event: write_event, events: [live_event, write_event] },
  { id: "edit", label: "修改文件", event: edit_event, events: [live_event, write_event, edit_event] },
  { id: "read", label: "读取文件", event: read_event, events: [live_event, write_event, read_event] },
  { id: "knowledge", label: "知识", event: knowledge_event, events: [live_event, knowledge_event] },
  { id: "tool", label: "工具窗口", event: generic_tool_followup_event, events: [live_event, generic_tool_event, generic_tool_followup_event] },
  { id: "search", label: "浏览搜索", event: web_search_event, events: [live_event, web_search_event] },
  { id: "fetch", label: "网页抓取", event: web_fetch_event, events: [live_event, web_search_event, web_fetch_event] },
  { id: "permission", label: "权限确认", event: permission_event, events: [live_event, write_event, permission_event] },
  { id: "question", label: "用户问题", event: question_event, events: [live_event, task_event, question_event] },
  { id: "terminal", label: "终端输出", event: terminal_event, events: [live_event, terminal_event] },
  { id: "kill-shell", label: "终止命令", event: kill_shell_event, events: [live_event, background_terminal_event, kill_shell_event] },
  { id: "open", label: "打开预览", event: open_event, events: [live_event, write_event, open_event] },
  { id: "done", label: "完成收束", event: summary_event, events: [live_event, write_event, open_event, summary_event] },
  {
    id: "mixed",
    label: "混合任务",
    event: summary_event,
    events: [
      live_event,
      task_event,
      finder_event,
      read_event,
      knowledge_event,
      write_event,
      edit_event,
      terminal_event,
      kill_shell_event,
      web_search_event,
      web_fetch_event,
      permission_event,
      question_event,
      open_event,
      summary_event,
    ],
  },
] as const;

type PreviewStepId = (typeof PREVIEW_STEPS)[number]["id"];

function build_snapshot(events: NexusOperationEvent[], active_event: NexusOperationEvent): NexusOperationSnapshot {
  const workspace_events = [
    ...(events.some((event) => (
    event.id === write_event.id ||
    event.id === read_event.id ||
    event.id === open_event.id ||
    event.id === summary_event.id ||
    event.id === permission_event.id
  ))
    ? [workspace_item]
    : []),
    ...(events.some((event) => event.id === edit_event.id) ? [edit_workspace_item] : []),
  ];

  return {
    active_event,
    events,
    key: session_key,
    recent_evidence: events.flatMap((event) => event.evidence ?? []).slice(-8),
    runtime_events: build_preview_runtime_events(events, workspace_events),
    session_key,
    updated_at: active_event.updated_at,
    workspace_events,
  };
}

function build_preview_runtime_events(
  events: NexusOperationEvent[],
  workspace_events: WorkspaceActivityItem[],
): OperationRuntimeEvent[] {
  const runtime_events = events.flatMap((event): OperationRuntimeEvent[] => {
    if (event.kind === "round_summary") {
      return [{
        agent_id: event.agent_id,
        artifact: {
          kind: "handoff",
          preview: event.result_preview ?? event.summary ?? null,
        },
        event_type: "round_handoff",
        id: `runtime:${event.id}:handoff`,
        input: event.input_preview ?? null,
        phase: event.phase,
        result: event.result_preview ?? event.summary ?? null,
        round_id: event.round_id,
        session_key: event.session_key,
        source_event_id: event.id,
        timestamp: event.updated_at,
        tool_name: event.tool_name ?? "RoundSummary",
        tool_use_id: event.tool_use_id ?? null,
      }];
    }

    if (event.permission_request_id) {
      return [{
        agent_id: event.agent_id,
        delta: {
          summary: event.summary,
        },
        event_type: "permission_request",
        id: `runtime:permission:${event.permission_request_id}`,
        input: event.input_preview ?? null,
        permission_interaction_mode: event.permission_interaction_mode ?? "permission",
        permission_request_id: event.permission_request_id,
        phase: "waiting",
        round_id: event.round_id,
        session_key: event.session_key,
        source_event_id: event.id,
        timestamp: event.updated_at,
        tool_name: event.tool_name,
        tool_use_id: event.tool_use_id ?? null,
      }];
    }

    if (!event.tool_use_id) {
      return [];
    }

    return [
      {
        agent_id: event.agent_id,
        event_type: "tool_start",
        id: `runtime:${event.id}:start`,
        input: event.input_preview ?? null,
        phase: event.phase === "waiting" ? "waiting" : "running",
        round_id: event.round_id,
        session_key: event.session_key,
        source_event_id: event.id,
        timestamp: Math.max(0, event.updated_at - 240),
        tool_name: event.tool_name,
        tool_use_id: event.tool_use_id,
      },
      {
        agent_id: event.agent_id,
        event_type: "tool_end",
        id: `runtime:${event.id}:end`,
        input: event.input_preview ?? null,
        phase: event.phase,
        result: event.result_preview ?? event.summary ?? null,
        round_id: event.round_id,
        session_key: event.session_key,
        source_event_id: event.id,
        timestamp: event.updated_at,
        tool_name: event.tool_name,
        tool_use_id: event.tool_use_id,
      },
    ];
  });

  runtime_events.push(...workspace_events.map((item) => ({
    agent_id: item.agent_id,
    artifact: {
      kind: "html" as const,
      live_content: item.live_content ?? null,
      path: item.path,
      status: item.status,
      diff_stats: item.diff_stats ?? null,
    },
    delta: {
      event_type: item.event_type,
      status: item.status,
    },
    event_type: "artifact_update" as const,
    id: `runtime:workspace:${item.id}`,
    input: {
      path: item.path,
      status: item.status,
    },
    phase: "done" as const,
    round_id,
    session_key: item.session_key ?? session_key,
    timestamp: item.updated_at,
    tool_name: "workspace_event",
    tool_use_id: item.tool_use_id ?? null,
  })));

  return runtime_events.sort((left, right) => left.timestamp - right.timestamp);
}

export function OperationStagePreview() {
  const [step_id, set_step_id] = useState<PreviewStepId>(() => read_preview_step_id());
  const [last_permission_response, set_last_permission_response] = useState<PermissionDecisionPayload | null>(null);
  const step = PREVIEW_STEPS.find((item) => item.id === step_id) ?? PREVIEW_STEPS[0];
  const snapshot = useMemo(() => build_snapshot([...step.events], step.event), [step]);
  const handle_permission_response = (payload: PermissionDecisionPayload) => {
    set_last_permission_response(payload);
    return true;
  };
  const select_step = (next_step_id: PreviewStepId) => {
    set_step_id(next_step_id);
    set_last_permission_response(null);
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
          <h1 className="text-[18px] font-black tracking-normal">Agent OS 桌面检查</h1>
        </div>
        <div className="flex max-w-[74vw] flex-wrap items-center justify-end gap-1.5 rounded-[18px] border border-white/70 bg-white/70 p-1 shadow-[0_16px_42px_rgba(18,28,42,0.10)] backdrop-blur-xl">
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
      {last_permission_response ? (
        <div className="-mt-1 mb-2 self-end rounded-full border border-white/70 bg-white/78 px-3 py-1 text-[11px] font-bold text-(--text-muted) shadow-[0_10px_28px_rgba(18,28,42,0.10)]">
          mock permission response: {last_permission_response.decision}
          {last_permission_response.user_answers?.length ? ` / ${last_permission_response.user_answers.length} answers` : ""}
        </div>
      ) : null}
      <section className="flex min-h-[620px] flex-1 overflow-hidden rounded-[24px] border border-white/70 bg-white/46 p-2 shadow-[0_28px_90px_rgba(18,28,42,0.16)]">
        <OperationStageDesktop
          event={step.event}
          headerAction={(
            <button aria-label="退出操作舞台预览" onClick={() => undefined} type="button">
              退出
            </button>
          )}
          onPermissionResponse={handle_permission_response}
          snapshot={snapshot}
        />
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

applyTheme(detectInitialTheme());

const root = document.getElementById("root");
if (!root) {
  throw new Error("Root container #root not found.");
}

createRoot(root).render(<OperationStagePreview />);
