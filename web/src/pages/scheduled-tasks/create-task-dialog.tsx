/**
 * 创建 / 编辑定时任务对话框
 *
 * 负责把表单状态转换成结构化的 scheduled task payload，并复用同一套 UI 处理创建和编辑。
 */

import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "lucide-react";

import { getAgentSessionsApi } from "@/lib/agent-api";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  getDialogActionClassName,
  getDialogChoiceClassName,
  getDialogChoiceStyle,
} from "@/shared/ui/dialog/dialog-styles";
import type { AgentSession } from "@/types/agent";
import type {
  CreateScheduledTaskParams,
  ScheduledTaskItem,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
  ScheduledTaskSessionTargetKind,
  UpdateScheduledTaskParams,
} from "@/types/scheduled-task";

type ScheduleKind = ScheduledTaskSchedule["kind"];
type EveryUnit = "seconds" | "minutes" | "hours" | "days";

interface ChoiceDef<TValue extends string> {
  key: TValue;
  label: string;
}

interface SessionOption {
  session_key: string;
  label: string;
}

interface CreateTaskDialogProps {
  agent_id: string;
  is_open: boolean;
  on_close: () => void;
  task?: ScheduledTaskItem | null;
  on_create_task: (params: CreateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  on_update_task: (job_id: string, params: UpdateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  on_saved?: (task: ScheduledTaskItem, mode: "create" | "edit") => void | Promise<void>;
}

const SESSION_TARGET_OPTIONS: ChoiceDef<ScheduledTaskSessionTargetKind>[] = [
  { key: "main", label: "主会话" },
  { key: "bound", label: "绑定现有会话" },
  { key: "named", label: "命名会话" },
  { key: "isolated", label: "独立会话" },
];

const SCHEDULE_OPTIONS: ChoiceDef<ScheduleKind>[] = [
  { key: "every", label: "循环间隔" },
  { key: "cron", label: "Cron 表达式" },
  { key: "at", label: "单次执行" },
];

const EVERY_UNIT_OPTIONS: ChoiceDef<EveryUnit>[] = [
  { key: "seconds", label: "秒" },
  { key: "minutes", label: "分钟" },
  { key: "hours", label: "小时" },
  { key: "days", label: "天" },
];

function get_default_timezone(): string {
  if (typeof Intl === "undefined") {
    return "Asia/Shanghai";
  }
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
}

function format_datetime_local_input(date: Date): string {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

function to_interval_seconds(value: string, unit: EveryUnit): number | null {
  const normalized_value = value.trim();
  if (!/^\d+$/.test(normalized_value)) {
    return null;
  }
  const numeric_value = Number(normalized_value);
  if (!Number.isInteger(numeric_value) || numeric_value <= 0) {
    return null;
  }
  if (unit === "days") {
    return numeric_value * 86400;
  }
  if (unit === "hours") {
    return numeric_value * 3600;
  }
  if (unit === "minutes") {
    return numeric_value * 60;
  }
  return numeric_value;
}

function get_interval_input(interval_seconds: number): { value: string; unit: EveryUnit } {
  if (interval_seconds % 86400 === 0) {
    return { value: String(interval_seconds / 86400), unit: "days" };
  }
  if (interval_seconds % 3600 === 0) {
    return { value: String(interval_seconds / 3600), unit: "hours" };
  }
  if (interval_seconds % 60 === 0) {
    return { value: String(interval_seconds / 60), unit: "minutes" };
  }
  return { value: String(interval_seconds), unit: "seconds" };
}

function format_session_label(session: AgentSession): string {
  const title = session.title?.trim() || "未命名会话";
  if (session.chat_type === "group") {
    return `${title} · Room 会话`;
  }
  if (session.room_id) {
    return `${title} · Room DM`;
  }
  return `${title} · 私有会话`;
}

function get_form_title(is_edit_mode: boolean): string {
  return is_edit_mode ? "编辑定时任务" : "创建定时任务";
}

function get_form_subtitle(is_edit_mode: boolean): string {
  return is_edit_mode
    ? "更新会话目标、调度规则和执行指令。"
    : "选择会话目标后，填写调度规则和执行指令。";
}

export function CreateTaskDialog({
  agent_id,
  is_open,
  on_close,
  task = null,
  on_create_task,
  on_update_task,
  on_saved,
}: CreateTaskDialogProps) {
  const is_edit_mode = task !== null;
  const name_ref = useRef<HTMLInputElement>(null);
  const [task_name, set_task_name] = useState("");
  const [schedule_kind, set_schedule_kind] = useState<ScheduleKind>("every");
  const [every_value, set_every_value] = useState("30");
  const [every_unit, set_every_unit] = useState<EveryUnit>("minutes");
  const [cron_expression, set_cron_expression] = useState("0 9 * * *");
  const [run_at, set_run_at] = useState(format_datetime_local_input(new Date(Date.now() + 3600_000)));
  const [timezone, set_timezone] = useState(get_default_timezone());
  const [session_target_kind, set_session_target_kind] = useState<ScheduledTaskSessionTargetKind>("main");
  const [selected_session_key, set_selected_session_key] = useState("");
  const [named_session_key, set_named_session_key] = useState("");
  const [enabled, set_enabled] = useState(true);
  const [instruction, set_instruction] = useState("");
  const [error_message, set_error_message] = useState<string | null>(null);
  const [is_submitting, set_is_submitting] = useState(false);
  const [agent_sessions, set_agent_sessions] = useState<AgentSession[]>([]);
  const [agent_sessions_loading, set_agent_sessions_loading] = useState(false);
  const [agent_sessions_error, set_agent_sessions_error] = useState<string | null>(null);

  useEffect(() => {
    if (is_open && name_ref.current) {
      name_ref.current.focus();
    }
  }, [is_open]);

  useEffect(() => {
    const handle_key_down = (e: KeyboardEvent) => {
      if (!is_open) return;
      if (e.key === "Escape") {
        e.preventDefault();
        on_close();
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_close]);

  useEffect(() => {
    if (!is_open) {
      return;
    }

    if (!task) {
      set_task_name("");
      set_schedule_kind("every");
      set_every_value("30");
      set_every_unit("minutes");
      set_cron_expression("0 9 * * *");
      set_run_at(format_datetime_local_input(new Date(Date.now() + 3600_000)));
      set_timezone(get_default_timezone());
      set_session_target_kind("main");
      set_selected_session_key("");
      set_named_session_key("");
      set_enabled(true);
      set_instruction("");
    } else {
      set_task_name(task.name);
      set_schedule_kind(task.schedule.kind);
      if (task.schedule.kind === "every") {
        const interval_input = get_interval_input(task.schedule.interval_seconds);
        set_every_value(interval_input.value);
        set_every_unit(interval_input.unit);
      } else {
        set_every_value("30");
        set_every_unit("minutes");
      }
      set_cron_expression(task.schedule.kind === "cron" ? task.schedule.cron_expression : "0 9 * * *");
      set_run_at(
        task.schedule.kind === "at"
          ? format_datetime_local_input(new Date(task.schedule.run_at))
          : format_datetime_local_input(new Date(Date.now() + 3600_000)),
      );
      set_timezone(task.schedule.timezone?.trim() || get_default_timezone());
      set_session_target_kind(task.session_target.kind);
      set_selected_session_key(task.session_target.kind === "bound" ? task.session_target.bound_session_key : "");
      set_named_session_key(task.session_target.kind === "named" ? task.session_target.named_session_key : "");
      set_enabled(task.enabled);
      set_instruction(task.instruction);
    }

    set_error_message(null);
    set_is_submitting(false);
    set_agent_sessions([]);
    set_agent_sessions_error(null);
  }, [agent_id, is_open, task]);

  useEffect(() => {
    if (!is_open || session_target_kind !== "bound") {
      set_agent_sessions([]);
      set_agent_sessions_loading(false);
      set_agent_sessions_error(null);
      return;
    }

    let cancelled = false;
    set_agent_sessions([]);
    set_agent_sessions_loading(true);
    set_agent_sessions_error(null);

    void getAgentSessionsApi(agent_id)
      .then((next_sessions) => {
        if (cancelled) return;
        set_agent_sessions(next_sessions);
      })
      .catch((error) => {
        if (cancelled) return;
        set_agent_sessions([]);
        set_agent_sessions_error(error instanceof Error ? error.message : "加载现有会话失败");
      })
      .finally(() => {
        if (!cancelled) {
          set_agent_sessions_loading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [agent_id, is_open, session_target_kind]);

  const session_options = useMemo<SessionOption[]>(() => {
    return agent_sessions.map((session) => ({
      session_key: session.session_key,
      label: format_session_label(session),
    }));
  }, [agent_sessions]);

  if (!is_open) {
    return null;
  }

  const build_schedule = (): ScheduledTaskSchedule => {
    if (schedule_kind === "every") {
      const interval_seconds = to_interval_seconds(every_value, every_unit);
      if (interval_seconds === null) {
        throw new Error("循环间隔必须是大于 0 的整数");
      }
      return {
        kind: "every",
        interval_seconds,
        timezone: timezone.trim() || "Asia/Shanghai",
      };
    }
    if (schedule_kind === "cron") {
      return {
        kind: "cron",
        cron_expression: cron_expression.trim(),
        timezone: timezone.trim() || "Asia/Shanghai",
      };
    }
    return {
      kind: "at",
      run_at: run_at.trim(),
      timezone: timezone.trim() || "Asia/Shanghai",
    };
  };

  const build_session_target = (): ScheduledTaskSessionTarget => {
    const wake_mode = task?.session_target.wake_mode ?? "next-heartbeat";

    if (session_target_kind === "main") {
      return { kind: "main", wake_mode };
    }
    if (session_target_kind === "isolated") {
      return { kind: "isolated", wake_mode };
    }
    if (session_target_kind === "named") {
      return {
        kind: "named",
        named_session_key: named_session_key.trim(),
        wake_mode,
      };
    }
    return {
      kind: "bound",
      bound_session_key: selected_session_key.trim(),
      wake_mode,
    };
  };

  const get_validation_error = (): string | null => {
    if (!task_name.trim()) {
      return "请输入任务名称";
    }
    if (!instruction.trim()) {
      return "请输入任务指令";
    }
    if (session_target_kind === "bound") {
      if (agent_sessions_error && session_options.length === 0) {
        return "现有会话加载失败，请先重试";
      }
      if (!selected_session_key.trim()) {
        return "请选择一个现有会话";
      }
      if (!session_options.some((option) => option.session_key === selected_session_key.trim())) {
        return "当前绑定会话已失效，请重新选择一个现有会话";
      }
    }
    if (session_target_kind === "named") {
      if (!named_session_key.trim()) {
        return "请输入命名会话 key";
      }
      if (named_session_key.trim().toLowerCase() === "main") {
        return "命名会话 key 不能使用保留字 main";
      }
    }
    if (schedule_kind === "every" && to_interval_seconds(every_value, every_unit) === null) {
      return "循环间隔必须是大于 0 的整数";
    }
    if (schedule_kind === "cron" && !cron_expression.trim()) {
      return "请输入 Cron 表达式";
    }
    if (schedule_kind === "at" && !run_at.trim()) {
      return "请选择有效的执行时间";
    }
    return null;
  };

  const handle_submit = async () => {
    const validation_error = get_validation_error();
    if (validation_error) {
      set_error_message(validation_error);
      return;
    }

    set_is_submitting(true);
    set_error_message(null);
    try {
      const payload = {
        name: task_name.trim(),
        schedule: build_schedule(),
        instruction: instruction.trim(),
        session_target: build_session_target(),
        enabled,
      };

      const saved_task = task
        ? await on_update_task(task.job_id, {
            ...payload,
            delivery: task.delivery,
          })
        : await on_create_task({
            ...payload,
            agent_id,
            delivery: { mode: "none" },
          });

      await on_saved?.(saved_task, task ? "edit" : "create");
      on_close();
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : `${task ? "更新" : "创建"}任务失败`);
    } finally {
      set_is_submitting(false);
    }
  };

  return (
    <div
      aria-labelledby="create-task-dialog-title"
      aria-modal="true"
      className="dialog-backdrop animate-in fade-in duration-(--motion-duration-fast)"
      role="dialog"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-lg animate-in zoom-in-95 duration-(--motion-duration-fast)">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title" id="create-task-dialog-title">
              {get_form_title(is_edit_mode)}
            </h3>
            <p className="dialog-subtitle">
              {get_form_subtitle(is_edit_mode)}
            </p>
          </div>
          <button
            aria-label="关闭"
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            onClick={on_close}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="dialog-body flex flex-col gap-4">
          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-name">
              任务名称
            </label>
            <input
              ref={name_ref}
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-name"
              onChange={(e) => set_task_name(e.target.value)}
              placeholder="输入任务名称"
              type="text"
              value={task_name}
            />
          </div>

          <div className="dialog-field">
            <span className="dialog-label">会话目标</span>
            <div className="flex flex-wrap gap-2">
              {SESSION_TARGET_OPTIONS.map((option) => (
                <button
                  className={getDialogChoiceClassName(session_target_kind === option.key)}
                  key={option.key}
                  onClick={() => {
                    set_session_target_kind(option.key);
                    if (option.key !== "bound") {
                      set_selected_session_key("");
                    }
                    if (option.key !== "named") {
                      set_named_session_key("");
                    }
                    set_error_message(null);
                  }}
                  style={getDialogChoiceStyle(session_target_kind === option.key)}
                  type="button"
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          {session_target_kind === "bound" ? (
            <div className="dialog-field">
              <label className="dialog-label" htmlFor="task-session-key">
                选择现有会话
              </label>
              <select
                className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                disabled={agent_sessions_loading || session_options.length === 0}
                id="task-session-key"
                onChange={(e) => {
                  set_selected_session_key(e.target.value);
                  set_error_message(null);
                }}
                value={selected_session_key}
              >
                <option value="">
                  {agent_sessions_loading
                    ? "正在加载现有会话..."
                    : session_options.length > 0
                      ? "请选择会话"
                      : "当前 Agent 暂无可绑定会话"}
                </option>
                {session_options.map((option) => (
                  <option key={option.session_key} value={option.session_key}>
                    {option.label}
                  </option>
                ))}
              </select>
              {agent_sessions_error ? (
                <p className="mt-2 text-xs text-(--destructive)">{agent_sessions_error}</p>
              ) : null}
              {!agent_sessions_loading && session_options.length === 0 && !agent_sessions_error ? (
                <p className="mt-2 text-xs text-(--text-muted)">请先创建或进入一个现有会话，再绑定到该任务</p>
              ) : null}
              {!agent_sessions_loading
                && selected_session_key.trim()
                && !session_options.some((option) => option.session_key === selected_session_key.trim())
                ? (
                  <p className="mt-2 text-xs text-(--warning)">当前绑定会话已不存在，请重新选择一个现有会话后再保存。</p>
                )
                : null}
            </div>
          ) : null}

          {session_target_kind === "named" ? (
            <div className="dialog-field">
              <label className="dialog-label" htmlFor="task-named-session-key">
                命名会话 key
              </label>
              <input
                className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                id="task-named-session-key"
                onChange={(e) => {
                  set_named_session_key(e.target.value);
                  set_error_message(null);
                }}
                placeholder="例如 morning-brief"
                type="text"
                value={named_session_key}
              />
              <p className="mt-2 text-xs text-(--text-muted)">
                任务会把内容发送到同名会话；`main` 是保留字，不能作为命名 key。
              </p>
            </div>
          ) : null}

          {session_target_kind === "main" ? (
            <div className="rounded-[18px] border border-(--divider-subtle-color) bg-white/45 px-4 py-3 text-sm text-(--text-default)">
              任务将发送到当前 Agent 的主会话。
            </div>
          ) : null}

          {session_target_kind === "isolated" ? (
            <div className="rounded-[18px] border border-(--divider-subtle-color) bg-white/45 px-4 py-3 text-sm text-(--text-default)">
              每次执行都会使用独立会话，不绑定现有会话，也不会写入主会话。
            </div>
          ) : null}

          <div className="dialog-field">
            <span className="dialog-label">调度类型</span>
            <div className="flex flex-wrap gap-2">
              {SCHEDULE_OPTIONS.map((option) => (
                <button
                  className={getDialogChoiceClassName(schedule_kind === option.key)}
                  key={option.key}
                  onClick={() => set_schedule_kind(option.key)}
                  style={getDialogChoiceStyle(schedule_kind === option.key)}
                  type="button"
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          {schedule_kind === "every" ? (
            <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr),140px]">
              <div className="dialog-field">
                <label className="dialog-label" htmlFor="task-every-value">
                  执行间隔
                </label>
                <input
                  className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                  id="task-every-value"
                  min="1"
                  onChange={(e) => set_every_value(e.target.value)}
                  step="1"
                  type="number"
                  value={every_value}
                />
              </div>
              <div className="dialog-field">
                <label className="dialog-label" htmlFor="task-every-unit">
                  单位
                </label>
                <select
                  className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                  id="task-every-unit"
                  onChange={(e) => set_every_unit(e.target.value as EveryUnit)}
                  value={every_unit}
                >
                  {EVERY_UNIT_OPTIONS.map((option) => (
                    <option key={option.key} value={option.key}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          ) : null}

          {schedule_kind === "cron" ? (
            <div className="dialog-field">
              <label className="dialog-label" htmlFor="task-cron-expression">
                Cron 表达式
              </label>
              <input
                className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                id="task-cron-expression"
                onChange={(e) => set_cron_expression(e.target.value)}
                placeholder="例如 0 9 * * *"
                type="text"
                value={cron_expression}
              />
            </div>
          ) : null}

          {schedule_kind === "at" ? (
            <div className="dialog-field">
              <label className="dialog-label" htmlFor="task-run-at">
                执行时间
              </label>
              <input
                className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                id="task-run-at"
                onChange={(e) => set_run_at(e.target.value)}
                type="datetime-local"
                value={run_at}
              />
            </div>
          ) : null}

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-timezone">
              时区
            </label>
            <input
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-timezone"
              onChange={(e) => set_timezone(e.target.value)}
              placeholder="Asia/Shanghai"
              type="text"
              value={timezone}
            />
          </div>

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-instruction">
              任务指令
            </label>
            <textarea
              className="dialog-input radius-shell-sm w-full resize-none px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-instruction"
              onChange={(e) => set_instruction(e.target.value)}
              placeholder="输入 Agent 需要执行的指令"
              rows={3}
              value={instruction}
            />
          </div>

          <label className="flex items-center gap-3 rounded-[18px] border border-(--divider-subtle-color) bg-white/45 px-4 py-3 text-sm text-(--text-default)">
            <input
              checked={enabled}
              className="h-4 w-4"
              onChange={(e) => set_enabled(e.target.checked)}
              type="checkbox"
            />
            {is_edit_mode ? "保存后保持任务启用" : "创建后立即启用任务"}
          </label>

          {error_message ? (
            <div className="rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_6%,transparent)] px-4 py-3 text-sm text-(--destructive)">
              {error_message}
            </div>
          ) : null}
        </div>

        <div className="dialog-footer">
          <button
            className={getDialogActionClassName("default")}
            disabled={is_submitting}
            onClick={on_close}
            type="button"
          >
            取消
          </button>
          <button
            className={getDialogActionClassName("primary")}
            disabled={is_submitting}
            onClick={() => void handle_submit()}
            type="button"
          >
            {is_submitting ? (is_edit_mode ? "保存中" : "创建中") : (is_edit_mode ? "保存" : "创建")}
          </button>
        </div>
      </div>
    </div>
  );
}
