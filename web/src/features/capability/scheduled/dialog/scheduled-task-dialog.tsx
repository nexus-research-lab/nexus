"use client";

import { Pencil } from "lucide-react";

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
import { useTaskDialogController } from "./use-task-dialog-controller";

interface ScheduledTaskDialogProps {
  agentId: string;
  initialTask?: ScheduledTaskItem | null;
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
}

export function ScheduledTaskDialog({
  agentId,
  initialTask = null,
  isOpen,
  onClose,
  onCreated,
  onSaved,
}: ScheduledTaskDialogProps) {
  const controller = useTaskDialogController({
    agentId,
    initialTask,
    isOpen,
    onClose,
    onCreated,
    onSaved,
  });

  if (!isOpen) {
    return null;
  }

  const submitLabel = initialTask ? "保存修改" : "创建";
  const submittingLabel = initialTask ? "保存中" : "创建中";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        initialFocusRef={controller.refs.nameRef}
        labelledBy="create-task-dialog-title"
        onClose={onClose}
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
      >
        <UiDialogShell className="max-h-[90vh] max-w-[960px]" size="wide">
          <UiDialogHeader
            onClose={onClose}
            subtitle={initialTask
              ? "修改任务内容或执行时间；不常用的选项收在高级设置里。"
              : "填写任务、目标和执行时间即可创建。"}
            title={initialTask ? "编辑任务" : "新建任务"}
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
              取消
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
