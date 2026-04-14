/**
 * 创建定时任务对话框
 *
 * 负责把表单状态转换成结构化的 scheduled task 创建 payload。
 */

import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "lucide-react";

import { getAgents } from "@/lib/agent-manage-api";
import { getAgentSessionsApi } from "@/lib/agent-api";
import { createScheduledTaskApi } from "@/lib/scheduled-task-api";
import { getRoomContexts, listRooms } from "@/lib/room-api";
import { buildRoomAgentSessionKey } from "@/lib/session-key";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  getDialogActionClassName,
  getDialogChoiceClassName,
  getDialogChoiceStyle,
} from "@/shared/ui/dialog/dialog-styles";
import type { Agent, AgentSession } from "@/types/agent";
import type { RoomAggregate, RoomContextAggregate, RoomSessionSelection } from "@/types/room";
import type {
  ScheduledTaskItem,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
} from "@/types/scheduled-task";

type ScheduleKind = ScheduledTaskSchedule["kind"];
type EveryUnit = "minutes" | "hours" | "days";
type TargetType = "agent" | "room";

interface ChoiceDef<TValue extends string> {
  key: TValue;
  label: string;
}

const TARGET_TYPE_OPTIONS: ChoiceDef<TargetType>[] = [
  { key: "agent", label: "智能体" },
  { key: "room", label: "Room" },
];

const SCHEDULE_OPTIONS: ChoiceDef<ScheduleKind>[] = [
  { key: "every", label: "循环间隔" },
  { key: "cron", label: "Cron 表达式" },
  { key: "at", label: "单次执行" },
];

const EVERY_UNIT_OPTIONS: ChoiceDef<EveryUnit>[] = [
  { key: "minutes", label: "分钟" },
  { key: "hours", label: "小时" },
  { key: "days", label: "天" },
];

interface ScheduledTaskDialogProps {
  agent_id: string;
  is_open: boolean;
  on_close: () => void;
  on_created?: (task: ScheduledTaskItem) => void | Promise<void>;
}

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

function format_session_label(title: string, agent_name: string): string {
  return `${title} · ${agent_name}`;
}

function build_room_session_selections(
  contexts: RoomContextAggregate[],
  agent_name_by_id: Map<string, string>,
): RoomSessionSelection[] {
  return contexts.flatMap((context) => {
    const room_title = context.conversation.title?.trim() || context.room.name?.trim() || "未命名会话";
    const room_type = context.room.room_type;

    return context.sessions.map((session) => ({
      session_key: buildRoomAgentSessionKey(
        context.conversation.id,
        session.agent_id,
        room_type === "dm" ? "dm" : "room",
      ),
      agent_id: session.agent_id,
      room_id: context.room.id,
      conversation_id: context.conversation.id,
      room_type,
      title: room_title,
      session,
      label: format_session_label(room_title, agent_name_by_id.get(session.agent_id) || session.agent_id),
    }));
  });
}

export function ScheduledTaskDialog({
  agent_id,
  is_open,
  on_close,
  on_created,
}: ScheduledTaskDialogProps) {
  const name_ref = useRef<HTMLInputElement>(null);
  const [task_name, set_task_name] = useState("");
  const [schedule_kind, set_schedule_kind] = useState<ScheduleKind>("every");
  const [every_value, set_every_value] = useState("30");
  const [every_unit, set_every_unit] = useState<EveryUnit>("minutes");
  const [cron_expression, set_cron_expression] = useState("0 9 * * *");
  const [run_at, set_run_at] = useState(format_datetime_local_input(new Date(Date.now() + 3600_000)));
  const [timezone, set_timezone] = useState(get_default_timezone());
  const [target_type, set_target_type] = useState<TargetType>("agent");
  const [selected_agent_id, set_selected_agent_id] = useState(agent_id);
  const [selected_room_id, set_selected_room_id] = useState("");
  const [selected_session_key, set_selected_session_key] = useState("");
  const [enabled, set_enabled] = useState(true);
  const [instruction, set_instruction] = useState("");
  const [error_message, set_error_message] = useState<string | null>(null);
  const [is_submitting, set_is_submitting] = useState(false);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_sessions, set_agent_sessions] = useState<AgentSession[]>([]);
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [room_contexts, set_room_contexts] = useState<RoomContextAggregate[]>([]);
  const [agents_loading, set_agents_loading] = useState(false);
  const [agent_sessions_loading, set_agent_sessions_loading] = useState(false);
  const [rooms_loading, set_rooms_loading] = useState(false);
  const [room_contexts_loading, set_room_contexts_loading] = useState(false);
  const [agents_error, set_agents_error] = useState<string | null>(null);
  const [agent_sessions_error, set_agent_sessions_error] = useState<string | null>(null);
  const [rooms_error, set_rooms_error] = useState<string | null>(null);
  const [room_contexts_error, set_room_contexts_error] = useState<string | null>(null);

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

    set_task_name("");
    set_schedule_kind("every");
    set_every_value("30");
    set_every_unit("minutes");
    set_cron_expression("0 9 * * *");
    set_run_at(format_datetime_local_input(new Date(Date.now() + 3600_000)));
    set_timezone(get_default_timezone());
    set_target_type("agent");
    set_selected_agent_id(agent_id);
    set_selected_room_id("");
    set_selected_session_key("");
    set_enabled(true);
    set_instruction("");
    set_error_message(null);
    set_is_submitting(false);
    set_agent_sessions([]);
    set_room_contexts([]);
    set_agents_error(null);
    set_agent_sessions_error(null);
    set_rooms_error(null);
    set_room_contexts_error(null);
  }, [agent_id, is_open]);

  useEffect(() => {
    if (!is_open) {
      return;
    }

    let cancelled = false;
    set_agents_loading(true);
    set_agents_error(null);

    void getAgents()
      .then((next_agents) => {
        if (cancelled) return;
        set_agents(next_agents);
      })
      .catch((error) => {
        if (cancelled) return;
        set_agents_error(error instanceof Error ? error.message : "加载智能体失败");
      })
      .finally(() => {
        if (!cancelled) {
          set_agents_loading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [is_open]);

  useEffect(() => {
    if (!is_open || target_type !== "room") {
      return;
    }

    let cancelled = false;
    set_rooms_loading(true);
    set_rooms_error(null);

    void listRooms(200)
      .then((next_rooms) => {
        if (cancelled) return;
        set_rooms(next_rooms);
      })
      .catch((error) => {
        if (cancelled) return;
        set_rooms_error(error instanceof Error ? error.message : "加载 Room 列表失败");
      })
      .finally(() => {
        if (!cancelled) {
          set_rooms_loading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [is_open, target_type]);

  useEffect(() => {
    if (!is_open || target_type !== "agent" || !selected_agent_id) {
      set_agent_sessions([]);
      return;
    }

    let cancelled = false;
    set_agent_sessions_loading(true);
    set_agent_sessions_error(null);
    set_selected_session_key("");

    void getAgentSessionsApi(selected_agent_id)
      .then((next_sessions) => {
        if (cancelled) return;
        set_agent_sessions(next_sessions);
      })
      .catch((error) => {
        if (cancelled) return;
        set_agent_sessions_error(error instanceof Error ? error.message : "加载智能体会话失败");
      })
      .finally(() => {
        if (!cancelled) {
          set_agent_sessions_loading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [is_open, selected_agent_id, target_type]);

  useEffect(() => {
    if (!is_open || target_type !== "room" || !selected_room_id) {
      set_room_contexts([]);
      return;
    }

    let cancelled = false;
    set_room_contexts_loading(true);
    set_room_contexts_error(null);
    set_selected_session_key("");

    void getRoomContexts(selected_room_id)
      .then((next_contexts) => {
        if (cancelled) return;
        set_room_contexts(next_contexts);
      })
      .catch((error) => {
        if (cancelled) return;
        set_room_contexts_error(error instanceof Error ? error.message : "加载 Room 会话失败");
      })
      .finally(() => {
        if (!cancelled) {
          set_room_contexts_loading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [is_open, selected_room_id, target_type]);

  const agent_name_by_id = useMemo(() => {
    return new Map(agents.map((agent) => [agent.agent_id, agent.name]));
  }, [agents]);

  const agent_options = useMemo(() => {
    return agents.map((agent) => ({
      value: agent.agent_id,
      label: agent.name || agent.agent_id,
    }));
  }, [agents]);

  const room_options = useMemo(() => {
    return rooms.map((room) => ({
      value: room.room.id,
      label: room.room.name?.trim() || room.room.id,
    }));
  }, [rooms]);

  const agent_session_options = useMemo(() => {
    return agent_sessions.map((session) => ({
      session_key: session.session_key,
      agent_id: session.agent_id,
      label: format_session_label(
        session.title?.trim() || "未命名会话",
        agent_name_by_id.get(session.agent_id) || session.agent_id,
      ),
    }));
  }, [agent_name_by_id, agent_sessions]);

  const room_session_options = useMemo(() => {
    const options = build_room_session_selections(room_contexts, agent_name_by_id);
    return options.map((option) => ({
      session_key: option.session_key,
      agent_id: option.agent_id,
      label: option.label,
    }));
  }, [agent_name_by_id, room_contexts]);

  const session_options = target_type === "agent" ? agent_session_options : room_session_options;
  const selected_session = session_options.find((option) => option.session_key === selected_session_key) ?? null;

  if (!is_open) return null;

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
    if (!selected_session) {
      throw new Error("请选择会话");
    }

    return {
      kind: "bound",
      bound_session_key: selected_session.session_key,
      wake_mode: "next-heartbeat",
    };
  };

  const get_validation_error = (): string | null => {
    if (!task_name.trim()) {
      return "请输入任务名称";
    }
    if (!instruction.trim()) {
      return "请输入任务指令";
    }
    if (target_type === "agent") {
      if (!selected_agent_id.trim()) {
        return "请选择智能体";
      }
    } else if (!selected_room_id.trim()) {
      return "请选择 Room";
    }
    if (!selected_session_key.trim()) {
      return "请选择会话";
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
      const target_session = selected_session;
      if (!target_session) {
        throw new Error("请选择会话");
      }

      const created = await createScheduledTaskApi({
        name: task_name.trim(),
        agent_id: target_type === "agent" ? selected_agent_id.trim() : target_session.agent_id,
        schedule: build_schedule(),
        instruction: instruction.trim(),
        session_target: build_session_target(),
        delivery: { mode: "none" },
        source: {
          kind: "user_page",
          context_type: target_type,
          context_id: target_type === "agent" ? selected_agent_id.trim() : selected_room_id.trim(),
          context_label: target_type === "agent"
            ? (agent_options.find((option) => option.value === selected_agent_id)?.label || selected_agent_id.trim())
            : (room_options.find((option) => option.value === selected_room_id)?.label || selected_room_id.trim()),
          session_key: target_session.session_key,
          session_label: target_session.label,
        },
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
      className="dialog-backdrop animate-in fade-in duration-(--motion-duration-fast)"
      role="dialog"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-lg animate-in zoom-in-95 duration-(--motion-duration-fast)">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title" id="create-task-dialog-title">
              创建定时任务
            </h3>
            <p className="dialog-subtitle">
              先选目标对象和会话，再填写调度和执行指令。
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
            <span className="dialog-label">发送到</span>
            <div className="flex flex-wrap gap-2">
              {TARGET_TYPE_OPTIONS.map((opt) => (
                <button
                  className={getDialogChoiceClassName(target_type === opt.key)}
                  key={opt.key}
                  onClick={() => {
                    set_target_type(opt.key);
                    set_selected_session_key("");
                    set_error_message(null);
                  }}
                  style={getDialogChoiceStyle(target_type === opt.key)}
                  type="button"
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-target-object">
              {target_type === "agent" ? "选择智能体" : "选择 Room"}
            </label>
            <select
              className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              disabled={
                target_type === "agent"
                  ? agents_loading || agents.length === 0
                  : rooms_loading || rooms.length === 0
              }
              id="task-target-object"
              onChange={(e) => {
                if (target_type === "agent") {
                  set_selected_agent_id(e.target.value);
                } else {
                  set_selected_room_id(e.target.value);
                }
                set_selected_session_key("");
                set_error_message(null);
              }}
              value={target_type === "agent" ? selected_agent_id : selected_room_id}
            >
              <option value="">
                {target_type === "agent"
                  ? (agents_loading ? "正在加载智能体..." : "请选择智能体")
                  : (rooms_loading ? "正在加载 Room..." : "请选择 Room")}
              </option>
              {target_type === "agent"
                ? agent_options.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))
                : room_options.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
            </select>
            {target_type === "agent" && agents_error ? (
              <p className="mt-2 text-xs text-(--destructive)">{agents_error}</p>
            ) : null}
            {target_type === "room" && rooms_error ? (
              <p className="mt-2 text-xs text-(--destructive)">{rooms_error}</p>
            ) : null}
          </div>

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-session-key">
              选择会话
            </label>
            <select
              className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              disabled={
                !selected_agent_id && target_type === "agent"
                  ? true
                  : !selected_room_id && target_type === "room"
                    ? true
                    : target_type === "agent"
                      ? agent_sessions_loading || agent_session_options.length === 0
                      : room_contexts_loading || room_session_options.length === 0
              }
              id="task-session-key"
              onChange={(e) => {
                set_selected_session_key(e.target.value);
                set_error_message(null);
              }}
              value={selected_session_key}
            >
              <option value="">
                {target_type === "agent"
                  ? (agent_sessions_loading
                    ? "正在加载会话..."
                    : selected_agent_id
                      ? "请选择会话"
                      : "先选择智能体")
                  : (room_contexts_loading
                    ? "正在加载会话..."
                    : selected_room_id
                      ? "请选择会话"
                      : "先选择 Room")}
              </option>
              {session_options.map((option) => (
                <option key={option.session_key} value={option.session_key}>
                  {option.label}
                </option>
              ))}
            </select>
            {target_type === "agent" && agent_sessions_error ? (
              <p className="mt-2 text-xs text-(--destructive)">{agent_sessions_error}</p>
            ) : null}
            {target_type === "room" && room_contexts_error ? (
              <p className="mt-2 text-xs text-(--destructive)">{room_contexts_error}</p>
            ) : null}
            {target_type === "agent" && selected_agent_id && !agent_sessions_loading && agent_session_options.length === 0 ? (
              <p className="mt-2 text-xs text-(--text-muted)">这个智能体没有可选会话</p>
            ) : null}
            {target_type === "room" && selected_room_id && !room_contexts_loading && room_session_options.length === 0 ? (
              <p className="mt-2 text-xs text-(--text-muted)">这个 Room 没有可选会话</p>
            ) : null}
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
            创建后立即启用任务
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
            {is_submitting ? "创建中" : "创建"}
          </button>
        </div>
      </div>
    </div>
  );
}
