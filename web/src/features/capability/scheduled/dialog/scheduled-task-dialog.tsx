"use client";

import { Pencil } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
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
        <UiDialogShell
          className="h-[min(82dvh,760px)] max-w-[960px] max-sm:h-[calc(100dvh-16px)]"
          size="wide"
        >
          <UiDialogHeader
            onClose={onClose}
            subtitle={initialTask
              ? t("capability.scheduled_dialog_edit_subtitle")
              : t("capability.scheduled_dialog_new_subtitle")}
            title={initialTask
              ? t("capability.scheduled_dialog_edit_title")
              : t("capability.scheduled_dialog_new_title")}
            titleId="create-task-dialog-title"
          />

          <UiDialogBody
            className="grid grid-cols-1 gap-6 md:grid-cols-2 md:items-start"
            scrollable
          >
            <TaskBasicsPanel
              actions={controller.form.actions}
              data={controller.data}
              form={controller.form.draft}
              isEditing={initialTask !== null}
              needsSessionRebind={controller.needsSessionRebind}
              nameRef={controller.refs.nameRef}
            />
            <TaskSchedulePanel
              actions={controller.schedule.actions}
              errorMessage={controller.errorMessage}
              form={controller.form.draft}
              formActions={controller.form.actions}
              refs={controller.refs}
              schedule={controller.schedule.draft}
              view={controller.schedule.view}
            />
          </UiDialogBody>

          <UiDialogFooter>
            <UiButton
              className="min-w-[104px]"
              disabled={controller.isSubmitting}
              onClick={onClose}
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              className="min-w-[124px]"
              disabled={controller.isSubmitting}
              onClick={() => void controller.handleSubmit()}
              tone="primary"
              type="button"
              variant="solid"
            >
              {controller.isSubmitting ? submittingLabel : (
                <>
                  {initialTask ? <Pencil className="h-3.5 w-3.5" /> : null}
                  {submitLabel}
                </>
              )}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
