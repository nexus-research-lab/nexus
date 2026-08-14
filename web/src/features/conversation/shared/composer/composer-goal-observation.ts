/**
 * INPUT: GoalPanel 已读取的当前 scope durable Goal 身份与版本。
 * OUTPUT: Goal Composer 提交前可冻结的非权威 baseline fence。
 * POS: Goal REST 投影与 Composer request identity 之间的有界只读观察缓存。
 */

import type { Goal } from "@/types/conversation/goal";

export interface ComposerGoalBaseline {
  goalId: string | null;
  version: number;
}

const MAX_OBSERVED_GOAL_SCOPES = 128;
const observedGoals = new Map<string, ComposerGoalBaseline>();

export function observeComposerGoal(
  scopeKey: string,
  goal: Goal | null,
): void {
  const normalizedScopeKey = scopeKey.trim();
  if (!normalizedScopeKey) {
    return;
  }
  observedGoals.delete(normalizedScopeKey);
  observedGoals.set(normalizedScopeKey, goal
    ? {
        goalId: goal.id,
        version: goal.version,
      }
    : {
        goalId: null,
        version: 0,
      });
  while (observedGoals.size > MAX_OBSERVED_GOAL_SCOPES) {
    const oldestScopeKey = observedGoals.keys().next().value;
    if (typeof oldestScopeKey !== "string") {
      break;
    }
    observedGoals.delete(oldestScopeKey);
  }
}

export function readObservedComposerGoal(
  scopeKey: string,
): ComposerGoalBaseline | null {
  return observedGoals.get(scopeKey.trim()) ?? null;
}
