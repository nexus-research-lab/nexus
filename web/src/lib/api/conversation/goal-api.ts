/**
 * INPUT: authenticated Goal identifiers, owner-scoped lifecycle payloads and session keys.
 * OUTPUT: Goal resources plus the server-derived Goal/Execution binding read view.
 * POS: Web Goal REST read/lifecycle adapter; creation belongs to the Composer set_goal command, and metadata never becomes WorkGraph state.
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  ClearGoalResult,
  Goal,
  GoalExecutionBinding,
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
