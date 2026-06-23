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
  label_key: TranslationKey;
  description_key: TranslationKey;
}> = [
  {
    value: "default",
    label_key: "agent_options.advanced.permission.default.label",
    description_key: "agent_options.advanced.permission.default.description",
  },
  {
    value: "plan",
    label_key: "agent_options.advanced.permission.plan.label",
    description_key: "agent_options.advanced.permission.plan.description",
  },
  {
    value: "acceptEdits",
    label_key: "agent_options.advanced.permission.accept_edits.label",
    description_key: "agent_options.advanced.permission.accept_edits.description",
  },
  {
    value: "bypassPermissions",
    label_key: "agent_options.advanced.permission.bypass.label",
    description_key: "agent_options.advanced.permission.bypass.description",
  },
] as const;

export const AVAILABLE_AGENT_TOOLS: ReadonlyArray<{
  name: string;
  description_key: TranslationKey;
}> = [
  { name: "Task", description_key: "agent_options.advanced.tool.task" },
  { name: "TaskOutput", description_key: "agent_options.advanced.tool.task_output" },
  { name: "Bash", description_key: "agent_options.advanced.tool.bash" },
  { name: "Glob", description_key: "agent_options.advanced.tool.glob" },
  { name: "Grep", description_key: "agent_options.advanced.tool.grep" },
  { name: "LS", description_key: "agent_options.advanced.tool.ls" },
  { name: "ExitPlanMode", description_key: "agent_options.advanced.tool.exit_plan_mode" },
  { name: "Read", description_key: "agent_options.advanced.tool.read" },
  { name: "Edit", description_key: "agent_options.advanced.tool.edit" },
  { name: "Write", description_key: "agent_options.advanced.tool.write" },
  { name: "NotebookEdit", description_key: "agent_options.advanced.tool.notebook_edit" },
  { name: "WebFetch", description_key: "agent_options.advanced.tool.web_fetch" },
  { name: "TodoWrite", description_key: "agent_options.advanced.tool.todo_write" },
  { name: "WebSearch", description_key: "agent_options.advanced.tool.web_search" },
  { name: "KillShell", description_key: "agent_options.advanced.tool.kill_shell" },
  { name: "AskUserQuestion", description_key: "agent_options.advanced.tool.ask_user_question" },
  { name: "Skill", description_key: "agent_options.advanced.tool.skill" },
  { name: "nexus_imagegen", description_key: "agent_options.advanced.tool.nexus_imagegen" },
] as const;

export const DEFAULT_AGENT_ALLOWED_TOOLS: string[] = [];

export function normalize_agent_option_provider(provider?: string | null): string {
  const normalized_provider = provider?.trim();
  return normalized_provider || DEFAULT_AGENT_OPTION_PROVIDER;
}

export function build_agent_options_save_payload(options: AgentOptions): AgentOptions {
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
