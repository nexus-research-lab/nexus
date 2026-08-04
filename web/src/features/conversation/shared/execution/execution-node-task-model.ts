/**
 * INPUT: 一个 WorkGraph 节点的 Attempt 投影与按 Agent round 隔离的 Task run。
 * OUTPUT: 只通过同 Agent、同 agent_round_id 命中的节点局部 Task run。
 * POS: Execution 节点与 Conversation Task 之间唯一的安全关联边界；禁止按 Agent 或时间猜测归属。
 */
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import type { ExecutionWorkItemView } from "@/types/conversation/execution";

export function resolveExecutionNodeTaskRun(
  item: ExecutionWorkItemView,
  runs: readonly ConversationTaskRun[],
): ConversationTaskRun | null {
  const attempts = item.attempts ?? [];
  for (let attemptIndex = attempts.length - 1; attemptIndex >= 0; attemptIndex--) {
    const attempt = attempts[attemptIndex];
    if (attempt.executor_kind !== "agent") {
      continue;
    }
    const agentId = attempt.executor_agent_id?.trim() ?? "";
    const agentRoundId = attempt.agent_round_id?.trim() ?? "";
    if (!agentId || !agentRoundId) {
      return null;
    }
    for (let runIndex = runs.length - 1; runIndex >= 0; runIndex--) {
      const run = runs[runIndex];
      if (run.agentId === agentId && run.agentRoundId === agentRoundId) {
        return run;
      }
    }
    return null;
  }
  return null;
}
