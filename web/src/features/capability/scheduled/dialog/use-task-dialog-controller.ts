// INPUT: Scheduled 创建/编辑表单、owner scope、服务端版本/回执与 FailureCore。
// OUTPUT: 表单资源、提交命令、精简失败事实和显式对账动作。
// POS: Scheduled 表单控制器；创建 ID 可跨重载恢复，更新结果以服务端事实为准。

"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  createScheduledTaskApi,
  getScheduledTaskCreateRequestApi,
  updateScheduledTaskApi,
} from "@/lib/api/capability/scheduled-task-api";
import { isExternalSessionChannel } from "@/lib/conversation/external-session";
import { parseSessionKey } from "@/lib/conversation/session-key";
import {
  getErrorMessage,
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import {
  type I18nContextValue,
  useI18n,
} from "@/shared/i18n/i18n-context";
import type {
  ScheduledTaskItem,
  ScheduledTaskCreateRequestStatus,
  UpdateScheduledTaskParams,
} from "@/types/capability/scheduled-task/task";

import {
  projectScheduledTaskMutationFailure,
  type ScheduledTaskMutationFailureProjection,
} from "../controller/scheduled-task-mutation-outcome";
import {
  scheduledTaskConfigurationCommandTarget,
} from "../controller/scheduled-task-directory-model";
import {
  clearScheduledTaskCreateRequestId,
  loadScheduledTaskCreateRequestId,
  saveScheduledTaskCreateRequestId,
} from "../controller/scheduled-task-create-intent";
import {
  buildDefaultTaskDialogInitialState,
  buildTaskDialogInitialState,
} from "./form/task-form-initializer";
import {
  buildScheduledTaskPayload,
  getTaskDialogValidationError,
  type TaskDialogSubmitContext,
} from "./form/task-form-submit";
import { useTaskForm } from "./form/use-task-form";
import { useTaskDialogData } from "./resources/use-task-dialog-data";
import { useTaskSchedule } from "./schedule/use-task-schedule";
import type {
  TaskDialogCreatePreset,
  TaskDialogRefs,
} from "./scheduled-task-dialog-types";

interface TaskDialogControllerOptions {
  agentId: string;
  createPreset?: TaskDialogCreatePreset | null;
  initialTask?: ScheduledTaskItem | null;
  isOpen: boolean;
  onClose: () => void;
  onAccessFailure?: (failure: ResourceFailure) => void;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onCreateIntentResolved?: (status?: ScheduledTaskCreateRequestStatus) => void;
  onConfirmMutationReviewed?: (command: "update", targetId: string) => void;
  onIsMutationBlocked?: (jobId: string) => boolean;
  onReconcile?: () => Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
  scopeKey: string | null;
}

interface SubmitTaskDialogOptions {
  context: TaskDialogSubmitContext;
  createRequestId: string;
  initialTask: ScheduledTaskItem | null;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
  t: I18nContextValue["t"];
}

function needsLegacyDeliveryRebind(task: ScheduledTaskItem | null): boolean {
  if (!task) {
    return false;
  }
  if (task.execution_kind === "script" || task.delivery.mode === "none") {
    return false;
  }
  const sessionKey = task.delivery.session_key?.trim()
    || task.delivery.to?.trim()
    || "";
  const parsed = parseSessionKey(sessionKey);
  return !parsed.is_structured || (parsed.kind === "agent"
    && parsed.channel === "internal"
    && parsed.ref === "automation-inbox");
}

function buildUpdatePayload(
  payload: UpdateScheduledTaskParams,
  initialTask: ScheduledTaskItem,
  expiresAtDraft: string,
  permissionModeDraft: TaskDialogSubmitContext["form"]["permissionMode"],
): UpdateScheduledTaskParams {
  // source 是任务创建来源，不是可编辑配置。更新时保持原始 provenance，避免
  // 历史 IM 会话已解绑后，修改名称或计划被误判成一次新的会话授权。
  const { source: _source, ...configurationPayload } = payload;
  const withPermissionRefresh = permissionModeDraft === "copy"
    ? { ...configurationPayload, permission_mode: "" as const }
    : configurationPayload;
  if (expiresAtDraft.trim() || initialTask.expires_at === null) {
    return withPermissionRefresh;
  }
  return { ...withPermissionRefresh, clear_expires_at: true };
}

async function submitTaskDialog({
  context,
  createRequestId,
  initialTask,
  onCreated,
  onSaved,
  t,
}: SubmitTaskDialogOptions): Promise<void> {
  const payload = buildScheduledTaskPayload(context, t);
  if (initialTask) {
    const updatePayload = buildUpdatePayload(
      payload,
      initialTask,
      context.form.expiresAt,
      context.form.permissionMode,
    );
    updatePayload.expected_configuration_version = initialTask.configuration_version;
    const updated = await updateScheduledTaskApi(initialTask.job_id, updatePayload);
    await onSaved?.(updated);
    return;
  }

  const created = await createScheduledTaskApi({
    ...payload,
    request_id: createRequestId,
  });
  await onCreated?.(created);
}

function getSubmitErrorMessage(
  error: unknown,
  initialTask: ScheduledTaskItem | null,
  t: I18nContextValue["t"],
): string {
  return getErrorMessage(error, initialTask
    ? t("capability.scheduled_dialog_save_failed")
    : t("capability.scheduled_dialog_create_failed"));
}

function notAppliedMutationFailure(
  message: string,
  code: string | null = null,
): ScheduledTaskMutationFailureProjection {
  return {
    access: null,
    blocksRepeat: false,
    category: null,
    code,
    effect: "not_applied",
    message,
    transportRequestId: null,
  };
}

export function useTaskDialogController({
  agentId,
  createPreset = null,
  initialTask = null,
  isOpen,
  onClose,
  onAccessFailure,
  onCreated,
  onCreateIntentResolved,
  onConfirmMutationReviewed,
  onIsMutationBlocked,
  onReconcile,
  onSaved,
  scopeKey,
}: TaskDialogControllerOptions) {
  const { t } = useI18n();
  const initialState = useMemo(
    () => initialTask
      ? buildTaskDialogInitialState(initialTask)
      : buildDefaultTaskDialogInitialState(agentId, createPreset),
    [agentId, createPreset, initialTask],
  );
  const [formError, setFormError] = useState<string | null>(null);
  const [createRequestId, setCreateRequestId] = useState(createTaskRequestId);
  const [isRestoredCreateIntent, setIsRestoredCreateIntent] = useState(false);
  const [isReconciling, setIsReconciling] = useState(false);
  const [isMutationReviewed, setIsMutationReviewed] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [mutationFailure, setMutationFailure] = useState<
    ScheduledTaskMutationFailureProjection | null
  >(null);
  const mutationFailureRef = useRef<ScheduledTaskMutationFailureProjection | null>(null);
  const activeCreateRequestCheckRef = useRef<string | null>(null);
  const activeScopeKeyRef = useRef(scopeKey);
  const submitInFlightRef = useRef(false);
  const refs: TaskDialogRefs = {
    dailyPickerAnchorRef: useRef<HTMLButtonElement>(null),
    nameRef: useRef<HTMLInputElement>(null),
    singlePickerAnchorRef: useRef<HTMLButtonElement>(null),
  };

  useLayoutEffect(() => {
    activeScopeKeyRef.current = scopeKey;
  }, [scopeKey]);

  const updateMutationFailure = useCallback((
    failure: ScheduledTaskMutationFailureProjection | null,
  ): void => {
    mutationFailureRef.current = failure;
    setMutationFailure(failure);
  }, []);
  const restoreCreateIntent = useCallback((requestId: string): void => {
    const restoredFailure: ScheduledTaskMutationFailureProjection = {
      access: null,
      blocksRepeat: true,
      category: null,
      code: null,
      effect: "unknown",
      message: t("capability.scheduled_dialog_create_restored_message"),
      transportRequestId: null,
    };
    setCreateRequestId(requestId);
    setIsRestoredCreateIntent(true);
    updateMutationFailure(restoredFailure);
  }, [t, updateMutationFailure]);
  const startFreshCreateIntent = useCallback((): void => {
    setCreateRequestId(createTaskRequestId());
    setIsRestoredCreateIntent(false);
    updateMutationFailure(null);
    setFormError(null);
  }, [updateMutationFailure]);
  const clearError = useCallback(() => {
    if (!mutationFailureRef.current?.blocksRepeat) {
      setFormError(null);
    }
  }, []);
  const form = useTaskForm(initialState.form, clearError);
  const schedule = useTaskSchedule(initialState.schedule, clearError);
  const hydrateForm = form.hydrate;
  const hydrateSchedule = schedule.hydrate;
  const data = useTaskDialogData({
    form: form.draft,
    isOpen,
  });
  const resolveSelectedRoomIds = form.actions.resolveSelectedRoomIds;
  const selectedSession = data.sessionOptions.find(
    (option) => option.value === form.draft.selectedSessionKey,
  ) ?? null;
  const selectedReplySession = data.deliverySessionOptions.find(
    (option) => option.value === form.draft.selectedReplySessionKey,
  ) ?? null;
  const hasUnavailableExternalSession = (
    !data.sessions.loading
    && !data.sessions.error
    && Boolean(form.draft.selectedSessionKey)
    && isExternalSessionChannel(null, form.draft.selectedSessionKey)
    && !data.sessionOptions.some(
      (option) => option.value === form.draft.selectedSessionKey,
    )
  ) || (
    !data.deliverySessions.loading
    && !data.deliverySessions.error
    && Boolean(form.draft.selectedReplySessionKey)
    && isExternalSessionChannel(null, form.draft.selectedReplySessionKey)
    && !data.deliverySessionOptions.some(
      (option) => option.value === form.draft.selectedReplySessionKey,
    )
  );
  const needsSessionRebind = initialTask?.session_binding_state === "rebind_required"
    || hasUnavailableExternalSession
    || needsLegacyDeliveryRebind(initialTask ?? null);
  const updateTargetId = initialTask
    ? scheduledTaskConfigurationCommandTarget(
        initialTask.job_id,
        initialTask.configuration_version,
      )
    : null;

  const submitContext = useMemo<TaskDialogSubmitContext>(() => ({
    defaultDeliveryRoomAgentId: data.defaultDeliveryRoomAgentId,
    defaultExecutionRoomAgentId: data.defaultExecutionRoomAgentId,
    form: form.draft,
    schedule: schedule.draft,
    selectedReplySession,
    selectedSession,
  }), [
    data.defaultDeliveryRoomAgentId,
    data.defaultExecutionRoomAgentId,
    form.draft,
    schedule.draft,
    selectedReplySession,
    selectedSession,
  ]);

  const hydrate = useCallback(() => {
    hydrateForm(initialState.form);
    hydrateSchedule(initialState.schedule);
    setFormError(null);
    setIsReconciling(false);
    setIsMutationReviewed(false);
    setIsSubmitting(false);
    updateMutationFailure(null);
    setIsRestoredCreateIntent(false);
    submitInFlightRef.current = false;
    if (!initialTask) {
      const restoredRequestId = loadScheduledTaskCreateRequestId(scopeKey);
      if (restoredRequestId) {
        restoreCreateIntent(restoredRequestId);
      } else {
        startFreshCreateIntent();
      }
    }
  }, [
    hydrateForm,
    hydrateSchedule,
    initialState,
    initialTask,
    restoreCreateIntent,
    scopeKey,
    startFreshCreateIntent,
    updateMutationFailure,
  ]);

  const handleSubmit = useCallback(async () => {
    if (submitInFlightRef.current || mutationFailureRef.current?.blocksRepeat) {
      return;
    }
    const validationError = getTaskDialogValidationError(submitContext, t);
    if (validationError) {
      setFormError(validationError);
      return;
    }

    const submissionScopeKey = scopeKey;
    submitInFlightRef.current = true;
    setIsSubmitting(true);
    setFormError(null);
    updateMutationFailure(null);
    setIsMutationReviewed(false);
    const submit = async (): Promise<void> => {
    if (!initialTask) {
      // 创建回执依赖 exact request ID；浏览器持久化只是重载恢复增强，
      // 不能成为提交动作的额外可用性门槛。
      saveScheduledTaskCreateRequestId(scopeKey, createRequestId);
    }
    try {
      await submitTaskDialog({
        context: submitContext,
        createRequestId,
        initialTask,
        onCreated,
        onSaved,
        t,
      });
      if (!initialTask) {
        clearScheduledTaskCreateRequestId(submissionScopeKey, createRequestId);
      }
      if (activeScopeKeyRef.current !== submissionScopeKey) {
        return;
      }
      if (!initialTask) {
        onCreateIntentResolved?.("committed");
        setCreateRequestId(createTaskRequestId());
      }
      onClose();
    } catch (error) {
      const projection = projectScheduledTaskMutationFailure(
        error,
        getSubmitErrorMessage(error, initialTask, t),
      );
      if (projection.access) {
        if (!initialTask) {
          if (!projection.blocksRepeat) {
            clearScheduledTaskCreateRequestId(submissionScopeKey, createRequestId);
            onCreateIntentResolved?.();
          }
        }
        if (activeScopeKeyRef.current === submissionScopeKey) {
          onAccessFailure?.({
            access: projection.access,
            message: projection.message,
          });
        }
        return;
      }
      if (projection.blocksRepeat) {
        setIsMutationReviewed(false);
      } else if (!initialTask) {
        // 服务端已明确证明旧创建没有提交；立刻清除旧 request_id，避免
        // 用户直接关闭后，下一次打开又把它误恢复成“结果未知”。
        clearScheduledTaskCreateRequestId(submissionScopeKey, createRequestId);
      }
      if (activeScopeKeyRef.current !== submissionScopeKey) {
        return;
      }
      if (!projection.blocksRepeat && !initialTask) {
        const nextRequestId = loadScheduledTaskCreateRequestId(submissionScopeKey);
        onCreateIntentResolved?.();
        if (nextRequestId) {
          // 多页面可能各自留下独立创建意图；证明当前请求未提交后，必须先
          // 接管下一条未确认 identity，不能直接开放一个新的创建提交。
          restoreCreateIntent(nextRequestId);
          return;
        }
      }
      updateMutationFailure(projection);
      if (!projection.blocksRepeat && !initialTask) {
        setCreateRequestId(createTaskRequestId());
        setIsRestoredCreateIntent(false);
      }
    } finally {
      if (activeScopeKeyRef.current === submissionScopeKey) {
        submitInFlightRef.current = false;
        setIsSubmitting(false);
      }
    }
    };
    await submit();
  }, [
    createRequestId,
    initialTask,
    onClose,
    onCreated,
    onCreateIntentResolved,
    onAccessFailure,
    onSaved,
    restoreCreateIntent,
    scopeKey,
    submitContext,
    t,
    updateMutationFailure,
  ]);

  const reconcileCreateRequest = useCallback(async (
    requestId: string,
  ): Promise<void> => {
    const requestScopeKey = scopeKey;
    const checkKey = `${requestScopeKey ?? "no-scope"}:${requestId}`;
    if (activeCreateRequestCheckRef.current === checkKey) {
      return;
    }
    activeCreateRequestCheckRef.current = checkKey;
    try {
      const result = await getScheduledTaskCreateRequestApi(requestId);
      if (activeScopeKeyRef.current !== requestScopeKey) {
        return;
      }
      if (
        result.request_id !== requestId
        || !["committed", "gone", "not_found"].includes(result.status)
        || (result.status === "committed" ? !result.task : Boolean(result.task))
      ) {
        throw new Error(t("capability.scheduled_dialog_create_status_invalid"));
      }
      if (result.status === "committed" && result.task) {
        clearScheduledTaskCreateRequestId(requestScopeKey, requestId);
        setCreateRequestId(createTaskRequestId());
        setIsRestoredCreateIntent(false);
        updateMutationFailure(null);
        onCreateIntentResolved?.(result.status);
        await onCreated?.(result.task);
        if (activeScopeKeyRef.current === requestScopeKey) {
          onClose();
        }
        return;
      }
      if (result.status === "gone") {
        clearScheduledTaskCreateRequestId(requestScopeKey, requestId);
        const nextRequestId = loadScheduledTaskCreateRequestId(requestScopeKey);
        if (nextRequestId) {
          restoreCreateIntent(nextRequestId);
          onCreateIntentResolved?.(result.status);
          return;
        }
        setCreateRequestId(createTaskRequestId());
        setIsRestoredCreateIntent(false);
        const goneFailure = notAppliedMutationFailure(
          t("capability.scheduled_dialog_create_gone"),
        );
        updateMutationFailure(goneFailure);
        onCreateIntentResolved?.(result.status);
        return;
      }

      // not_found 只是查询时没有 durable ledger，不能线性化地
      // 排除另一个已受理请求正在提交。保留 exact request_id 和锁，
      // 只允许用户在看过重复风险后显式开始新意图。
      const unresolvedFailure: ScheduledTaskMutationFailureProjection = {
        access: null,
        blocksRepeat: true,
        category: "not_found",
        code: null,
        effect: "unknown",
        message: t("capability.scheduled_dialog_create_not_found"),
        transportRequestId: null,
      };
      setIsRestoredCreateIntent(true);
      updateMutationFailure(unresolvedFailure);
    } catch (error) {
      if (activeScopeKeyRef.current !== requestScopeKey) {
        return;
      }
      const failure = getResourceFailure(
        error,
        t("capability.scheduled_dialog_create_status_failed"),
      );
      if (failure.access) {
        onAccessFailure?.(failure);
      }
      throw error;
    } finally {
      if (activeCreateRequestCheckRef.current === checkKey) {
        activeCreateRequestCheckRef.current = null;
      }
    }
  }, [
    onAccessFailure,
    onClose,
    onCreated,
    onCreateIntentResolved,
    restoreCreateIntent,
    scopeKey,
    t,
    updateMutationFailure,
  ]);

  const startNewCreateIntent = useCallback((): void => {
    if (initialTask || isReconciling || isSubmitting) {
      return;
    }
    clearScheduledTaskCreateRequestId(scopeKey, createRequestId);
    onCreateIntentResolved?.();
    const nextRequestId = loadScheduledTaskCreateRequestId(scopeKey);
    if (nextRequestId) {
      restoreCreateIntent(nextRequestId);
      return;
    }
    startFreshCreateIntent();
  }, [
    initialTask,
    createRequestId,
    isReconciling,
    isSubmitting,
    onCreateIntentResolved,
    restoreCreateIntent,
    scopeKey,
    startFreshCreateIntent,
  ]);

  const confirmReviewedMutation = useCallback((): void => {
    if (!initialTask || !isMutationReviewed || isReconciling || isSubmitting) {
      return;
    }
    const reviewedScopeKey = scopeKey;
    const targetId = updateTargetId ?? initialTask.job_id;
    onConfirmMutationReviewed?.("update", targetId);
    if (activeScopeKeyRef.current !== reviewedScopeKey) {
      return;
    }
    updateMutationFailure(null);
    setFormError(null);
    setIsMutationReviewed(false);
    // 当前表单仍基于旧 configuration_version，解除保护后必须重新打开，
    // 避免用户在旧版本草稿上继续提交。
    onClose();
  }, [
    initialTask,
    isMutationReviewed,
    isReconciling,
    isSubmitting,
    onClose,
    onConfirmMutationReviewed,
    scopeKey,
    updateTargetId,
    updateMutationFailure,
  ]);

  const reconcileMutation = useCallback(async (): Promise<void> => {
    if (
      !mutationFailureRef.current?.blocksRepeat
      || (initialTask && (!onReconcile || !onIsMutationBlocked))
      || isReconciling
    ) {
      return;
    }
    const reconcileScopeKey = scopeKey;
    setIsReconciling(true);
    try {
      if (!initialTask) {
        await reconcileCreateRequest(createRequestId);
        return;
      }
      await onReconcile?.();
      if (activeScopeKeyRef.current !== reconcileScopeKey) {
        return;
      }
      // 同一版本的权威读取仍不能排除旧请求稍后提交。刷新只完成核对，
      // 当前页面继续保护，直到用户明确接受重复修改风险。
      setIsMutationReviewed(true);
    } catch (error) {
      if (activeScopeKeyRef.current !== reconcileScopeKey) {
        return;
      }
    } finally {
      if (activeScopeKeyRef.current === reconcileScopeKey) {
        setIsReconciling(false);
      }
    }
  }, [
    createRequestId,
    initialTask,
    isReconciling,
    onIsMutationBlocked,
    onReconcile,
    reconcileCreateRequest,
    scopeKey,
  ]);

  useEffect(() => {
    resolveSelectedRoomIds({
      deliveryRoomId: data.resolvedDeliveryRoomId,
      executionRoomId: data.resolvedExecutionRoomId,
    });
  }, [
    data.resolvedDeliveryRoomId,
    data.resolvedExecutionRoomId,
    resolveSelectedRoomIds,
  ]);

  useEffect(() => {
    const restoredRequestId = loadScheduledTaskCreateRequestId(scopeKey);
    if (!restoredRequestId) {
      return;
    }
    void reconcileCreateRequest(restoredRequestId).catch(() => undefined);
  }, [reconcileCreateRequest, scopeKey]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    hydrate();
  }, [hydrate, isOpen]);

  return {
    clearError,
    confirmReviewedMutation,
    data,
    formError,
    form,
    handleSubmit,
    isCloseBlocked: isSubmitting || isReconciling || Boolean(mutationFailure?.blocksRepeat),
    isReconciling,
    isMutationReviewed,
    isRestoredCreateIntent,
    isSubmitting,
    mutationFailure,
    needsSessionRebind,
    reconcileMutation,
    refs,
    schedule,
    startNewCreateIntent,
  };
}

function createTaskRequestId(): string {
  return `web-create:${globalThis.crypto.randomUUID()}`;
}
