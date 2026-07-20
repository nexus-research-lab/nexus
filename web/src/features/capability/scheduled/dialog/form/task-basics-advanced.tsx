"use client";

import { Settings2 } from "lucide-react";

import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type {
  ChoiceDef,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  buildExecutionSessionPresentation,
  buildReplySessionPresentation,
  buildTaskAdvancedSummary,
  EXECUTION_KIND_HELP,
  EXECUTION_MODE_HELP,
  REPLY_MODE_HELP,
  type TaskBasicsActions,
  type TaskBasicsData,
  type TaskSelectPresentation,
} from "./task-basics-model";
import {
  EXECUTION_KIND_OPTIONS,
  EXECUTION_MODE_OPTIONS,
  REPLY_MODE_OPTIONS,
  TARGET_TYPE_OPTIONS,
} from "./task-form-options";

interface TaskBasicsAdvancedProps {
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
}

interface TaskChoiceFieldProps<Value extends string> {
  help?: string;
  isDisabled?: (value: Value) => boolean;
  label: string;
  onChange: (value: Value) => void;
  options: ChoiceDef<Value>[];
  value: Value;
}

const OPTION_ENABLED = () => false;

function TaskChoiceField<Value extends string>({
  help,
  isDisabled = OPTION_ENABLED,
  label,
  onChange,
  options,
  value,
}: TaskChoiceFieldProps<Value>) {
  return (
    <div className="dialog-field">
      <span className="dialog-label">{label}</span>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => (
          <UiChoiceButton
            active={value === option.key}
            disabled={isDisabled(option.key)}
            key={option.key}
            onClick={() => onChange(option.key)}
          >
            {option.label}
          </UiChoiceButton>
        ))}
      </div>
      {help ? (
        <p className="mt-2 text-xs leading-5 text-(--text-muted)">{help}</p>
      ) : null}
    </div>
  );
}

function TaskSessionField({
  id,
  onChange,
  presentation,
}: {
  id: string;
  onChange: (value: string) => void;
  presentation: TaskSelectPresentation;
}) {
  return (
    <UiField
      description={presentation.description}
      error={presentation.error}
      htmlFor={id}
      label={presentation.label}
    >
      <UiSelectMenu
        ariaLabel={presentation.ariaLabel}
        disabled={presentation.disabled}
        id={id}
        onChange={onChange}
        options={presentation.options}
        surface="dialog"
        value={presentation.value}
      />
    </UiField>
  );
}

function TaskTargetTypeField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  if (form.executionKind !== "agent") {
    return null;
  }
  return (
    <TaskChoiceField
      label="发送到"
      onChange={actions.setTargetType}
      options={TARGET_TYPE_OPTIONS}
      value={form.targetType}
    />
  );
}

function TaskDedicatedSessionField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  if (form.executionMode !== "dedicated") {
    return null;
  }
  return (
    <UiField htmlFor="task-dedicated-session-key" label="专用长期会话名称">
      <UiInput
        id="task-dedicated-session-key"
        onChange={(event) => actions.setDedicatedSessionKey(event.target.value)}
        placeholder="例如 daily-ops"
        value={form.dedicatedSessionKey}
      />
    </UiField>
  );
}

function TaskExecutionModeField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  if (form.executionKind !== "agent" || form.targetType !== "agent") {
    return null;
  }
  return (
    <>
      <TaskChoiceField
        help={EXECUTION_MODE_HELP[form.executionMode]}
        label="执行会话"
        onChange={actions.setExecutionMode}
        options={EXECUTION_MODE_OPTIONS}
        value={form.executionMode}
      />
      <TaskDedicatedSessionField actions={actions} form={form} />
    </>
  );
}

function TaskExecutionSessionField({
  actions,
  data,
  form,
}: TaskBasicsAdvancedProps) {
  const presentation = buildExecutionSessionPresentation(form, data);
  if (!presentation) {
    return null;
  }
  return (
    <TaskSessionField
      id="task-session-key"
      onChange={actions.setSelectedSessionKey}
      presentation={presentation}
    />
  );
}

function TaskReplySessionField({
  actions,
  data,
  form,
}: TaskBasicsAdvancedProps) {
  if (form.replyMode !== "selected") {
    return null;
  }
  return (
    <TaskSessionField
      id="task-reply-session-key"
      onChange={actions.setSelectedReplySessionKey}
      presentation={buildReplySessionPresentation(form, data)}
    />
  );
}

function TaskDeliveryFields(props: TaskBasicsAdvancedProps) {
  const { actions, form } = props;
  if (form.executionKind !== "agent") {
    return null;
  }
  return (
    <>
      <TaskChoiceField
        help={REPLY_MODE_HELP[form.replyMode]}
        isDisabled={(replyMode) => (
          form.executionMode === "main" && replyMode !== "none"
        )}
        label="结果回传"
        onChange={actions.setReplyMode}
        options={REPLY_MODE_OPTIONS}
        value={form.replyMode}
      />
      <TaskReplySessionField {...props} />
    </>
  );
}

function TaskExpirationField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  return (
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
  );
}

export function TaskBasicsAdvanced(props: TaskBasicsAdvancedProps) {
  const { actions, form } = props;
  return (
    <details className="group rounded-[10px] border border-(--divider-subtle-color) px-3 py-2.5">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium text-(--text-default)">
        <span className="inline-flex items-center gap-2">
          <Settings2 className="h-3.5 w-3.5 text-(--icon-default)" />
          高级设置
        </span>
        <span className="truncate text-xs font-normal text-(--text-muted)">
          {buildTaskAdvancedSummary(form)}
        </span>
      </summary>

      <div className="mt-4 flex flex-col gap-4 border-t border-(--divider-subtle-color) pt-4">
        <TaskChoiceField
          help={EXECUTION_KIND_HELP[form.executionKind]}
          label="执行方式"
          onChange={actions.setExecutionKind}
          options={EXECUTION_KIND_OPTIONS}
          value={form.executionKind}
        />
        <TaskTargetTypeField actions={actions} form={form} />
        <TaskExecutionModeField actions={actions} form={form} />
        <TaskExecutionSessionField {...props} />
        <TaskDeliveryFields {...props} />
        <TaskExpirationField actions={actions} form={form} />
      </div>
    </details>
  );
}
