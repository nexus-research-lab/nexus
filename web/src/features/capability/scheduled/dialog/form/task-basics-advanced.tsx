"use client";

import { useEffect, useState } from "react";

import { Link2Off, Settings2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
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
  type TaskBasicsActions,
  type TaskBasicsData,
  type TaskSelectPresentation,
} from "./task-basics-model";
import {
  buildExecutionKindOptions,
  buildExecutionModeOptions,
  buildPermissionModeOptions,
  buildReplyModeOptions,
  buildTargetTypeOptions,
  getExecutionKindHelp,
  getExecutionModeHelp,
  getPermissionModeHelp,
  getReplyModeHelp,
} from "./task-form-options";

interface TaskBasicsAdvancedProps {
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
  isEditing: boolean;
  needsSessionRebind: boolean;
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
  const { t } = useI18n();
  if (form.executionKind !== "agent") {
    return null;
  }
  return (
    <TaskChoiceField
      label={t("capability.scheduled_dialog_send_to")}
      onChange={actions.setTargetType}
      options={buildTargetTypeOptions(t)}
      value={form.targetType}
    />
  );
}

function TaskDedicatedSessionField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  const { t } = useI18n();
  if (form.executionMode !== "dedicated") {
    return null;
  }
  return (
    <UiField
      htmlFor="task-dedicated-session-key"
      label={t("capability.scheduled_dialog_dedicated_session")}
    >
      <UiInput
        id="task-dedicated-session-key"
        onChange={(event) => actions.setDedicatedSessionKey(event.target.value)}
        placeholder={t("capability.scheduled_dialog_dedicated_session_placeholder")}
        value={form.dedicatedSessionKey}
      />
    </UiField>
  );
}

function TaskExecutionModeField({
  actions,
  form,
  isEditing,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form" | "isEditing">) {
  const { t } = useI18n();
  if (form.executionKind !== "agent" || form.targetType !== "agent") {
    return null;
  }
  const options = buildExecutionModeOptions(t).filter((option) => (
    option.key === "existing"
      || option.key === "temporary"
      || (isEditing && option.key === form.executionMode)
  ));
  return (
    <>
      <TaskChoiceField
        help={getExecutionModeHelp(form.executionMode, t)}
        label={t("capability.scheduled_dialog_execution_session")}
        onChange={actions.setExecutionMode}
        options={options}
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
  const { t } = useI18n();
  const presentation = buildExecutionSessionPresentation(form, data, t);
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
  const { t } = useI18n();
  if (form.replyMode !== "selected") {
    return null;
  }
  return (
    <TaskSessionField
      id="task-reply-session-key"
      onChange={actions.setSelectedReplySessionKey}
      presentation={buildReplySessionPresentation(form, data, t)}
    />
  );
}

function TaskDeliveryFields(props: TaskBasicsAdvancedProps) {
  const { t } = useI18n();
  const { actions, form } = props;
  if (form.executionKind !== "agent") {
    return null;
  }
  const options = buildReplyModeOptions(t).filter((option) => (
    option.key === "none"
      || option.key === "selected"
      || (props.isEditing && option.key === form.replyMode)
  ));
  return (
    <>
      <TaskChoiceField
        help={getReplyModeHelp(form.replyMode, t)}
        isDisabled={(replyMode) => (
          form.executionMode === "main" && replyMode !== "none"
        )}
        label={t("capability.scheduled_dialog_delivery")}
        onChange={actions.setReplyMode}
        options={options}
        value={form.replyMode}
      />
      <TaskReplySessionField {...props} />
    </>
  );
}

function TaskPermissionModeField({
  actions,
  form,
  isEditing,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form" | "isEditing">) {
  const { t } = useI18n();
  if (form.executionKind !== "agent") {
    return null;
  }
  return (
    <TaskChoiceField
      help={getPermissionModeHelp(form.permissionMode, t)}
      label={t("capability.scheduled_dialog_permission_mode")}
      onChange={actions.setPermissionMode}
      options={buildPermissionModeOptions(t, !isEditing)}
      value={form.permissionMode}
    />
  );
}

function TaskExpirationField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  const { t } = useI18n();
  return (
    <UiField
      description={t("capability.scheduled_dialog_expiration_description")}
      htmlFor="task-expires-at"
      label={t("capability.scheduled_dialog_expiration")}
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
  const { t } = useI18n();
  const { actions, form } = props;
  const [isOpen, setIsOpen] = useState(props.needsSessionRebind);
  useEffect(() => {
    if (props.needsSessionRebind) {
      setIsOpen(true);
    }
  }, [props.needsSessionRebind]);
  return (
    <details
      className="group rounded-[10px] border border-(--divider-subtle-color) px-3 py-2.5"
      onToggle={(event) => setIsOpen(event.currentTarget.open)}
      open={isOpen}
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium text-(--text-default)">
        <span className="inline-flex items-center gap-2">
          <Settings2 className="h-3.5 w-3.5 text-(--icon-default)" />
          {t("capability.scheduled_dialog_advanced")}
        </span>
        <span className="truncate text-xs font-normal text-(--text-muted)">
          {buildTaskAdvancedSummary(form, t)}
        </span>
      </summary>

      <div className="mt-4 flex flex-col gap-4 border-t border-(--divider-subtle-color) pt-4">
        {props.needsSessionRebind ? (
          <div
            className="flex gap-2.5 rounded-[8px] border border-[color:color-mix(in_srgb,var(--warning)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)] p-3"
            role="status"
          >
            <Link2Off className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
            <div className="min-w-0">
              <p className="text-xs font-medium text-(--text-strong)">
                {t("capability.scheduled_dialog_session_rebind_required")}
              </p>
              <p className="mt-1 text-xs leading-5 text-(--text-muted)">
                {t("capability.scheduled_dialog_session_rebind_description")}
              </p>
            </div>
          </div>
        ) : null}
        <TaskChoiceField
          help={getExecutionKindHelp(form.executionKind, t)}
          label={t("capability.scheduled_dialog_execution_kind")}
          onChange={actions.setExecutionKind}
          options={buildExecutionKindOptions(t)}
          value={form.executionKind}
        />
        <TaskTargetTypeField actions={actions} form={form} />
        <TaskExecutionModeField actions={actions} form={form} isEditing={props.isEditing} />
        <TaskExecutionSessionField {...props} />
        <TaskPermissionModeField actions={actions} form={form} isEditing={props.isEditing} />
        <TaskDeliveryFields {...props} />
        <TaskExpirationField actions={actions} form={form} />
      </div>
    </details>
  );
}
