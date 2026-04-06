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
      backdropFilter: "blur(16px)",
    }
    : {
      background: "var(--chip-default-background)",
      border: "1px solid var(--chip-default-border)",
      backdropFilter: "blur(16px)",
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
            "inline-flex items-center gap-1.5 rounded-full text-left transition duration-150 ease-out",
            density === "compact" ? "h-[30px] px-2.5" : "h-8 px-3",
          )}
          style={trigger_style}
          onClick={handle_toggle_panel}
          type="button"
        >
          <ListChecks className="h-3.5 w-3.5 text-slate-700/88"/>
          <span className="text-[10px] font-semibold tracking-[0.08em] text-slate-700/94">
            {t("tasks.label")}
          </span>
          <span className="text-[10px] font-medium tabular-nums text-slate-600/88">
            {completed_count}/{total_count}
          </span>
          <div className="hidden w-14 overflow-hidden rounded-full bg-slate-200/88 sm:block">
            <div
              className="h-1 rounded-full bg-slate-700/74 transition-[width] duration-300"
              style={{width: `${progress}%`}}
            />
          </div>
          <span
            className="inline-flex items-center justify-center gap-1 text-[10px] font-medium tabular-nums text-slate-600/84">
            {has_running_task ? (
              <LoadingOrb />
            ) : active_count > 0 ? (
              <span className="h-2 w-2 rounded-full bg-slate-400/80"/>
            ) : null}
            {active_count}
          </span>
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 shrink-0 text-slate-500 transition-transform duration-300",
              is_open && "rotate-180 text-slate-700",
            )}
          />
        </button>

        {is_open ? (
          <div
            className="absolute right-0 top-[calc(100%+10px)] z-40 w-[min(540px,calc(100vw-48px))] overflow-hidden rounded-[20px]"
            style={{
              background: "var(--surface-popover-background)",
              border: "1px solid var(--surface-popover-border)",
              backdropFilter: "blur(18px)",
            }}
          >


            <div className="max-h-[320px] overflow-y-auto px-4">
              <div
                className="grid grid-cols-[36px_76px_minmax(0,1fr)_24px] items-center gap-3 border-b divider-subtle px-2 py-1.5 text-[9px] font-semibold uppercase tracking-[0.14em] text-slate-500/76">
                <span>{t("tasks.id")}</span>
                <span>{t("tasks.status")}</span>
                <span>{t("tasks.subject")}</span>
                <button
                  aria-label={t("tasks.close_panel")}
                  className="ml-1 inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  onClick={() => {
                    set_expanded_task_index(null);
                    set_is_open(false);
                  }}
                  type="button"
                >
                  <X className="h-3.5 w-3.5"/>
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
                            "grid w-full grid-cols-[36px_76px_minmax(0,1fr)_24px] gap-3 rounded-[12px] px-2 py-1.25 text-left transition-colors",
                            is_expanded && has_detail ? "" : "hover:bg-[var(--surface-interactive-hover-background)]",
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
                          <span className="pt-0.5 text-[10px] font-semibold tabular-nums text-slate-400">
                            #{index + 1}
                          </span>

                          <span
                            className={cn(
                              "inline-flex h-fit items-center gap-1 rounded-full px-2 py-0.5 text-[9.5px] font-semibold",
                              is_completed && "bg-emerald-100/90 text-emerald-700",
                              is_running && "bg-sky-100/92 text-sky-700",
                              todo.status === "pending" && "bg-slate-100/92 text-slate-600",
                            )}
                          >
                            {is_completed ? (
                              <Check className="h-3 w-3"/>
                            ) : is_running ? (
                              <LoaderCircle className="h-3 w-3 animate-spin"/>
                            ) : (
                              <Circle className="h-3 w-3"/>
                            )}
                            {is_completed ? t("tasks.done") : is_running ? t("tasks.running") : t("tasks.pending")}
                          </span>

                          <div className="min-w-0">
                            <p className="truncate text-[11px] font-medium text-slate-900/84">
                              {todo.content}
                            </p>
                          </div>

                          <span className="flex items-center justify-end">
                            {has_detail ? (
                              <ChevronDown
                                className={cn(
                                  "h-3.5 w-3.5 text-slate-400 transition-transform duration-300",
                                  is_expanded && "rotate-180 text-slate-600",
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
                                className="rounded-[12px] px-3 py-1.5 text-[10px] leading-5 text-slate-600/82"
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
                <p className="px-1 py-4 text-[11px] text-slate-500/76">
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
