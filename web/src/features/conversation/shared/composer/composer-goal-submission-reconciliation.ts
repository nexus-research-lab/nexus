/**
 * INPUT: 当前 Composer Goal 提交作用域与 owner-scoped durable Goal 快照。
 * OUTPUT: 只在原始 objective（含服务端 source_objective）匹配时收口确认中提交。
 * POS: Goal REST 正向证据到 Composer 提交状态的精确对账边界。
 */

import { useCallback, useEffect, useRef } from "react";

import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type { Goal } from "@/types/conversation/goal";
import type { Message } from "@/types/conversation/message/entity";

import { useComposerDraftStore } from "./composer-draft-store";
import { observeComposerGoal } from "./composer-goal-observation";

const SOURCE_OBJECTIVE_METADATA_KEY = "source_objective";

/** durable Goal 是 ACK 丢失后的正向证据；其他 scope 或其他目标不得误收口。 */
export function reconcileComposerGoalSubmission(
  scopeKey: string,
  goal: Goal | null,
): boolean {
  if (!goal) {
    return false;
  }
  const state = useComposerDraftStore.getState();
  const submission = state.goal_submission_by_scope[scopeKey];
  if (
    !submission
    || submission.phase !== "confirming"
    || !isNewerThanBaseline(goal, submission.baselineGoal)
    || !matchesSubmittedObjective(goal, submission.submittedDraft.input)
  ) {
    observeComposerGoal(scopeKey, goal);
    return false;
  }
  observeComposerGoal(scopeKey, goal);
  return state.complete_goal_submission(submission);
}

/**
 * Goal 可能在用户返回原 Session 前已经终态或被清除；此时 Goal REST 快照
 * 不再足以对账，但 durable `/goal` 控制记录仍保存 exact client_message_id。
 */
export function reconcileComposerGoalSubmissionFromMessages(
  scopeKey: string,
  messages: readonly Message[],
): boolean {
  const state = useComposerDraftStore.getState();
  const submission = state.goal_submission_by_scope[scopeKey];
  const owner = submission?.confirmationIdentity;
  if (!submission || submission.phase !== "confirming" || !owner) {
    return false;
  }
  const hasDurableControlRecord = messages.some((message) => (
    message.role === "user"
    && message.client_message_id === owner.clientMessageId
    && message.message_id !== owner.clientMessageId
    && message.metadata?.subtype === "goal_set"
    && areEquivalentSessionKeys(message.session_key, owner.sessionKey)
  ));
  return hasDurableControlRecord
    ? state.complete_goal_submission(submission)
    : false;
}

/**
 * 保留 GoalPanel 最近一次 durable 快照；当 ACK timeout 把同一提交切到
 * confirming 时，用已经到达的新版快照立即对账，不依赖下一条 WS 事件。
 */
export function useComposerGoalSubmissionReconciliation(
  scopeKey: string,
  messages: readonly Message[],
): (goal: Goal | null) => void {
  const currentGoalRef = useRef<Goal | null>(null);
  const submissionPhase = useComposerDraftStore(
    (state) => state.goal_submission_by_scope[scopeKey]?.phase ?? null,
  );
  const reconcile = useCallback((goal: Goal | null) => {
    currentGoalRef.current = goal;
    if (!goal) {
      observeComposerGoal(scopeKey, null);
      return;
    }
    reconcileComposerGoalSubmission(scopeKey, goal);
  }, [scopeKey]);
  useEffect(() => {
    if (submissionPhase === "confirming" && currentGoalRef.current) {
      reconcileComposerGoalSubmission(scopeKey, currentGoalRef.current);
    }
  }, [scopeKey, submissionPhase]);
  useEffect(() => {
    if (submissionPhase === "confirming") {
      reconcileComposerGoalSubmissionFromMessages(scopeKey, messages);
    }
  }, [messages, scopeKey, submissionPhase]);
  return reconcile;
}

function isNewerThanBaseline(
  goal: Goal,
  baseline: {goalId: string | null; version: number} | null,
): boolean {
  return baseline !== null && (
    goal.id !== baseline.goalId
    || goal.version > baseline.version
  );
}

function matchesSubmittedObjective(goal: Goal, submittedObjective: string): boolean {
  const expected = submittedObjective.trim();
  if (!expected) {
    return false;
  }
  const sourceObjective = goal.metadata?.[SOURCE_OBJECTIVE_METADATA_KEY];
  return [
    goal.objective,
    typeof sourceObjective === "string" ? sourceObjective : "",
  ].some((candidate) => candidate.trim() === expected);
}
