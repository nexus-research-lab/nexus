"use client";

import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

export type ContactsRuntimeStatus = "Running" | "Idle" | "Active";
export type ContactsRuntimeTone = "running" | "idle" | "active";
export type ContactsFilterKey =
  | "all"
  | "recent"
  | "running"
  | "active"
  | "idle"
  | "skills_on"
  | "skills_off";

export function get_contacts_runtime_status(agent: Agent): ContactsRuntimeStatus {
  const normalized_status = agent.status?.toLowerCase() ?? "";

  if (normalized_status.includes("run") || normalized_status.includes("busy")) {
    return "Running";
  }
  if (normalized_status.includes("idle")) {
    return "Idle";
  }
  return "Active";
}

export function get_contacts_runtime_label(status: ContactsRuntimeStatus): string {
  if (status === "Running") {
    return "协作中";
  }
  if (status === "Idle") {
    return "待命";
  }
  return "在线";
}

export function get_contacts_runtime_tone(status: ContactsRuntimeStatus): ContactsRuntimeTone {
  if (status === "Running") {
    return "running";
  }
  if (status === "Idle") {
    return "idle";
  }
  return "active";
}

export function get_contacts_model_label(agent: Agent): string {
  return agent.options.model || "inherit";
}

export function get_contacts_agent_conversations(
  conversations: Conversation[],
  agent_id: string,
): Conversation[] {
  return conversations
    .filter((conversation) => conversation.agent_id === agent_id)
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

export function get_contacts_agent_description(agent: Agent): string {
  const prompt = agent.options.system_prompt?.trim();
  if (prompt) {
    return prompt.replace(/\s+/g, " ").slice(0, 120);
  }

  if (agent.options.skills_enabled) {
    return "支持技能扩展与多步协作，可直接投入复杂任务推进。";
  }

  return "可直接发起 1v1 协作，适合稳定执行日常任务与目录工作流。";
}

export function matches_contacts_filter(
  agent: Agent,
  conversations: Conversation[],
  filter: ContactsFilterKey,
): boolean {
  const runtime_status = get_contacts_runtime_status(agent);
  const has_recent_history = conversations.length > 0;

  if (filter === "all") {
    return true;
  }
  if (filter === "recent") {
    return has_recent_history;
  }
  if (filter === "running") {
    return runtime_status === "Running";
  }
  if (filter === "active") {
    return runtime_status === "Active";
  }
  if (filter === "idle") {
    return runtime_status === "Idle";
  }
  if (filter === "skills_on") {
    return Boolean(agent.options.skills_enabled);
  }
  if (filter === "skills_off") {
    return !agent.options.skills_enabled;
  }
  return true;
}

export function matches_contacts_search(agent: Agent, query: string): boolean {
  if (!query.trim()) {
    return true;
  }

  const normalized_query = query.trim().toLowerCase();
  const searchable_text = [
    agent.name,
    agent.workspace_path,
    agent.status,
    agent.options.model,
    agent.options.permission_mode,
    agent.options.system_prompt,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();

  return searchable_text.includes(normalized_query);
}
