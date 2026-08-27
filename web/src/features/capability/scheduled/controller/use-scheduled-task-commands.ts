// INPUT: owner-scoped Scheduled 资源、exact mutation journal 与 FailureCore 结果。
// OUTPUT: 运行/配置/权限/删除命令、人工停止确认、权威对账与三段式反馈。
// POS: Scheduled 写命令控制器；不以 HTTP trace ID 推测业务结果，不重放未确认副作用。

"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

import {
  confirmScheduledTaskDeletionStoppedApi,
  decideAutomationPermissionRequestApi,
  deleteScheduledTaskApi,
  recoverScheduledTaskRunApi,
  resumeAutomationPermissionRunApi,
  retryScheduledTaskRunDeliveryApi,
  runScheduledTaskApi,
  updateScheduledTaskStatusApi,
} from "@/lib/api/capability/scheduled-task-api";
import { getErrorMessage, type ResourceFailure } from "@/lib/error-message";
import { generateUuid } from "@/lib/uuid";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  AutomationPermissionDecision,
  AutomationPermissionDecisionResult,
  AutomationPermissionRequest,
} from "@/types/capability/scheduled-task/permission";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type {
  ScheduledTaskRunItem,
  ScheduledTaskRunNowResponse,
} from "@/types/capability/scheduled-task/run";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import {
  SCHEDULED_TASK_COMMAND_KINDS,
  allowScheduledTaskRepeat,
  hasScheduledTaskCommandForJob,
  isScheduledTaskMutationBlocked,
  reconcileScheduledTaskUnconfirmed,
  scheduledTaskCommandKey,
  scheduledTaskConfigurationCommandTarget,
  scheduledTaskDeliveryCommandTarget,
  scheduledTaskCommandTarget,
  scheduledTaskCommandTargetsJob,
  scheduledTaskPermissionCommandTarget,
  scheduledTaskRunCommandTarget,
  type ScheduledTaskCommandKind,
  type ScheduledTaskFeedback,
  type ScheduledTaskReconcileExpectation,
  type ScheduledTaskUnconfirmedCommands,
} from "./scheduled-task-directory-model";
import {
  createPendingCommandState,
  setPendingCommand,
} from "./pending-command-model";
import {
  loadScheduledTaskMutationJournal,
  removeScheduledTaskMutationJournalEntry,
  ScheduledTaskMutationCoordinationUnavailableError,
  ScheduledTaskMutationLockUnavailableError,
  subscribeScheduledTaskMutationJournal,
  upsertScheduledTaskMutationJournalEntry,
  withScheduledTaskMutationGate,
  type ScheduledTaskMutationJournalEntry,
} from "./scheduled-task-mutation-journal";
import {
  projectScheduledTaskMutationFailure,
  type ScheduledTaskMutationFailureProjection,
} from "./scheduled-task-mutation-outcome";
import type { ScheduledTasksPrimarySnapshot } from "./use-scheduled-tasks-resource";

interface ScheduledTaskCommandResource {
  invalidateAccess: (failure: ResourceFailure) => void;
  isAccessInvalidated: () => boolean;
  refresh: (options?: {
    includePermissions?: boolean;
    silent?: boolean;
  }) => Promise<ScheduledTasksPrimarySnapshot>;
  refreshPermissionRequests: () => Promise<AutomationPermissionRequest[]>;
  removeTask: (jobId: string, expectedScopeKey: string | null) => void;
  upsertTask: (
    task: ScheduledTaskItem,
    expectedScopeKey: string | null,
  ) => void;
}

interface MutationFeedback {
  message: string;
  nextStep: string;
  title: string;
}

interface CommandFailureFeedback {
  actionLabel?: string;
  expectation?: ScheduledTaskReconcileExpectation;
  fallbackMessage: string;
  nextStep?: string;
  onReconcile?: () => void;
  reconcileWhenNotApplied?: boolean;
  title: string;
}

interface RunPendingOptions {
  ignoreUnconfirmedCommands?: ReadonlySet<ScheduledTaskCommandKind>;
}

const ORIGINAL_DELETE_REVIEW_LOCK = new Set<ScheduledTaskCommandKind>(["delete"]);

class ScheduledTaskCommandScopeSupersededError extends Error {
  constructor() {
    super("任务页面作用域已变化，已忽略旧响应");
    this.name = "ScheduledTaskCommandScopeSupersededError";
  }
}

type Translate = ReturnType<typeof useI18n>["t"];

function buildMutationFailureFeedback(
  projection: ScheduledTaskMutationFailureProjection,
  failure: CommandFailureFeedback,
  onReconcile: () => void,
  t: Translate,
): ScheduledTaskFeedback {
  if (projection.effect === "not_applied") {
    return {
      action: failure.reconcileWhenNotApplied
        ? {
            label: failure.actionLabel ?? t("capability.scheduled_reconcile_action"),
            onClick: failure.onReconcile ?? onReconcile,
          }
        : undefined,
      impact: t("capability.scheduled_mutation_not_applied_impact"),
      message: projection.message,
      nextStep: t("capability.scheduled_mutation_not_applied_next_step"),
      title: failure.title,
      tone: "error",
    };
  }

  const copy = projection.effect === "accepted"
    ? {
        impact: t("capability.scheduled_mutation_accepted_impact"),
        title: t("capability.scheduled_mutation_accepted_title"),
      }
    : projection.effect === "committed"
      ? {
          impact: t("capability.scheduled_mutation_committed_impact"),
          title: t("capability.scheduled_mutation_committed_title"),
        }
      : {
          impact: t("capability.scheduled_mutation_unknown_impact"),
          title: t("capability.scheduled_mutation_unknown_title"),
        };
  return {
    action: {
      label: failure.actionLabel ?? t("capability.scheduled_reconcile_action"),
      onClick: onReconcile,
    },
    impact: copy.impact,
    message: projection.message,
    nextStep: failure.nextStep ?? t("capability.scheduled_mutation_unknown_next_step"),
    title: copy.title,
    tone: "warning",
  };
}

interface ManualReviewTarget {
  command: "permission" | "retryDelivery" | "run";
  targetId: string;
}

interface ManualMutationReviewTarget {
  command:
    | "confirmDeletionStopped"
    | "delete"
    | "permission"
    | "toggle"
    | "update";
  targetId: string;
}

function findManualReviewTarget(
  unconfirmed: ScheduledTaskUnconfirmedCommands,
  expectations: ReadonlyMap<string, ScheduledTaskReconcileExpectation>,
  jobId?: string,
): ManualReviewTarget | null {
  for (const command of ["run", "retryDelivery"] as const) {
    const targetId = [...(unconfirmed.get(command) ?? [])].find((candidate) => (
      !jobId || candidate === jobId || candidate.startsWith(`${jobId}:`)
    ));
    if (targetId) {
      return { command, targetId };
    }
  }
  const permissionTargetId = [...(unconfirmed.get("permission") ?? [])].find(
    (candidate) => {
      if (jobId && candidate !== jobId && !candidate.startsWith(`${jobId}:`)) {
        return false;
      }
      return expectations.get(
        scheduledTaskCommandKey("permission", candidate),
      )?.kind === "permission_resume";
    },
  );
  return permissionTargetId
    ? { command: "permission", targetId: permissionTargetId }
    : null;
}

function findManualMutationReviewTarget(
  unconfirmed: ScheduledTaskUnconfirmedCommands,
  expectations: ReadonlyMap<string, ScheduledTaskReconcileExpectation>,
  commands: ReadonlySet<ManualMutationReviewTarget["command"]>,
  itemsById?: ReadonlyMap<string, ScheduledTaskItem>,
): ManualMutationReviewTarget | null {
  for (const command of [
    "confirmDeletionStopped",
    "delete",
    "toggle",
    "update",
    "permission",
  ] as const) {
    if (!commands.has(command)) {
      continue;
    }
    const targetId = [...(unconfirmed.get(command) ?? [])].find((candidate) => {
      const expectation = expectations.get(scheduledTaskCommandKey(command, candidate));
      if (command === "permission") {
        return expectation?.kind === "permission_decision";
      }
      if (command === "confirmDeletionStopped") {
        return expectation?.kind === "confirm_deletion_stopped";
      }
      if (
        command === "delete"
        && expectation?.kind === "delete"
        && Boolean(itemsById?.get(expectation.jobId)?.deletion_state?.trim())
      ) {
        // durable claim 已证明删除被受理；收尾期间不能提供“解除保护”。
        return false;
      }
      return expectation?.kind === command;
    });
    if (targetId) {
      return { command, targetId };
    }
  }
  return null;
}

function findDeletingTaskForUnconfirmedDelete(
  unconfirmed: ScheduledTaskUnconfirmedCommands,
  expectations: ReadonlyMap<string, ScheduledTaskReconcileExpectation>,
  itemsById: ReadonlyMap<string, ScheduledTaskItem>,
): ScheduledTaskItem | null {
  for (const targetId of unconfirmed.get("delete") ?? []) {
    const expectation = expectations.get(
      scheduledTaskCommandKey("delete", targetId),
    );
    if (expectation?.kind !== "delete") {
      continue;
    }
    const task = itemsById.get(expectation.jobId);
    if (task?.deletion_state?.trim()) {
      return task;
    }
  }
  return null;
}

function hasUnconfirmedCommands(
  unconfirmed: ScheduledTaskUnconfirmedCommands,
): boolean {
  return SCHEDULED_TASK_COMMAND_KINDS.some((command) => (
    (unconfirmed.get(command)?.size ?? 0) > 0
  ));
}

function restoreUnconfirmedCommands(
  entries: ScheduledTaskMutationJournalEntry[],
): ScheduledTaskUnconfirmedCommands {
  return entries.reduce<ScheduledTaskUnconfirmedCommands>(
    (current, entry) => setPendingCommand(
      current,
      entry.command,
      entry.targetId,
      true,
    ),
    createPendingCommandState(SCHEDULED_TASK_COMMAND_KINDS),
  );
}

function taskFromPermissionResult(
  result: AutomationPermissionDecisionResult,
): ScheduledTaskItem {
  const request = result.request;
  return {
    ...result.task,
    pending_permission_request: result.task.pending_permission_request_id &&
      request?.request_id === result.task.pending_permission_request_id
      ? request
      : null,
  };
}

function permissionDecisionMessage(decision: AutomationPermissionDecision): string {
  switch (decision) {
    case "allow_once":
      return "已仅授权本次运行";
    case "allow_task":
      return "已授权该任务后续使用此能力";
    case "retry":
      return "已重新检查连接并继续运行";
    default:
      return "已拒绝本次权限请求";
  }
}

export function useScheduledTaskCommands(
  resource: ScheduledTaskCommandResource,
  scopeKey: string | null,
) {
  const { t } = useI18n();
  const {
    refresh,
    refreshPermissionRequests,
    invalidateAccess,
    isAccessInvalidated,
    removeTask,
    upsertTask,
  } = resource;
  const [feedback, setFeedback] = useState<ScheduledTaskFeedback | null>(null);
  const [pending, setPending] = useState(() => (
    createPendingCommandState(SCHEDULED_TASK_COMMAND_KINDS)
  ));
  const initialJournalEntriesRef = useRef<ScheduledTaskMutationJournalEntry[] | null>(
    null,
  );
  if (initialJournalEntriesRef.current === null) {
    initialJournalEntriesRef.current = loadScheduledTaskMutationJournal(scopeKey);
  }
  const [unconfirmed, setUnconfirmed] = useState<ScheduledTaskUnconfirmedCommands>(() => (
    restoreUnconfirmedCommands(initialJournalEntriesRef.current ?? [])
  ));
  const pendingPromisesRef = useRef(new Map<string, Promise<unknown>>());
  const pendingRef = useRef(pending);
  const accessBlockedRef = useRef(false);
  const activeScopeKeyRef = useRef(scopeKey);
  const reconcileExpectationsRef = useRef(new Map(
    (initialJournalEntriesRef.current ?? []).flatMap((entry) => (
      entry.expectation
        ? [[
            scheduledTaskCommandKey(entry.command, entry.targetId),
            entry.expectation,
          ] as const]
        : []
    )),
  ));
  const unconfirmedRef = useRef(unconfirmed);

  useLayoutEffect(() => {
    activeScopeKeyRef.current = scopeKey;
    const entries = loadScheduledTaskMutationJournal(scopeKey);
    const restored = restoreUnconfirmedCommands(entries);
    pendingRef.current = createPendingCommandState(SCHEDULED_TASK_COMMAND_KINDS);
    unconfirmedRef.current = restored;
    reconcileExpectationsRef.current = new Map(entries.flatMap((entry) => (
      entry.expectation
        ? [[
            scheduledTaskCommandKey(entry.command, entry.targetId),
            entry.expectation,
          ] as const]
        : []
    )));
    setPending(pendingRef.current);
    setUnconfirmed(restored);
    setFeedback(entries.length > 0
      ? {
          impact: t("capability.scheduled_restored_unknown_impact"),
          message: t("capability.scheduled_restored_unknown_message"),
          nextStep: t("capability.scheduled_restored_unknown_next_step"),
          title: t("capability.scheduled_restored_unknown_title"),
          tone: "warning",
        }
      : null);
  }, [scopeKey, t]);

  useEffect(() => subscribeScheduledTaskMutationJournal(scopeKey, () => {
    if (activeScopeKeyRef.current !== scopeKey) {
      return;
    }
    const entries = loadScheduledTaskMutationJournal(scopeKey);
    const restored = restoreUnconfirmedCommands(entries);
    unconfirmedRef.current = restored;
    reconcileExpectationsRef.current = new Map(entries.flatMap((entry) => (
      entry.expectation
        ? [[
            scheduledTaskCommandKey(entry.command, entry.targetId),
            entry.expectation,
          ] as const]
        : []
    )));
    setUnconfirmed(restored);
    const restoredTitle = t("capability.scheduled_restored_unknown_title");
    setFeedback((current) => entries.length > 0
      ? {
        impact: t("capability.scheduled_restored_unknown_impact"),
        message: t("capability.scheduled_restored_unknown_message"),
        nextStep: t("capability.scheduled_restored_unknown_next_step"),
        title: restoredTitle,
        tone: "warning",
      }
      : current?.title === restoredTitle
        ? null
        : current);
  }), [scopeKey, t]);

  const updateUnconfirmed = useCallback((
    command: ScheduledTaskCommandKind,
    targetId: string,
    blocked: boolean,
  ): void => {
    if (
      accessBlockedRef.current
      || activeScopeKeyRef.current !== scopeKey
    ) {
      return;
    }
    const next = setPendingCommand(
      unconfirmedRef.current,
      command,
      targetId,
      blocked,
    );
    unconfirmedRef.current = next;
    setUnconfirmed(next);
  }, [scopeKey]);

  const applyReconcileEvidence = useCallback((snapshot: {
    commands?: ReadonlySet<
      | "confirmDeletionStopped"
      | "delete"
      | "permission"
      | "recover"
      | "run"
      | "toggle"
      | "update"
    >;
    items: ScheduledTaskItem[] | null;
    permissionRequests: AutomationPermissionRequest[] | null;
    runs: ScheduledTaskRunItem[] | null;
  }): ScheduledTaskUnconfirmedCommands => {
    if (
      accessBlockedRef.current
      || activeScopeKeyRef.current !== scopeKey
    ) {
      return unconfirmedRef.current;
    }
    const previous = unconfirmedRef.current;
    const reconciled = reconcileScheduledTaskUnconfirmed(
      previous,
      {
        expectations: reconcileExpectationsRef.current,
        ...snapshot,
      },
    );
    for (const [key] of reconcileExpectationsRef.current) {
      const stillLocked = SCHEDULED_TASK_COMMAND_KINDS.some((command) => (
        [...(reconciled.get(command) ?? [])].some((targetId) => (
          scheduledTaskCommandKey(command, targetId) === key
        ))
      ));
      if (!stillLocked) {
        reconcileExpectationsRef.current.delete(key);
      }
    }
    for (const command of SCHEDULED_TASK_COMMAND_KINDS) {
      for (const targetId of previous.get(command) ?? []) {
        if (!reconciled.get(command)?.has(targetId)) {
          removeScheduledTaskMutationJournalEntry(scopeKey, command, targetId);
        }
      }
    }
    unconfirmedRef.current = reconciled;
    setUnconfirmed(reconciled);
    return reconciled;
  }, [scopeKey]);

  const confirmReviewedMutation = useCallback((
    command: ManualMutationReviewTarget["command"],
    targetId: string,
  ): void => {
    if (
      accessBlockedRef.current
      || activeScopeKeyRef.current !== scopeKey
    ) {
      return;
    }
    const reconciled = allowScheduledTaskRepeat(
      unconfirmedRef.current,
      command,
      targetId,
    );
    reconcileExpectationsRef.current.delete(
      scheduledTaskCommandKey(command, targetId),
    );
    removeScheduledTaskMutationJournalEntry(scopeKey, command, targetId);
    unconfirmedRef.current = reconciled;
    setUnconfirmed(reconciled);
    setFeedback({
      impact: t("capability.scheduled_mutation_review_unlocked_impact"),
      message: t("capability.scheduled_mutation_review_unlocked_message"),
      nextStep: t("capability.scheduled_mutation_review_unlocked_next_step"),
      title: t("capability.scheduled_mutation_review_unlocked_title"),
      tone: "warning",
    });
  }, [scopeKey, t]);

  const showManualMutationConfirmation = useCallback((
    target: ManualMutationReviewTarget,
  ): void => {
    setFeedback({
      action: {
        label: t("capability.scheduled_mutation_review_unlock_action"),
        onClick: () => confirmReviewedMutation(target.command, target.targetId),
      },
      impact: t("capability.scheduled_mutation_review_impact"),
      message: t("capability.scheduled_mutation_review_message"),
      nextStep: t("capability.scheduled_mutation_review_next_step"),
      title: t("capability.scheduled_mutation_review_title"),
      tone: "warning",
    });
  }, [confirmReviewedMutation, t]);

  const reconcile = useCallback(async (): Promise<void> => {
    // 权限读与主任务读分别落状态；辅助请求不会延迟或否定主列表对账。
    void refreshPermissionRequests().then((permissionRequests) => {
      const reconciled = applyReconcileEvidence({
        commands: new Set(["permission"]),
        items: null,
        permissionRequests,
        runs: null,
      });
      const manualMutationReview = findManualMutationReviewTarget(
        reconciled,
        reconcileExpectationsRef.current,
        new Set(["permission"]),
      );
      if (manualMutationReview) {
        showManualMutationConfirmation(manualMutationReview);
      } else if (!hasUnconfirmedCommands(reconciled)) {
        setFeedback(null);
      }
    }).catch(() => undefined);
    const snapshot = await refresh({ includePermissions: false });
    const itemsById = new Map(snapshot.items.map((item) => [item.job_id, item]));
    const reconciled = applyReconcileEvidence({
      commands: new Set([
        "confirmDeletionStopped",
        "delete",
        "recover",
        "toggle",
        "update",
      ]),
      items: snapshot.items,
      permissionRequests: null,
      runs: null,
    });
    const manualReview = findManualReviewTarget(
      reconciled,
      reconcileExpectationsRef.current,
    );
    const manualMutationReview = findManualMutationReviewTarget(
      reconciled,
      reconcileExpectationsRef.current,
      new Set(["confirmDeletionStopped", "delete", "toggle", "update"]),
      itemsById,
    );
    const deletingTask = findDeletingTaskForUnconfirmedDelete(
      reconciled,
      reconcileExpectationsRef.current,
      itemsById,
    );
    if (manualMutationReview?.command === "confirmDeletionStopped") {
      showManualMutationConfirmation(manualMutationReview);
    } else if (deletingTask) {
      const deletionNeedsReview = deletingTask.deletion_state?.trim() === "review_required";
      setFeedback({
        impact: t(deletionNeedsReview
          ? "capability.scheduled_delete_review_required_impact"
          : "capability.scheduled_delete_finishing_impact"),
        message: t(deletionNeedsReview
          ? "capability.scheduled_delete_review_required_message"
          : "capability.scheduled_delete_finishing_message", {
          name: deletingTask.name,
        }),
        nextStep: t(deletionNeedsReview
          ? "capability.scheduled_delete_review_required_next_step"
          : "capability.scheduled_delete_finishing_next_step"),
        title: t(deletionNeedsReview
          ? "capability.scheduled_delete_review_required_title"
          : "capability.scheduled_delete_finishing_title"),
        tone: "warning",
      });
    } else if (manualReview) {
      setFeedback({
        impact: t("capability.scheduled_reconcile_run_unproven_impact"),
        message: t("capability.scheduled_reconcile_run_unproven_message"),
        nextStep: t("capability.scheduled_reconcile_run_unproven_next_step"),
        title: t("capability.scheduled_reconcile_incomplete_title"),
        tone: "warning",
      });
    } else if (manualMutationReview) {
      showManualMutationConfirmation(manualMutationReview);
    } else if (hasUnconfirmedCommands(reconciled)) {
      setFeedback({
        impact: t("capability.scheduled_reconcile_incomplete_impact"),
        message: t("capability.scheduled_reconcile_incomplete_message"),
        nextStep: t("capability.scheduled_reconcile_incomplete_next_step"),
        title: t("capability.scheduled_reconcile_incomplete_title"),
        tone: "warning",
      });
    } else {
      setFeedback(null);
    }
  }, [
    applyReconcileEvidence,
    refresh,
    refreshPermissionRequests,
    showManualMutationConfirmation,
    t,
  ]);

  const confirmRunHistoryReconciled = useCallback((
    task: ScheduledTaskItem,
    runs: ScheduledTaskRunItem[],
  ): void => {
    if (
      accessBlockedRef.current
      || activeScopeKeyRef.current !== scopeKey
    ) {
      return;
    }
    const jobId = task.job_id;
    const reconciledFromHistory = applyReconcileEvidence({
      commands: new Set(["recover", "run"]),
      items: [task],
      permissionRequests: null,
      runs,
    });
    const showConfirmation = (target: ManualReviewTarget): void => {
      const isDelivery = target.command === "retryDelivery";
      const isPermissionResume = target.command === "permission";
      const runExpectation = target.command === "run"
        ? reconcileExpectationsRef.current.get(
            scheduledTaskCommandKey(target.command, target.targetId),
          )
        : null;
      setFeedback({
        action: {
          label: t(isDelivery
            ? "capability.scheduled_delivery_allow_repeat_action"
            : isPermissionResume
              ? "capability.scheduled_permission_resume_allow_repeat_action"
              : "capability.scheduled_run_allow_repeat_action"),
          onClick: () => {
            if (runExpectation?.kind === "run") {
              setFeedback({
                impact: "这次核对会沿用原来的启动请求，不会创建第二个运行意图。",
                message: "正在确认这次启动是否已经进入运行记录…",
                nextStep: "请稍候，Nexus 会返回同一次启动的权威结果。",
                title: "正在核对启动结果",
                tone: "warning",
              });
              void runScheduledTaskApi(
                runExpectation.jobId,
                runExpectation.baseConfigurationVersion,
                runExpectation.requestId,
              ).then(async (result) => {
                if (
                  accessBlockedRef.current
                  || isAccessInvalidated()
                  || activeScopeKeyRef.current !== scopeKey
                ) {
                  return;
                }
                const reconciled = allowScheduledTaskRepeat(
                  unconfirmedRef.current,
                  target.command,
                  target.targetId,
                );
                reconcileExpectationsRef.current.delete(
                  scheduledTaskCommandKey(target.command, target.targetId),
                );
                removeScheduledTaskMutationJournalEntry(
                  scopeKey,
                  target.command,
                  target.targetId,
                );
                unconfirmedRef.current = reconciled;
                setUnconfirmed(reconciled);
                notifyCapabilitySummaryMutated({
                  agent_id: task.agent_id,
                  source: "scheduled_tasks",
                });
                try {
                  await refresh({ silent: true });
                } catch {
                  // exact run receipt 已足以解除重复保护；列表刷新失败只影响
                  // 当前画面的新鲜度，后续 realtime/手动刷新仍会补齐。
                }
                if (
                  accessBlockedRef.current
                  || isAccessInvalidated()
                  || activeScopeKeyRef.current !== scopeKey
                ) {
                  return;
                }
                setFeedback({
                  impact: t("capability.scheduled_mutation_confirmed_impact"),
                  message: result.status === "queued_to_main_session"
                    ? `${task.name} 已排入主会话执行`
                    : `${task.name} 的这次启动已确认`,
                  nextStep: t("capability.scheduled_run_next_step"),
                  title: "启动结果已确认",
                  tone: "success",
                });
              }).catch((error: unknown) => {
                if (activeScopeKeyRef.current !== scopeKey) {
                  return;
                }
                const projection = projectScheduledTaskMutationFailure(
                  error,
                  "核对启动结果失败",
                );
                // 这里处理的是“原请求结果未知”之后的再次核对。重新鉴权前
                // 被 401/403 拒绝，只能证明本次核对没有进入业务层，不能证明
                // 原启动未受理，因此必须保留原 request identity 和动作锁。
                if (projection.access) {
                  accessBlockedRef.current = true;
                  invalidateAccess({
                    access: projection.access,
                    message: projection.message,
                  });
                  return;
                }
                if (!projection.blocksRepeat) {
                  const reconciled = allowScheduledTaskRepeat(
                    unconfirmedRef.current,
                    target.command,
                    target.targetId,
                  );
                  reconcileExpectationsRef.current.delete(
                    scheduledTaskCommandKey(target.command, target.targetId),
                  );
                  removeScheduledTaskMutationJournalEntry(
                    scopeKey,
                    target.command,
                    target.targetId,
                  );
                  unconfirmedRef.current = reconciled;
                  setUnconfirmed(reconciled);
                }
                setFeedback(buildMutationFailureFeedback(
                  projection,
                  {
                    actionLabel: t("capability.scheduled_review_history_action"),
                    fallbackMessage: "核对启动结果失败",
                    onReconcile: () => showConfirmation(target),
                    title: "启动结果仍未确认",
                  },
                  () => showConfirmation(target),
                  t,
                ));
              });
              return;
            }
            const reconciled = allowScheduledTaskRepeat(
              unconfirmedRef.current,
              target.command,
              target.targetId,
            );
            reconcileExpectationsRef.current.delete(
              scheduledTaskCommandKey(target.command, target.targetId),
            );
            removeScheduledTaskMutationJournalEntry(
              scopeKey,
              target.command,
              target.targetId,
            );
            unconfirmedRef.current = reconciled;
            setUnconfirmed(reconciled);
            const remaining = findManualReviewTarget(
              reconciled,
              reconcileExpectationsRef.current,
              jobId,
            );
            if (remaining) {
              showConfirmation(remaining);
              return;
            }
            setFeedback({
              impact: t("capability.scheduled_repeat_unlocked_impact"),
              message: t("capability.scheduled_repeat_unlocked_message"),
              nextStep: t("capability.scheduled_repeat_unlocked_next_step"),
              title: t("capability.scheduled_repeat_unlocked_title"),
              tone: "warning",
            });
          },
        },
        impact: t(isDelivery
          ? "capability.scheduled_delivery_manual_review_impact"
          : isPermissionResume
            ? "capability.scheduled_permission_resume_manual_review_impact"
            : "capability.scheduled_run_manual_review_impact"),
        message: t(isDelivery
          ? "capability.scheduled_delivery_manual_review_message"
          : isPermissionResume
            ? "capability.scheduled_permission_resume_manual_review_message"
            : "capability.scheduled_run_manual_review_message"),
        nextStep: t(isDelivery
          ? "capability.scheduled_delivery_manual_review_next_step"
          : isPermissionResume
            ? "capability.scheduled_permission_resume_manual_review_next_step"
            : "capability.scheduled_run_manual_review_next_step"),
        title: t(isDelivery
          ? "capability.scheduled_delivery_manual_review_title"
          : isPermissionResume
            ? "capability.scheduled_permission_resume_manual_review_title"
            : "capability.scheduled_run_manual_review_title"),
        tone: "warning",
      });
    };

    const target = findManualReviewTarget(
      reconciledFromHistory,
      reconcileExpectationsRef.current,
      jobId,
    );
    if (target) {
      showConfirmation(target);
    } else if (!SCHEDULED_TASK_COMMAND_KINDS.some((command) => (
      hasScheduledTaskCommandForJob(reconciledFromHistory, command, jobId)
    ))) {
      setFeedback(null);
    }
  }, [
    applyReconcileEvidence,
    invalidateAccess,
    isAccessInvalidated,
    refresh,
    scopeKey,
    t,
  ]);

  const requestReconcile = useCallback((): void => {
    void reconcile().catch(() => undefined);
  }, [reconcile]);

  const setAccessBlocked = useCallback((blocked: boolean): void => {
    accessBlockedRef.current = blocked;
    if (!blocked) {
      // 同一 owner 重新取得访问权时，从 durable journal 恢复在失效期间
      // 保留的 exact 保护；访问失败只能隐藏快照，不能证明其他动作未执行。
      const entries = loadScheduledTaskMutationJournal(scopeKey);
      const restored = restoreUnconfirmedCommands(entries);
      unconfirmedRef.current = restored;
      reconcileExpectationsRef.current = new Map(entries.flatMap((entry) => (
        entry.expectation
          ? [[
              scheduledTaskCommandKey(entry.command, entry.targetId),
              entry.expectation,
            ] as const]
          : []
      )));
      setUnconfirmed(restored);
      return;
    }
    // 只清当前页面的瞬时 pending 投影。owner-scoped journal 与既有
    // unconfirmed identity 必须保留到重新鉴权后的领域对账。
    const clearedPending = createPendingCommandState(SCHEDULED_TASK_COMMAND_KINDS);
    pendingRef.current = clearedPending;
    setPending(clearedPending);
    setFeedback(null);
  }, [scopeKey]);

  const runPending = useCallback(<Result,>(
    command: ScheduledTaskCommandKind,
    jobId: string,
    targetId: string,
    execute: () => Promise<Result>,
    options: RunPendingOptions = {},
  ): Promise<Result> => {
    const commandScopeKey = scopeKey;
    if (accessBlockedRef.current) {
      return Promise.reject(new Error(t("state.permission_title")));
    }
    if (unconfirmedRef.current.get(command)?.has(targetId)) {
      return Promise.reject(new Error(t("capability.scheduled_mutation_locked")));
    }
    // 同一个 job_id 可以出现在不同 owner 下；in-flight Promise 也必须按
    // scope 隔离，不能让旧账号的请求占用新账号的动作槽。
    const commandKey = `${commandScopeKey ?? "no-scope"}:${command}:${targetId}`;
    const pendingPromise = pendingPromisesRef.current.get(commandKey);
    if (pendingPromise) {
      return pendingPromise as Promise<Result>;
    }
    const mutationBlocked = options.ignoreUnconfirmedCommands
      ? SCHEDULED_TASK_COMMAND_KINDS.some((candidate) => (
          hasScheduledTaskCommandForJob(pendingRef.current, candidate, jobId)
          || (
            !options.ignoreUnconfirmedCommands?.has(candidate)
            && hasScheduledTaskCommandForJob(
              unconfirmedRef.current,
              candidate,
              jobId,
            )
          )
        ))
      : isScheduledTaskMutationBlocked(
          pendingRef.current,
          unconfirmedRef.current,
          jobId,
        );
    if (mutationBlocked) {
      return Promise.reject(new Error(t("capability.scheduled_mutation_conflict_locked")));
    }
    const nextPending = setPendingCommand(
      pendingRef.current,
      command,
      targetId,
      true,
    );
    pendingRef.current = nextPending;
    setPending(nextPending);
    const nextPromise = withScheduledTaskMutationGate(
      commandScopeKey,
      jobId,
      execute,
      options.ignoreUnconfirmedCommands,
    ).catch((error: unknown) => {
      if (
        error instanceof ScheduledTaskMutationLockUnavailableError
        && activeScopeKeyRef.current === commandScopeKey
        && !accessBlockedRef.current
      ) {
        setFeedback({
          impact: t("capability.scheduled_cross_window_lock_impact"),
          message: t("capability.scheduled_cross_window_lock_message"),
          nextStep: t("capability.scheduled_cross_window_lock_next_step"),
          title: t("capability.scheduled_cross_window_lock_title"),
          tone: "warning",
        });
      } else if (
        error instanceof ScheduledTaskMutationCoordinationUnavailableError
        && activeScopeKeyRef.current === commandScopeKey
        && !accessBlockedRef.current
      ) {
        setFeedback({
          impact: t("capability.scheduled_coordination_unavailable_impact"),
          message: t("capability.scheduled_coordination_unavailable_message"),
          nextStep: t("capability.scheduled_coordination_unavailable_next_step"),
          title: t("capability.scheduled_coordination_unavailable_title"),
          tone: "error",
        });
      }
      throw error;
    }).finally(() => {
      pendingPromisesRef.current.delete(commandKey);
      if (activeScopeKeyRef.current !== commandScopeKey) {
        return;
      }
      const settledPending = setPendingCommand(
        pendingRef.current,
        command,
        targetId,
        false,
      );
      pendingRef.current = settledPending;
      setPending(settledPending);
    });
    pendingPromisesRef.current.set(commandKey, nextPromise);
    return nextPromise;
  }, [scopeKey, t]);

  const executeCommand = useCallback(async <Result,>(
    command: ScheduledTaskCommandKind,
    targetId: string,
    execute: () => Promise<Result>,
    failure: CommandFailureFeedback,
  ): Promise<Result> => {
    const commandScopeKey = scopeKey;
    const journalReady = upsertScheduledTaskMutationJournalEntry(scopeKey, {
      command,
      expectation: failure.expectation,
      phase: "pending",
      targetId,
      updatedAt: Date.now(),
    });
    if (!journalReady) {
      setFeedback({
        impact: t("capability.scheduled_journal_unavailable_impact"),
        message: t("capability.scheduled_journal_unavailable_message"),
        nextStep: t("capability.scheduled_journal_unavailable_next_step"),
        title: t("capability.scheduled_journal_unavailable_title"),
        tone: "error",
      });
      throw new Error(t("capability.scheduled_journal_unavailable_message"));
    }
    try {
      const result = await execute();
      removeScheduledTaskMutationJournalEntry(scopeKey, command, targetId);
      if (
        isAccessInvalidated()
        || activeScopeKeyRef.current !== commandScopeKey
      ) {
        throw new ScheduledTaskCommandScopeSupersededError();
      }
      return result;
    } catch (error) {
      if (error instanceof ScheduledTaskCommandScopeSupersededError) {
        throw error;
      }
      const projection = projectScheduledTaskMutationFailure(
        error,
        failure.fallbackMessage,
      );
      if (projection.access) {
        if (projection.blocksRepeat) {
          upsertScheduledTaskMutationJournalEntry(commandScopeKey, {
            command,
            expectation: failure.expectation,
            phase: "unconfirmed",
            targetId,
            updatedAt: Date.now(),
          });
          if (failure.expectation) {
            reconcileExpectationsRef.current.set(
              scheduledTaskCommandKey(command, targetId),
              failure.expectation,
            );
          }
          updateUnconfirmed(command, targetId, true);
        } else {
          removeScheduledTaskMutationJournalEntry(
            commandScopeKey,
            command,
            targetId,
          );
          reconcileExpectationsRef.current.delete(
            scheduledTaskCommandKey(command, targetId),
          );
          updateUnconfirmed(command, targetId, false);
        }
        if (activeScopeKeyRef.current === commandScopeKey) {
          // React state 关闭页面前先同步 fence 当前 scope；其他已在途命令即使
          // 随后成功，也只能结束其服务端事实，不能再投影到已失效页面。
          accessBlockedRef.current = true;
          invalidateAccess({
            access: projection.access,
            message: projection.message,
          });
        }
        throw error;
      }
      if (projection.blocksRepeat) {
        upsertScheduledTaskMutationJournalEntry(commandScopeKey, {
          command,
          expectation: failure.expectation,
          phase: "unconfirmed",
          targetId,
          updatedAt: Date.now(),
        });
      } else {
        removeScheduledTaskMutationJournalEntry(commandScopeKey, command, targetId);
      }
      if (
        accessBlockedRef.current
        || isAccessInvalidated()
        || activeScopeKeyRef.current !== commandScopeKey
      ) {
        throw error;
      }
      if (projection.blocksRepeat) {
        if (failure.expectation) {
          reconcileExpectationsRef.current.set(
            scheduledTaskCommandKey(command, targetId),
            failure.expectation,
          );
        }
        updateUnconfirmed(command, targetId, true);
      }
      setFeedback(buildMutationFailureFeedback(
        projection,
        failure,
        failure.onReconcile ?? requestReconcile,
        t,
      ));
      throw error;
    }
  }, [
    invalidateAccess,
    isAccessInvalidated,
    requestReconcile,
    scopeKey,
    t,
    updateUnconfirmed,
  ]);

  const synchronizeMutation = useCallback(async (
    agentId: string,
    success: MutationFeedback,
    expectedScopeKey: string | null,
  ): Promise<void> => {
    if (
      accessBlockedRef.current
      || isAccessInvalidated()
      || activeScopeKeyRef.current !== expectedScopeKey
    ) {
      return;
    }
    notifyCapabilitySummaryMutated({
      agent_id: agentId,
      source: "scheduled_tasks",
    });
    try {
      await refresh({ silent: true });
      if (
        accessBlockedRef.current
        || isAccessInvalidated()
        || activeScopeKeyRef.current !== expectedScopeKey
      ) {
        return;
      }
      setFeedback({
        impact: t("capability.scheduled_mutation_confirmed_impact"),
        message: success.message,
        nextStep: success.nextStep,
        title: success.title,
        tone: "success",
      });
    } catch (error) {
      if (
        accessBlockedRef.current
        || isAccessInvalidated()
        || activeScopeKeyRef.current !== expectedScopeKey
      ) {
        return;
      }
      setFeedback({
        action: {
          label: t("capability.scheduled_reconcile_action"),
          onClick: requestReconcile,
        },
        impact: t("capability.scheduled_mutation_confirmed_stale_impact"),
        message: `${success.message}；${getErrorMessage(error, t("capability.scheduled_refresh_failed"))}`,
        nextStep: t("capability.scheduled_mutation_confirmed_stale_next_step"),
        title: success.title,
        tone: "warning",
      });
    }
  }, [isAccessInvalidated, refresh, requestReconcile, t]);

  const acceptCreatedTask = useCallback(async (task: ScheduledTaskItem): Promise<void> => {
    upsertTask(task, scopeKey);
    await synchronizeMutation(task.agent_id, {
      message: `${task.name} 已加入自动化任务列表`,
      nextStep: t("capability.scheduled_created_next_step"),
      title: "任务已创建",
    }, scopeKey);
  }, [scopeKey, synchronizeMutation, t, upsertTask]);

  const acceptSavedTask = useCallback(async (task: ScheduledTaskItem): Promise<void> => {
    upsertTask(task, scopeKey);
    await synchronizeMutation(task.agent_id, {
      message: `${task.name} 的配置已保存`,
      nextStep: t("capability.scheduled_saved_next_step"),
      title: "任务已更新",
    }, scopeKey);
  }, [scopeKey, synchronizeMutation, t, upsertTask]);

  const runTask = useCallback(async (
    task: ScheduledTaskItem,
    reviewHistory?: () => void,
  ): Promise<ScheduledTaskRunNowResponse> => {
    const requestId = `web-run:${generateUuid()}`;
    const targetId = scheduledTaskRunCommandTarget(task.job_id, requestId);
    return runPending(
      "run",
      task.job_id,
      targetId,
      () => executeCommand("run", targetId, async () => {
        const result = await runScheduledTaskApi(
          task.job_id,
          task.configuration_version,
          requestId,
        );
        await synchronizeMutation(task.agent_id, {
          message: result.status === "queued_to_main_session"
            ? `${task.name} 已排入主会话执行`
            : `${task.name} 已开始执行`,
          nextStep: t("capability.scheduled_run_next_step"),
          title: "任务已触发",
        }, scopeKey);
        return result;
      }, {
        actionLabel: reviewHistory
          ? t("capability.scheduled_review_history_action")
          : undefined,
        expectation: {
          baseConfigurationVersion: task.configuration_version,
          jobId: task.job_id,
          kind: "run",
          requestId,
        },
        fallbackMessage: "立即运行失败",
        nextStep: reviewHistory
          ? t("capability.scheduled_run_unknown_next_step")
          : undefined,
        onReconcile: reviewHistory,
        title: "任务执行失败",
      }),
    );
  }, [executeCommand, runPending, scopeKey, synchronizeMutation, t]);

  const toggleTask = useCallback(async (
    task: ScheduledTaskItem,
  ): Promise<ScheduledTaskItem> => {
    const targetId = scheduledTaskConfigurationCommandTarget(
      task.job_id,
      task.configuration_version,
    );
    return runPending(
      "toggle",
      task.job_id,
      targetId,
      () => executeCommand("toggle", targetId, async () => {
      const updatedTask = await updateScheduledTaskStatusApi(task.job_id, {
        enabled: !task.enabled,
        expected_configuration_version: task.configuration_version,
      });
      upsertTask(updatedTask, scopeKey);
      await synchronizeMutation(updatedTask.agent_id, {
        message: updatedTask.enabled
          ? `${updatedTask.name} 已恢复自动调度`
          : `${updatedTask.name} 不再参与后续调度`,
        nextStep: t("capability.scheduled_toggle_next_step"),
        title: updatedTask.enabled ? "任务已启用" : "任务已暂停",
      }, scopeKey);
      return updatedTask;
    }, {
      expectation: {
        baseConfigurationVersion: task.configuration_version,
        expectedEnabled: !task.enabled,
        jobId: task.job_id,
        kind: "toggle",
      },
      fallbackMessage: "切换任务状态失败",
      title: "状态更新失败",
      }),
    );
  }, [executeCommand, runPending, scopeKey, synchronizeMutation, t, upsertTask]);

  const decidePermission = useCallback(async (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ): Promise<AutomationPermissionDecisionResult> => {
    const request = task.pending_permission_request;
    if (!request) {
      throw new Error("权限请求已变化，请刷新任务后重试");
    }
    const targetId = scheduledTaskPermissionCommandTarget(task.job_id, request);
    return runPending(
      "permission",
      task.job_id,
      targetId,
      () => executeCommand("permission", targetId, async () => {
        const result = await decideAutomationPermissionRequestApi(
          request.request_id,
          {
            decision,
            job_id: request.job_id,
            policy_revision: request.policy_revision,
            run_id: request.run_id ?? "",
          },
        );
        const updatedTask = taskFromPermissionResult(result);
        upsertTask(updatedTask, scopeKey);
        await synchronizeMutation(updatedTask.agent_id, {
          message: `${task.name}：${permissionDecisionMessage(decision)}`,
          nextStep: t("capability.scheduled_permission_next_step"),
          title: decision === "deny" ? "权限已拒绝" : "权限已更新",
        }, scopeKey);
        return result;
      }, {
        expectation: {
          decision,
          jobId: task.job_id,
          kind: "permission_decision",
          originalStatus: request.status,
          policyRevision: request.policy_revision,
          requestId: request.request_id,
          runId: request.run_id ?? null,
        },
        fallbackMessage: "处理任务权限失败",
        title: "权限操作失败",
      }),
    );
  }, [executeCommand, runPending, scopeKey, synchronizeMutation, t, upsertTask]);

  const resumePermissionRun = useCallback(async (
    task: ScheduledTaskItem,
  ): Promise<AutomationPermissionDecisionResult> => {
    const request = task.pending_permission_request;
    const runId = request?.run_id;
    if (!request || request.status !== "approved" || !runId) {
      throw new Error("待重试的运行记录已变化，请刷新后重试");
    }
    const targetId = scheduledTaskPermissionCommandTarget(task.job_id, request);
    return runPending(
      "permission",
      task.job_id,
      targetId,
      () => executeCommand("permission", targetId, async () => {
        const result = await resumeAutomationPermissionRunApi(task.job_id, runId, {
          policy_revision: request.policy_revision,
          request_id: request.request_id,
        });
        const updatedTask = taskFromPermissionResult(result);
        upsertTask(updatedTask, scopeKey);
        await synchronizeMutation(updatedTask.agent_id, {
          message: `${task.name} 已确认重试同一次运行`,
          nextStep: t("capability.scheduled_run_next_step"),
          title: "任务已继续",
        }, scopeKey);
        return result;
      }, {
        expectation: {
          jobId: task.job_id,
          kind: "permission_resume",
          policyRevision: request.policy_revision,
          requestId: request.request_id,
          runId,
        },
        fallbackMessage: "继续任务运行失败",
        title: "任务重试失败",
      }),
    );
  }, [executeCommand, runPending, scopeKey, synchronizeMutation, t, upsertTask]);

  const confirmDeletionStopped = useCallback(async (
    task: ScheduledTaskItem,
  ): Promise<void> => {
    if (task.deletion_state?.trim() !== "review_required") {
      throw new Error("任务当前不需要人工确认，请刷新任务状态");
    }
    const targetId = scheduledTaskConfigurationCommandTarget(
      task.job_id,
      task.configuration_version,
    );
    return runPending(
      "confirmDeletionStopped",
      task.job_id,
      targetId,
      () => executeCommand(
        "confirmDeletionStopped",
        targetId,
        async () => {
          await confirmScheduledTaskDeletionStoppedApi(
            task.job_id,
            task.configuration_version,
          );
          // confirm-stopped 的成功回执同时证明原删除 intent 已完成；清掉
          // review_required 期间保留的旧 delete 保护，避免重载后出现幽灵待核对项。
          for (const deleteTargetId of unconfirmedRef.current.get("delete") ?? []) {
            if (!scheduledTaskCommandTargetsJob(deleteTargetId, task.job_id)) {
              continue;
            }
            removeScheduledTaskMutationJournalEntry(
              scopeKey,
              "delete",
              deleteTargetId,
            );
            reconcileExpectationsRef.current.delete(
              scheduledTaskCommandKey("delete", deleteTargetId),
            );
            updateUnconfirmed("delete", deleteTargetId, false);
          }
          removeTask(task.job_id, scopeKey);
          await synchronizeMutation(task.agent_id, {
            message: `${task.name} 已删除；未完成的运行已停止，运行和投递记录会保留用于核对`,
            nextStep: t("capability.scheduled_delete_confirmed_next_step"),
                title: "任务已删除",
          }, scopeKey);
        },
        {
          actionLabel: t("capability.scheduled_reconcile_action"),
          expectation: {
            baseConfigurationVersion: task.configuration_version,
            jobId: task.job_id,
            kind: "confirm_deletion_stopped",
          },
          fallbackMessage: "继续删除失败",
          nextStep: t("capability.scheduled_delete_confirmation_failed_next_step"),
          reconcileWhenNotApplied: true,
          title: "继续删除失败",
        },
      ),
      { ignoreUnconfirmedCommands: ORIGINAL_DELETE_REVIEW_LOCK },
    );
  }, [
    executeCommand,
    removeTask,
    runPending,
    scopeKey,
    synchronizeMutation,
    t,
    updateUnconfirmed,
  ]);

  const deleteTask = useCallback(async (task: ScheduledTaskItem): Promise<void> => {
    const targetId = scheduledTaskConfigurationCommandTarget(
      task.job_id,
      task.configuration_version,
    );
    return runPending(
      "delete",
      task.job_id,
      targetId,
      () => executeCommand("delete", targetId, async () => {
        await deleteScheduledTaskApi(
          task.job_id,
          task.configuration_version,
        );
        removeTask(task.job_id, scopeKey);
        await synchronizeMutation(task.agent_id, {
          message: `${task.name} 已从自动化任务列表移除`,
          nextStep: t("capability.scheduled_delete_next_step"),
          title: "任务已删除",
        }, scopeKey);
      }, {
        expectation: {
          baseConfigurationVersion: task.configuration_version,
          jobId: task.job_id,
          kind: "delete",
        },
        fallbackMessage: "删除任务失败",
        reconcileWhenNotApplied: true,
        title: "删除失败",
      }),
    );
  }, [executeCommand, removeTask, runPending, scopeKey, synchronizeMutation, t]);

  const recoverRun = useCallback(async (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ): Promise<ScheduledTaskItem> => {
    const targetId = scheduledTaskCommandTarget(task.job_id, run.run_id);
    return runPending("recover", task.job_id, targetId, () => executeCommand(
      "recover",
      targetId,
      async () => {
        const updatedTask = await recoverScheduledTaskRunApi(task.job_id, {
          run_id: run.run_id,
        });
        upsertTask(updatedTask, scopeKey);
        await synchronizeMutation(updatedTask.agent_id, {
          message: `${task.name} 的当前 run 已标记为 cancelled`,
          nextStep: t("capability.scheduled_recover_next_step"),
          title: "运行占用已释放",
        }, scopeKey);
        return updatedTask;
      },
      {
        expectation: {
          jobId: task.job_id,
          kind: "recover",
          runId: run.run_id,
        },
        fallbackMessage: "释放运行占用失败",
        title: "释放运行占用失败",
      },
    ));
  }, [executeCommand, runPending, scopeKey, synchronizeMutation, t, upsertTask]);

  const retryDelivery = useCallback(async (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
    reconcileHistory?: () => Promise<void>,
    options?: { confirmUnverifiedAttempt?: boolean },
  ): Promise<void> => {
    const targetId = scheduledTaskDeliveryCommandTarget(
      task.job_id,
      run.run_id,
      run.delivery_attempts,
    );
    return runPending("retryDelivery", task.job_id, targetId, () => executeCommand(
      "retryDelivery",
      targetId,
      async () => {
        const updatedRun = await retryScheduledTaskRunDeliveryApi(
          task.job_id,
          run.run_id,
          task.configuration_version,
          run.delivery_attempts ?? undefined,
          options?.confirmUnverifiedAttempt === true,
        );
        const deliverySucceeded = updatedRun.delivery_status === "succeeded";
        await synchronizeMutation(task.agent_id, {
          message: deliverySucceeded
            ? `${task.name} 的运行结果已重新投递`
            : `${task.name} 的投递状态已更新为 ${updatedRun.delivery_status ?? "unknown"}`,
          nextStep: t("capability.scheduled_delivery_next_step"),
          title: deliverySucceeded ? "投递已恢复" : "投递已重试",
        }, scopeKey);
      },
      {
        actionLabel: t("capability.scheduled_review_history_action"),
        fallbackMessage: "重试投递失败",
        nextStep: t("capability.scheduled_delivery_unknown_next_step"),
        onReconcile: reconcileHistory
          ? () => void reconcileHistory().catch(() => undefined)
          : undefined,
        title: "重试投递失败",
      },
    ));
  }, [executeCommand, runPending, scopeKey, synchronizeMutation, t]);
  const dismissFeedback = useCallback(() => setFeedback(null), []);
  const isTaskMutationBlocked = useCallback((jobId: string): boolean => (
    isScheduledTaskMutationBlocked(
      pendingRef.current,
      unconfirmedRef.current,
      jobId,
    )
  ), []);

  return {
    acceptCreatedTask,
    acceptSavedTask,
    confirmDeletionStopped,
    confirmRunHistoryReconciled,
    confirmReviewedMutation,
    decidePermission,
    deleteTask,
    dismissFeedback,
    feedback,
    isTaskMutationBlocked,
    pending,
    reconcile,
    recoverRun,
    resumePermissionRun,
    retryDelivery,
    runTask,
    setAccessBlocked,
    toggleTask,
    unconfirmed,
  };
}
