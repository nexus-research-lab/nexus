// INPUT: Automation 草稿、资源候选与当前语言。
// OUTPUT: 共享表单输入/命令契约与高级、提交摘要；目标候选展示由目标选择器模型负责。
// POS: 基础/高级表单的只读投影，候选资格和提交校验保持各自领域所有者。
import { formatScheduledTaskSchedule } from "../../scheduled-formatters";
import { buildSchedule } from "./task-form-submit";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type {
  DeliveryTargetType,
  ExecutionMode,
  PermissionMode,
  ReplyMode,
  TargetType,
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
  TaskScheduleDraft,
  TaskDestinationOption,
} from "../scheduled-task-dialog-types";
import {
  buildExecutionModeOptions,
  buildPermissionModeOptions,
  buildReplyModeOptions,
} from "./task-form-options";

type Translate = I18nContextValue["t"];

interface ResourceStatus {
  error: string | null;
  loading: boolean;
  retry: () => void;
}

export interface TaskBasicsData {
  inheritedPermissionMode: string | null;
  destinations: TaskDestinationOption[];
  destinationStatus: ResourceStatus;
  agentOptions: TaskDialogLabelOption[];
  agents: ResourceStatus;
  deliveryRoomOptions: TaskDialogLabelOption[];
  deliveryRoomAgentOptions: TaskDialogLabelOption[];
  defaultDeliveryRoomAgentId: string;
  defaultExecutionRoomAgentId: string;
  executionRoomAgentOptions: TaskDialogLabelOption[];
  deliverySessionOptions: TaskDialogSessionOption[];
  deliverySessions: ResourceStatus;
  roomOptions: TaskDialogLabelOption[];
  rooms: ResourceStatus;
  sessionOptions: TaskDialogSessionOption[];
  sessions: ResourceStatus;
}

export interface TaskBasicsActions {
  selectExecution: (option: TaskDestinationOption) => void;
  selectDelivery: (option: TaskDestinationOption | null) => void;
  setDedicatedSessionKey: (value: string) => void;
  setDeliveryTargetType: (value: DeliveryTargetType) => void;
  setExpiresAt: (value: string) => void;
  setExecutionMode: (value: ExecutionMode) => void;
  setPermissionMode: (value: PermissionMode) => void;
  setReplyMode: (value: ReplyMode) => void;
  setSelectedAgentId: (value: string) => void;
  setSelectedDeliveryAgentId: (value: string) => void;
  setSelectedDeliveryPresenterAgentId: (value: string) => void;
  setSelectedDeliveryRoomId: (value: string) => void;
  setSelectedReplySessionKey: (value: string) => void;
  setSelectedRoomId: (value: string) => void;
  setSelectedSessionKey: (value: string) => void;
  setTargetType: (value: TargetType) => void;
  setTaskName: (value: string) => void;
}

function choiceLabel<Value extends string>(
  options: Array<{ key: Value; label: string }>,
  value: Value,
): string {
  return options.find((option) => option.key === value)?.label ?? value;
}

export function buildTaskAdvancedSummary(
  form: TaskFormDraft,
  t: Translate,
): string {
  return [
    choiceLabel(buildExecutionModeOptions(t), form.executionMode),
    choiceLabel(buildPermissionModeOptions(t), form.permissionMode),
    choiceLabel(buildReplyModeOptions(t), form.replyMode),
  ].join(" · ");
}

export function buildTaskConfirmationSummary(
  form: TaskFormDraft,
  schedule: TaskScheduleDraft,
  data: TaskBasicsData,
  t: Translate,
): string {
  let when: string;
  try {
    const value = buildSchedule(schedule, t);
    // 单次时间使用任务时区的输入值，避免浏览器本地时区转换。
    when = value.kind === "at" ? value.run_at.replace("T", " ") : formatScheduledTaskSchedule(value, t);
  } catch {
    when = t("capability.scheduled_dialog_summary_time_pending");
  }
  const agentId = form.selectedAgentId || data.defaultExecutionRoomAgentId;
  const agent = data.agentOptions.find((item) => item.value === agentId)?.label
    || t("capability.scheduled_dialog_select_agent");
  const instruction = form.instruction.trim().replace(/\s+/g, " ");
  const task = instruction.length > 60 ? `${instruction.slice(0, 60)}…` : instruction;
  const recipient = data.destinations.find((item) => item.sessionKey === form.selectedReplySessionKey
    && item.targetType === form.deliveryTargetType
    && (item.targetType === "agent" ? item.agentId === form.selectedDeliveryAgentId : item.roomId === form.selectedDeliveryRoomId));
  const delivery = form.replyMode === "none"
    ? t("capability.scheduled_dialog_summary_no_delivery")
    : recipient
      ? t("capability.scheduled_dialog_summary_delivery", { chat: `${recipient.group} · ${recipient.label}` })
      : t("capability.scheduled_dialog_summary_recipient_pending");
  return t("capability.scheduled_dialog_summary", {
    when: `${when} (${schedule.timezone.trim() || "Asia/Shanghai"})`,
    agent,
    task: task || t("capability.scheduled_dialog_summary_instruction_pending"),
    delivery,
  }) + (form.enabled ? "" : ` ${t("capability.scheduled_dialog_summary_paused")}`);
}
