"use client";

import { Check, ChevronDown, Circle, ListChecks, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { TodoItem } from "@/types/conversation/todo";

interface WorkspaceTaskStripProps {
  todos: TodoItem[];
  density?: "default" | "compact";
}

const TASK_PANEL_SURFACE_CLASS_NAME =
  "border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_84%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_16px_36px_rgba(15,23,42,0.14)]";

export function WorkspaceTaskStrip({
  todos,
  density = "compact",
}: WorkspaceTaskStripProps) {
  const { t } = useI18n();
  const totalCount = todos.length;
  const completedCount = todos.filter((todo) => todo.status === "completed").length;
  const activeCount = todos.filter((todo) => todo.status !== "completed").length;
  const hasRunningTask = todos.some((todo) => todo.status === "in_progress");
  const progress = totalCount === 0 ? 0 : Math.round((completedCount / totalCount) * 100);
  const [isOpen, setIsOpen] = useState(false);
  const [expandedTaskIndex, setExpandedTaskIndex] = useState<number | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  if (expandedTaskIndex !== null && (todos.length === 0 || expandedTaskIndex >= todos.length)) {
    setExpandedTaskIndex(null);
  }

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && rootRef.current?.contains(target)) {
        return;
      }
      setExpandedTaskIndex(null);
      setIsOpen(false);
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    return () => document.removeEventListener("pointerdown", handlePointerDown, true);
  }, [isOpen]);

  const handleTogglePanel = () => {
    setExpandedTaskIndex(null);
    setIsOpen((prev) => !prev);
  };
  const triggerStyle = isOpen
    ? {
      background: "color-mix(in srgb, var(--background) 94%, white)",
      border: "1px solid color-mix(in srgb, var(--divider-subtle-color) 84%, transparent)",
    }
    : {
      background: "var(--chip-default-background)",
      border: "1px solid var(--chip-default-border)",
    };

  const renderStatusMarker = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return <Check className="h-3.5 w-3.5 text-(--success)" />;
    }

    if (status === "in_progress") {
      return <Circle className="h-2.5 w-2.5 fill-current text-primary" />;
    }

    return <Circle className="h-2.5 w-2.5 text-(--icon-muted)" />;
  };

  const getStatusLabel = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return t("tasks.done");
    }
    if (status === "in_progress") {
      return t("tasks.running");
    }
    return t("tasks.pending");
  };

  return (
    <div ref={rootRef} className="relative">
      <div className="relative z-40">
        <button
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full text-left transition duration-(--motion-duration-fast) ease-out",
            density === "compact" ? "h-7 px-2" : "h-8 px-3",
          )}
          style={triggerStyle}
          onClick={handleTogglePanel}
          type="button"
        >
          <ListChecks className="h-3.5 w-3.5 text-(--icon-default)" />
          <span className={cn("font-medium tracking-[0.06em] text-(--text-default)", density === "compact" ? "text-[10px]" : "text-2xs")}>
            {t("tasks.label")}
          </span>
          <span className={cn("font-normal tabular-nums text-(--text-soft)", density === "compact" ? "text-[9.5px]" : "text-[10px]")}>
            {completedCount}/{totalCount}
          </span>
          <div className="hidden w-14 overflow-hidden rounded-full bg-(--surface-progress-track) sm:block">
            <div
              className="h-1 rounded-full bg-(--surface-progress-fill) transition-[width] duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          <span
            className={cn(
              "inline-flex items-center justify-center gap-1 tabular-nums text-(--text-soft)",
              density === "compact" ? "text-[9.5px] font-normal" : "text-[10px] font-normal",
            )}>
            {hasRunningTask ? (
              <LoadingOrb />
            ) : activeCount > 0 ? (
              <span className="h-2 w-2 rounded-full bg-(--icon-muted)" />
            ) : null}
            {activeCount}
          </span>
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-300",
              isOpen && "rotate-180 text-(--icon-default)",
            )}
          />
        </button>

        {isOpen ? (
          <div
            className={cn(
              "absolute right-0 top-[calc(100%+8px)] z-40 w-[min(520px,calc(100vw-44px))] overflow-hidden rounded-[16px]",
              TASK_PANEL_SURFACE_CLASS_NAME,
            )}
          >
            <div className="soft-scrollbar max-h-[18.5rem] overflow-y-auto p-2">
              <div
                className="grid grid-cols-[36px_88px_minmax(0,1fr)_20px] items-center gap-3 rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_64%,transparent)] px-2 py-1.5 text-[9.5px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                <span className="text-center">{t("tasks.id")}</span>
                <span className="text-center">{t("tasks.status")}</span>
                <span className="text-center">{t("tasks.subject")}</span>
                <button
                  aria-label={t("tasks.close_panel")}
                  className="ml-1 inline-flex h-5 w-5 items-center justify-center rounded-full text-(--icon-muted) transition-[background,color] hover:bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] hover:text-(--icon-default)"
                  onClick={() => {
                    setExpandedTaskIndex(null);
                    setIsOpen(false);
                  }}
                  type="button"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>

              {todos.length ? (
                <div className="mt-1.5 space-y-1.5">
                  {todos.map((todo, index) => {
                    const isCompleted = todo.status === "completed";
                    const isRunning = todo.status === "in_progress";
                    const detailText = todo.active_form?.trim() || "";
                    const hasDetail = detailText.length > 0 && detailText !== todo.content.trim();
                    const isExpanded = expandedTaskIndex === index;
                    return (
                      <div
                        key={`${todo.content}-${index}`}
                        className="rounded-[12px]"
                      >
                        <button
                          className={cn(
                            "grid w-full grid-cols-[36px_88px_minmax(0,1fr)_20px] gap-3 rounded-[12px] border px-2 py-1.5 text-left transition-[background,border-color]",
                            isExpanded && hasDetail
                              ? "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)]"
                              : "border-[color:color-mix(in_srgb,var(--divider-subtle-color)_58%,transparent)] bg-transparent hover:border-[color:color-mix(in_srgb,var(--primary)_16%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)]",
                          )}
                          onClick={() => {
                            if (!hasDetail) {
                              return;
                            }
                            setExpandedTaskIndex((prev) => prev === index ? null : index);
                          }}
                          type="button"
                        >
                          <span className="pt-0.5 text-center text-[10px] font-medium tabular-nums text-(--text-soft)">
                            #{index + 1}
                          </span>

                          <span
                            className={cn(
                            "inline-flex items-center justify-center gap-1.5 pt-0.25 text-[10.5px] font-medium",
                              isCompleted && "text-(--success)",
                              isRunning && "text-primary",
                              todo.status === "pending" && "text-(--text-muted)",
                            )}
                          >
                            {renderStatusMarker(todo.status)}
                            {getStatusLabel(todo.status)}
                          </span>

                          <div className="min-w-0">
                            <p className="truncate text-[12.5px] font-medium text-(--text-strong)">
                              {todo.content}
                            </p>
                          </div>

                          <span className="flex items-center justify-end">
                            {hasDetail ? (
                              <ChevronDown
                                className={cn(
                                  "h-3.5 w-3.5 text-(--icon-muted) transition-transform duration-300",
                                  isExpanded && "rotate-180 text-(--icon-default)",
                                )}
                              />
                            ) : null}
                          </span>
                        </button>

                        <div
                          className={cn(
                            "grid transition-[grid-template-rows,opacity] duration-300 ease-out",
                            isExpanded && hasDetail ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0",
                          )}
                        >
                          <div className="min-h-0 overflow-hidden">
                            <div className="px-[calc(36px+76px+0.75rem)] pb-1.5 pr-2">
                              <div
                                className="border-l border-(--divider-subtle-color) pl-3 text-[10.5px] leading-5 text-(--text-muted)"
                                style={{
                                  background: "transparent",
                                }}
                              >
                                {detailText}
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="mt-1.5 rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_58%,transparent)] px-2.5 py-4 text-xs text-(--text-soft)">
                  {t("tasks.no_active")}
                </p>
              )}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
