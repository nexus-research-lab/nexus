import type { AgentOptions } from "@/types/agent/agent";

export const DEFAULT_AGENT_OPTION_PROVIDER = "";
export const DEFAULT_AGENT_OPTION_MODEL = "";
// 新建 Agent 默认独立审核未决操作，无法确认时交给用户。
export const DEFAULT_AGENT_PERMISSION_MODE = "auto";

export const AGENT_PERMISSION_MODES = [
  {
    value: "default",
    labelKey: "agent_options.advanced.permission.default.label",
    descriptionKey: "agent_options.advanced.permission.default.description",
  },
  {
    value: "plan",
    labelKey: "agent_options.advanced.permission.plan.label",
    descriptionKey: "agent_options.advanced.permission.plan.description",
  },
  {
    value: "acceptEdits",
    labelKey: "agent_options.advanced.permission.accept_edits.label",
    descriptionKey: "agent_options.advanced.permission.accept_edits.description",
  },
  {
    value: "auto",
    labelKey: "agent_options.advanced.permission.auto.label",
    descriptionKey: "agent_options.advanced.permission.auto.description",
  },
  {
    value: "bypassPermissions",
    labelKey: "agent_options.advanced.permission.bypass.label",
    descriptionKey: "agent_options.advanced.permission.bypass.description",
  },
  {
    value: "dontAsk",
    labelKey: "agent_options.advanced.permission.dont_ask.label",
    descriptionKey: "agent_options.advanced.permission.dont_ask.description",
  },
] as const;

// 旧模式仍可读取，但产品不再提供规划、不询问和自动接受编辑模式的设置入口。
export const AGENT_PERMISSION_CHOICES = AGENT_PERMISSION_MODES.filter(
  (mode) => mode.value !== "plan"
    && mode.value !== "dontAsk"
    && mode.value !== "acceptEdits",
);

export const DEFAULT_AGENT_ALLOWED_TOOLS: readonly string[] = [];

// 历史持久化值只在编辑入口清洗，内部草稿不继续传播已退休工具名。
const RETIRED_AGENT_PREAUTH_TOOL_ALIASES: Record<string, string | null> = {
  Task: "Agent",
  TaskOutput: null,
  Glob: null,
  Grep: null,
  LS: null,
  Read: null,
  TodoWrite: null,
  NotebookEdit: null,
  KillShell: null,
  AskUserQuestion: null,
  Skill: null,
  EnterPlanMode: null,
  ExitPlanMode: null,
  nexus_imagegen: null,
  generate_image: null,
  edit_image: null,
  mcp__nexus__generate_image: null,
  mcp__nexus__edit_image: null,
  mcp__nexus_imagegen__generate_image: null,
  mcp__nexus_imagegen__edit_image: null,
};

const RETIRED_AGENT_PREAUTH_TOOL_PREFIXES = [
  "Skill(",
  "mcp__nexus_imagegen__",
  "nexus_imagegen__",
  "nexus_imagegen.",
] as const;

function normalizeAgentAllowedToolName(toolName: string): string | null {
  const normalizedToolName = toolName.trim();
  if (!normalizedToolName) {
    return null;
  }
  if (RETIRED_AGENT_PREAUTH_TOOL_PREFIXES.some(
    (prefix) => normalizedToolName.startsWith(prefix),
  )) {
    return null;
  }
  if (Object.prototype.hasOwnProperty.call(
    RETIRED_AGENT_PREAUTH_TOOL_ALIASES,
    normalizedToolName,
  )) {
    return RETIRED_AGENT_PREAUTH_TOOL_ALIASES[normalizedToolName] ?? null;
  }
  return normalizedToolName;
}

export function normalizeAgentAllowedToolsForEditor(
  tools?: readonly string[] | null,
): string[] {
  const result: string[] = [];
  const seenTools = new Set<string>();
  for (const toolName of tools ?? []) {
    const normalizedToolName = normalizeAgentAllowedToolName(toolName);
    if (!normalizedToolName || seenTools.has(normalizedToolName)) {
      continue;
    }
    seenTools.add(normalizedToolName);
    result.push(normalizedToolName);
  }
  return result;
}

export function normalizeAgentOptionProvider(provider?: string | null): string {
  const normalizedProvider = provider?.trim();
  return normalizedProvider || DEFAULT_AGENT_OPTION_PROVIDER;
}

export function pickAgentEditableOptions(options: AgentOptions): AgentOptions {
  return {
    provider: options.provider,
    model: options.model,
    permission_mode: options.permission_mode,
    allowed_tools: options.allowed_tools,
    disallowed_tools: options.disallowed_tools,
    max_turns: options.max_turns,
    max_thinking_tokens: options.max_thinking_tokens,
    mcp_servers: options.mcp_servers,
    connector_ids: options.connector_ids,
    // Skill 绑定由技能域独立保存，不能随普通 Agent 草稿回写旧快照。
    setting_sources: options.setting_sources,
  };
}

/** 两个运行时共用权限入口；后端分别协商 nxs 能力和确认 Claude 原生模式。 */
export function getAgentPermissionChoices(runtimeKind: string) {
  return AGENT_PERMISSION_CHOICES.filter((mode) => runtimeKind === "nxs" || runtimeKind === "claude" || mode.value !== "auto");
}

/** 运行时尚未确定时，保守展示人工审批。 */
export function resolveRuntimePermissionMode<T extends string>(mode: T, runtimeKind: string): T | "default" {
  return mode === "auto" && runtimeKind !== "nxs" && runtimeKind !== "claude" ? "default" : mode;
}
