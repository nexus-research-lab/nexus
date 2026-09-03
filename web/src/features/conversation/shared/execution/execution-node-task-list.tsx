/**
 * INPUT: 已通过 WorkAttempt 精确关联的 Conversation Task run。
 * OUTPUT: WorkGraph 节点浮层内紧凑、只读的局部步骤清单。
 * POS: Task 在 WorkGraph 中的唯一展示面；不创建第二套节点状态或独立任务面板。
 */
import { Circle, CircleCheck } from "lucide-react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import { resolveWorkspaceTaskState } from "@/shared/ui/workspace/surface/workspace-task-strip-model";
import type { TodoItem } from "@/types/conversation/todo";

const MAX_VISIBLE_TASKS = 5;

export function ExecutionNodeTaskList({
  run,
}: {
  run: ConversationTaskRun;
}) {
  const { t } = useI18n();
  const taskState = resolveWorkspaceTaskState(run.todos);
  if (!taskState) {
    return null;
  }
  const { summary } = taskState;
  const visibleTodos = resolveVisibleTodos(run.todos, summary.currentStep - 1);
  const hiddenCount = run.todos.length - visibleTodos.length;

  return (
    <section
      className="mt-2 border-t border-(--divider-subtle-color) pt-2"
      data-execution-node-task-agent-round={run.agentRoundId}
      data-execution-node-tasks
    >
      <header className="mb-1.5 flex items-center justify-between gap-2 text-2xs leading-4">
        <span className="font-medium text-(--text-default)">
          {t("execution.local_tasks")}
        </span>
        <span className="tabular-nums text-(--text-soft)">
          {summary.completedCount}/{summary.totalCount}
        </span>
      </header>
      <ol className="space-y-1">
        {visibleTodos.map(({ index, todo }) => (
          <li
            aria-label={taskStatusLabel(todo, t)}
            className="flex min-w-0 items-start gap-1.5 text-2xs leading-4"
            data-execution-node-task-status={todo.status}
            key={`${index}:${todo.content}`}
          >
            <TaskStatusIcon status={todo.status} />
            <span
              className={todo.status === "completed"
                ? "min-w-0 truncate text-(--text-soft) line-through"
                : "min-w-0 truncate text-(--text-default)"}
              title={todo.content}
            >
              {todo.status === "in_progress"
                ? todo.active_form?.trim() || todo.content
                : todo.content}
            </span>
          </li>
        ))}
      </ol>
      {hiddenCount > 0 ? (
        <p className="mt-1 pl-4 text-[9px] leading-4 text-(--text-soft)">
          {t("execution.more_local_tasks", { count: hiddenCount })}
        </p>
      ) : null}
    </section>
  );
}

function resolveVisibleTodos(
  todos: readonly TodoItem[],
  currentIndex: number,
): Array<{index: number; todo: TodoItem}> {
  if (todos.length <= MAX_VISIBLE_TASKS) {
    return todos.map((todo, index) => ({ index, todo }));
  }
  const start = Math.max(
    0,
    Math.min(currentIndex - 2, todos.length - MAX_VISIBLE_TASKS),
  );
  return todos.slice(start, start + MAX_VISIBLE_TASKS).map((todo, offset) => ({
    index: start + offset,
    todo,
  }));
}

function TaskStatusIcon({ status }: { status: TodoItem["status"] }) {
  if (status === "completed") {
    return (
      <CircleCheck
        aria-hidden="true"
        className="mt-0.5 h-3 w-3 shrink-0 text-(--success)"
      />
    );
  }
  return (
    <Circle
      aria-hidden="true"
      className={status === "in_progress"
        ? "mt-1 h-2 w-2 shrink-0 fill-current text-(--primary)"
        : "mt-1 h-2 w-2 shrink-0 text-(--icon-muted)"}
    />
  );
}

function taskStatusLabel(
  todo: TodoItem,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (todo.status === "completed") {
    return t("tasks.status_completed");
  }
  if (todo.status === "in_progress") {
    return t("tasks.status_in_progress");
  }
  return t("tasks.status_pending");
}
