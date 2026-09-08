// INPUT: Real task form/schedule hooks and fixed local Agent/Room/Session catalog facts.
// OUTPUT: Editable production form panels, exact local drafts and responsive field/group evidence.
// POS: Development-only fixture; no dialog mutation controller, network request or task submission.

import { useRef } from "react";
import { TaskBasicsPanel } from "@/features/capability/scheduled/dialog/form/task-basics-panel";
import { useTaskForm } from "@/features/capability/scheduled/dialog/form/use-task-form";
import { TaskSchedulePanel, TaskScheduleAdvanced } from "@/features/capability/scheduled/dialog/schedule/task-schedule-panel";
import { createDefaultTaskSchedule } from "@/features/capability/scheduled/dialog/schedule/task-schedule-model";
import { useTaskSchedule } from "@/features/capability/scheduled/dialog/schedule/use-task-schedule";
import type { TaskBasicsData } from "@/features/capability/scheduled/dialog/form/task-basics-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { galleryText } from "./ui-gallery-copy";

const noOp = () => undefined;
const ready = { error: null, loading: false, retry: noOp };
const agentOptions = [{ value: "nova", label: "Nova" }, { value: "pixel", label: "Pixel" }];
const roomOptions = [{ value: "research", label: "Research" }];
const dmSessions = [{ value: "nova/dm", sessionKey: "nova/dm", label: "Research notes" }];
const roomSessions = [{ value: "research/session", sessionKey: "research/session", label: "Research discussion" }];

export function TaskFormGallery() {
  const { locale } = useI18n();
  const nameRef = useRef<HTMLInputElement>(null);
  const dailyPickerAnchorRef = useRef<HTMLButtonElement>(null);
  const singlePickerAnchorRef = useRef<HTMLButtonElement>(null);
  const form = useTaskForm({
    dedicatedSessionKey: "", deliveryTargetType: "agent", enabled: true, executionKind: "agent",
    executionMode: "existing", expiresAt: "", instruction: galleryText(locale, "整理今日进展并列出下一步。", "Summarize today’s progress and list the next steps."),
    permissionMode: "copy", replyMode: "selected", selectedAgentId: "nova", selectedDeliveryAgentId: "nova",
    selectedDeliveryPresenterAgentId: "", selectedDeliveryRoomId: "", selectedReplySessionKey: "nova/dm",
    selectedRoomId: "", selectedSessionKey: "nova/dm", targetType: "agent",
    taskName: galleryText(locale, "每日进展", "Daily progress"),
  }, noOp);
  const schedule = useTaskSchedule({ ...createDefaultTaskSchedule(), kind: "every", everyValue: "2", everyUnit: "hours" }, noOp);
  const data: TaskBasicsData = {
    destinations: [
      {...dmSessions[0], group: "Nova", targetType: "agent", agentId: "nova", roomId: ""},
      {...roomSessions[0], group: "Research", targetType: "room", agentId: "", roomId: "research"},
    ],
    destinationStatus: ready,
  inheritedPermissionMode: "auto",
    agentOptions, agents: ready, rooms: ready, sessions: ready, deliverySessions: ready,
    roomOptions, deliveryRoomOptions: roomOptions,
    sessionOptions: form.draft.targetType === "room" ? roomSessions : dmSessions,
    deliverySessionOptions: form.draft.deliveryTargetType === "room" ? roomSessions : dmSessions,
    defaultDeliveryRoomAgentId: "nova", defaultExecutionRoomAgentId: "nova",
    executionRoomAgentOptions: agentOptions, deliveryRoomAgentOptions: agentOptions,
  };
  return <section className="min-w-0 space-y-4" data-gallery-task-form>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "定时任务表单", "Scheduled task form")}
    </h2>
    <div className="mx-auto flex max-w-3xl min-w-0 flex-col gap-6">
      <div className="min-w-0" data-gallery-task-basics>
        <TaskBasicsPanel actions={form.actions} data={data} form={form.draft} isEditing={false} nameRef={nameRef} needsSessionRebind={false}
          advancedFields={<TaskScheduleAdvanced actions={schedule.actions} form={form.draft} formActions={form.actions} schedule={schedule.draft} />}>
      <div className="min-w-0" data-gallery-task-schedule>
        <TaskSchedulePanel actions={schedule.actions} form={form.draft} formActions={form.actions}
          formError={null} isReconciling={false} isRestoredCreateIntent={false} isMutationReviewed={false}
          mutationFailure={null} onConfirmMutationReviewed={noOp} onReconcile={noOp} onStartNewCreateIntent={noOp}
          refs={{ dailyPickerAnchorRef, singlePickerAnchorRef }} schedule={schedule.draft} view={schedule.view} />
      </div>
        </TaskBasicsPanel>
      </div>
    </div>
    <output className="sr-only" data-gallery-task-draft>{JSON.stringify({ form: form.draft, schedule: schedule.draft })}</output>
  </section>;
}
