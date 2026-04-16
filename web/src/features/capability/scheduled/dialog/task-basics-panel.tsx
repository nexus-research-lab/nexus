"use client";

import { type RefObject } from "react";

import {
  get_dialog_choice_class_name,
  get_dialog_choice_style,
} from "@/shared/ui/dialog/dialog-styles";

type TargetType = "agent" | "room";
type ExecutionMode = "main" | "existing" | "temporary" | "dedicated";
type ReplyMode = "none" | "execution" | "selected";

interface OptionItem {
  key?: string;
  label: string;
  value?: string;
}

interface TaskBasicsPanelProps {
  name_ref: RefObject<HTMLInputElement | null>;
  task_name: string;
  set_task_name: (value: string) => void;
  target_type: TargetType;
  set_target_type: (value: TargetType) => void;
  target_type_options: Array<{ key: TargetType; label: string }>;
  selected_agent_id: string;
  set_selected_agent_id: (value: string) => void;
  selected_room_id: string;
  set_selected_room_id: (value: string) => void;
  agent_options: OptionItem[];
  room_options: OptionItem[];
  agents_loading: boolean;
  rooms_loading: boolean;
  agents_error: string | null;
  rooms_error: string | null;
  execution_mode: ExecutionMode;
  set_execution_mode: (value: ExecutionMode) => void;
  execution_mode_options: Array<{ key: ExecutionMode; label: string }>;
  dedicated_session_key: string;
  set_dedicated_session_key: (value: string) => void;
  selected_session_key: string;
  set_selected_session_key: (value: string) => void;
  session_options: Array<{ session_key: string; label: string }>;
  session_loading: boolean;
  session_error: string | null;
  session_empty_message: string | null;
  require_session_selection: boolean;
  reply_mode: ReplyMode;
  set_reply_mode: (value: ReplyMode) => void;
  reply_mode_options: Array<{ key: ReplyMode; label: string }>;
  disabled_reply_modes?: ReplyMode[];
  selected_reply_session_key: string;
  set_selected_reply_session_key: (value: string) => void;
  on_reset_context_error: () => void;
}

function get_execution_mode_help_text(mode: ExecutionMode): string {
  if (mode === "main") {
    return "交给目标智能体的主会话处理，适合把任务继续接在主线对话里。";
  }
  if (mode === "existing") {
    return "复用当前已有的执行上下文。";
  }
  if (mode === "temporary") {
    return "每次执行都会新开一个临时会话，不延续旧上下文。";
  }
  return "第一次执行时创建一个专用长期会话，之后持续复用。";
}

function get_reply_mode_help_text(mode: ReplyMode): string {
  if (mode === "none") {
    return "执行结果只保存在任务自己的执行会话里。";
  }
  if (mode === "execution") {
    return "结果回到这次执行关联的会话；Agent 的主会话和临时会话模式默认不额外回传。";
  }
  return "结果会额外推送到你指定的一个已有会话。";
}

export function TaskBasicsPanel(props: TaskBasicsPanelProps) {
  const {
    name_ref,
    task_name,
    set_task_name,
    target_type,
    set_target_type,
    target_type_options,
    selected_agent_id,
    set_selected_agent_id,
    selected_room_id,
    set_selected_room_id,
    agent_options,
    room_options,
    agents_loading,
    rooms_loading,
    agents_error,
    rooms_error,
    execution_mode,
    set_execution_mode,
    execution_mode_options,
    dedicated_session_key,
    set_dedicated_session_key,
    selected_session_key,
    set_selected_session_key,
    session_options,
    session_loading,
    session_error,
    session_empty_message,
    require_session_selection,
    reply_mode,
    set_reply_mode,
    reply_mode_options,
    disabled_reply_modes = [],
    selected_reply_session_key,
    set_selected_reply_session_key,
    on_reset_context_error,
  } = props;

  return (
    <div className="flex min-w-0 flex-col gap-4">
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
          {target_type_options.map((opt) => (
            <button
              className={get_dialog_choice_class_name(target_type === opt.key)}
              key={opt.key}
              onClick={() => {
                set_target_type(opt.key);
                on_reset_context_error();
              }}
              style={get_dialog_choice_style(target_type === opt.key)}
              type="button"
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-target-object">
          {target_type === "agent" ? "目标智能体" : "目标 Room"}
        </label>
        <select
          className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
          disabled={target_type === "agent" ? agents_loading || agent_options.length === 0 : rooms_loading || room_options.length === 0}
          id="task-target-object"
          onChange={(e) => {
            if (target_type === "agent") {
              set_selected_agent_id(e.target.value);
            } else {
              set_selected_room_id(e.target.value);
            }
            on_reset_context_error();
          }}
          value={target_type === "agent" ? selected_agent_id : selected_room_id}
        >
          <option value="">
            {target_type === "agent"
              ? (agents_loading ? "正在加载智能体..." : "请选择智能体")
              : (rooms_loading ? "正在加载 Room..." : "请选择 Room")}
          </option>
          {(target_type === "agent" ? agent_options : room_options).map((option) => (
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
        <span className="dialog-label">执行会话</span>
        <div className="flex flex-wrap gap-2">
          {execution_mode_options.map((opt) => (
            <button
              className={get_dialog_choice_class_name(execution_mode === opt.key)}
              key={opt.key}
              onClick={() => {
                set_execution_mode(opt.key);
                on_reset_context_error();
              }}
              style={get_dialog_choice_style(execution_mode === opt.key)}
              type="button"
            >
              {opt.label}
            </button>
          ))}
        </div>
        <p className="mt-2 text-xs leading-5 text-(--text-muted)">
          {get_execution_mode_help_text(execution_mode)}
        </p>
      </div>

      {execution_mode === "dedicated" ? (
        <div className="dialog-field">
          <label className="dialog-label" htmlFor="task-dedicated-session-key">
            专用长期会话名称
          </label>
          <input
            className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
            id="task-dedicated-session-key"
            onChange={(e) => set_dedicated_session_key(e.target.value)}
            placeholder="例如 daily-ops"
            type="text"
            value={dedicated_session_key}
          />
        </div>
      ) : null}

      {require_session_selection ? (
        <div className="dialog-field">
          <label className="dialog-label" htmlFor="task-session-key">
            执行会话
          </label>
          <select
            className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
            disabled={session_loading || session_options.length === 0}
            id="task-session-key"
            onChange={(e) => {
              set_selected_session_key(e.target.value);
              on_reset_context_error();
            }}
            value={selected_session_key}
          >
            <option value="">{session_loading ? "正在加载会话..." : "请选择会话"}</option>
            {session_options.map((option) => (
              <option key={option.session_key} value={option.session_key}>
                {option.label}
              </option>
            ))}
          </select>
          {session_error ? <p className="mt-2 text-xs text-(--destructive)">{session_error}</p> : null}
          {session_empty_message ? <p className="mt-2 text-xs text-(--text-muted)">{session_empty_message}</p> : null}
        </div>
      ) : null}

      <div className="dialog-field">
        <span className="dialog-label">结果回传</span>
        <div className="flex flex-wrap gap-2">
          {reply_mode_options.map((opt) => (
            <button
              className={get_dialog_choice_class_name(reply_mode === opt.key)}
              disabled={disabled_reply_modes.includes(opt.key)}
              key={opt.key}
              onClick={() => {
                set_reply_mode(opt.key);
                on_reset_context_error();
              }}
              style={get_dialog_choice_style(reply_mode === opt.key)}
              type="button"
            >
              {opt.label}
            </button>
          ))}
        </div>
        <p className="mt-2 text-xs leading-5 text-(--text-muted)">
          {get_reply_mode_help_text(reply_mode)}
        </p>
      </div>

      {reply_mode === "selected" ? (
        <div className="dialog-field">
          <label className="dialog-label" htmlFor="task-reply-session-key">
            回复会话
          </label>
          <select
            className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
            disabled={session_loading || session_options.length === 0}
            id="task-reply-session-key"
            onChange={(e) => {
              set_selected_reply_session_key(e.target.value);
              on_reset_context_error();
            }}
            value={selected_reply_session_key}
          >
            <option value="">{session_loading ? "正在加载会话..." : "请选择回复会话"}</option>
            {session_options.map((option) => (
              <option key={option.session_key} value={option.session_key}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      ) : null}
    </div>
  );
}
