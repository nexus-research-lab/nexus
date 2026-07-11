import { useCallback, useMemo, useState } from "react";

import {
  goalContinuationHoldForPermission,
  type GoalContinuationHold,
} from "@/features/conversation/shared/goal/goal-continuation-hold";
import { createGoalApi } from "@/lib/api/goal-api";
import { useI18n } from "@/shared/i18n/i18n-context";

interface UseDmGoalControllerOptions {
  agentName: string | null;
  permissionMode: string | null;
  sessionKey: string | null;
}

export interface DmGoalControllerModel {
  continuationHold: GoalContinuationHold | null;
  createGoal: (objective: string) => Promise<void>;
  refresh: () => void;
  refreshSequence: number;
}

export function useDmGoalController({
  agentName,
  permissionMode,
  sessionKey,
}: UseDmGoalControllerOptions): DmGoalControllerModel {
  const { t } = useI18n();
  const [refreshSequence, setRefreshSequence] = useState(0);
  const refresh = useCallback(() => {
    setRefreshSequence((value) => value + 1);
  }, []);
  const continuationHold = useMemo(
    () => goalContinuationHoldForPermission(agentName, permissionMode),
    [agentName, permissionMode],
  );
  const createGoal = useCallback(
    async (objective: string): Promise<void> => {
      if (!sessionKey) {
        throw new Error(t("dm.goal_session_not_ready"));
      }
      await createGoalApi({
        objective,
        session_key: sessionKey,
        token_budget: null,
      });
      refresh();
    },
    [refresh, sessionKey, t],
  );

  return {
    continuationHold,
    createGoal,
    refresh,
    refreshSequence,
  };
}
