"use client";

import { type RefObject } from "react";
import { Settings2 } from "lucide-react";

import { UiChoiceButton } from "@/shared/ui/choice";
import { UiField, UiInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type {
  ExecutionKind,
  ExecutionMode,
  ReplyMode,
  TargetType,
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  EXECUTION_KIND_OPTIONS,
  EXECUTION_MODE_OPTIONS,
  REPLY_MODE_OPTIONS,
  TARGET_TYPE_OPTIONS,
} from "./task-form-options";

interface ResourceStatus {
  error: string | null;
  loading: boolean;
}

interface TaskBasicsData {
  agentOptions: TaskDialogLabelOption[];
  agents: ResourceStatus;
  roomOptions: TaskDialogLabelOption[];
  rooms: ResourceStatus;
  sessionOptions: TaskDialogSessionOption[];
  sessions: ResourceStatus;
}

interface TaskBasicsActions {
  setDedicatedSessionKey: (value: string) => void;
  setExpiresAt: (value: string) => void;
  setExecutionKind: (value: ExecutionKind) => void;
  setExecutionMode: (value: ExecutionMode) => void;
  setReplyMode: (value: ReplyMode) => void;
  setSelectedAgentId: (value: string) => void;
  setSelectedReplySessionKey: (value: string) => void;
  setSelectedRoomId: (value: string) => void;
  setSelectedSessionKey: (value: string) => void;
  setTargetType: (value: TargetType) => void;
  setTaskName: (value: string) => void;
}

interface TaskBasicsPanelProps {
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
  nameRef: RefObject<HTMLInputElement | null>;
}

const EXECUTION_KIND_HELP: Record<ExecutionKind, string> = {
  agent: "由 Agent 会话执行任务，适合需要上下文、工具调用或自然语言处理的任务。",
  script: "在目标智能体工作区直接执行脚本，输出会记录到运行历史和产物文件。",
};

const EXECUTION_MODE_HELP: Record<ExecutionMode, string> = {
  dedicated: "第一次执行时创建一个专用长期会话，之后持续复用。",
  existing: "复用当前已有的执行上下文。",
  main: "交给目标智能体的主会话处理，适合把任务继续接在主线对话里。",
  temporary: "每次执行都会新开一个临时会话，不延续旧上下文。",
};

const REPLY_MODE_HELP: Record<ReplyMode, string> = {
  execution: "结果回到这次执行关联的会话。",
  none: "执行结果只保存在任务自己的执行会话里。",
  selected: "结果会额外推送到你指定的一个已有会话。",
};

function selectOptions(
  placeholder: string,
  options: TaskDialogLabelOption[],
) {
  return [{ label: placeholder, value: "" }, ...options];
}

function sessionEmptyMessage(
  form: TaskFormDraft,
  data: TaskBasicsData,
): string | null {
  const targetSelected = form.targetType === "room"
    ? form.selectedRoomId
    : form.selectedAgentId;
  if (!targetSelected || data.sessions.loading || data.sessionOptions.length > 0) {
    return null;
  }
  return form.targetType === "room"
    ? "这个 Room 没有可选会话"
    : "这个智能体没有可选会话";
}

export function TaskBasicsPanel({
  actions,
  data,
  form,
  nameRef,
}: TaskBasicsPanelProps) {
  const usesAgentTarget = form.executionKind === "script"
    || form.targetType === "agent";
  const targetOptions = usesAgentTarget ? data.agentOptions : data.roomOptions;
  const targetResource = usesAgentTarget ? data.agents : data.rooms;
  const targetLabel = usesAgentTarget ? "目标智能体" : "目标 Room";
  const needsExecutionSession = form.executionKind === "agent"
    && (form.targetType === "room" || form.executionMode === "existing");
  const emptySessionMessage = sessionEmptyMessage(form, data);
  const advancedSummary = form.executionKind === "script"
    ? "脚本"
    : [
        EXECUTION_MODE_OPTIONS.find((option) => option.key === form.executionMode)?.label,
        REPLY_MODE_OPTIONS.find((option) => option.key === form.replyMode)?.label,
      ].filter(Boolean).join(" · ");

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <UiField htmlFor="task-name" label="任务名称">
        <UiInput
          ref={nameRef}
          id="task-name"
          onChange={(event) => actions.setTaskName(event.target.value)}
          placeholder="输入任务名称"
          value={form.taskName}
        />
      </UiField>

      <UiField
        error={targetResource.error}
        htmlFor="task-target-object"
        label={targetLabel}
      >
        <UiSelectMenu
          ariaLabel={`选择${targetLabel}`}
          disabled={targetResource.loading || targetOptions.length === 0}
          id="task-target-object"
          onChange={usesAgentTarget
            ? actions.setSelectedAgentId
            : actions.setSelectedRoomId}
          options={selectOptions(
            targetResource.loading
              ? `正在加载${targetLabel.replace("目标", "")}...`
              : `请选择${targetLabel.replace("目标", "")}`,
            targetOptions,
          )}
          surface="dialog"
          value={usesAgentTarget ? form.selectedAgentId : form.selectedRoomId}
        />
      </UiField>

      <details className="group rounded-[12px] border border-(--divider-subtle-color) px-3 py-2.5">
        <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium text-(--text-default)">
          <span className="inline-flex items-center gap-2">
            <Settings2 className="h-3.5 w-3.5 text-(--icon-default)" />
            高级设置
          </span>
          <span className="truncate text-xs font-normal text-(--text-muted)">
            {advancedSummary}
          </span>
        </summary>

        <div className="mt-4 flex flex-col gap-4 border-t border-(--divider-subtle-color) pt-4">
          <div className="dialog-field">
            <span className="dialog-label">执行方式</span>
            <div className="flex flex-wrap gap-2">
              {EXECUTION_KIND_OPTIONS.map((option) => (
                <UiChoiceButton
                  active={form.executionKind === option.key}
                  key={option.key}
                  onClick={() => actions.setExecutionKind(option.key)}
                >
                  {option.label}
                </UiChoiceButton>
              ))}
            </div>
            <p className="mt-2 text-xs leading-5 text-(--text-muted)">
              {EXECUTION_KIND_HELP[form.executionKind]}
            </p>
          </div>

          {form.executionKind === "agent" ? (
            <div className="dialog-field">
              <span className="dialog-label">发送到</span>
              <div className="flex flex-wrap gap-2">
                {TARGET_TYPE_OPTIONS.map((option) => (
                  <UiChoiceButton
                    active={form.targetType === option.key}
                    key={option.key}
                    onClick={() => actions.setTargetType(option.key)}
                  >
                    {option.label}
                  </UiChoiceButton>
                ))}
              </div>
            </div>
          ) : null}

          {form.executionKind === "agent" && form.targetType === "agent" ? (
            <div className="dialog-field">
              <span className="dialog-label">执行会话</span>
              <div className="flex flex-wrap gap-2">
                {EXECUTION_MODE_OPTIONS.map((option) => (
                  <UiChoiceButton
                    active={form.executionMode === option.key}
                    key={option.key}
                    onClick={() => actions.setExecutionMode(option.key)}
                  >
                    {option.label}
                  </UiChoiceButton>
                ))}
              </div>
              <p className="mt-2 text-xs leading-5 text-(--text-muted)">
                {EXECUTION_MODE_HELP[form.executionMode]}
              </p>
            </div>
          ) : null}

          {form.executionKind === "agent" && form.executionMode === "dedicated" ? (
            <UiField htmlFor="task-dedicated-session-key" label="专用长期会话名称">
              <UiInput
                id="task-dedicated-session-key"
                onChange={(event) => actions.setDedicatedSessionKey(event.target.value)}
                placeholder="例如 daily-ops"
                value={form.dedicatedSessionKey}
              />
            </UiField>
          ) : null}

          {needsExecutionSession ? (
            <UiField
              description={emptySessionMessage}
              error={data.sessions.error}
              htmlFor="task-session-key"
              label={form.targetType === "room" ? "执行成员" : "执行会话"}
            >
              <UiSelectMenu
                ariaLabel={form.targetType === "room" ? "选择执行成员" : "选择执行会话"}
                disabled={data.sessions.loading || data.sessionOptions.length === 0}
                id="task-session-key"
                onChange={actions.setSelectedSessionKey}
                options={selectOptions(
                  data.sessions.loading
                    ? "正在加载会话..."
                    : form.targetType === "room" ? "请选择执行成员" : "请选择会话",
                  data.sessionOptions,
                )}
                surface="dialog"
                value={form.selectedSessionKey}
              />
            </UiField>
          ) : null}

          {form.executionKind === "agent" ? (
            <div className="dialog-field">
              <span className="dialog-label">结果回传</span>
              <div className="flex flex-wrap gap-2">
                {REPLY_MODE_OPTIONS.map((option) => (
                  <UiChoiceButton
                    active={form.replyMode === option.key}
                    disabled={form.executionMode === "main" && option.key !== "none"}
                    key={option.key}
                    onClick={() => actions.setReplyMode(option.key)}
                  >
                    {option.label}
                  </UiChoiceButton>
                ))}
              </div>
              <p className="mt-2 text-xs leading-5 text-(--text-muted)">
                {REPLY_MODE_HELP[form.replyMode]}
              </p>
            </div>
          ) : null}

          {form.executionKind === "agent" && form.replyMode === "selected" ? (
            <UiField htmlFor="task-reply-session-key" label="回复会话">
              <UiSelectMenu
                ariaLabel="选择回复会话"
                disabled={data.sessions.loading || data.sessionOptions.length === 0}
                id="task-reply-session-key"
                onChange={actions.setSelectedReplySessionKey}
                options={selectOptions(
                  data.sessions.loading ? "正在加载会话..." : "请选择回复会话",
                  data.sessionOptions,
                )}
                surface="dialog"
                value={form.selectedReplySessionKey}
              />
            </UiField>
          ) : null}

          <UiField
            description="可选。到期后任务会自动停用，时间按任务时区计算。"
            htmlFor="task-expires-at"
            label="任务有效期"
          >
            <UiInput
              id="task-expires-at"
              onChange={(event) => actions.setExpiresAt(event.target.value)}
              type="datetime-local"
              value={form.expiresAt}
            />
          </UiField>
        </div>
      </details>
    </div>
  );
}
