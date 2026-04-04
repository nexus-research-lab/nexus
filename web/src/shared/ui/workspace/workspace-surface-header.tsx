"use client";

import { Check, ChevronDown, Circle, ListChecks, LoaderCircle, LucideIcon, X, } from "lucide-react";
import { ReactNode, useEffect, useState } from "react";

import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { TodoItem } from "@/types/todo";

interface WorkspaceSurfaceHeaderTab<TTabKey extends string> {
  key: TTabKey;
  label: string;
  icon?: LucideIcon;
}

interface WorkspaceSurfaceHeaderProps<TTabKey extends string> {
  title: string;
  badge?: string;
  leading?: ReactNode;
  title_trailing?: ReactNode;
  subtitle?: ReactNode;
  trailing?: ReactNode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabs_trailing?: ReactNode;
  active_tab?: TTabKey;
  on_change_tab?: (tab: TTabKey) => void;
  class_name?: string;
}

export function WorkspaceSurfaceHeader<TTabKey extends string>({
                                                                 title,
                                                                 badge,
                                                                 leading,
                                                                 title_trailing,
                                                                 subtitle,
                                                                 trailing,
                                                                 tabs = [],
                                                                 tabs_trailing,
                                                                 active_tab,
                                                                 on_change_tab,
                                                                 class_name,
                                                               }: WorkspaceSurfaceHeaderProps<TTabKey>) {
  return (
    <div className={cn("relative z-10 border-b workspace-divider bg-white/60", class_name)}>
      <div className="flex min-w-0 items-center justify-between gap-4 px-5 py-3 xl:px-6">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          {leading ? (
            <div
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-white/40 bg-white/60 text-slate-700 shadow-sm">
              {leading}
            </div>
          ) : null}

          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-2">
              <div className="truncate text-[17px] font-black tracking-[-0.04em] text-slate-950/88">
                {title}
              </div>
              {badge ? (
                <span
                  className="inline-flex rounded-full border border-white/40 bg-white/56 px-2 py-0.5 text-[10px] font-semibold text-slate-600">
                  {badge}
                </span>
              ) : null}
              {title_trailing ? (
                <div className="min-w-0 shrink">{title_trailing}</div>
              ) : null}
            </div>
            {subtitle ? (
              <div className="mt-1 flex items-center gap-2 text-[12px] text-slate-700/52">
                {subtitle}
              </div>
            ) : null}
          </div>
        </div>

        {trailing ? (
          <div className="ml-3 flex shrink-0 items-center gap-2">
            {trailing}
          </div>
        ) : null}
      </div>

      {tabs.length || tabs_trailing ? (
        <div className="flex min-w-0 items-center gap-3 px-5 pb-2 xl:px-6">
          <div className="flex min-w-0 flex-1 items-center gap-1">
            {tabs.map((tab) => {
              const Icon = tab.icon;
              const is_active = active_tab === tab.key;
              return (
                <button
                  key={tab.key}
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[12px] font-semibold transition-all",
                    is_active
                      ? "workspace-chip text-slate-950 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
                      : "border-transparent text-slate-600 hover:bg-slate-100 hover:text-slate-950",
                  )}
                  onClick={() => on_change_tab?.(tab.key)}
                  type="button"
                >
                  {Icon ? <Icon className="h-3.5 w-3.5"/> : null}
                  {tab.label}
                </button>
              );
            })}
          </div>
          {tabs_trailing ? (
            <div className="shrink-0">
              {tabs_trailing}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

interface WorkspaceTaskStripProps {
  todos: TodoItem[];
  class_name?: string;
}

export function WorkspaceTaskStrip({
                                     todos,
                                     class_name,
}: WorkspaceTaskStripProps) {
  const total_count = todos.length;
  const completed_count = todos.filter((todo) => todo.status === "completed").length;
  const active_count = todos.filter((todo) => todo.status !== "completed").length;
  const has_running_task = todos.some((todo) => todo.status === "in_progress");
  const progress = total_count === 0 ? 0 : Math.round((completed_count / total_count) * 100);
  const [is_open, set_is_open] = useState(false);
  const [expanded_task_index, set_expanded_task_index] = useState<number | null>(null);

  useEffect(() => {
    // 中文注释：任务列表变化时，只清理已经失效的展开项，默认保持二级收起。
    if (todos.length === 0 || (expanded_task_index !== null && expanded_task_index >= todos.length)) {
      set_expanded_task_index(null);
    }
  }, [expanded_task_index, todos.length]);

  const handle_toggle_panel = () => {
    set_expanded_task_index(null);
    set_is_open((prev) => !prev);
  };

  return (
    <div className={cn("relative", class_name)}>
      {is_open ? (
        <button
          aria-label="关闭任务面板"
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
            "inline-flex h-8 items-center gap-2 rounded-full border border-white/70 bg-white/70 px-3 text-left shadow-[0_8px_18px_rgba(106,124,158,0.12)] backdrop-blur-xl transition-all",
            is_open && "border-slate-200/70 bg-white text-slate-950 shadow-[0_12px_28px_rgba(106,124,158,0.16)]",
          )}
          onClick={handle_toggle_panel}
          type="button"
        >
          <ListChecks className="h-3.5 w-3.5 text-slate-600/84"/>
          <span className="text-[11px] font-semibold tracking-[0.08em] text-slate-700/88">
            TASKS
          </span>
          <span className="text-[11px] font-medium tabular-nums text-slate-500/88">
            {completed_count}/{total_count}
          </span>
          <div className="hidden w-16 overflow-hidden rounded-full bg-slate-200/80 sm:block">
            <div
              className="h-1.5 rounded-full bg-slate-700/74 transition-[width] duration-300"
              style={{width: `${progress}%`}}
            />
          </div>
          <span
            className="inline-flex items-center justify-center gap-1 text-[11px] font-medium tabular-nums text-slate-500/84">
            {has_running_task ? (
              <LoadingOrb />
            ) : active_count > 0 ? (
              <span className="h-2 w-2 rounded-full bg-slate-400/80"/>
            ) : null}
            {active_count}
          </span>
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 shrink-0 text-slate-400 transition-transform duration-300",
              is_open && "rotate-180 text-slate-600",
            )}
          />
        </button>

        {is_open ? (
          <div
            className="absolute right-0 top-[calc(100%+10px)] z-40 w-[min(560px,calc(100vw-48px))] overflow-hidden rounded-[22px] border border-white/80 bg-white/94 shadow-[0_18px_44px_rgba(106,124,158,0.18)] backdrop-blur-xl">


            <div className="max-h-[320px] overflow-y-auto px-4">
              <div
                className="grid grid-cols-[40px_84px_minmax(0,1fr)_28px] gap-3 border-b border-slate-200/60 px-2 py-1 text-[9px] items-center font-semibold uppercase tracking-[0.14em] text-slate-500/76">
                <span>ID</span>
                <span>Status</span>
                <span>Subject</span>
                <button
                  aria-label="关闭任务面板"
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
                <div className="divide-y divide-slate-200/60">
                  {todos.map((todo, index) => {
                    const is_completed = todo.status === "completed";
                    const is_running = todo.status === "in_progress";
                    const detail_text = todo.active_form?.trim() || "";
                    const has_detail = detail_text.length > 0 && detail_text !== todo.content.trim();
                    const is_expanded = expanded_task_index === index;
                    return (
                      <div
                        key={`${todo.content}-${index}`}
                        className="py-1"
                      >
                        <button
                          className={cn(
                            "grid w-full grid-cols-[40px_84px_minmax(0,1fr)_28px] gap-3 rounded-[14px] px-2 py-1.5 text-left transition-colors",
                            is_expanded && has_detail ? "bg-white/78" : "hover:bg-white/62",
                          )}
                          onClick={() => {
                            if (!has_detail) {
                              return;
                            }
                            set_expanded_task_index((prev) => prev === index ? null : index);
                          }}
                          type="button"
                        >
                          <span className="pt-0.5 text-[11px] font-semibold tabular-nums text-slate-400">
                            #{index + 1}
                          </span>

                          <span
                            className={cn(
                              "inline-flex h-fit items-center gap-1 rounded-full px-2 py-0.75 text-[10px] font-semibold",
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
                            {is_completed ? "Done" : is_running ? "Running" : "Pending"}
                          </span>

                          <div className="min-w-0">
                            <p className="truncate text-[12px] font-medium text-slate-900/84">
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
                            <div className="px-[calc(40px+84px+0.75rem)] pb-1.5 pr-2">
                              <div
                                className="workspace-card rounded-[14px] px-3 py-2 text-[11px] leading-5 text-slate-600/82">
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
                  当前没有活跃任务。
                </p>
              )}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
