// INPUT: 领域已验证的 Agent 候选目录、当前语言与可选当前绑定。
// OUTPUT: 同名/缺名可区分且 value 不变的选项；缺项绑定只增加禁用的显示项。
// POS: 跨领域 Agent 选择文字适配；通用序号/缺项算法归 shared/lib，领域候选资格不变。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { getAgentDisplayName } from "./agent-display-name";
import { buildNamedSelectionOptions, includeUnavailableSelection, type SelectionOption } from "@/shared/lib/selection-options";

interface AgentSelectionSource {
  agent_id: string;
  name?: string | null;
  created_at?: number;
}

export type AgentSelectionOption = SelectionOption;

export function buildAgentSelectionOptions(
  agents: readonly AgentSelectionSource[],
  t: I18nContextValue["t"],
  directory: readonly AgentSelectionSource[] = agents,
): AgentSelectionOption[] {
  return buildNamedSelectionOptions(agents.map(selectionSource), {
    fallbackLabel: getAgentDisplayName(null, t),
    numberedLabel: (name, number) => t("common.selection_numbered", { name, number }),
  }, directory.map(selectionSource));
}

function selectionSource(agent: AgentSelectionSource) {
  return { value: agent.agent_id, label: agent.name, createdAt: agent.created_at };
}

export function includeUnavailableAgentSelection(
  options: AgentSelectionOption[],
  value: string,
  t: I18nContextValue["t"],
): AgentSelectionOption[] {
  return includeUnavailableSelection(options, value, t("agent.selection_unavailable"));
}
