/**
 * INPUT: 已通过 WorkAttempt 精确关联的 Conversation Task run。
 * OUTPUT: 复用已归一化任务的可读局部步骤清单；默认聚焦当前步骤，可展开完整只读内容。
 * POS: Task 在 WorkGraph 中的唯一展示面；不创建第二套节点状态或独立任务面板。
 */
import { Circle, CircleCheck } from "lucide-react";
import { useId } from "react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { resolveWorkspaceTaskState } from "@/shared/ui/workspace/surface/workspace-task-strip-model";
import type { TodoItem } from "@/types/conversation/todo";

const MAX_VISIBLE_TASKS = 5;

export function ExecutionNodeTaskList({
  run,
}: {
  run: ConversationTaskRun;
}) {
  const { t } = useI18n();
  const listId = useId();
  const [expanded, setExpanded] = useResettableState(
    false, JSON.stringify([run.agentId, run.agentRoundId]),
  );
  const taskState = resolveWorkspaceTaskState(run.todos);
  if (!taskState) {
    return null;
  }
  const { summary, todos } = taskState;
  const visibleTodos = expanded
    ? todos.map((todo, index) => ({ index, todo }))
    : resolveVisibleTodos(todos, summary.currentStep - 1);
  const hiddenCount = todos.length - visibleTodos.length;

  return (
    <section
      aria-labelledby={`${listId}-heading`}
      className="mt-2 border-t border-(--divider-subtle-color) pt-2"
      data-execution-node-task-agent-round={run.agentRoundId}
      data-execution-node-tasks
    >
      <header className="mb-1.5 flex items-center justify-between gap-2">
        <h4
          className={getUiTypographyClassName({ role: "metadata", tone: "default", weight: "medium" })}
          id={`${listId}-heading`}
        >
          {t("execution.local_tasks")}
        </h4>
        <span className={cn("tabular-nums", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
          {summary.completedCount}/{summary.totalCount}
        </span>
      </header>
      <ol className="space-y-1.5" id={listId}>
        {visibleTodos.map(({ index, todo }) => {
          const label = todo.status === "in_progress" ? todo.active_form || todo.content : todo.content;
          return (
            <li
              className={cn("flex min-w-0 items-start gap-1.5", getUiTypographyClassName({ role: "supporting" }))}
              data-execution-node-task-status={todo.status}
              key={`${index}:${todo.content}`}
              value={index + 1}
            >
              <TaskStatusIcon status={todo.status} />
              <span className="sr-only">{taskStatusLabel(todo, t)}: </span>
              <span
                className={todo.status === "completed"
                  ? "min-w-0 break-words text-(--text-soft) line-through [overflow-wrap:anywhere]"
                  : "min-w-0 break-words text-(--text-default) [overflow-wrap:anywhere]"}
                title={label}
              >
                {label}
              </span>
            </li>
          );
        })}
      </ol>
      {todos.length > MAX_VISIBLE_TASKS ? (
        <UiButton
          aria-controls={listId}
          aria-expanded={expanded}
          className="mt-1"
          onClick={() => setExpanded((value) => !value)}
          size="2xs"
          variant="text"
        >
          {expanded
            ? t("execution.collapse_local_tasks")
            : t("execution.more_local_tasks", { count: hiddenCount })}
        </UiButton>
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
  return (
    <span aria-hidden="true" className="mt-0.5 grid h-4 w-4 shrink-0 place-items-center">
      {status === "completed" ? (
        <CircleCheck className="h-3 w-3 text-(--success)" />
      ) : (
        <Circle
          className={status === "in_progress"
            ? "h-2 w-2 fill-current text-(--primary)"
            : "h-2 w-2 text-(--icon-muted)"}
        />
      )}
    </span>
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
