"use client";

import { Agent } from "@/types/agent/agent";


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
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();

  return searchableText.includes(normalizedQuery);
}
