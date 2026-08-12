/**
 * INPUT: authenticated Goal identifiers, owner-scoped lifecycle payloads and session keys.
 * OUTPUT: Goal resources plus the server-derived Goal/Execution binding read view.
 * POS: Web Goal REST adapter; it never interprets Goal metadata as WorkGraph state.
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  ClearGoalResult,
  CreateGoalInput,
  Goal,
  GoalExecutionBinding,
  GoalUsageReport,
  UpdateGoalInput,
} from "@/types/conversation/goal";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export async function getCurrentGoalApi(sessionKey: string): Promise<Goal | null> {
  const query = new URLSearchParams({ session_key: sessionKey });
  return requestApi<Goal | null>(
    `${AGENT_API_BASE_URL}/goals/current?${query.toString()}`,
    {
      method: "GET",
    },
  );
}

export async function getGoalUsageApi(goalId: string): Promise<GoalUsageReport> {
  return requestApi<GoalUsageReport>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goalId)}/usage`,
    {
      method: "GET",
    },
  );
}

export async function getGoalExecutionBindingApi(
  goalId: string,
): Promise<GoalExecutionBinding> {
  return requestApi<GoalExecutionBinding>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goalId)}/execution-binding`,
    {
      method: "GET",
    },
  );
}

export async function createGoalApi(input: CreateGoalInput): Promise<Goal> {
  return requestApi<Goal>(`${AGENT_API_BASE_URL}/goals`, {
    method: "POST",
    body: {
      session_key: input.session_key,
      objective: input.objective,
      token_budget: input.token_budget ?? null,
      replace_existing: input.replace_existing ?? false,
      room_lead_agent_id: input.room_lead_agent_id ?? null,
      metadata: input.metadata ?? null,
    },
  });
}

export async function updateGoalApi(
  goalId: string,
  input: UpdateGoalInput,
): Promise<Goal> {
  return requestApi<Goal>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goalId)}`,
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

export async function pauseGoalApi(goalId: string): Promise<Goal> {
  return requestApi<Goal>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goalId)}/pause`,
    {
      method: "POST",
    },
  );
}

export async function resumeGoalApi(goalId: string): Promise<Goal> {
  return requestApi<Goal>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goalId)}/resume`,
    {
      method: "POST",
    },
  );
}

export async function clearGoalApi(goalId: string): Promise<ClearGoalResult> {
  return requestApi<ClearGoalResult>(
    `${AGENT_API_BASE_URL}/goals/${encodeURIComponent(goalId)}/clear`,
    {
      method: "POST",
    },
  );
}
