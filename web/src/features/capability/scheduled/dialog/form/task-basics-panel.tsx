// INPUT: 基础任务草稿、资源投影、变更命令与名称输入引用。
// OUTPUT: 以实例级字段身份、具名选择组和共享说明展示任务身份与执行位置。
// POS: Scheduled 创建/编辑执行对象与高级设置纯视图；不加载资源或提交任务。

"use client";

import { type ReactNode, type RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiPanel } from "@/shared/ui/panel";
import { UiInput } from "@/shared/ui/form/form-control";
import { TaskDestinationPicker } from "./task-destination-picker";

import type {
  TargetType,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  TaskBasicsAdvanced,
} from "./task-basics-advanced";
import {
  buildTaskDeliveryTargetPresentation,
  type TaskBasicsActions,
  type TaskBasicsData,
} from "./task-basics-model";

interface TaskBasicsPanelProps {
  children?: ReactNode;
  advancedFields?: ReactNode;
  titleAction?: ReactNode;
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
  isEditing: boolean;
  needsSessionRebind: boolean;
  expandAdvanced?: boolean;
  nameRef: RefObject<HTMLInputElement | null>;
}

export function TaskBasicsPanel({
  actions,
  advancedFields,
  titleAction,
  children,
  data,
  form,
  isEditing,
  needsSessionRebind,
  expandAdvanced,
  nameRef,
}: TaskBasicsPanelProps) {
  const { t } = useI18n();
  const deliveryTarget = buildTaskDeliveryTargetPresentation(form, data, t);
  const deliveryTargetActions: Record<TargetType, (value: string) => void> = {
    agent: actions.setSelectedDeliveryAgentId,
    room: actions.setSelectedDeliveryRoomId,
  };

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="flex min-w-0 items-center gap-3">
      <UiInput
        ref={nameRef}
        aria-label={t("capability.scheduled_dialog_task_name")}
        onChange={(event) => actions.setTaskName(event.target.value)}
        placeholder={t("capability.scheduled_dialog_task_name_placeholder")}
        className="min-w-0 flex-1"
        textRole="title"
        value={form.taskName}
      />
        {titleAction}
      </div>
      {children}
      <UiPanel padding="none" radius="md" className="divide-y divide-(--divider-subtle-color) p-2">
        <TaskDestinationPicker kind="execution" actions={actions} data={data} form={form} />
        <TaskDestinationPicker kind="delivery" actions={actions} data={data} form={form} />
      </UiPanel>

      <TaskBasicsAdvanced
        actions={actions}
        data={data}
        deliveryTarget={deliveryTarget}
        deliveryTargetActions={deliveryTargetActions}
        form={form}
        isEditing={isEditing}
        needsSessionRebind={needsSessionRebind}
        expandAdvanced={expandAdvanced}
      >
        {advancedFields}
      </TaskBasicsAdvanced>
    </div>
  );
}
