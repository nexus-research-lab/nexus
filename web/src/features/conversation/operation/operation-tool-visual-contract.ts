import type { OperationActionKind } from "./operation-tool-catalog";
import { resolve_operation_tool_profile } from "./operation-tool-catalog";
import type { NexusOperationEvent } from "./operation-types";

export type OperationToolVisualGroup =
  | "workspace_navigation"
  | "workspace_reader"
  | "workspace_writer"
  | "command_runner"
  | "web_browser"
  | "task_planner"
  | "knowledge_tool"
  | "human_gate"
  | "handoff"
  | "unclassified_action";

export type OperationToolVisualComponent =
  | "finder"
  | "code_reader"
  | "code_writer"
  | "terminal"
  | "browser"
  | "activity_monitor"
  | "knowledge_viewer"
  | "system_gate"
  | "handoff"
  | "execution_path";

export type OperationSharedControl =
  | "window_close"
  | "window_minimize"
  | "window_zoom"
  | "window_drag"
  | "window_focus"
  | "dock_restore"
  | "confirm"
  | "deny";

export interface OperationToolVisualGroupSpec {
  app_label: string;
  component: OperationToolVisualComponent;
  interaction_label: string;
  label: string;
  tools: readonly string[];
}

export interface OperationToolVisualContract extends OperationToolVisualGroupSpec {
  action: OperationActionKind;
  common_controls: readonly OperationSharedControl[];
  group: OperationToolVisualGroup;
}

const WINDOW_CONTROLS = [
  "window_close",
  "window_minimize",
  "window_zoom",
  "window_drag",
  "window_focus",
  "dock_restore",
] as const satisfies readonly OperationSharedControl[];

export const OPERATION_TOOL_VISUAL_GROUPS: Record<OperationToolVisualGroup, OperationToolVisualGroupSpec> = {
  workspace_navigation: {
    app_label: "访达",
    component: "finder",
    interaction_label: "浏览目录、搜索文件、选中文件",
    label: "工作区导航",
    tools: ["Glob", "Grep", "LS"],
  },
  workspace_reader: {
    app_label: "Code",
    component: "code_reader",
    interaction_label: "打开文件并扫描内容",
    label: "文件读取",
    tools: ["Read"],
  },
  workspace_writer: {
    app_label: "Code",
    component: "code_writer",
    interaction_label: "新建文件、流式输入、展示 diff",
    label: "文件写入",
    tools: ["Write", "Edit", "MultiEdit", "NotebookEdit"],
  },
  command_runner: {
    app_label: "终端",
    component: "terminal",
    interaction_label: "输入命令并流式显示 stdout/stderr",
    label: "命令执行",
    tools: ["Bash", "KillShell"],
  },
  web_browser: {
    app_label: "Safari",
    component: "browser",
    interaction_label: "加载搜索结果、网页内容或本地 HTML 预览",
    label: "网页浏览",
    tools: ["WebSearch", "WebFetch"],
  },
  task_planner: {
    app_label: "活动监视器",
    component: "activity_monitor",
    interaction_label: "展示子任务、计划和进度状态",
    label: "任务计划",
    tools: ["Task", "TaskOutput", "TodoWrite", "EnterPlanMode", "ExitPlanMode"],
  },
  knowledge_tool: {
    app_label: "Nexus",
    component: "knowledge_viewer",
    interaction_label: "展示技能、文档片段和工具上下文",
    label: "知识工具",
    tools: ["Skill"],
  },
  human_gate: {
    app_label: "系统设置",
    component: "system_gate",
    interaction_label: "等待用户确认、拒绝或回答",
    label: "用户确认",
    tools: ["AskUserQuestion"],
  },
  handoff: {
    app_label: "交付台",
    component: "handoff",
    interaction_label: "归档窗口现场和关键产物",
    label: "完成收束",
    tools: [],
  },
  unclassified_action: {
    app_label: "Nexus",
    component: "execution_path",
    interaction_label: "只记录在执行路径中，不打开桌面窗口",
    label: "未分类动作",
    tools: [],
  },
};

export function resolve_operation_tool_visual_contract(
  event: NexusOperationEvent,
): OperationToolVisualContract {
  const profile = resolve_operation_tool_profile(event.tool_name, event.kind, event.surface);
  const group = resolve_visual_group(event, profile.action);
  const spec = OPERATION_TOOL_VISUAL_GROUPS[group];
  return {
    ...spec,
    action: profile.action,
    common_controls: group === "human_gate"
      ? [...WINDOW_CONTROLS, "confirm", "deny"]
      : WINDOW_CONTROLS,
    group,
  };
}

function resolve_visual_group(
  event: NexusOperationEvent,
  action: OperationActionKind,
): OperationToolVisualGroup {
  if (event.kind === "round_summary" || action === "summary") {
    return "handoff";
  }
  if (event.kind === "human_gate" || action === "question") {
    return "human_gate";
  }
  if (action === "list" || action === "search") {
    return "workspace_navigation";
  }
  if (action === "read") {
    return "workspace_reader";
  }
  if (action === "create" || action === "edit") {
    return "workspace_writer";
  }
  if (action === "run" || action === "stop") {
    return "command_runner";
  }
  if (action === "web_search" || action === "web_fetch") {
    return "web_browser";
  }
  if (action === "task" || action === "task_progress" || action === "plan") {
    return "task_planner";
  }
  if (action === "skill" || event.surface === "knowledge") {
    return "knowledge_tool";
  }
  return "unclassified_action";
}
