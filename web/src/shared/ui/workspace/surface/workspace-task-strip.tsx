"use client";

import { ChevronDown, ChevronUp, Circle, CircleCheck, ListChecks } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { TodoItem } from "@/types/conversation/todo";

interface WorkspaceTaskPanelProps {
  todos: TodoItem[];
  className?: string;
}

const TASK_PANEL_SURFACE_CLASS_NAME =
  "border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_97%,white)] shadow-[0_8px_24px_rgba(15,23,42,0.1)]";

export function WorkspaceTaskPanel({
  todos,
  className,
}: WorkspaceTaskPanelProps) {
  const { t } = useI18n();
  const hasTasks = todos.length > 0;
  const completedCount = todos.filter((todo) => todo.status === "completed").length;
  const hasRunningTask = todos.some((todo) => todo.status === "in_progress");
  const [isExpanded, setIsExpanded] = useState(false);
  const [expandedTaskIndex, setExpandedTaskIndex] = useState<number | null>(null);

  useEffect(() => {
    if (!hasTasks) {
      setIsExpanded(false);
      setExpandedTaskIndex(null);
    }
  }, [hasTasks]);

  useEffect(() => {
    if (expandedTaskIndex !== null && expandedTaskIndex >= todos.length) {
      setExpandedTaskIndex(null);
    }
  }, [expandedTaskIndex, todos.length]);

  if (!hasTasks) {
    return null;
  }

  const renderStatusMarker = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return <CircleCheck className="h-[18px] w-[18px] text-(--success)" />;
    }
    if (status === "in_progress") {
      return <Circle className="h-2.5 w-2.5 fill-current text-(--primary)" />;
    }
    return <Circle className="h-2.5 w-2.5 text-(--icon-muted)" />;
  };

  return (
    <aside
      aria-label={t("tasks.label")}
      aria-live="polite"
      className={cn(
        "pointer-events-none absolute inset-x-0 bottom-2 top-2 z-30 flex items-start justify-end px-2.5 sm:px-3",
        className,
      )}
    >
      {isExpanded ? (
        <section
          className={cn(
            "pointer-events-auto flex max-h-full w-[300px] max-w-full flex-col overflow-hidden rounded-[10px]",
            TASK_PANEL_SURFACE_CLASS_NAME,
          )}
        >
          <div className="flex h-9 shrink-0 items-center gap-2 px-3">
            <span className="text-[12px] font-semibold text-(--text-strong)">
              {t("tasks.label")}
            </span>
            <span className="text-[12px] tabular-nums text-(--text-soft)">
              {completedCount}/{todos.length}
            </span>
            <span className="flex-1" />
            {hasRunningTask ? <LoadingOrb /> : null}
            <button
              aria-label={t("tasks.collapse_panel")}
              className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-[6px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
              onClick={() => setIsExpanded(false)}
              title={t("tasks.collapse_panel")}
              type="button"
            >
              <ChevronUp className="h-3.5 w-3.5" />
            </button>
          </div>

          <div className="soft-scrollbar min-h-0 overflow-y-auto pb-1.5">
            {todos.map((todo, index) => {
              const detailText = todo.active_form?.trim() || "";
              const hasDetail = detailText.length > 0 && detailText !== todo.content.trim();
              const isDetailExpanded = expandedTaskIndex === index;

              return (
                <div
                  className="flex min-w-0 items-start gap-2 px-3 py-1.5"
                  key={`${todo.content}-${index}`}
                >
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center">
                    {renderStatusMarker(todo.status)}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p
                      className={cn(
                        "text-[12px] leading-5 text-(--text-default)",
                        todo.status === "completed" && "text-(--text-soft) line-through",
                      )}
                    >
                      {todo.content}
                    </p>
                    {isDetailExpanded && hasDetail ? (
                      <p className="mt-0.5 border-l border-(--divider-subtle-color) pl-2 text-[10.5px] leading-4.5 text-(--text-muted)">
                        {detailText}
                      </p>
                    ) : null}
                  </div>
                  {hasDetail ? (
                    <button
                      aria-expanded={isDetailExpanded}
                      aria-label={isDetailExpanded ? t("tasks.collapse_detail") : t("tasks.expand_detail")}
                      className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-[5px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
                      onClick={() => setExpandedTaskIndex((currentIndex) => (
                        currentIndex === index ? null : index
                      ))}
                      type="button"
                    >
                      <ChevronDown
                        className={cn(
                          "h-3.5 w-3.5 transition-transform duration-200",
                          isDetailExpanded && "rotate-180",
                        )}
                      />
                    </button>
                  ) : null}
                </div>
              );
            })}
          </div>
        </section>
      ) : (
        <button
          aria-label={t("tasks.expand_panel")}
          className={cn(
            "pointer-events-auto inline-flex h-7 items-center gap-1.5 rounded-[7px] px-2.5 text-[11px] font-semibold text-(--text-default) transition-[background,border-color,color,box-shadow] hover:text-(--text-strong)",
            TASK_PANEL_SURFACE_CLASS_NAME,
          )}
          onClick={() => setIsExpanded(true)}
          title={t("tasks.expand_panel")}
          type="button"
        >
          <ListChecks className="h-3.5 w-3.5" />
          <span className="tabular-nums">{completedCount}/{todos.length}</span>
          {hasRunningTask ? <LoadingOrb /> : null}
          <ChevronDown className="h-3.5 w-3.5 text-(--icon-muted)" />
        </button>
      )}
    </aside>
  );
}
