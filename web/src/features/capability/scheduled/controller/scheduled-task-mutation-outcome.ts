// INPUT: Scheduled mutation 抛出的旧/新失败与用户级兜底文案。
// OUTPUT: 复用共享 mutation failure 投影，只补充 Scheduled 的同动作锁定决策。
// POS: Scheduled 命令恢复投影，不决定重试、发送请求或清除结果未知锁。

import {
  getResourceFailure,
  projectMutationFailure,
  type MutationFailure,
  type ResourceAccessFailure,
} from "@/lib/error-message";

export interface ScheduledTaskMutationFailureProjection extends MutationFailure {
  access: ResourceAccessFailure | null;
  blocksRepeat: boolean;
}

export function projectScheduledTaskMutationFailure(
  error: unknown,
  fallbackMessage: string,
): ScheduledTaskMutationFailureProjection {
  const failure = projectMutationFailure(error, fallbackMessage);
  return {
    ...failure,
    access: getResourceFailure(error, fallbackMessage).access,
    blocksRepeat: failure.effect !== "not_applied",
  };
}
