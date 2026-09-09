// INPUT: 执行/投递/权限草稿、资源投影与字段变更命令。
// OUTPUT: 权限、时区、到期与旧专用会话设置；执行/回复成员由基础目标区展示。
// POS: Scheduled 基础表单的高级视图；不维护资源请求或提交事务。

"use client";

import { AGENT_PERMISSION_MODES, resolveRuntimePermissionMode } from "@/lib/agent-options";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import { useEffect, useId, useState, type ReactNode } from "react";

import { Link2Off, Settings2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type {
  ChoiceDef,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  buildTaskAdvancedSummary,
  type TaskBasicsActions,
  type TaskBasicsData,
} from "./task-basics-model";
import {
  buildPermissionModeOptions,
  getPermissionModeHelp,
} from "./task-form-options";

interface TaskBasicsAdvancedProps {
  children?: ReactNode;
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
  needsSessionRebind: boolean;
  expandAdvanced?: boolean;
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
  const id = useId();
  return (
    <UiField htmlFor={id} description={help} className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4 gap-y-1 [&>p]:col-span-2" label={label}>
      <UiSelectMenu id={id} ariaLabel={label} value={value} onChange={(next) => onChange(next as Value)}
        options={options.map((option) => ({label: option.label, value: option.key, disabled: isDisabled?.(option.key)}))}
        surface="plain" className="text-right" />
    </UiField>
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

function TaskPermissionModeField({
  actions,
  form,
  data,
}: Pick<TaskBasicsAdvancedProps, "actions" | "form" | "data">) {
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
      options={buildPermissionModeOptions(t, true, runtimeKind).map((option) => {
        if (option.key !== "copy") return option;
        const mode = AGENT_PERMISSION_MODES.find((item) => item.value === data.inheritedPermissionMode);
        return { ...option, label: `${option.label} · ${mode ? t(mode.labelKey) : t("capability.scheduled_dialog_permission_pending")}` };
      })}
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
    if (props.needsSessionRebind || props.expandAdvanced) {
      setIsOpen(true);
    }
  }, [props.needsSessionRebind, props.expandAdvanced]);
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
        variant="inline"
      >
          {props.children}
          <TaskDedicatedSessionField actions={actions} form={form} />
          <TaskPermissionModeField actions={actions} form={form} data={props.data} />
          <TaskExpirationField actions={actions} form={form} />
      </UiDisclosure>
    </>
  );
}
