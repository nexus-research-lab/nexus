/**
 * INPUT: 定时任务初值、创建/更新回调与当前 Agent 作用域。
 * OUTPUT: 页内右侧编辑表单、显式提交 busy 与原有提交事务。
 * POS: 定时任务创建/编辑工作面，不在标题区复述表单结构。
 */
"use client";

import { useEffect } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ResourceFailure } from "@/lib/error-message";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiButton } from "@/shared/ui/button/button";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import {
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogFooter,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import type {
  ScheduledTaskCreateRequestStatus,
  ScheduledTaskItem,
} from "@/types/capability/scheduled-task/task";

import { buildTaskConfirmationSummary } from "./form/task-basics-model";
import { TaskBasicsPanel } from "./form/task-basics-panel";
import { TaskSchedulePanel, TaskScheduleAdvanced } from "./schedule/task-schedule-panel";
import type { TaskDialogCreatePreset } from "./scheduled-task-dialog-types";
import { useTaskDialogController } from "./use-task-dialog-controller";

interface ScheduledTaskDialogProps {
  agentId: string;
  createPreset?: TaskDialogCreatePreset | null;
  initialTask?: ScheduledTaskItem | null;
  isOpen: boolean;
  onBusyChange?: (busy: boolean) => void;
  onAccessFailure?: (failure: ResourceFailure) => void;
  onClose: () => void;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onCreateIntentResolved?: (status?: ScheduledTaskCreateRequestStatus) => void;
  onConfirmMutationReviewed?: (command: "update", targetId: string) => void;
  onIsMutationBlocked?: (jobId: string) => boolean;
  onReconcile?: () => Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
  scopeKey: string | null;
}

export function ScheduledTaskDialog({
  agentId,
  createPreset = null,
  initialTask = null,
  isOpen,
  onBusyChange,
  onAccessFailure,
  onClose,
  onCreated,
  onCreateIntentResolved,
  onConfirmMutationReviewed,
  onIsMutationBlocked,
  onReconcile,
  onSaved,
  scopeKey,
}: ScheduledTaskDialogProps) {
  const { t } = useI18n();
  const controller = useTaskDialogController({
    agentId,
    createPreset,
    initialTask,
    isOpen,
    onAccessFailure,
    onClose,
    onCreated,
    onCreateIntentResolved,
    onConfirmMutationReviewed,
    onIsMutationBlocked,
    onReconcile,
    onSaved,
    scopeKey,
  });

  useEffect(() => {
    onBusyChange?.(isOpen && controller.isCloseBlocked);
    return () => onBusyChange?.(false);
  }, [isOpen, controller.isCloseBlocked, onBusyChange]);

  const nameRef = controller.refs.nameRef;
  useEffect(() => {
    if (isOpen) nameRef.current?.focus();
  }, [isOpen, nameRef]);

  if (!isOpen) {
    return null;
  }

  const isLegacyScriptTask = initialTask?.execution_kind === "script";
  const canClose = !controller.isCloseBlocked;

  const submitLabel = initialTask
    ? t("capability.scheduled_dialog_save")
    : t("capability.scheduled_dialog_create");
  const submittingLabel = initialTask
    ? t("capability.scheduled_dialog_saving")
    : t("capability.scheduled_dialog_creating");

  return (
    <form
      aria-label={initialTask ? t("capability.scheduled_dialog_edit_title") : t("capability.scheduled_dialog_new_title")}
      className="flex h-full min-h-0 min-w-0 flex-col"
      onSubmit={(event) => {
        event.preventDefault();
        void controller.handleSubmit();
      }}
    >
          {isLegacyScriptTask ? <UiDialogHeader
            appearance="plain"
            onClose={canClose ? onClose : undefined}
            title={t("capability.scheduled_dialog_edit_title")}
          /> : null}

          <UiDialogBody
            className="flex flex-col gap-6"
            scrollable
          >
            {isLegacyScriptTask ? (
              <div className="min-w-0">
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
                needsSessionRebind={controller.needsSessionRebind}
                expandAdvanced={Boolean(controller.formError)}
                nameRef={controller.refs.nameRef}
                titleAction={<UiDialogCloseButton disabled={!canClose} onClose={onClose} />}
                advancedFields={<TaskScheduleAdvanced actions={controller.schedule.actions} form={controller.form.draft} formActions={controller.form.actions} schedule={controller.schedule.draft} />}
              >
                <TaskSchedulePanel
                  actions={controller.schedule.actions}
                  formError={controller.formError}
                  form={controller.form.draft}
                  formActions={controller.form.actions}
                  isReconciling={controller.isReconciling}
                  isRestoredCreateIntent={controller.isRestoredCreateIntent}
                  isMutationReviewed={controller.isMutationReviewed}
                  mutationFailure={controller.mutationFailure}
                  onConfirmMutationReviewed={controller.confirmReviewedMutation}
                  onReconcile={() => void controller.reconcileMutation()}
                  onStartNewCreateIntent={controller.startNewCreateIntent}
                  refs={controller.refs}
                  schedule={controller.schedule.draft}
                  view={controller.schedule.view}
                />
              </TaskBasicsPanel>
            )}
          </UiDialogBody>

          {!isLegacyScriptTask ? (
            <p aria-live="polite" className={cn("shrink-0 px-6 pt-3 break-words", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
              {buildTaskConfirmationSummary(controller.form.draft, controller.schedule.draft, controller.data, t)}
            </p>
          ) : null}
          <UiDialogFooter appearance="plain">
            <UiButton
              className="min-w-[104px]"
              disabled={!canClose}
              onClick={onClose}
              type="button"
              variant="surface"
            >
              {t(isLegacyScriptTask ? "common.close" : "common.cancel")}
            </UiButton>
            {!isLegacyScriptTask ? (
              <UiButton
                aria-busy={controller.isSubmitting || undefined}
                className="min-w-[124px]"
                disabled={controller.isCloseBlocked}
                tone="primary"
                type="submit"
                variant="solid"
              >
                {controller.isSubmitting ? submittingLabel : submitLabel}
              </UiButton>
            ) : null}
          </UiDialogFooter>
    </form>
  );
}
