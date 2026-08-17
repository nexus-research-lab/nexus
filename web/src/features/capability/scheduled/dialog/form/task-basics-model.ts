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
}

export interface TaskBasicsData {
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

export interface TaskSelectPresentation {
  ariaLabel: string;
  description: string | null;
  disabled: boolean;
  error: string | null;
  label: string;
  options: TaskDialogLabelOption[];
  value: string;
}

interface TaskTargetPresentation extends TaskSelectPresentation {
  targetType: TargetType;
}

interface TaskDeliveryTargetPresentation extends TaskSelectPresentation {
  targetType: DeliveryTargetType;
}

interface TargetCopy {
  ariaLabel: string;
  emptyPlaceholder: string;
  label: string;
  loadingPlaceholder: string;
}

interface TargetSource {
  options: TaskDialogLabelOption[];
  resource: ResourceStatus;
  value: string;
}

interface SessionCopy {
  ariaLabel: string;
  emptyMessage: string;
  emptyPlaceholder: string;
  label: string;
}

function buildTargetCopy(t: Translate): Record<TargetType, TargetCopy> {
  return {
    agent: {
      ariaLabel: t("capability.scheduled_dialog_select_target_agent"),
      emptyPlaceholder: t("capability.scheduled_dialog_select_agent"),
      label: t("capability.scheduled_dialog_target_agent"),
      loadingPlaceholder: t("capability.scheduled_dialog_loading_agents"),
    },
    room: {
      ariaLabel: t("capability.scheduled_dialog_select_target_room"),
      emptyPlaceholder: t("capability.scheduled_dialog_select_room"),
      label: t("capability.scheduled_dialog_target_room"),
      loadingPlaceholder: t("capability.scheduled_dialog_loading_rooms"),
    },
  };
}

const TARGET_VALUE: Record<TargetType, (form: TaskFormDraft) => string> = {
  agent: (form) => form.selectedAgentId,
  room: (form) => form.selectedRoomId,
};

const TARGET_SOURCE: Record<
  TargetType,
  (form: TaskFormDraft, data: TaskBasicsData) => TargetSource
> = {
  agent: (form, data) => ({
    options: data.agentOptions,
    resource: data.agents,
    value: TARGET_VALUE.agent(form),
  }),
  room: (form, data) => ({
    options: data.roomOptions,
    resource: data.rooms,
    value: TARGET_VALUE.room(form),
  }),
};

const NEEDS_EXECUTION_SESSION: Record<
  TargetType,
  (form: TaskFormDraft) => boolean
> = {
  agent: (form) => form.executionMode === "existing",
  room: () => true,
};

function buildSessionCopy(t: Translate): Record<TargetType, SessionCopy> {
  return {
    agent: {
      ariaLabel: t("capability.scheduled_dialog_select_execution_session"),
      emptyMessage: t("capability.scheduled_dialog_no_agent_sessions"),
      emptyPlaceholder: t("capability.scheduled_dialog_select_session"),
      label: t("capability.scheduled_dialog_execution_session"),
    },
    room: {
      ariaLabel: t("capability.scheduled_dialog_select_execution_session"),
      emptyMessage: t("capability.scheduled_dialog_no_room_sessions"),
      emptyPlaceholder: t("capability.scheduled_dialog_select_session"),
      label: t("capability.scheduled_dialog_execution_session"),
    },
  };
}

function buildTaskSelectOptions(
  placeholder: string,
  options: TaskDialogLabelOption[],
): TaskDialogLabelOption[] {
  return [{ label: placeholder, value: "" }, ...options];
}

export function buildTaskTargetPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
  t: Translate,
): TaskTargetPresentation {
  const targetType = form.targetType;
  const copy = buildTargetCopy(t)[targetType];
  const source = TARGET_SOURCE[targetType](form, data);
  const placeholder = source.resource.loading
    ? copy.loadingPlaceholder
    : copy.emptyPlaceholder;

  return {
    ariaLabel: copy.ariaLabel,
    description: null,
    disabled: source.resource.loading || source.options.length === 0,
    error: source.resource.error,
    label: copy.label,
    options: buildTaskSelectOptions(placeholder, source.options),
    targetType,
    value: source.value,
  };
}

export function buildTaskDeliveryTargetPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
  t: Translate,
): TaskDeliveryTargetPresentation {
  const targetType = form.deliveryTargetType;
  const copy = buildTargetCopy(t)[targetType];
  const source = targetType === "room"
    ? {
        options: data.deliveryRoomOptions,
        resource: data.rooms,
        value: form.selectedDeliveryRoomId,
      }
    : {
        options: data.agentOptions,
        resource: data.agents,
        value: form.selectedDeliveryAgentId,
      };
  const placeholder = source.resource.loading
    ? copy.loadingPlaceholder
    : copy.emptyPlaceholder;
  const label = t(targetType === "room"
    ? "capability.scheduled_dialog_delivery_room"
    : "capability.scheduled_dialog_delivery_agent");
  return {
    ariaLabel: label,
    description: null,
    disabled: source.resource.loading || source.options.length === 0,
    error: source.resource.error,
    label,
    options: buildTaskSelectOptions(placeholder, source.options),
    targetType,
    value: source.value,
  };
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

function sessionEmptyMessage(
  form: TaskFormDraft,
  data: TaskBasicsData,
  copy: Record<TargetType, SessionCopy>,
): string | null {
  if (!isSessionCatalogEmpty(form, data)) {
    return null;
  }
  return copy[form.targetType].emptyMessage;
}

function isSessionCatalogEmpty(
  form: TaskFormDraft,
  data: TaskBasicsData,
): boolean {
  const hasTarget = Boolean(TARGET_VALUE[form.targetType](form));
  const loadedWithoutOptions = !data.sessions.loading
    && data.sessionOptions.length === 0;
  return hasTarget && loadedWithoutOptions;
}

function needsExecutionSession(form: TaskFormDraft): boolean {
  return form.executionKind === "agent"
    && NEEDS_EXECUTION_SESSION[form.targetType](form);
}

function sessionPlaceholder(
  loading: boolean,
  emptyPlaceholder: string,
  t: Translate,
): string {
  return loading
    ? t("capability.scheduled_dialog_loading_sessions")
    : emptyPlaceholder;
}

function sessionSelectDisabled(data: TaskBasicsData): boolean {
  return data.sessions.loading || data.sessionOptions.length === 0;
}

export function buildExecutionSessionPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
  t: Translate,
): TaskSelectPresentation | null {
  if (!needsExecutionSession(form)) {
    return null;
  }

  const sessionCopy = buildSessionCopy(t);
  const copy = sessionCopy[form.targetType];
  return {
    ariaLabel: copy.ariaLabel,
    description: sessionEmptyMessage(form, data, sessionCopy),
    disabled: sessionSelectDisabled(data),
    error: data.sessions.error,
    label: copy.label,
    options: buildTaskSelectOptions(
      sessionPlaceholder(data.sessions.loading, copy.emptyPlaceholder, t),
      data.sessionOptions,
    ),
    value: form.selectedSessionKey,
  };
}

export function buildReplySessionPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
  t: Translate,
): TaskSelectPresentation {
  const hasRecipient = form.deliveryTargetType === "room"
    ? Boolean(form.selectedDeliveryRoomId)
    : Boolean(form.selectedDeliveryAgentId);
  const emptyDescription = hasRecipient
    && !data.deliverySessions.loading
    && data.deliverySessionOptions.length === 0
    ? t(form.deliveryTargetType === "room"
        ? "capability.scheduled_dialog_no_room_delivery_sessions"
        : "capability.scheduled_dialog_no_agent_delivery_sessions")
    : null;
  return {
    ariaLabel: t("capability.scheduled_dialog_select_reply_session"),
    description: emptyDescription,
    disabled: data.deliverySessions.loading || data.deliverySessionOptions.length === 0,
    error: data.deliverySessions.error,
    label: t("capability.scheduled_dialog_delivery_conversation"),
    options: buildTaskSelectOptions(
      sessionPlaceholder(
        data.deliverySessions.loading,
        t("capability.scheduled_dialog_choose_reply_session"),
        t,
      ),
      data.deliverySessionOptions,
    ),
    value: form.selectedReplySessionKey,
  };
}
