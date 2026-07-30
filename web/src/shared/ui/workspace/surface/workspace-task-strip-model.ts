/**
 * INPUT: 当前会话投影出的只读任务列表。
 * OUTPUT: 当前步骤、总数、执行态与胶囊摘要的稳定纯投影。
 * POS: Workspace Task 浮动入口的无状态展示模型。
 */
import type { TodoItem } from "@/types/conversation/todo";

export interface WorkspaceTaskSummary {
  completedCount: number;
  currentStep: number;
  hasRunningTask: boolean;
  summary: string;
  totalCount: number;
}

export function resolveWorkspaceTaskSummary(
  todos: readonly TodoItem[],
): WorkspaceTaskSummary | null {
  if (todos.length === 0) {
    return null;
  }

  const completedCount = todos.filter((todo) => todo.status === "completed").length;
  const runningIndex = todos.findIndex((todo) => todo.status === "in_progress");
  const pendingIndex = todos.findIndex((todo) => todo.status === "pending");
  const currentIndex = runningIndex >= 0
    ? runningIndex
    : pendingIndex >= 0
      ? pendingIndex
      : todos.length - 1;
  const currentTask = todos[currentIndex];
  const activeSummary = currentTask.status === "in_progress"
    ? currentTask.active_form?.trim()
    : "";

  return {
    completedCount,
    currentStep: currentIndex + 1,
    hasRunningTask: runningIndex >= 0,
    summary: activeSummary || currentTask.content.trim(),
    totalCount: todos.length,
  };
}
