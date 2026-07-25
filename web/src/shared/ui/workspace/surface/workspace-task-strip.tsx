/**
 * INPUT: 当前会话投影出的任务列表。
 * OUTPUT: 位于聊天画布顶部的任务摘要与锚定浮层明细。
 * POS: Workspace 会话级任务状态条；展开不得改变聊天视口高度。
 */
"use client";

import { ChevronDown, ChevronUp, Circle, CircleCheck, ListChecks } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import type { TodoItem } from "@/types/conversation/todo";

interface WorkspaceTaskPanelProps {
  todos: TodoItem[];
  className?: string;
}

const TASK_PANEL_TRIGGER_CLASS_NAME =
  "border border-(--surface-control-border) bg-(--surface-control-background) shadow-(--surface-control-shadow)";

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
      data-workspace-task-panel
      className={cn(
        "relative z-30 h-11 shrink-0",
        className,
      )}
    >
      <div className="pointer-events-none absolute inset-x-2.5 top-2 flex justify-end sm:inset-x-3">
        {isExpanded ? (
          <section
            className={cn(
              "pointer-events-auto flex max-h-[min(320px,42vh)] w-[300px] max-w-full origin-top-right flex-col overflow-hidden",
              OVERLAY_SURFACE_CLASS_NAME,
              ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            )}
            data-placement="bottom"
          >
            <div className="flex h-9 shrink-0 items-center gap-2 px-3">
              <span className="text-compact font-semibold text-(--text-strong)">
                {t("tasks.label")}
              </span>
              <span className="text-compact tabular-nums text-(--text-soft)">
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
                          "text-compact leading-5 text-(--text-default)",
                          todo.status === "completed" && "text-(--text-soft) line-through",
                        )}
                      >
                        {todo.content}
                      </p>
                      {isDetailExpanded && hasDetail ? (
                        <p className="mt-0.5 border-l border-(--divider-subtle-color) pl-2 text-xs leading-4.5 text-(--text-muted)">
                          {detailText}
                        </p>
                      ) : null}
                    </div>
                    {hasDetail ? (
                      <button
                        aria-expanded={isDetailExpanded}
                        aria-label={isDetailExpanded ? t("tasks.collapse_detail") : t("tasks.expand_detail")}
                        className="inline-flex h-5 w-5 shrink-0 items-center justify-center radius-control-xs text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
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
              "pointer-events-auto inline-flex h-7 items-center gap-1.5 radius-control-sm px-2.5 text-xs font-semibold text-(--text-default) transition-[background,border-color,color,box-shadow] hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--text-strong)",
              TASK_PANEL_TRIGGER_CLASS_NAME,
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
      </div>
    </aside>
  );
}
