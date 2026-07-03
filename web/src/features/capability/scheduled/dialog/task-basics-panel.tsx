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
  nameRef: RefObject<HTMLInputElement | null>;
  taskName: string;
  setTaskName: (value: string) => void;
  executionKind: ExecutionKind;
  setExecutionKind: (value: ExecutionKind) => void;
  executionKindOptions: Array<{ key: ExecutionKind; label: string }>;
  targetType: TargetType;
  setTargetType: (value: TargetType) => void;
  targetTypeOptions: Array<{ key: TargetType; label: string }>;
  selectedAgentId: string;
  setSelectedAgentId: (value: string) => void;
  selectedRoomId: string;
  setSelectedRoomId: (value: string) => void;
  agentOptions: OptionItem[];
  roomOptions: OptionItem[];
  agentsLoading: boolean;
  roomsLoading: boolean;
  agentsError: string | null;
  roomsError: string | null;
  executionMode: ExecutionMode;
  setExecutionMode: (value: ExecutionMode) => void;
  executionModeOptions: Array<{ key: ExecutionMode; label: string }>;
  dedicatedSessionKey: string;
  setDedicatedSessionKey: (value: string) => void;
  selectedSessionKey: string;
  setSelectedSessionKey: (value: string) => void;
  sessionOptions: Array<{ value: string; sessionKey: string; label: string }>;
  sessionLoading: boolean;
  sessionError: string | null;
  sessionEmptyMessage: string | null;
  requireSessionSelection: boolean;
  replyMode: ReplyMode;
  setReplyMode: (value: ReplyMode) => void;
  replyModeOptions: Array<{ key: ReplyMode; label: string }>;
  disabledReplyModes?: ReplyMode[];
  selectedReplySessionKey: string;
  setSelectedReplySessionKey: (value: string) => void;
  onResetContextError: () => void;
}

function getExecutionModeHelpText(mode: ExecutionMode): string {
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

function getExecutionKindHelpText(kind: ExecutionKind): string {
  if (kind === "script") {
    return "在目标智能体工作区直接执行脚本，输出会记录到运行历史和产物文件。";
  }
  return "由 Agent 会话执行任务，适合需要上下文、工具调用或自然语言处理的任务。";
}

function getReplyModeHelpText(mode: ReplyMode): string {
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
    nameRef: nameRef,
    taskName: taskName,
    setTaskName: setTaskName,
    executionKind: executionKind,
    setExecutionKind: setExecutionKind,
    executionKindOptions: executionKindOptions,
    targetType: targetType,
    setTargetType: setTargetType,
    targetTypeOptions: targetTypeOptions,
    selectedAgentId: selectedAgentId,
    setSelectedAgentId: setSelectedAgentId,
    selectedRoomId: selectedRoomId,
    setSelectedRoomId: setSelectedRoomId,
    agentOptions: agentOptions,
    roomOptions: roomOptions,
    agentsLoading: agentsLoading,
    roomsLoading: roomsLoading,
    agentsError: agentsError,
    roomsError: roomsError,
    executionMode: executionMode,
    setExecutionMode: setExecutionMode,
    executionModeOptions: executionModeOptions,
    dedicatedSessionKey: dedicatedSessionKey,
    setDedicatedSessionKey: setDedicatedSessionKey,
    selectedSessionKey: selectedSessionKey,
    setSelectedSessionKey: setSelectedSessionKey,
    sessionOptions: sessionOptions,
    sessionLoading: sessionLoading,
    sessionError: sessionError,
    sessionEmptyMessage: sessionEmptyMessage,
    requireSessionSelection: requireSessionSelection,
    replyMode: replyMode,
    setReplyMode: setReplyMode,
    replyModeOptions: replyModeOptions,
    disabledReplyModes: disabledReplyModes = [],
    selectedReplySessionKey: selectedReplySessionKey,
    setSelectedReplySessionKey: setSelectedReplySessionKey,
    onResetContextError: onResetContextError,
  } = props;

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <UiField htmlFor="task-name" label="任务名称">
        <UiInput
          ref={nameRef}
          id="task-name"
          onChange={(e) => setTaskName(e.target.value)}
          placeholder="输入任务名称"
          value={taskName}
        />
      </UiField>

      <div className="dialog-field">
        <span className="dialog-label">执行方式</span>
        <div className="flex flex-wrap gap-2">
          {executionKindOptions.map((opt) => (
            <UiChoiceButton
              active={executionKind === opt.key}
              key={opt.key}
              onClick={() => {
                setExecutionKind(opt.key);
                onResetContextError();
              }}
            >
              {opt.label}
            </UiChoiceButton>
          ))}
        </div>
        <p className="mt-2 text-xs leading-5 text-(--text-muted)">
          {getExecutionKindHelpText(executionKind)}
        </p>
      </div>

      {executionKind === "agent" ? (
        <div className="dialog-field">
          <span className="dialog-label">发送到</span>
          <div className="flex flex-wrap gap-2">
            {targetTypeOptions.map((opt) => (
              <UiChoiceButton
                active={targetType === opt.key}
                key={opt.key}
                onClick={() => {
                  setTargetType(opt.key);
                  onResetContextError();
                }}
              >
                {opt.label}
              </UiChoiceButton>
            ))}
          </div>
        </div>
      ) : null}

      <UiField
        error={executionKind === "script" || targetType === "agent" ? agentsError : roomsError}
        htmlFor="task-target-object"
        label={executionKind === "script" || targetType === "agent" ? "目标智能体" : "目标 Room"}
      >
        <UiSelectMenu
          ariaLabel={executionKind === "script" || targetType === "agent" ? "选择目标智能体" : "选择目标 Room"}
          disabled={executionKind === "script" || targetType === "agent" ? agentsLoading || agentOptions.length === 0 : roomsLoading || roomOptions.length === 0}
          id="task-target-object"
          onChange={(value) => {
            if (executionKind === "script" || targetType === "agent") {
              setSelectedAgentId(value);
            } else {
              setSelectedRoomId(value);
            }
            onResetContextError();
          }}
          options={[
            {
              value: "",
              label: executionKind === "script" || targetType === "agent"
                ? (agentsLoading ? "正在加载智能体..." : "请选择智能体")
                : (roomsLoading ? "正在加载 Room..." : "请选择 Room"),
            },
            ...(executionKind === "script" || targetType === "agent" ? agentOptions : roomOptions).map((option) => ({
              value: option.value ?? "",
              label: option.label,
            })),
          ]}
          surface="dialog"
          value={executionKind === "script" || targetType === "agent" ? selectedAgentId : selectedRoomId}
        />
      </UiField>

      {executionKind === "agent" ? (
        <div className="dialog-field">
          <span className="dialog-label">执行会话</span>
          <div className="flex flex-wrap gap-2">
            {executionModeOptions.map((opt) => (
              <UiChoiceButton
                active={executionMode === opt.key}
                key={opt.key}
                onClick={() => {
                  setExecutionMode(opt.key);
                  onResetContextError();
                }}
              >
                {opt.label}
              </UiChoiceButton>
            ))}
          </div>
          <p className="mt-2 text-xs leading-5 text-(--text-muted)">
            {getExecutionModeHelpText(executionMode)}
          </p>
        </div>
      ) : null}

      {executionKind === "agent" && executionMode === "dedicated" ? (
        <UiField htmlFor="task-dedicated-session-key" label="专用长期会话名称">
          <UiInput
            id="task-dedicated-session-key"
            onChange={(e) => setDedicatedSessionKey(e.target.value)}
            placeholder="例如 daily-ops"
            value={dedicatedSessionKey}
          />
        </UiField>
      ) : null}

      {executionKind === "agent" && requireSessionSelection ? (
        <UiField
          description={sessionEmptyMessage}
          error={sessionError}
          htmlFor="task-session-key"
          label={targetType === "room" ? "执行成员" : "执行会话"}
        >
          <UiSelectMenu
            ariaLabel={targetType === "room" ? "选择执行成员" : "选择执行会话"}
            disabled={sessionLoading || sessionOptions.length === 0}
            id="task-session-key"
            onChange={(value) => {
              setSelectedSessionKey(value);
              onResetContextError();
            }}
            options={[
              { value: "", label: sessionLoading ? "正在加载会话..." : targetType === "room" ? "请选择执行成员" : "请选择会话" },
              ...sessionOptions.map((option) => ({
                value: option.value,
                label: option.label,
              })),
            ]}
            surface="dialog"
            value={selectedSessionKey}
          />
        </UiField>
      ) : null}

      {executionKind === "agent" ? (
        <div className="dialog-field">
          <span className="dialog-label">结果回传</span>
          <div className="flex flex-wrap gap-2">
            {replyModeOptions.map((opt) => (
              <UiChoiceButton
                active={replyMode === opt.key}
                disabled={disabledReplyModes.includes(opt.key)}
                key={opt.key}
                onClick={() => {
                  setReplyMode(opt.key);
                  onResetContextError();
                }}
              >
                {opt.label}
              </UiChoiceButton>
            ))}
          </div>
          <p className="mt-2 text-xs leading-5 text-(--text-muted)">
            {getReplyModeHelpText(replyMode)}
          </p>
        </div>
      ) : null}

      {executionKind === "agent" && replyMode === "selected" ? (
        <UiField htmlFor="task-reply-session-key" label="回复会话">
          <UiSelectMenu
            ariaLabel="选择回复会话"
            disabled={sessionLoading || sessionOptions.length === 0}
            id="task-reply-session-key"
            onChange={(value) => {
              setSelectedReplySessionKey(value);
              onResetContextError();
            }}
            options={[
              { value: "", label: sessionLoading ? "正在加载会话..." : "请选择回复会话" },
              ...sessionOptions.map((option) => ({
                value: option.value,
                label: option.label,
              })),
            ]}
            surface="dialog"
            value={selectedReplySessionKey}
          />
        </UiField>
      ) : null}
    </div>
  );
}
