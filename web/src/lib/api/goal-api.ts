import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type {
  ClearGoalResult,
  CreateGoalInput,
  Goal,
  GoalEvent,
  UpdateGoalInput,
} from "@/types/conversation/goal";

const AGENT_API_BASE_URL = get_agent_api_base_url();

export async function get_current_goal_api(session_key: string): Promise<Goal | null> {
  const query = new URLSearchParams({ session_key });
  return request_api<Goal | null>(
    `${AGENT_API_BASE_URL}/goals/current?${query.toString()}`,
    {
      method: "GET",
    },
  );
}

export async function create_goal_api(input: CreateGoalInput): Promise<Goal> {
  return request_api<Goal>(`${AGENT_API_BASE_URL}/goals`, {
    method: "POST",
    body: {
      session_key: input.session_key,
      objective: input.objective,
      token_budget: input.token_budget ?? null,
    },
  });
}

export async function update_goal_api(
  goal_id: string,
  input: UpdateGoalInput,
): Promise<Goal> {
  return request_api<Goal>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goal_id)}`,
    {
      method: "PATCH",
      body: {
        objective: input.objective,
        token_budget: input.token_budget,
        metadata: input.metadata,
      },
    },
  );
}

export async function list_goal_events_api(goal_id: string): Promise<GoalEvent[]> {
  return request_api<GoalEvent[]>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goal_id)}/events`,
    {
      method: "GET",
    },
  );
}

export async function pause_goal_api(goal_id: string): Promise<Goal> {
  return request_api<Goal>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goal_id)}/pause`,
    {
      method: "POST",
    },
  );
}

export async function resume_goal_api(goal_id: string): Promise<Goal> {
  return request_api<Goal>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goal_id)}/resume`,
    {
      method: "POST",
    },
  );
}

export async function clear_goal_api(goal_id: string): Promise<ClearGoalResult> {
  return request_api<ClearGoalResult>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goal_id)}/clear`,
    {
      method: "POST",
    },
  );
}
