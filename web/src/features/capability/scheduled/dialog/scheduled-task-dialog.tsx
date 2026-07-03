/**
 * =====================================================
 * @File   : scheduled-task-dialog.tsx
 * @Date   : 2026-04-16 14:30
 * @Author : leemysw
 * 2026-04-16 14:30   Create
 * =====================================================
 */

"use client";

import { Pencil } from "lucide-react";

import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";

import {
  EVERY_UNIT_OPTIONS,
  EXECUTION_KIND_OPTIONS,
  EXECUTION_MODE_OPTIONS,
  REPLY_MODE_OPTIONS,
  SCHEDULE_OPTIONS,
  TARGET_TYPE_OPTIONS,
  TIMEZONE_OPTIONS,
} from "./scheduled-task-dialog-options";
import { TaskBasicsPanel } from "./task-basics-panel";
import { TaskSchedulePanel } from "./task-schedule-panel";
import { useScheduledTaskDialogState } from "./use-scheduled-task-dialog-state";

interface ScheduledTaskDialogProps {
  agentId: string;
  isOpen: boolean;
  onClose: () => void;
  initialTask?: ScheduledTaskItem | null;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
}

export function ScheduledTaskDialog({
  agentId: agentId,
  isOpen: isOpen,
  initialTask: initialTask = null,
  onClose: onClose,
  onCreated: onCreated,
  onSaved: onSaved,
}: ScheduledTaskDialogProps) {
  const state = useScheduledTaskDialogState({
    agentId,
    initialTask,
    isOpen,
    onClose,
    onCreated,
    onSaved,
  });

  if (!isOpen) return null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        labelledBy="create-task-dialog-title"
        onClose={onClose}
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
      >
        <UiDialogShell className="max-h-[90vh] max-w-[1120px]" size="wide">
          <UiDialogHeader
            onClose={onClose}
            subtitle={
              initialTask
                ? "修改调度、执行会话和结果回传方式。"
                : "先选目标对象，再决定执行会话和结果回传方式。"
            }
            title={initialTask ? "编辑任务" : "新建任务"}
            titleId="create-task-dialog-title"
          />

          <UiDialogBody
            className="grid grid-cols-1 gap-6 md:grid-cols-2 md:items-start"
            scrollable
          >
            <TaskBasicsPanel
              agentOptions={state.agentOptions}
              agentsError={state.agentsError}
              agentsLoading={state.agentsLoading}
              dedicatedSessionKey={state.dedicatedSessionKey}
              executionKind={state.executionKind}
              executionKindOptions={EXECUTION_KIND_OPTIONS}
              executionMode={state.executionMode}
              executionModeOptions={EXECUTION_MODE_OPTIONS}
              nameRef={state.nameRef}
              onResetContextError={() => state.setErrorMessage(null)}
              replyMode={state.replyMode}
              replyModeOptions={REPLY_MODE_OPTIONS}
              disabledReplyModes={state.executionMode === "main" ? ["execution", "selected"] : []}
              roomOptions={state.roomOptions}
              roomsError={state.roomsError}
              roomsLoading={state.roomsLoading}
              selectedAgentId={state.selectedAgentId}
              selectedReplySessionKey={state.selectedReplySessionKey}
              selectedRoomId={state.selectedRoomId}
              selectedSessionKey={state.selectedSessionKey}
              sessionEmptyMessage={
                state.targetType === "agent"
                  ? state.selectedAgentId && !state.agentSessionsLoading && state.sessionOptions.length === 0
                    ? "这个智能体没有可选会话"
                    : null
                  : state.selectedRoomId && !state.roomContextsLoading && state.sessionOptions.length === 0
                    ? "这个 Room 没有可选会话"
                    : null
              }
              sessionError={state.targetType === "agent" ? state.agentSessionsError : state.roomContextsError}
              sessionLoading={state.targetType === "agent" ? state.agentSessionsLoading : state.roomContextsLoading}
              sessionOptions={state.sessionOptions}
              setDedicatedSessionKey={state.setDedicatedSessionKey}
              setExecutionKind={state.setExecutionKind}
              setExecutionMode={state.setExecutionMode}
              setReplyMode={state.setReplyMode}
              setSelectedAgentId={state.setSelectedAgentId}
              setSelectedReplySessionKey={state.setSelectedReplySessionKey}
              setSelectedRoomId={state.setSelectedRoomId}
              setSelectedSessionKey={state.setSelectedSessionKey}
              setTargetType={state.setTargetType}
              setTaskName={state.setTaskName}
              targetType={state.targetType}
              targetTypeOptions={TARGET_TYPE_OPTIONS}
              taskName={state.taskName}
              requireSessionSelection={state.executionKind === "agent" && (state.executionMode === "existing" || state.isRoomExecutorSelectionRequired())}
            />

            <TaskSchedulePanel
              closeDailyPicker={() => state.setIsDailyPickerOpen(false)}
              closeSinglePicker={() => state.setIsSinglePickerOpen(false)}
              dailyAnchorRef={state.dailyPickerAnchorRef}
              dailyDisplay={state.dailyDisplay}
              dailyHour12={state.dailyMeridiemParts.hour12}
              dailyMeridiem={state.dailyMeridiemParts.meridiem}
              dailyMinute={state.dailyMeridiemParts.minute}
              enabled={state.enabled}
              errorMessage={state.errorMessage}
              everyUnit={state.everyUnit}
              everyUnitOptions={EVERY_UNIT_OPTIONS}
              everyValue={state.everyValue}
              instruction={state.instruction}
              instructionLabel={state.executionKind === "script" ? "脚本内容" : "任务指令"}
              instructionPlaceholder={state.executionKind === "script" ? "输入要在目标工作区执行的 shell 脚本" : "输入 Agent 需要执行的指令"}
              isDailyPickerOpen={state.isDailyPickerOpen}
              isSinglePickerOpen={state.isSinglePickerOpen}
              isSingleDateDisabled={state.isSingleDateDisabled}
              isSingleHourDisabled={state.isSingleHourDisabled}
              isSingleMeridiemDisabled={state.isSingleMeridiemDisabled}
              isSingleMinuteDisabled={state.isSingleMinuteDisabled}
              isSingleSecondDisabled={state.isSingleSecondDisabled}
              onDailyHourSelect={(value) => state.updateDailyPicker({ hour12: value })}
              onDailyMeridiemSelect={(value) => state.updateDailyPicker({ meridiem: value })}
              onDailyMinuteSelect={(value) => state.updateDailyPicker({ minute: value })}
              onDailyTriggerClick={() => {
                state.setIsDailyPickerOpen((value) => !value);
                state.setIsSinglePickerOpen(false);
              }}
              onNextMonth={state.goToNextMonth}
              onPrevMonth={state.goToPrevMonth}
              onSingleDateSelect={(value) => state.updateSinglePicker({ date: value })}
              onSingleHourSelect={(value) => state.updateSinglePicker({ hour12: value })}
              onSingleMeridiemSelect={(value) => state.updateSinglePicker({ meridiem: value })}
              onSingleMinuteSelect={(value) => state.updateSinglePicker({ minute: value })}
              onSingleSecondSelect={(value) => state.updateSinglePicker({ second: value })}
              onSingleTriggerClick={() => {
                state.syncSinglePickerToNow();
                state.setIsSinglePickerOpen((value) => !value);
                state.setIsDailyPickerOpen(false);
              }}
              onToggleWeekday={state.toggleWeekday}
              runAtDisplay={state.runAtDisplay}
              scheduleKind={state.scheduleKind}
              scheduleOptions={SCHEDULE_OPTIONS}
              selectedRunDate={state.runAtParts.date}
              selectedWeekdays={state.selectedWeekdays}
              setEnabled={state.setEnabled}
              setEveryUnit={state.setEveryUnit}
              setEveryValue={state.setEveryValue}
              setInstruction={state.setInstruction}
              setScheduleKind={state.setScheduleKind}
              setTimezone={state.setTimezone}
              singleAnchorRef={state.singlePickerAnchorRef}
              singleHour12={state.singleMeridiemParts.hour12}
              singleMeridiem={state.singleMeridiemParts.meridiem}
              singleMinute={state.singleMeridiemParts.minute}
              singlePickerDays={state.singlePickerDays}
              singlePickerMonth={state.singlePickerMonth}
              singleSecond={state.singleMeridiemParts.second}
              timezone={state.timezone}
              timezoneOptions={TIMEZONE_OPTIONS}
            />
          </UiDialogBody>

          <UiDialogFooter>
            <UiButton
              className="min-w-[104px]"
              disabled={state.isSubmitting}
              onClick={onClose}
              type="button"
              variant="surface"
            >
              取消
            </UiButton>
            <UiButton
              className="min-w-[124px]"
              disabled={state.isSubmitting}
              onClick={() => void state.handleSubmit()}
              tone="primary"
              type="button"
              variant="solid"
            >
              {state.isSubmitting ? (initialTask ? "保存中" : "创建中") : (
                <>
                  {initialTask ? <Pencil className="h-3.5 w-3.5" /> : null}
                  {initialTask ? "保存修改" : "创建"}
                </>
              )}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
