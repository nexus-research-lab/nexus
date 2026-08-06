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

export interface WorkspaceTaskState {
  summary: WorkspaceTaskSummary;
  todos: TodoItem[];
}

/** 外部 transcript 与旧 runtime 载荷进入 Workspace UI 前的最后一道归一化。 */
function normalizeWorkspaceTaskTodos(
  todos: readonly TodoItem[] | null | undefined,
): TodoItem[] {
  const values: readonly unknown[] = Array.isArray(todos) ? todos : [];
  return values.flatMap((value) => {
    if (typeof value !== "object" || value === null || Array.isArray(value)) {
      return [];
    }
    const task = value as Record<string, unknown>;
    const rawContent = typeof task.content === "string"
      ? task.content
      : typeof task.task === "string"
        ? task.task
        : "";
    const content = rawContent.trim();
    const status = typeof task.status === "string" ? task.status.trim() : "";
    if (
      !content
      || (
        status !== "pending"
        && status !== "in_progress"
        && status !== "completed"
      )
    ) {
      return [];
    }
    const activeForm = typeof task.active_form === "string"
      ? task.active_form.trim()
      : typeof task.activeForm === "string"
        ? task.activeForm.trim()
        : "";
    return [{
      ...(activeForm ? { active_form: activeForm } : {}),
      content,
      status,
    }];
  });
}

export function resolveWorkspaceTaskState(
  todos: readonly TodoItem[] | null | undefined,
): WorkspaceTaskState | null {
  const normalizedTodos = normalizeWorkspaceTaskTodos(todos);
  if (normalizedTodos.length === 0) {
    return null;
  }

  const completedCount = normalizedTodos.filter(
    (todo) => todo.status === "completed",
  ).length;
  const runningIndex = normalizedTodos.findIndex(
    (todo) => todo.status === "in_progress",
  );
  const pendingIndex = normalizedTodos.findIndex(
    (todo) => todo.status === "pending",
  );
  const currentIndex = runningIndex >= 0
    ? runningIndex
    : pendingIndex >= 0
      ? pendingIndex
      : normalizedTodos.length - 1;
  const currentTask = normalizedTodos[currentIndex];
  const activeSummary = currentTask.status === "in_progress"
    ? currentTask.active_form?.trim()
    : "";

  return {
    summary: {
      completedCount,
      currentStep: currentIndex + 1,
      hasRunningTask: runningIndex >= 0,
      summary: activeSummary || currentTask.content.trim(),
      totalCount: normalizedTodos.length,
    },
    todos: normalizedTodos,
  };
}
