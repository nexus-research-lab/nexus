/**
 * 创建定时任务对话框
 *
 * 负责把表单状态转换成结构化的 scheduled task 创建 payload。
 */

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";

import { createScheduledTaskApi } from "@/lib/scheduled-task-api";
import {
  getDialogChoiceClassName,
  getDialogChoiceStyle,
} from "@/shared/ui/dialog/dialog-styles";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import type { ScheduledTaskItem, ScheduledTaskSchedule, ScheduledTaskSessionTarget } from "@/types/scheduled-task";

type ScheduleKind = ScheduledTaskSchedule["kind"];
type SessionTargetKind = ScheduledTaskSessionTarget["kind"];
type EveryUnit = "minutes" | "hours" | "days";

interface ChoiceDef<TValue extends string> {
  key: TValue;
  label: string;
}

const SCHEDULE_OPTIONS: ChoiceDef<ScheduleKind>[] = [
  { key: "every", label: "循环间隔" },
  { key: "cron", label: "Cron 表达式" },
  { key: "at", label: "单次执行" },
];

interface CreateTaskDialogProps {
  agent_id: string;
  is_open: boolean;
  on_close: () => void;
  on_created?: (task: ScheduledTaskItem) => void | Promise<void>;
}

const SESSION_TARGET_OPTIONS: ChoiceDef<SessionTargetKind>[] = [
  { key: "isolated", label: "独立会话" },
  { key: "main", label: "主会话" },
  { key: "bound", label: "绑定会话" },
  { key: "named", label: "命名会话" },
];

const WAKE_MODE_OPTIONS: ChoiceDef<"now" | "next-heartbeat">[] = [
  { key: "next-heartbeat", label: "心跳唤醒" },
  { key: "now", label: "立即唤醒" },
];

const EVERY_UNIT_OPTIONS: ChoiceDef<EveryUnit>[] = [
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
  return numeric_value * 60;
}

export function CreateTaskDialog({
  agent_id,
  is_open,
  on_close,
  on_created,
}: CreateTaskDialogProps) {
  const name_ref = useRef<HTMLInputElement>(null);
  const [task_name, set_task_name] = useState("");
  const [schedule_kind, set_schedule_kind] = useState<ScheduleKind>("every");
  const [every_value, set_every_value] = useState("30");
  const [every_unit, set_every_unit] = useState<EveryUnit>("minutes");
  const [cron_expression, set_cron_expression] = useState("0 9 * * *");
  const [run_at, set_run_at] = useState(format_datetime_local_input(new Date(Date.now() + 3600_000)));
  const [timezone, set_timezone] = useState(get_default_timezone());
  const [session_target_kind, set_session_target_kind] = useState<SessionTargetKind>("isolated");
  const [session_target_key, set_session_target_key] = useState("");
  const [wake_mode, set_wake_mode] = useState<"now" | "next-heartbeat">("next-heartbeat");
  const [enabled, set_enabled] = useState(true);
  const [instruction, set_instruction] = useState("");
  const [error_message, set_error_message] = useState<string | null>(null);
  const [is_submitting, set_is_submitting] = useState(false);

  // 打开时聚焦到名称输入框
  useEffect(() => {
    if (is_open && name_ref.current) {
      name_ref.current.focus();
    }
  }, [is_open]);

  // ESC 关闭
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

  // 重置表单
  useEffect(() => {
    if (is_open) {
      set_task_name("");
      set_schedule_kind("every");
      set_every_value("30");
      set_every_unit("minutes");
      set_cron_expression("0 9 * * *");
      set_run_at(format_datetime_local_input(new Date(Date.now() + 3600_000)));
      set_timezone(get_default_timezone());
      set_session_target_kind("isolated");
      set_session_target_key("");
      set_wake_mode("next-heartbeat");
      set_enabled(true);
      set_instruction("");
      set_error_message(null);
      set_is_submitting(false);
    }
  }, [is_open]);

  if (!is_open) return null;

  // 中文注释：创建接口要求 schedule / session_target 都是结构化对象，这里先统一收口表单校验，再拼出最终 payload。
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
    if (session_target_kind === "bound") {
      return {
        kind: "bound",
        bound_session_key: session_target_key.trim(),
        wake_mode,
      };
    }
    if (session_target_kind === "named") {
      return {
        kind: "named",
        named_session_key: session_target_key.trim(),
        wake_mode,
      };
    }
    return {
      kind: session_target_kind,
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
    if (schedule_kind === "every") {
      if (to_interval_seconds(every_value, every_unit) === null) {
        return "循环间隔必须是大于 0 的整数";
      }
    }
    if (schedule_kind === "cron" && !cron_expression.trim()) {
      return "请输入 Cron 表达式";
    }
    if (schedule_kind === "at") {
      if (!run_at.trim()) {
        return "请选择有效的执行时间";
      }
    }
    if ((session_target_kind === "bound" || session_target_kind === "named") && !session_target_key.trim()) {
      return "请输入会话标识";
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
      const created = await createScheduledTaskApi({
        name: task_name.trim(),
        agent_id,
        schedule: build_schedule(),
        instruction: instruction.trim(),
        session_target: build_session_target(),
        delivery: { mode: "none" },
        enabled,
      });
      void on_created?.(created);
      on_close();
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : "创建任务失败");
    } finally {
      set_is_submitting(false);
    }
  };

  return (
    <div
      aria-labelledby="create-task-dialog-title"
      aria-modal="true"
      className="dialog-backdrop animate-in fade-in duration-150"
      role="dialog"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-lg animate-in zoom-in-95 duration-150">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title" id="create-task-dialog-title">
              创建定时任务
            </h3>
            <p className="dialog-subtitle">
              先定义频率和时间，再把执行指令交给 Agent。
            </p>
          </div>
          <WorkspacePillButton
            aria-label="关闭"
            density="compact"
            onClick={on_close}
            size="icon"
            variant="icon"
          >
            <X className="h-4 w-4" />
          </WorkspacePillButton>
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
            <label className="dialog-label" htmlFor="task-agent">
              执行 Agent
            </label>
            <input
              readOnly
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground/80 focus-visible:outline-none"
              id="task-agent"
              type="text"
              value={agent_id}
            />
          </div>

          <div className="dialog-field">
            <span className="dialog-label">调度类型</span>
            <div className="flex flex-wrap gap-2">
              {SCHEDULE_OPTIONS.map((opt) => (
                <button
                  className={getDialogChoiceClassName(schedule_kind === opt.key)}
                  key={opt.key}
                  onClick={() => set_schedule_kind(opt.key)}
                  style={getDialogChoiceStyle(schedule_kind === opt.key)}
                  type="button"
                >
                  {opt.label}
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
            <span className="dialog-label">目标会话</span>
            <div className="flex flex-wrap gap-2">
              {SESSION_TARGET_OPTIONS.map((opt) => (
                <button
                  className={getDialogChoiceClassName(session_target_kind === opt.key)}
                  key={opt.key}
                  onClick={() => set_session_target_kind(opt.key)}
                  style={getDialogChoiceStyle(session_target_kind === opt.key)}
                  type="button"
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {session_target_kind === "bound" || session_target_kind === "named" ? (
            <div className="dialog-field">
              <label className="dialog-label" htmlFor="task-session-key">
                会话标识
              </label>
              <input
                className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                id="task-session-key"
                onChange={(e) => set_session_target_key(e.target.value)}
                placeholder={session_target_kind === "bound" ? "输入 bound session key" : "输入命名会话 key"}
                type="text"
                value={session_target_key}
              />
            </div>
          ) : null}

          <div className="dialog-field">
            <span className="dialog-label">唤醒模式</span>
            <div className="flex flex-wrap gap-2">
              {WAKE_MODE_OPTIONS.map((opt) => (
                <button
                  className={getDialogChoiceClassName(wake_mode === opt.key)}
                  key={opt.key}
                  onClick={() => set_wake_mode(opt.key)}
                  style={getDialogChoiceStyle(wake_mode === opt.key)}
                  type="button"
                >
                  {opt.label}
                </button>
              ))}
            </div>
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

          <label className="flex items-center gap-3 rounded-[18px] border border-[var(--divider-subtle-color)] bg-white/45 px-4 py-3 text-sm text-[color:var(--text-default)]">
            <input
              checked={enabled}
              className="h-4 w-4"
              onChange={(e) => set_enabled(e.target.checked)}
              type="checkbox"
            />
            创建后立即启用任务
          </label>

          {error_message ? (
            <div className="rounded-[18px] border border-rose-500/15 bg-rose-500/6 px-4 py-3 text-sm text-rose-600">
              {error_message}
            </div>
          ) : null}
        </div>

        <div className="dialog-footer">
          <WorkspacePillButton disabled={is_submitting} onClick={on_close} size="md" variant="tonal">
            取消
          </WorkspacePillButton>
          <WorkspacePillButton disabled={is_submitting} onClick={() => void handle_submit()} size="md" variant="primary">
            {is_submitting ? "创建中" : "创建"}
          </WorkspacePillButton>
        </div>
      </div>
    </div>
  );
}
