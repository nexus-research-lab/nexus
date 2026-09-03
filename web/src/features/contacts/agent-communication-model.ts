// INPUT: Agent/联系人目录与用户查询。
// OUTPUT: 稳定的展示名称，以及按展示名称排序的筛选结果。
// POS: Contacts 联络视图纯投影；不持有 React 状态、请求或 mutation 生命周期。

import type { Agent, AgentContact } from "@/types/agent/agent";

export function getCommunicationAgentName(agent: Agent): string {
  return agent.display_name?.trim() || agent.name;
}

export function getCommunicationContactLabel(contact: AgentContact): string {
  return contact.alias?.trim() || contact.display_name?.trim() || contact.name;
}

export function filterCommunicationContacts(
  contacts: AgentContact[],
  query: string,
): AgentContact[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  return contacts
    .filter((contact) => !normalizedQuery || [
      contact.alias,
      contact.display_name,
      contact.name,
    ].some((value) => value?.toLocaleLowerCase().includes(normalizedQuery)))
    .sort((left, right) => (
      getCommunicationContactLabel(left).localeCompare(
        getCommunicationContactLabel(right),
      )
    ));
}
