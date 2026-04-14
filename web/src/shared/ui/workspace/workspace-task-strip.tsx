"use client";

import { Check, ChevronDown, Circle, ListChecks, LoaderCircle, X, } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { TodoItem } from "@/types/todo";

interface WorkspaceTaskStripProps {
  todos: TodoItem[];
  density?: "default" | "compact";
}

export function WorkspaceTaskStrip({
  todos,
  density = "compact",
}: WorkspaceTaskStripProps) {
  const { t } = useI18n();
  const total_count = todos.length;
  const completed_count = todos.filter((todo) => todo.status === "completed").length;
  const active_count = todos.filter((todo) => todo.status !== "completed").length;
  const has_running_task = todos.some((todo) => todo.status === "in_progress");
  const progress = total_count === 0 ? 0 : Math.round((completed_count / total_count) * 100);
  const [is_open, set_is_open] = useState(false);
  const [expanded_task_index, set_expanded_task_index] = useState<number | null>(null);

  useEffect(() => {
    if (todos.length === 0 || (expanded_task_index !== null && expanded_task_index >= todos.length)) {
      set_expanded_task_index(null);
    }
  }, [expanded_task_index, todos.length]);

  const handle_toggle_panel = () => {
    set_expanded_task_index(null);
    set_is_open((prev) => !prev);
  };
  const trigger_style = is_open
    ? {
      background: "var(--surface-popover-background)",
      border: "1px solid var(--surface-popover-border)",
    }
    : {
      background: "var(--chip-default-background)",
      border: "1px solid var(--chip-default-border)",
    };

  return (
    <div className="relative">
      {is_open ? (
        <button
          aria-label={t("tasks.close_panel")}
          className="fixed inset-0 z-30"
          onClick={() => {
            set_expanded_task_index(null);
            set_is_open(false);
          }}
          type="button"
        />
      ) : null}

      <div className="relative z-40">
        <button
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full text-left transition duration-(--motion-duration-fast) ease-out",
            density === "compact" ? "h-7 px-2" : "h-8 px-3",
          )}
          style={trigger_style}
          onClick={handle_toggle_panel}
          type="button"
        >
          <ListChecks className="h-3.5 w-3.5 text-(--icon-default)" />
          <span className={cn("font-semibold tracking-[0.08em] text-(--text-default)", density === "compact" ? "text-[10px]" : "text-2xs")}>
            {t("tasks.label")}
          </span>
          <span className={cn("font-medium tabular-nums text-(--text-muted)", density === "compact" ? "text-[10px]" : "text-2xs")}>
            {completed_count}/{total_count}
          </span>
          <div className="hidden w-14 overflow-hidden rounded-full bg-(--surface-progress-track) sm:block">
            <div
              className="h-1 rounded-full bg-(--surface-progress-fill) transition-[width] duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          <span
            className="inline-flex items-center justify-center gap-1 text-2xs font-medium tabular-nums text-(--text-muted)">
            {has_running_task ? (
              <LoadingOrb />
            ) : active_count > 0 ? (
              <span className="h-2 w-2 rounded-full bg-(--icon-muted)" />
            ) : null}
            {active_count}
          </span>
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-300",
              is_open && "rotate-180 text-(--icon-default)",
            )}
          />
        </button>

        {is_open ? (
          <div
            className="surface-popover absolute right-0 top-[calc(100%+10px)] z-40 w-[min(540px,calc(100vw-48px))] overflow-hidden rounded-[20px]"
          >


            <div className="max-h-80 overflow-y-auto px-4">
              <div
                className="grid grid-cols-[36px_76px_minmax(0,1fr)_24px] items-center gap-3 border-b divider-subtle px-2 py-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
                <span>{t("tasks.id")}</span>
                <span>{t("tasks.status")}</span>
                <span>{t("tasks.subject")}</span>
                <button
                  aria-label={t("tasks.close_panel")}
                  className="ml-1 inline-flex h-6 w-6 items-center justify-center rounded-full text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
                  onClick={() => {
                    set_expanded_task_index(null);
                    set_is_open(false);
                  }}
                  type="button"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>

              {todos.length ? (
                <div className="divide-y divider-subtle">
                  {todos.map((todo, index) => {
                    const is_completed = todo.status === "completed";
                    const is_running = todo.status === "in_progress";
                    const detail_text = todo.active_form?.trim() || "";
                    const has_detail = detail_text.length > 0 && detail_text !== todo.content.trim();
                    const is_expanded = expanded_task_index === index;
                    return (
                      <div
                        key={`${todo.content}-${index}`}
                        className="py-0.5"
                      >
                        <button
                          className={cn(
                            "grid w-full grid-cols-[36px_76px_minmax(0,1fr)_24px] gap-3 rounded-xl px-2 py-1.25 text-left transition-colors",
                            is_expanded && has_detail ? "" : "hover:bg-(--surface-interactive-hover-background)",
                          )}
                          style={is_expanded && has_detail ? {
                            background: "var(--surface-interactive-active-background)",
                          } : undefined}
                          onClick={() => {
                            if (!has_detail) {
                              return;
                            }
                            set_expanded_task_index((prev) => prev === index ? null : index);
                          }}
                          type="button"
                        >
                          <span className="pt-0.5 text-[10px] font-semibold tabular-nums text-(--text-soft)">
                            #{index + 1}
                          </span>

                          <span
                            className={cn(
                              "inline-flex h-fit items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold",
                              is_completed && "bg-[color-mix(in_srgb,var(--success)_12%,transparent)] text-success",
                              is_running && "bg-[color-mix(in_srgb,var(--primary)_12%,transparent)] text-primary",
                              todo.status === "pending" && "bg-(--surface-panel-subtle-background) text-(--text-muted)",
                            )}
                          >
                            {is_completed ? (
                              <Check className="h-3 w-3" />
                            ) : is_running ? (
                              <LoaderCircle className="h-3 w-3 animate-spin" />
                            ) : (
                              <Circle className="h-3 w-3" />
                            )}
                            {is_completed ? t("tasks.done") : is_running ? t("tasks.running") : t("tasks.pending")}
                          </span>

                          <div className="min-w-0">
                            <p className="truncate text-[12.5px] font-medium text-(--text-strong)">
                              {todo.content}
                            </p>
                          </div>

                          <span className="flex items-center justify-end">
                            {has_detail ? (
                              <ChevronDown
                                className={cn(
                                  "h-3.5 w-3.5 text-(--icon-muted) transition-transform duration-300",
                                  is_expanded && "rotate-180 text-(--icon-default)",
                                )}
                              />
                            ) : null}
                          </span>
                        </button>

                        <div
                          className={cn(
                            "grid transition-[grid-template-rows,opacity] duration-300 ease-out",
                            is_expanded && has_detail ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0",
                          )}
                        >
                          <div className="min-h-0 overflow-hidden">
                            <div className="px-[calc(36px+76px+0.75rem)] pb-1.5 pr-2">
                              <div
                                className="rounded-xl px-3 py-1.5 text-[10.5px] leading-5 text-(--text-muted)"
                                style={{
                                  background: "var(--card-default-background)",
                                  border: "1px solid var(--card-default-border)",
                                }}
                              >
                                {detail_text}
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="px-1 py-4 text-xs text-(--text-soft)">
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
