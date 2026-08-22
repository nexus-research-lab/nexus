/**
 * INPUT: 定时任务初值、创建/更新回调与当前 Agent 作用域。
 * OUTPUT: plain 双栏任务表单及原有提交事务。
 * POS: 定时任务创建/编辑模态边界，不在标题区复述表单结构。
 */
"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { TaskBasicsPanel } from "./form/task-basics-panel";
import { TaskSchedulePanel } from "./schedule/task-schedule-panel";
import type { TaskDialogCreatePreset } from "./scheduled-task-dialog-types";
import { useTaskDialogController } from "./use-task-dialog-controller";

interface ScheduledTaskDialogProps {
  agentId: string;
  createPreset?: TaskDialogCreatePreset | null;
  initialTask?: ScheduledTaskItem | null;
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
}

export function ScheduledTaskDialog({
  agentId,
  createPreset = null,
  initialTask = null,
  isOpen,
  onClose,
  onCreated,
  onSaved,
}: ScheduledTaskDialogProps) {
  const { t } = useI18n();
  const controller = useTaskDialogController({
    agentId,
    createPreset,
    initialTask,
    isOpen,
    onClose,
    onCreated,
    onSaved,
  });

  if (!isOpen) {
    return null;
  }

  const isLegacyScriptTask = initialTask?.execution_kind === "script";

  const submitLabel = initialTask
    ? t("capability.scheduled_dialog_save")
    : t("capability.scheduled_dialog_create");
  const submittingLabel = initialTask
    ? t("capability.scheduled_dialog_saving")
    : t("capability.scheduled_dialog_creating");

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999] max-sm:p-2"
        initialFocusRef={controller.refs.nameRef}
        labelledBy="create-task-dialog-title"
        onClose={onClose}
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
      >
        <UiDialogFormShell
          className="h-[min(82dvh,760px)] max-w-[960px] max-sm:h-[calc(100dvh-16px)]"
          onSubmit={(event) => {
            event.preventDefault();
            void controller.handleSubmit();
          }}
          size="wide"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={initialTask
              ? t("capability.scheduled_dialog_edit_title")
              : t("capability.scheduled_dialog_new_title")}
            titleId="create-task-dialog-title"
          />

          <UiDialogBody
            className="grid grid-cols-1 gap-6 md:grid-cols-2 md:items-start"
            scrollable
          >
            {isLegacyScriptTask ? (
              <div className="md:col-span-2">
                <UiStateBlock
                  description={t("capability.scheduled_dialog_legacy_script_description")}
                  size="sm"
                  title={t("capability.scheduled_dialog_legacy_script_title")}
                />
              </div>
            ) : (
              <TaskBasicsPanel
                actions={controller.form.actions}
                data={controller.data}
                form={controller.form.draft}
                isEditing={initialTask !== null}
                needsSessionRebind={controller.needsSessionRebind}
                nameRef={controller.refs.nameRef}
              />
            )}
            {!isLegacyScriptTask ? (
              <TaskSchedulePanel
                actions={controller.schedule.actions}
                errorMessage={controller.errorMessage}
                form={controller.form.draft}
                formActions={controller.form.actions}
                refs={controller.refs}
                schedule={controller.schedule.draft}
                view={controller.schedule.view}
              />
            ) : null}
          </UiDialogBody>

          <UiDialogFooter appearance="plain">
            <UiButton
              className="min-w-[104px]"
              disabled={controller.isSubmitting}
              onClick={onClose}
              type="button"
              variant="surface"
            >
              {t(isLegacyScriptTask ? "common.close" : "common.cancel")}
            </UiButton>
            {!isLegacyScriptTask ? (
              <UiButton
                className="min-w-[124px]"
                disabled={controller.isSubmitting}
                tone="primary"
                type="submit"
                variant="solid"
              >
                {controller.isSubmitting ? submittingLabel : submitLabel}
              </UiButton>
            ) : null}
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
