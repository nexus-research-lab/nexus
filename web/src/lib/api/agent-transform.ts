/**
 * Agent API 数据转换工具。
 */

import type { Agent, ApiAgent } from "@/types/agent/agent";

export function transform_api_agent(api_agent: ApiAgent): Agent {
  return {
    agent_id: api_agent.agent_id,
    name: api_agent.name,
    workspace_path: api_agent.workspace_path,
    display_name: api_agent.display_name ?? null,
    headline: api_agent.headline ?? null,
    profile_markdown: api_agent.profile_markdown ?? null,
    options: api_agent.options || {},
    created_at: new Date(api_agent.created_at).getTime(),
    status: api_agent.status,
    avatar: api_agent.avatar ?? null,
    description: api_agent.description ?? null,
    vibe_tags: api_agent.vibe_tags ?? [],
    skills_count: api_agent.skills_count ?? null,
  };
}
