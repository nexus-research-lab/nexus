// INPUT: 执行/投递/权限草稿、资源投影与字段变更命令。
// OUTPUT: 实例级字段/具名选择组与高级摘要；显式缺项 Agent 保留为禁用显示项。
// POS: Scheduled 基础表单的高级视图；不维护资源请求或提交事务。

"use client";

import { resolveRuntimePermissionMode } from "@/lib/agent-options";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import { useEffect, useId, useState } from "react";

import { Link2Off, Settings2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { includeUnavailableAgentSelection } from "@/lib/agent-selection-options";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type {
  ChoiceDef,
  TargetType,
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
  buildDeliveryTargetTypeOptions,
  buildExecutionModeOptions,
  buildPermissionModeOptions,
  buildReplyModeOptions,
  getExecutionModeHelp,
  getPermissionModeHelp,
  getReplyModeHelp,
} from "./task-form-options";

interface TaskBasicsAdvancedProps {
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  deliveryTarget: TaskSelectPresentation;
  deliveryTargetActions: Record<TargetType, (value: string) => void>;
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

function TaskChoiceField<Value extends string>({
  help,
  isDisabled,
  label,
  onChange,
  options,
  value,
}: TaskChoiceFieldProps<Value>) {
  return (
    <UiField description={help} label={label}>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => (
          <UiChoiceButton
            active={value === option.key}
            disabled={isDisabled?.(option.key)}
            key={option.key}
            onClick={() => onChange(option.key)}
          >
            {option.label}
          </UiChoiceButton>
        ))}
      </div>
    </UiField>
  );
}

function TaskSessionField({
  onChange,
  presentation,
}: {
  onChange: (value: string) => void;
  presentation: TaskSelectPresentation;
}) {
  const id = useId();
  return (
    <div className="space-y-2">
      <UiField
        description={presentation.error ? undefined : presentation.description}
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
      {presentation.error && presentation.retry ? (
        <TaskResourceFailure onRetry={presentation.retry} />
      ) : null}
    </div>
  );
}

export function TaskResourceFailure({
  onRetry,
}: {
  onRetry: () => void;
}) {
  const { t } = useI18n();
  return (
    <UiResourceState
      className="min-h-0 py-3"
      impact={t("capability.scheduled_dialog_resource_load_impact")}
      primaryAction={{
        label: t("state.retry"),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={t("capability.scheduled_dialog_resource_load_title")}
      urgency="polite"
      variant="card"
    />
  );
}

function TaskDeliveryTargetTypeField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  const { t } = useI18n();
  if (form.executionKind !== "agent" || form.replyMode !== "selected") {
    return null;
  }
  return (
    <TaskChoiceField
      label={t("capability.scheduled_dialog_delivery_target_type")}
      onChange={actions.setDeliveryTargetType}
      options={buildDeliveryTargetTypeOptions(t)}
      value={form.deliveryTargetType}
    />
  );
}

function TaskDeliveryTargetField({
  deliveryTarget,
  deliveryTargetActions,
  form,
}: TaskBasicsAdvancedProps) {
  if (form.executionKind !== "agent" || form.replyMode !== "selected") {
    return null;
  }
  return (
    <TaskSessionField
      onChange={deliveryTargetActions[form.deliveryTargetType]}
      presentation={deliveryTarget}
    />
  );
}

function TaskDedicatedSessionField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  const { t } = useI18n();
  const id = useId();
  if (form.executionMode !== "dedicated") {
    return null;
  }
  return (
    <UiField
      htmlFor={id}
      label={t("capability.scheduled_dialog_dedicated_session")}
    >
      <UiInput
        id={id}
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
      onChange={actions.setSelectedSessionKey}
      presentation={presentation}
    />
  );
}

function TaskRoomAgentField({
  actions,
  data,
  form,
}: TaskBasicsAdvancedProps) {
  const { t } = useI18n();
  if (form.targetType !== "room" || !form.selectedSessionKey) {
    return null;
  }
  const defaultLabel = data.defaultExecutionRoomAgentId
    ? t("capability.scheduled_dialog_default_room_host")
    : t("capability.scheduled_dialog_select_room_agent");
  return (
    <TaskSessionField
      onChange={actions.setSelectedAgentId}
      presentation={{
        ariaLabel: t("capability.scheduled_dialog_select_room_agent"),
        description: data.executionRoomAgentOptions.length === 0
          ? t("capability.scheduled_dialog_no_room_agents")
          : t("capability.scheduled_dialog_room_agent_default_help"),
        disabled: data.executionRoomAgentOptions.length === 0,
        error: null,
        label: t("capability.scheduled_dialog_execution_agent"),
        options: [
          { label: defaultLabel, value: "" },
          ...includeUnavailableAgentSelection(data.executionRoomAgentOptions, form.selectedAgentId, t),
        ],
        value: form.selectedAgentId,
      }}
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
      onChange={actions.setSelectedReplySessionKey}
      presentation={buildReplySessionPresentation(form, data, t)}
    />
  );
}

function TaskDeliveryRoomAgentField({
  actions,
  data,
  form,
}: TaskBasicsAdvancedProps) {
  const { t } = useI18n();
  if (form.replyMode !== "selected"
    || form.deliveryTargetType !== "room"
    || !form.selectedReplySessionKey) {
    return null;
  }
  const defaultLabel = data.defaultDeliveryRoomAgentId
    ? t("capability.scheduled_dialog_default_room_host")
    : t("capability.scheduled_dialog_select_room_agent");
  return (
    <TaskSessionField
      onChange={actions.setSelectedDeliveryPresenterAgentId}
      presentation={{
        ariaLabel: t("capability.scheduled_dialog_select_delivery_room_agent"),
        description: data.deliveryRoomAgentOptions.length === 0
          ? t("capability.scheduled_dialog_no_room_agents")
          : t("capability.scheduled_dialog_delivery_room_agent_help"),
        disabled: data.deliveryRoomAgentOptions.length === 0,
        error: null,
        label: t("capability.scheduled_dialog_delivery_room_agent"),
        options: [
          { label: defaultLabel, value: "" },
          ...includeUnavailableAgentSelection(data.deliveryRoomAgentOptions, form.selectedDeliveryPresenterAgentId, t),
        ],
        value: form.selectedDeliveryPresenterAgentId,
      }}
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
      <TaskDeliveryTargetTypeField actions={actions} form={form} />
      <TaskDeliveryTargetField {...props} />
      <TaskReplySessionField {...props} />
      <TaskDeliveryRoomAgentField {...props} />
    </>
  );
}

function TaskPermissionModeField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  const { t } = useI18n();
  const runtimeKind = useDefaultAgentRuntimeKind();
  const effectivePermissionMode = resolveRuntimePermissionMode(form.permissionMode, runtimeKind);
  if (form.executionKind !== "agent") {
    return null;
  }
  return (
    <TaskChoiceField
      help={getPermissionModeHelp(effectivePermissionMode, t)}
      label={t("capability.scheduled_dialog_permission_mode")}
      onChange={actions.setPermissionMode}
      options={buildPermissionModeOptions(t, true, runtimeKind)}
      value={effectivePermissionMode}
    />
  );
}

function TaskExpirationField({
  actions,
  form,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form">) {
  const { t } = useI18n();
  const id = useId();
  return (
    <UiField
      description={t("capability.scheduled_dialog_expiration_description")}
      htmlFor={id}
      label={t("capability.scheduled_dialog_expiration")}
    >
      <UiInput
        id={id}
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
    <>
      {props.needsSessionRebind ? (
        <UiInlineNotice
          icon={<Link2Off />}
          message={t("capability.scheduled_dialog_session_rebind_description")}
          title={t("capability.scheduled_dialog_session_rebind_required")}
          tone="warning"
        />
      ) : null}

      <UiPanel className="flex flex-col gap-4" padding="sm" radius="md">
        <TaskExecutionModeField actions={actions} form={form} isEditing={props.isEditing} />
        <TaskExecutionSessionField {...props} />
        <TaskRoomAgentField {...props} />
      </UiPanel>

      <UiPanel className="flex flex-col gap-4" padding="sm" radius="md">
        <TaskDeliveryFields {...props} />
      </UiPanel>

      <UiDisclosure
        contentClassName="flex flex-col gap-4"
        label={t("capability.scheduled_dialog_advanced")}
        leading={<Settings2 className="h-3.5 w-3.5 text-(--icon-default)" />}
        meta={(
          <span className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>
            {buildTaskAdvancedSummary(form, t)}
          </span>
        )}
        onToggle={(event) => setIsOpen(event.currentTarget.open)}
        open={isOpen}
        summaryRole="control"
        variant="panel"
      >
          <TaskPermissionModeField actions={actions} form={form} />
          <TaskExpirationField actions={actions} form={form} />
      </UiDisclosure>
    </>
  );
}
