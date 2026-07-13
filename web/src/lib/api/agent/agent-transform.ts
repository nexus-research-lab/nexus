/**
 * Agent API 数据转换工具。
 */

import type { Agent, ApiAgent } from "@/types/agent/agent";

export function transformApiAgent(apiAgent: ApiAgent): Agent {
  return {
    agent_id: apiAgent.agent_id,
    name: apiAgent.name,
    workspace_path: apiAgent.workspace_path,
    display_name: apiAgent.display_name ?? null,
    headline: apiAgent.headline ?? null,
    profile_markdown: apiAgent.profile_markdown ?? null,
    options: apiAgent.options || {},
    created_at: new Date(apiAgent.created_at).getTime(),
    status: apiAgent.status,
    avatar: apiAgent.avatar ?? null,
    description: apiAgent.description ?? null,
    vibe_tags: apiAgent.vibe_tags ?? [],
    skills_count: apiAgent.skills_count ?? null,
  };
}
