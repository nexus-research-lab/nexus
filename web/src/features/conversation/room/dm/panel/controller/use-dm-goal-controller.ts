/**
 * INPUT: 当前 DM Agent 权限与 Goal 事件刷新信号。
 * OUTPUT: continuation hold 与 Goal 面板刷新序列。
 * POS: DM Goal 展示控制器；Goal 创建由 Composer 的独立宿主控制链负责。
 */
import { useCallback, useMemo, useState } from "react";

import {
  goalContinuationHoldForPermission,
  type GoalContinuationHold,
} from "@/features/conversation/shared/goal/goal-continuation-hold";
interface UseDmGoalControllerOptions {
  agentName: string | null;
  permissionMode: string | null;
}

export interface DmGoalControllerModel {
  continuationHold: GoalContinuationHold | null;
  refresh: () => void;
  refreshSequence: number;
}

export function useDmGoalController({
  agentName,
  permissionMode,
}: UseDmGoalControllerOptions): DmGoalControllerModel {
  const [refreshSequence, setRefreshSequence] = useState(0);
  const refresh = useCallback(() => {
    setRefreshSequence((value) => value + 1);
  }, []);
  const continuationHold = useMemo(
    () => goalContinuationHoldForPermission(agentName, permissionMode),
    [agentName, permissionMode],
  );
  return {
    continuationHold,
    refresh,
    refreshSequence,
  };
}
