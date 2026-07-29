/**
 * INPUT: 当前会话投影出的任务列表。
 * OUTPUT: 具备 44px 局部热区、当前步骤摘要、顺序键盘导航与可读状态的居中任务胶囊及向上明细。
 * POS: 锚在 Composer 顶边的 Workspace 会话级只读任务入口。
 */
"use client";

import { ChevronDown, ChevronUp, Circle, CircleCheck, ListChecks } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import type { TodoItem } from "@/types/conversation/todo";

import { resolveWorkspaceTaskSummary } from "./workspace-task-strip-model";

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
  const taskSummary = resolveWorkspaceTaskSummary(todos);
  const hasTasks = taskSummary !== null;
  const [isExpanded, setIsExpanded] = useState(false);
  const [expandedTaskIndex, setExpandedTaskIndex] = useState<number | null>(null);
  const panelId = useId();
  const triggerRef = useRef<HTMLButtonElement | null>(null);

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

  const {
    completedCount,
    currentStep,
    hasRunningTask,
    summary,
    totalCount,
  } = taskSummary;

  const taskStatusLabel = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return t("tasks.status_completed");
    }
    if (status === "in_progress") {
      return t("tasks.status_in_progress");
    }
    return t("tasks.status_pending");
  };

  const renderStatusMarker = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return <CircleCheck aria-hidden="true" className="h-[18px] w-[18px] text-(--success)" />;
    }
    if (status === "in_progress") {
      return <Circle aria-hidden="true" className="h-2.5 w-2.5 fill-current text-(--primary)" />;
    }
    return <Circle aria-hidden="true" className="h-2.5 w-2.5 text-(--icon-muted)" />;
  };

  const collapsePanel = () => {
    triggerRef.current?.focus();
    setIsExpanded(false);
    setExpandedTaskIndex(null);
  };

  return (
    <aside
      aria-label={t("tasks.label")}
      aria-live="polite"
      data-workspace-task-panel
      className={cn(
        "pointer-events-none relative flex min-w-0 max-w-[min(32rem,calc(100vw-4rem))] justify-center",
        className,
      )}
    >
      <button
        ref={triggerRef}
        aria-controls={panelId}
        aria-expanded={isExpanded}
        aria-label={isExpanded ? t("tasks.collapse_panel") : t("tasks.expand_panel")}
        className={cn(
          "pointer-events-auto inline-flex h-11 min-w-0 max-w-full items-center gap-2 rounded-full px-3.5 text-xs text-(--text-default) transition-[background,border-color,color,box-shadow] hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--text-strong)",
          TASK_PANEL_TRIGGER_CLASS_NAME,
        )}
        data-workspace-task-summary={summary}
        data-workspace-task-trigger
        onClick={() => setIsExpanded((current) => !current)}
        type="button"
      >
        <span className="grid h-4 w-4 shrink-0 place-items-center">
          {hasRunningTask ? (
            <LoadingOrb />
          ) : completedCount === totalCount ? (
            <CircleCheck aria-hidden="true" className="h-4 w-4 text-(--success)" />
          ) : (
            <ListChecks aria-hidden="true" className="h-4 w-4 text-(--icon-muted)" />
          )}
        </span>
        <span className="shrink-0 font-medium tabular-nums">
          {t("tasks.step_progress", {
            current: currentStep,
            total: totalCount,
          })}
        </span>
        <span aria-hidden="true" className="shrink-0 text-(--text-soft)">·</span>
        <span className="min-w-0 truncate text-(--text-soft)">{summary}</span>
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-200",
            isExpanded && "rotate-180",
          )}
        />
      </button>
      {isExpanded ? (
        <div
          className="pointer-events-none absolute bottom-[calc(100%+0.5rem)] left-1/2 -translate-x-1/2"
        >
          <section
            aria-label={t("tasks.label")}
            className={cn(
              "pointer-events-auto flex max-h-[min(360px,45dvh)] w-[min(360px,calc(100vw-4rem))] origin-bottom flex-col overflow-hidden",
              OVERLAY_SURFACE_CLASS_NAME,
              ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            )}
            data-placement="top"
            id={panelId}
          >
            <div className="flex h-10 shrink-0 items-center gap-2 px-3">
              <span className="text-compact font-semibold text-(--text-strong)">
                {t("tasks.label")}
              </span>
              <span className="text-compact tabular-nums text-(--text-soft)">
                {completedCount}/{totalCount}
              </span>
              <span className="flex-1" />
              {hasRunningTask ? <LoadingOrb /> : null}
              <button
                aria-label={t("tasks.collapse_panel")}
                className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-[6px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
                onClick={collapsePanel}
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
                      <span className="sr-only">{taskStatusLabel(todo.status)}</span>
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
                        className="inline-flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
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
        </div>
      ) : null}
    </aside>
  );
}
