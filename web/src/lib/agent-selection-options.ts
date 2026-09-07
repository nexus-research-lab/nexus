// INPUT: 领域已验证的 Agent 候选目录、当前语言与可选当前绑定。
// OUTPUT: 同名/缺名可区分且 value 不变的选项；缺项绑定只增加禁用的显示项。
// POS: 跨领域 Agent 选择文字的纯所有者；序号只在当前目录内显示，不持久化、不授予选择资格。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { getAgentDisplayName } from "./agent-display-name";

interface AgentSelectionSource {
  agent_id: string;
  name?: string | null;
  created_at?: number;
}

export interface AgentSelectionOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export function buildAgentSelectionOptions(
  agents: readonly AgentSelectionSource[],
  t: I18nContextValue["t"],
  directory: readonly AgentSelectionSource[] = agents,
): AgentSelectionOption[] {
  // 目录只提供稳定显示上下文，不能把不在领域候选中的对象加入选项。
  const identities = new Map(directory.map((agent) => [agent.agent_id, agent]));
  for (const agent of agents) {
    const known = identities.get(agent.agent_id);
    identities.set(agent.agent_id, {
      ...agent,
      ...known,
      name: known?.name?.trim() ? known.name : agent.name,
      created_at: known?.created_at ?? agent.created_at,
    });
  }
  const groups = new Map<string, AgentSelectionSource[]>();
  for (const agent of identities.values()) {
    const key = getAgentDisplayName(agent.name, t).toLowerCase();
    const group = groups.get(key) ?? [];
    group.push(agent);
    groups.set(key, group);
  }
  const labels = new Map<string, string>();
  for (const group of groups.values()) {
    // 保留候选原顺序；显示序号不因服务端排序或调用方过滤而随机互换。
    const ordered = [...group].sort((a, b) => (a.created_at ?? 0) - (b.created_at ?? 0)
      || (a.agent_id < b.agent_id ? -1 : a.agent_id > b.agent_id ? 1 : 0));
    ordered.forEach((agent, index) => {
      const name = getAgentDisplayName(agent.name, t);
      labels.set(agent.agent_id, group.length > 1 || !agent.name?.trim()
        ? t("agent.selection_numbered", { name, number: index + 1 })
        : name);
    });
  }
  return agents.map((agent) => ({ value: agent.agent_id, label: labels.get(agent.agent_id)! }));
}

export function includeUnavailableAgentSelection(
  options: AgentSelectionOption[],
  value: string,
  t: I18nContextValue["t"],
): AgentSelectionOption[] {
  if (!value || options.some((option) => option.value === value)) return options;
  return [{ value, label: t("agent.selection_unavailable"), disabled: true }, ...options];
}
