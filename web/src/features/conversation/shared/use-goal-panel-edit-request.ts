import { useCallback, useState } from "react";

import type { Goal } from "@/types/conversation/goal";

export interface GoalPanelEditRequest {
  goal: Goal;
  request_id: number;
}

export function useGoalPanelEditRequest() {
  const [goal_edit_request, set_goal_edit_request] =
    useState<GoalPanelEditRequest | null>(null);

  const request_goal_edit = useCallback((goal: Goal) => {
    set_goal_edit_request((current) => ({
      goal,
      request_id: (current?.request_id ?? 0) + 1,
    }));
  }, []);

  return { goal_edit_request, request_goal_edit };
}
