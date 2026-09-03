// INPUT: 基础任务草稿、资源投影、变更命令与名称输入引用。
// OUTPUT: 使用共享字段、选择控件和高级分组的任务身份与执行位置表单。
// POS: Scheduled 创建/编辑左栏纯视图；不加载资源或提交任务。

"use client";

import { type RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type {
  TargetType,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  TaskBasicsAdvanced,
  TaskResourceFailure,
} from "./task-basics-advanced";
import {
  buildTaskDeliveryTargetPresentation,
  buildTaskTargetPresentation,
  type TaskBasicsActions,
  type TaskBasicsData,
} from "./task-basics-model";
import { buildTargetTypeOptions } from "./task-form-options";

interface TaskBasicsPanelProps {
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
  isEditing: boolean;
  needsSessionRebind: boolean;
  nameRef: RefObject<HTMLInputElement | null>;
}

export function TaskBasicsPanel({
  actions,
  data,
  form,
  isEditing,
  needsSessionRebind,
  nameRef,
}: TaskBasicsPanelProps) {
  const { t } = useI18n();
  const target = buildTaskTargetPresentation(form, data, t);
  const targetActions: Record<TargetType, (value: string) => void> = {
    agent: actions.setSelectedAgentId,
    room: actions.setSelectedRoomId,
  };
  const setTarget = targetActions[target.targetType];
  const deliveryTarget = buildTaskDeliveryTargetPresentation(form, data, t);
  const deliveryTargetActions: Record<TargetType, (value: string) => void> = {
    agent: actions.setSelectedDeliveryAgentId,
    room: actions.setSelectedDeliveryRoomId,
  };

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <UiField
        htmlFor="task-name"
        label={t("capability.scheduled_dialog_task_name")}
        required
      >
        <UiInput
          ref={nameRef}
          id="task-name"
          onChange={(event) => actions.setTaskName(event.target.value)}
          placeholder={t("capability.scheduled_dialog_task_name_placeholder")}
          required
          value={form.taskName}
        />
      </UiField>

      <div className="dialog-field">
        <span className="dialog-label">
          {t("capability.scheduled_dialog_execution_location")}
        </span>
        <div className="flex flex-wrap gap-2">
          {buildTargetTypeOptions(t).map((option) => (
            <UiChoiceButton
              active={form.targetType === option.key}
              key={option.key}
              onClick={() => actions.setTargetType(option.key)}
            >
              {option.label}
            </UiChoiceButton>
          ))}
        </div>
        <p className={cn(
          "mt-2",
          getUiTypographyClassName({ role: "caption", tone: "muted" }),
        )}>
          {t("capability.scheduled_dialog_execution_location_help")}
        </p>
      </div>

      <div className="space-y-2">
        <UiField
          htmlFor="task-target-object"
          label={target.label}
          required
        >
          <UiSelectMenu
            ariaLabel={target.ariaLabel}
            disabled={target.disabled}
            id="task-target-object"
            onChange={setTarget}
            options={target.options}
            surface="dialog"
            value={target.value}
          />
        </UiField>
        {target.error && target.retry ? (
          <TaskResourceFailure onRetry={target.retry} />
        ) : null}
      </div>

      <TaskBasicsAdvanced
        actions={actions}
        data={data}
        deliveryTarget={deliveryTarget}
        deliveryTargetActions={deliveryTargetActions}
        form={form}
        isEditing={isEditing}
        needsSessionRebind={needsSessionRebind}
      />
    </div>
  );
}
