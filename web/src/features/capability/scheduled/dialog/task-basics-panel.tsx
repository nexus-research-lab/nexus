"use client";

import { type RefObject } from "react";

import { UiChoiceButton } from "@/shared/ui/choice";
import { UiField, UiInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/select-menu";

type TargetType = "agent" | "room";
type ExecutionKind = "agent" | "script";
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
  execution_kind: ExecutionKind;
  set_execution_kind: (value: ExecutionKind) => void;
  execution_kind_options: Array<{ key: ExecutionKind; label: string }>;
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

function get_execution_kind_help_text(kind: ExecutionKind): string {
  if (kind === "script") {
    return "在目标智能体工作区直接执行脚本，输出会记录到运行历史和产物文件。";
  }
  return "由 Agent 会话执行任务，适合需要上下文、工具调用或自然语言处理的任务。";
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
    execution_kind,
    set_execution_kind,
    execution_kind_options,
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
      <UiField html_for="task-name" label="任务名称">
        <UiInput
          ref={name_ref}
          id="task-name"
          onChange={(e) => set_task_name(e.target.value)}
          placeholder="输入任务名称"
          value={task_name}
        />
      </UiField>

      <div className="dialog-field">
        <span className="dialog-label">执行方式</span>
        <div className="flex flex-wrap gap-2">
          {execution_kind_options.map((opt) => (
            <UiChoiceButton
              active={execution_kind === opt.key}
              key={opt.key}
              onClick={() => {
                set_execution_kind(opt.key);
                on_reset_context_error();
              }}
            >
              {opt.label}
            </UiChoiceButton>
          ))}
        </div>
        <p className="mt-2 text-xs leading-5 text-(--text-muted)">
          {get_execution_kind_help_text(execution_kind)}
        </p>
      </div>

      {execution_kind === "agent" ? (
        <div className="dialog-field">
          <span className="dialog-label">发送到</span>
          <div className="flex flex-wrap gap-2">
            {target_type_options.map((opt) => (
              <UiChoiceButton
                active={target_type === opt.key}
                key={opt.key}
                onClick={() => {
                  set_target_type(opt.key);
                  on_reset_context_error();
                }}
              >
                {opt.label}
              </UiChoiceButton>
            ))}
          </div>
        </div>
      ) : null}

      <UiField
        error={execution_kind === "script" || target_type === "agent" ? agents_error : rooms_error}
        html_for="task-target-object"
        label={execution_kind === "script" || target_type === "agent" ? "目标智能体" : "目标 Room"}
      >
        <UiSelectMenu
          aria_label={execution_kind === "script" || target_type === "agent" ? "选择目标智能体" : "选择目标 Room"}
          disabled={execution_kind === "script" || target_type === "agent" ? agents_loading || agent_options.length === 0 : rooms_loading || room_options.length === 0}
          id="task-target-object"
          on_change={(value) => {
            if (execution_kind === "script" || target_type === "agent") {
              set_selected_agent_id(value);
            } else {
              set_selected_room_id(value);
            }
            on_reset_context_error();
          }}
          options={[
            {
              value: "",
              label: execution_kind === "script" || target_type === "agent"
                ? (agents_loading ? "正在加载智能体..." : "请选择智能体")
                : (rooms_loading ? "正在加载 Room..." : "请选择 Room"),
            },
            ...(execution_kind === "script" || target_type === "agent" ? agent_options : room_options).map((option) => ({
              value: option.value ?? "",
              label: option.label,
            })),
          ]}
          surface="dialog"
          value={execution_kind === "script" || target_type === "agent" ? selected_agent_id : selected_room_id}
        />
      </UiField>

      {execution_kind === "agent" ? (
        <div className="dialog-field">
          <span className="dialog-label">执行会话</span>
          <div className="flex flex-wrap gap-2">
            {execution_mode_options.map((opt) => (
              <UiChoiceButton
                active={execution_mode === opt.key}
                key={opt.key}
                onClick={() => {
                  set_execution_mode(opt.key);
                  on_reset_context_error();
                }}
              >
                {opt.label}
              </UiChoiceButton>
            ))}
          </div>
          <p className="mt-2 text-xs leading-5 text-(--text-muted)">
            {get_execution_mode_help_text(execution_mode)}
          </p>
        </div>
      ) : null}

      {execution_kind === "agent" && execution_mode === "dedicated" ? (
        <UiField html_for="task-dedicated-session-key" label="专用长期会话名称">
          <UiInput
            id="task-dedicated-session-key"
            onChange={(e) => set_dedicated_session_key(e.target.value)}
            placeholder="例如 daily-ops"
            value={dedicated_session_key}
          />
        </UiField>
      ) : null}

      {execution_kind === "agent" && require_session_selection ? (
        <UiField
          description={session_empty_message}
          error={session_error}
          html_for="task-session-key"
          label="执行会话"
        >
          <UiSelectMenu
            aria_label="选择执行会话"
            disabled={session_loading || session_options.length === 0}
            id="task-session-key"
            on_change={(value) => {
              set_selected_session_key(value);
              on_reset_context_error();
            }}
            options={[
              { value: "", label: session_loading ? "正在加载会话..." : "请选择会话" },
              ...session_options.map((option) => ({
                value: option.session_key,
                label: option.label,
              })),
            ]}
            surface="dialog"
            value={selected_session_key}
          />
        </UiField>
      ) : null}

      {execution_kind === "agent" ? (
        <div className="dialog-field">
          <span className="dialog-label">结果回传</span>
          <div className="flex flex-wrap gap-2">
            {reply_mode_options.map((opt) => (
              <UiChoiceButton
                active={reply_mode === opt.key}
                disabled={disabled_reply_modes.includes(opt.key)}
                key={opt.key}
                onClick={() => {
                  set_reply_mode(opt.key);
                  on_reset_context_error();
                }}
              >
                {opt.label}
              </UiChoiceButton>
            ))}
          </div>
          <p className="mt-2 text-xs leading-5 text-(--text-muted)">
            {get_reply_mode_help_text(reply_mode)}
          </p>
        </div>
      ) : null}

      {execution_kind === "agent" && reply_mode === "selected" ? (
        <UiField html_for="task-reply-session-key" label="回复会话">
          <UiSelectMenu
            aria_label="选择回复会话"
            disabled={session_loading || session_options.length === 0}
            id="task-reply-session-key"
            on_change={(value) => {
              set_selected_reply_session_key(value);
              on_reset_context_error();
            }}
            options={[
              { value: "", label: session_loading ? "正在加载会话..." : "请选择回复会话" },
              ...session_options.map((option) => ({
                value: option.session_key,
                label: option.label,
              })),
            ]}
            surface="dialog"
            value={selected_reply_session_key}
          />
        </UiField>
      ) : null}
    </div>
  );
}
