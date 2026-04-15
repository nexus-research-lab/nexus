"use client";

import { Agent } from "@/types/agent/agent";


export function matches_contacts_search(agent: Agent, query: string): boolean {
  if (!query.trim()) {
    return true;
  }

  const normalized_query = query.trim().toLowerCase();
  const searchable_text = [
    agent.name,
    agent.workspace_path,
    agent.status,
    agent.options.provider,
    agent.options.permission_mode,
    agent.options.system_prompt,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();

  return searchable_text.includes(normalized_query);
}
