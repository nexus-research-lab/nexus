import { CheckSquare, LoaderCircle } from "lucide-react";

import { LoadingOrb } from "@/shared/ui/loading-orb";
import { TodoItem } from "@/types/todo";

interface RoomProgressSectionProps {
  todos: TodoItem[];
}

export function RoomProgressSection({ todos }: RoomProgressSectionProps) {
  const completedTodoCount = todos.filter((todo) => todo.status === "completed").length;
  const activeTodo = todos.find((todo) => todo.status === "in_progress") ?? null;
  const visible_todos = todos.slice(0, 4);

  return (
    <section className="border-b workspace-divider px-4 py-4">
      <div className="mb-3 flex items-center justify-between gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
        <div className="flex items-center gap-2">
          <CheckSquare className="h-3.5 w-3.5" />
          当前计划
        </div>
        <div className="workspace-chip flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium tracking-normal normal-case text-slate-700/54">
          {todos.length > 0 && <LoadingOrb />}
          <span>{todos.length === 0 ? "0 / 0" : `${completedTodoCount} / ${todos.length}`}</span>
        </div>
      </div>
      {activeTodo && (
        <div className="workspace-card flex items-start gap-2 rounded-[22px] px-3 py-3 text-sm text-slate-900/84">
          <LoaderCircle className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-primary" />
          <div className="min-w-0">
            <p className="truncate font-medium">{activeTodo.content}</p>
            {activeTodo.active_form && (
              <p className="mt-0.5 text-[11px] text-slate-700/52">{activeTodo.active_form}</p>
            )}
          </div>
        </div>
      )}
      {todos.length > 0 && (
        <div className="mt-3 space-y-1">
          {visible_todos.map((todo, index) => (
            <div
              key={`${index}-${todo.content}`}
              className="workspace-card flex items-start gap-2 rounded-[18px] px-3 py-2 text-[11px]"
            >
              <span
                className={
                  todo.status === "completed"
                    ? "text-success"
                    : todo.status === "in_progress"
                      ? "text-primary"
                      : "text-muted-foreground"
                }
              >
                {todo.status === "completed" ? "✓" : todo.status === "in_progress" ? "•" : "○"}
              </span>
              <span className="min-w-0 flex-1 break-words text-slate-900/84">{todo.content}</span>
            </div>
          ))}
          {todos.length > visible_todos.length ? (
            <p className="px-1 text-[11px] text-slate-700/52">
              还有 {todos.length - visible_todos.length} 项已折叠
            </p>
          ) : null}
        </div>
      )}
      {todos.length === 0 && (
        <p className="text-[11px] text-slate-700/52">当前没有活跃推进项。</p>
      )}
    </section>
  );
}
