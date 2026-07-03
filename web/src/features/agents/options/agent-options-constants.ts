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
  { name: "Task", descriptionKey: "agent_options.advanced.tool.task" },
  { name: "TaskOutput", descriptionKey: "agent_options.advanced.tool.task_output" },
  { name: "Bash", descriptionKey: "agent_options.advanced.tool.bash" },
  { name: "Glob", descriptionKey: "agent_options.advanced.tool.glob" },
  { name: "Grep", descriptionKey: "agent_options.advanced.tool.grep" },
  { name: "LS", descriptionKey: "agent_options.advanced.tool.ls" },
  { name: "ExitPlanMode", descriptionKey: "agent_options.advanced.tool.exit_plan_mode" },
  { name: "Read", descriptionKey: "agent_options.advanced.tool.read" },
  { name: "Edit", descriptionKey: "agent_options.advanced.tool.edit" },
  { name: "Write", descriptionKey: "agent_options.advanced.tool.write" },
  { name: "NotebookEdit", descriptionKey: "agent_options.advanced.tool.notebook_edit" },
  { name: "WebFetch", descriptionKey: "agent_options.advanced.tool.web_fetch" },
  { name: "TodoWrite", descriptionKey: "agent_options.advanced.tool.todo_write" },
  { name: "WebSearch", descriptionKey: "agent_options.advanced.tool.web_search" },
  { name: "KillShell", descriptionKey: "agent_options.advanced.tool.kill_shell" },
  { name: "AskUserQuestion", descriptionKey: "agent_options.advanced.tool.ask_user_question" },
  { name: "Skill", descriptionKey: "agent_options.advanced.tool.skill" },
  { name: "nexus_imagegen", descriptionKey: "agent_options.advanced.tool.nexus_imagegen" },
] as const;

export const DEFAULT_AGENT_ALLOWED_TOOLS: string[] = [];

export function normalizeAgentOptionProvider(provider?: string | null): string {
  const normalizedProvider = provider?.trim();
  return normalizedProvider || DEFAULT_AGENT_OPTION_PROVIDER;
}

export function buildAgentOptionsSavePayload(options: AgentOptions): AgentOptions {
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
