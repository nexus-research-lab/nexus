/**
 * INPUT: 当前 DM Agent 权限与 Goal 事件刷新信号。
 * OUTPUT: 随当前语言更新的 continuation hold 与 Goal 面板刷新序列。
 * POS: DM Goal 展示控制器；Goal 创建由 Composer 的独立宿主控制链负责。
 */
import { useCallback, useMemo, useState } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";

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
  const { t } = useI18n();
  const [refreshSequence, setRefreshSequence] = useState(0);
  const refresh = useCallback(() => {
    setRefreshSequence((value) => value + 1);
  }, []);
  const continuationHold = useMemo(
    () => goalContinuationHoldForPermission(agentName, permissionMode, t),
    [agentName, permissionMode, t],
  );
  return {
    continuationHold,
    refresh,
    refreshSequence,
  };
}
