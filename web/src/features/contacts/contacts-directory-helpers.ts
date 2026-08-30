"use client";

import type { Agent } from "@/types/agent/agent";

export const CONTACTS_DEFAULT_PROVIDER_FILTER = "__follow_default__";

export function matchesContactsSearch(agent: Agent, query: string): boolean {
  if (!query.trim()) {
    return true;
  }

  const normalizedQuery = query.trim().toLowerCase();
  const searchableText = [
    agent.name,
    agent.display_name,
    agent.headline,
    agent.profile_markdown,
    agent.description,
    agent.workspace_path,
    agent.status,
    agent.options.provider,
    agent.options.permission_mode,
    ...(agent.business_tags ?? []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();

  return searchableText.includes(normalizedQuery);
}

export interface ContactsDirectoryFilters {
  permissionMode: string;
  provider: string;
  query: string;
  tag: string;
}

export function filterContactsAgents(
  agents: Agent[],
  filters: ContactsDirectoryFilters,
): Agent[] {
  const normalizedTag = filters.tag.toLowerCase();
  return agents.filter((agent) => {
    const provider = agent.options.provider?.trim()
      || CONTACTS_DEFAULT_PROVIDER_FILTER;
    const permissionMode = agent.options.permission_mode?.trim() || "default";
    return matchesContactsSearch(agent, filters.query)
      && (!normalizedTag || agent.business_tags?.some(
        (tag) => tag.trim().toLowerCase() === normalizedTag,
      ))
      && (!filters.provider || provider === filters.provider)
      && (!filters.permissionMode || permissionMode === filters.permissionMode);
  });
}

export function getContactsDirectoryBusinessTags(agents: Agent[]): string[] {
  return uniqueSortedValues(agents.flatMap((agent) => agent.business_tags ?? []));
}

export function getContactsDirectoryProviders(agents: Agent[]): string[] {
  return uniqueSortedValues(agents.map((agent) => (
    agent.options.provider?.trim() || CONTACTS_DEFAULT_PROVIDER_FILTER
  )));
}

export function getContactsDirectoryPermissionModes(agents: Agent[]): string[] {
  return uniqueSortedValues(agents.map((agent) => (
    agent.options.permission_mode?.trim() || "default"
  )));
}

function uniqueSortedValues(values: string[]): string[] {
  const uniqueValues = new Map<string, string>();
  for (const value of values) {
    const trimmedValue = value.trim();
    if (trimmedValue && !uniqueValues.has(trimmedValue.toLowerCase())) {
      uniqueValues.set(trimmedValue.toLowerCase(), trimmedValue);
    }
  }
  return [...uniqueValues.values()].sort((left, right) => left.localeCompare(right));
}
