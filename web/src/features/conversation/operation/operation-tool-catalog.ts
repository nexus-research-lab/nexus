import type {
  OperationEvidence,
  OperationKind,
  OperationPhase,
  OperationSurface,
} from "./operation-types";
import { inferOperationToolProfile } from "./operation-tool-inference";

export type OperationActionKind =
  | "read"
  | "list"
  | "search"
  | "create"
  | "edit"
  | "run"
  | "stop"
  | "web_search"
  | "web_fetch"
  | "skill"
  | "task"
  | "task_progress"
  | "plan"
  | "question"
  | "summary"
  | "generic";

export interface OperationToolProfile {
  action: OperationActionKind;
  action_label: string;
  title: string;
  kind: OperationKind;
  surface: OperationSurface;
  target_keys: readonly string[];
  evidence_type: OperationEvidence["type"];
}

export const DEFAULT_TARGET_KEYS = [
  "command",
  "cmd",
  "query",
  "q",
  "url",
  "path",
  "target",
  "file",
  "file_path",
  "filename",
  "notebook_path",
  "pattern",
  "search_query",
  "description",
  "prompt",
  "task",
  "input",
  "mode",
] as const;

export const OPERATION_TOOL_PROFILES: Record<string, OperationToolProfile> = {
  Task: {
    action: "task",
    action_label: "委派",
    title: "委派子任务",
    kind: "task_delegate",
    surface: "task",
    target_keys: ["description", "prompt", "task"],
    evidence_type: "task",
  },
  TaskOutput: {
    action: "task_progress",
    action_label: "进度",
    title: "读取子任务输出",
    kind: "task_progress",
    surface: "task",
    target_keys: ["task_id", "description"],
    evidence_type: "task",
  },
  Bash: {
    action: "run",
    action_label: "运行",
    title: "运行命令",
    kind: "command_run",
    surface: "terminal",
    target_keys: ["command", "cmd", "description"],
    evidence_type: "terminal",
  },
  KillShell: {
    action: "stop",
    action_label: "停止",
    title: "终止命令",
    kind: "command_stop",
    surface: "terminal",
    target_keys: ["command", "cmd", "shell_id", "description"],
    evidence_type: "terminal",
  },
  Glob: {
    action: "list",
    action_label: "匹配",
    title: "匹配文件",
    kind: "workspace_inspect",
    surface: "workspace",
    target_keys: ["pattern", "path"],
    evidence_type: "file",
  },
  Grep: {
    action: "search",
    action_label: "搜索",
    title: "搜索内容",
    kind: "workspace_search",
    surface: "workspace",
    target_keys: ["pattern", "query", "path", "glob"],
    evidence_type: "file",
  },
  LS: {
    action: "list",
    action_label: "查看",
    title: "查看目录",
    kind: "workspace_inspect",
    surface: "workspace",
    target_keys: ["path"],
    evidence_type: "file",
  },
  Read: {
    action: "read",
    action_label: "读取",
    title: "读取文件",
    kind: "workspace_read",
    surface: "workspace",
    target_keys: ["file_path", "path", "file"],
    evidence_type: "file",
  },
  Edit: {
    action: "edit",
    action_label: "修改",
    title: "修改文件",
    kind: "workspace_edit",
    surface: "editor",
    target_keys: ["file_path", "path"],
    evidence_type: "diff",
  },
  MultiEdit: {
    action: "edit",
    action_label: "批改",
    title: "批量修改",
    kind: "workspace_edit",
    surface: "editor",
    target_keys: ["file_path", "path"],
    evidence_type: "diff",
  },
  Write: {
    action: "create",
    action_label: "创建",
    title: "创建/覆盖文件",
    kind: "workspace_edit",
    surface: "editor",
    target_keys: ["file_path", "path"],
    evidence_type: "diff",
  },
  NotebookEdit: {
    action: "edit",
    action_label: "修改",
    title: "编辑 Notebook",
    kind: "workspace_edit",
    surface: "editor",
    target_keys: ["notebook_path", "file_path", "path"],
    evidence_type: "diff",
  },
  WebFetch: {
    action: "web_fetch",
    action_label: "抓取",
    title: "抓取网页",
    kind: "web_research",
    surface: "web",
    target_keys: ["url", "prompt"],
    evidence_type: "url",
  },
  WebSearch: {
    action: "web_search",
    action_label: "搜索",
    title: "搜索网页",
    kind: "web_research",
    surface: "web",
    target_keys: ["query"],
    evidence_type: "url",
  },
  Skill: {
    action: "skill",
    action_label: "技能",
    title: "读取技能上下文",
    kind: "context_read",
    surface: "knowledge",
    target_keys: ["skill_name", "name", "description"],
    evidence_type: "skill",
  },
  TodoWrite: {
    action: "plan",
    action_label: "计划",
    title: "更新计划",
    kind: "plan_update",
    surface: "task",
    target_keys: ["todos", "items"],
    evidence_type: "status",
  },
  EnterPlanMode: {
    action: "plan",
    action_label: "规划",
    title: "进入规划模式",
    kind: "plan_update",
    surface: "task",
    target_keys: ["mode"],
    evidence_type: "status",
  },
  ExitPlanMode: {
    action: "plan",
    action_label: "执行",
    title: "退出规划模式",
    kind: "plan_update",
    surface: "task",
    target_keys: ["mode"],
    evidence_type: "status",
  },
  AskUserQuestion: {
    action: "question",
    action_label: "等待",
    title: "等待用户输入",
    kind: "human_gate",
    surface: "conversation",
    target_keys: ["question", "prompt"],
    evidence_type: "permission",
  },
};

export const FIELD_LABELS: Record<string, string> = {
  command: "命令",
  cmd: "命令",
  query: "搜索",
  q: "查询",
  url: "网址",
  path: "路径",
  target: "目标",
  file: "文件",
  file_path: "文件",
  filename: "文件",
  notebook_path: "Notebook",
  pattern: "模式",
  search_query: "搜索",
  glob: "范围",
  description: "说明",
  prompt: "提示",
  task: "任务",
  input: "输入",
  task_id: "任务",
  skill_name: "技能",
  name: "名称",
  mode: "模式",
  todos: "计划",
  items: "条目",
  question: "问题",
  shell_id: "进程",
};

export const PHASE_LABELS: Record<OperationPhase, string> = {
  queued: "排队中",
  running: "执行中",
  waiting: "等待确认",
  done: "已完成",
  error: "失败",
  cancelled: "已中断",
};

export function resolveOperationToolProfile(
  tool_name?: string | null,
  kind?: OperationKind,
  surface?: OperationSurface,
): OperationToolProfile {
  const normalized_tool_name = tool_name?.trim();
  if (normalized_tool_name && OPERATION_TOOL_PROFILES[normalized_tool_name]) {
    return OPERATION_TOOL_PROFILES[normalized_tool_name];
  }
  const inferred_profile = normalized_tool_name
    ? inferOperationToolProfile(normalized_tool_name, OPERATION_TOOL_PROFILES, DEFAULT_TARGET_KEYS)
    : null;
  if (inferred_profile) {
    return inferred_profile;
  }

  if (kind === "workspace_read") {
    return OPERATION_TOOL_PROFILES.Read;
  }
  if (kind === "workspace_search") {
    return OPERATION_TOOL_PROFILES.Grep;
  }
  if (kind === "workspace_inspect") {
    return OPERATION_TOOL_PROFILES.LS;
  }
  if (kind === "workspace_edit" && surface === "editor") {
    return {
      ...OPERATION_TOOL_PROFILES.Edit,
      title: "写入工作区文件",
    };
  }
  if (kind === "command_run") {
    return OPERATION_TOOL_PROFILES.Bash;
  }
  if (kind === "command_stop") {
    return OPERATION_TOOL_PROFILES.KillShell;
  }
  if (kind === "web_research") {
    return OPERATION_TOOL_PROFILES.WebFetch;
  }
  if (kind === "context_read") {
    return OPERATION_TOOL_PROFILES.Skill;
  }
  if (kind === "task_delegate" || kind === "task_progress") {
    return kind === "task_delegate" ? OPERATION_TOOL_PROFILES.Task : OPERATION_TOOL_PROFILES.TaskOutput;
  }
  if (kind === "human_gate") {
    return OPERATION_TOOL_PROFILES.AskUserQuestion;
  }
  if (kind === "plan_update") {
    return OPERATION_TOOL_PROFILES.TodoWrite;
  }
  if (kind === "round_summary") {
    return {
      action: "summary",
      action_label: "收口",
      title: "执行收口",
      kind: "round_summary",
      surface: "summary",
      target_keys: DEFAULT_TARGET_KEYS,
      evidence_type: "status",
    };
  }
  if (kind === "unknown" && surface === "conversation") {
    return {
      action: "generic",
      action_label: "Nexus",
      title: "桌面待命",
      kind: "unknown",
      surface: "conversation",
      target_keys: DEFAULT_TARGET_KEYS,
      evidence_type: "status",
    };
  }

  return {
    action: "generic",
    action_label: "工具",
    title: normalized_tool_name || "工具调用",
    kind: kind ?? "unknown",
    surface: surface ?? "fallback",
    target_keys: DEFAULT_TARGET_KEYS,
    evidence_type: "status",
  };
}

export function extractOperationInputValue(
  input: Record<string, unknown> | null | undefined,
  keys: readonly string[],
): { key: string; value: string } | null {
  if (!input) {
    return null;
  }

  for (const key of keys) {
    const value = input[key];
    const formatted = formatOperationValue(value);
    if (formatted) {
      return { key, value: formatted };
    }
  }
  return extract_nested_operation_input_value(input, keys);
}

function extract_nested_operation_input_value(
  input: Record<string, unknown>,
  keys: readonly string[],
): { key: string; value: string } | null {
  for (const collection_key of ["edits", "creates", "files"] as const) {
    const collection = input[collection_key];
    if (!Array.isArray(collection)) {
      continue;
    }
    for (const item of collection) {
      if (!item || typeof item !== "object" || Array.isArray(item)) {
        continue;
      }
      const record = item as Record<string, unknown>;
      for (const key of keys) {
        const formatted = formatOperationValue(record[key]);
        if (formatted) {
          return { key: `${collection_key}.${key}`, value: formatted };
        }
      }
    }
  }
  return null;
}

export function buildOperationInputRows(
  input: Record<string, unknown> | null | undefined,
  keys: readonly string[],
  limit = 4,
): Array<{ key: string; label: string; value: string }> {
  if (!input) {
    return [];
  }

  const ordered_keys = [...keys, ...Object.keys(input)].filter((key, index, array) => (
    array.indexOf(key) === index
  ));

  return ordered_keys
    .map((key) => {
      const value = formatOperationValue(input[key]);
      return value ? { key, label: FIELD_LABELS[key] ?? key, value } : null;
    })
    .filter((item): item is { key: string; label: string; value: string } => Boolean(item))
    .slice(0, limit);
}

export function formatOperationValue(value: unknown): string | null {
  if (value == null || value === "") {
    return null;
  }
  if (typeof value === "string") {
    return value.trim() || null;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    const rendered = value.map((item) => formatOperationValue(item)).filter(Boolean).join(", ");
    return rendered || null;
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
