"use client";

import { type RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type {
  TargetType,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import { TaskBasicsAdvanced } from "./task-basics-advanced";
import {
  buildTaskTargetPresentation,
  type TaskBasicsActions,
  type TaskBasicsData,
} from "./task-basics-model";

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

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <UiField
        htmlFor="task-name"
        label={t("capability.scheduled_dialog_task_name")}
      >
        <UiInput
          ref={nameRef}
          id="task-name"
          onChange={(event) => actions.setTaskName(event.target.value)}
          placeholder={t("capability.scheduled_dialog_task_name_placeholder")}
          value={form.taskName}
        />
      </UiField>

      <UiField
        error={target.error}
        htmlFor="task-target-object"
        label={target.label}
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

      <TaskBasicsAdvanced
        actions={actions}
        data={data}
        form={form}
        isEditing={isEditing}
        needsSessionRebind={needsSessionRebind}
      />
    </div>
  );
}
