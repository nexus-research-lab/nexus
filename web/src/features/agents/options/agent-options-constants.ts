/**
 * AgentOptions Provider 常量与归一化工具
 */

import { format_provider_label, type ProviderOption } from "@/types/capability/provider";
import type { TranslationKey } from "@/shared/i18n/messages";

export const DEFAULT_AGENT_OPTION_PROVIDER = "";

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
  { name: "EnterPlanMode", description_key: "agent_options.advanced.tool.enter_plan_mode" },
] as const;

export const DEFAULT_AGENT_ALLOWED_TOOLS = AVAILABLE_AGENT_TOOLS.map((tool) => tool.name);

export function normalize_agent_option_provider(provider?: string | null): string {
  const normalized_provider = provider?.trim();
  return normalized_provider || DEFAULT_AGENT_OPTION_PROVIDER;
}

export function build_agent_option_provider_options(
  provider_options: ProviderOption[],
  current_provider?: string,
): ProviderOption[] {
  const normalized_provider = current_provider?.trim();
  if (!normalized_provider || provider_options.some((item) => item.provider === normalized_provider)) {
    return provider_options;
  }
  return [
    ...provider_options,
    {
      provider: normalized_provider,
      display_name: format_provider_label(normalized_provider),
      is_default: false,
    },
  ];
}
