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
  EXECUTION_MODE_OPTIONS,
  REPLY_MODE_OPTIONS,
} from "./task-form-options";

interface ResourceStatus {
  error: string | null;
  loading: boolean;
}

export interface TaskBasicsData {
  agentOptions: TaskDialogLabelOption[];
  agents: ResourceStatus;
  roomOptions: TaskDialogLabelOption[];
  rooms: ResourceStatus;
  sessionOptions: TaskDialogSessionOption[];
  sessions: ResourceStatus;
}

export interface TaskBasicsActions {
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

const TARGET_COPY: Record<TargetType, TargetCopy> = {
  agent: {
    ariaLabel: "选择目标智能体",
    emptyPlaceholder: "请选择智能体",
    label: "目标智能体",
    loadingPlaceholder: "正在加载智能体...",
  },
  room: {
    ariaLabel: "选择目标 Room",
    emptyPlaceholder: "请选择 Room",
    label: "目标 Room",
    loadingPlaceholder: "正在加载 Room...",
  },
};

const TARGET_TYPE_BY_EXECUTION_KIND: Record<
  ExecutionKind,
  (form: TaskFormDraft) => TargetType
> = {
  agent: (form) => form.targetType,
  script: () => "agent",
};

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

const SESSION_COPY: Record<TargetType, SessionCopy> = {
  agent: {
    ariaLabel: "选择执行会话",
    emptyMessage: "这个智能体没有可选会话",
    emptyPlaceholder: "请选择会话",
    label: "执行会话",
  },
  room: {
    ariaLabel: "选择执行成员",
    emptyMessage: "这个 Room 没有可选会话",
    emptyPlaceholder: "请选择执行成员",
    label: "执行成员",
  },
};

export const EXECUTION_KIND_HELP: Record<ExecutionKind, string> = {
  agent: "由 Agent 会话执行任务，适合需要上下文、工具调用或自然语言处理的任务。",
  script: "在目标智能体工作区直接执行脚本，输出会记录到运行历史和产物文件。",
};

export const EXECUTION_MODE_HELP: Record<ExecutionMode, string> = {
  dedicated: "第一次执行时创建一个专用长期会话，之后持续复用。",
  existing: "复用当前已有的执行上下文。",
  main: "交给目标智能体的主会话处理，适合把任务继续接在主线对话里。",
  temporary: "每次执行都会新开一个临时会话，不延续旧上下文。",
};

export const REPLY_MODE_HELP: Record<ReplyMode, string> = {
  execution: "结果回到这次执行关联的会话。",
  none: "执行结果只保存在任务自己的执行会话里。",
  selected: "结果会额外推送到你指定的一个已有会话。",
};

function buildTaskSelectOptions(
  placeholder: string,
  options: TaskDialogLabelOption[],
): TaskDialogLabelOption[] {
  return [{ label: placeholder, value: "" }, ...options];
}

export function buildTaskTargetPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
): TaskTargetPresentation {
  const targetType = TARGET_TYPE_BY_EXECUTION_KIND[form.executionKind](form);
  const copy = TARGET_COPY[targetType];
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

function choiceLabel<Value extends string>(
  options: Array<{ key: Value; label: string }>,
  value: Value,
): string {
  return options.find((option) => option.key === value)?.label ?? value;
}

const ADVANCED_SUMMARY: Record<ExecutionKind, (form: TaskFormDraft) => string> = {
  agent: (form) => [
    choiceLabel(EXECUTION_MODE_OPTIONS, form.executionMode),
    choiceLabel(REPLY_MODE_OPTIONS, form.replyMode),
  ].join(" · "),
  script: () => "脚本",
};

export function buildTaskAdvancedSummary(form: TaskFormDraft): string {
  return ADVANCED_SUMMARY[form.executionKind](form);
}

function sessionEmptyMessage(
  form: TaskFormDraft,
  data: TaskBasicsData,
): string | null {
  if (!isSessionCatalogEmpty(form, data)) {
    return null;
  }
  return SESSION_COPY[form.targetType].emptyMessage;
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
): string {
  return loading ? "正在加载会话..." : emptyPlaceholder;
}

function sessionSelectDisabled(data: TaskBasicsData): boolean {
  return data.sessions.loading || data.sessionOptions.length === 0;
}

export function buildExecutionSessionPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
): TaskSelectPresentation | null {
  if (!needsExecutionSession(form)) {
    return null;
  }

  const copy = SESSION_COPY[form.targetType];
  return {
    ariaLabel: copy.ariaLabel,
    description: sessionEmptyMessage(form, data),
    disabled: sessionSelectDisabled(data),
    error: data.sessions.error,
    label: copy.label,
    options: buildTaskSelectOptions(
      sessionPlaceholder(data.sessions.loading, copy.emptyPlaceholder),
      data.sessionOptions,
    ),
    value: form.selectedSessionKey,
  };
}

export function buildReplySessionPresentation(
  form: TaskFormDraft,
  data: TaskBasicsData,
): TaskSelectPresentation {
  return {
    ariaLabel: "选择回复会话",
    description: null,
    disabled: sessionSelectDisabled(data),
    error: data.sessions.error,
    label: "回复会话",
    options: buildTaskSelectOptions(
      sessionPlaceholder(data.sessions.loading, "请选择回复会话"),
      data.sessionOptions,
    ),
    value: form.selectedReplySessionKey,
  };
}
