/**
 * AgentOptions Provider 常量与归一化工具
 */

import type { TranslationKey } from "@/shared/i18n/messages";
import type { AgentOptions } from "@/types/agent/agent";

export const DEFAULT_AGENT_OPTION_PROVIDER = "";
export const DEFAULT_AGENT_OPTION_MODEL = "";
export const DEFAULT_AGENT_PERMISSION_MODE = "default";

export const AGENT_PERMISSION_MODES: ReadonlyArray<{
  value: string;
  labelKey: TranslationKey;
  descriptionKey: TranslationKey;
}> = [
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
    value: "bypassPermissions",
    labelKey: "agent_options.advanced.permission.bypass.label",
    descriptionKey: "agent_options.advanced.permission.bypass.description",
  },
] as const;

export const AVAILABLE_AGENT_TOOLS: ReadonlyArray<{
  name: string;
  descriptionKey: TranslationKey;
}> = [
  { name: "Agent", descriptionKey: "agent_options.advanced.tool.agent" },
  { name: "Bash", descriptionKey: "agent_options.advanced.tool.bash" },
  { name: "Edit", descriptionKey: "agent_options.advanced.tool.edit" },
  { name: "Write", descriptionKey: "agent_options.advanced.tool.write" },
  { name: "NotebookEdit", descriptionKey: "agent_options.advanced.tool.notebook_edit" },
  { name: "WebFetch", descriptionKey: "agent_options.advanced.tool.web_fetch" },
  { name: "WebSearch", descriptionKey: "agent_options.advanced.tool.web_search" },
] as const;

export const DEFAULT_AGENT_ALLOWED_TOOLS: string[] = [];

const VISIBLE_AGENT_PREAUTHORIZED_TOOLS = new Set(AVAILABLE_AGENT_TOOLS.map((tool) => tool.name));

const RETIRED_AGENT_PREAUTH_TOOL_ALIASES: Record<string, string | null> = {
  Task: "Agent",
  TaskOutput: null,
  Glob: null,
  Grep: null,
  LS: null,
  Read: null,
  TodoWrite: null,
  KillShell: null,
  AskUserQuestion: null,
  Skill: null,
  EnterPlanMode: null,
  ExitPlanMode: null,
  nexus_imagegen: null,
  generate_image: null,
  edit_image: null,
  mcp__nexus_imagegen__generate_image: null,
  mcp__nexus_imagegen__edit_image: null,
};

function normalizeAgentAllowedToolName(toolName: string): string | null {
  const normalizedToolName = toolName.trim();
  if (!normalizedToolName) {
    return null;
  }
  if (
    normalizedToolName.startsWith("Skill(") ||
    normalizedToolName.startsWith("mcp__nexus_imagegen__") ||
    normalizedToolName.startsWith("nexus_imagegen__") ||
    normalizedToolName.startsWith("nexus_imagegen.")
  ) {
    return null;
  }
  if (Object.prototype.hasOwnProperty.call(RETIRED_AGENT_PREAUTH_TOOL_ALIASES, normalizedToolName)) {
    return RETIRED_AGENT_PREAUTH_TOOL_ALIASES[normalizedToolName] ?? null;
  }
  return normalizedToolName;
}

export function normalizeAgentAllowedToolsForEditor(tools?: string[] | null): string[] {
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

export function countVisibleAgentPreauthorizedTools(tools: string[]): number {
  return tools.filter((toolName) => VISIBLE_AGENT_PREAUTHORIZED_TOOLS.has(toolName.trim())).length;
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
    setting_sources: options.setting_sources,
  };
}
