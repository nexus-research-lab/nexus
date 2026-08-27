import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("Scheduled mutation 只在服务端明确未生效时允许重复", async () => {
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { projectScheduledTaskMutationFailure } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/controller/scheduled-task-mutation-outcome.ts",
  );

  const notApplied = projectScheduledTaskMutationFailure(new ApiRequestError(
    "revision conflict",
    409,
    failureCore("not_applied"),
  ), "fallback");
  assert.equal(notApplied.effect, "not_applied");
  assert.equal(notApplied.blocksRepeat, false);
  assert.equal(notApplied.code, "automation.test_failure");

  for (const effect of ["accepted", "committed", "unknown"]) {
    const projection = projectScheduledTaskMutationFailure(new ApiRequestError(
      "mutation incomplete",
      500,
      failureCore(effect),
    ), "fallback");
    assert.equal(projection.effect, effect);
    assert.equal(projection.blocksRepeat, true);
  }

  const future = projectScheduledTaskMutationFailure(new ApiRequestError(
    "future failure",
    500,
    failureCore("future_effect"),
  ), "fallback");
  assert.equal(future.effect, "unknown");
  assert.equal(future.blocksRepeat, true);

  const disconnected = projectScheduledTaskMutationFailure(new ApiTransportError(
    "response interrupted",
    "response_interrupted",
    "unknown",
  ), "fallback");
  assert.equal(disconnected.effect, "unknown");
  assert.equal(disconnected.blocksRepeat, true);
});

test("Scheduled 主视图区分首次加载、旧快照、空态和访问失效", async () => {
  const { getScheduledTaskBoardState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/board/scheduled-task-board-state.ts",
  );
  const base = {
    failure: null,
    hasSnapshot: false,
    isLoading: false,
    itemCount: 0,
  };

  assert.equal(getScheduledTaskBoardState({
    ...base,
    isLoading: true,
  }), "loading");
  assert.equal(getScheduledTaskBoardState({
    ...base,
    hasSnapshot: true,
    isLoading: true,
  }), "empty");
  assert.equal(getScheduledTaskBoardState({
    ...base,
    failure: { access: null, message: "refresh failed" },
    hasSnapshot: true,
    itemCount: 1,
  }), "ready");
  assert.equal(getScheduledTaskBoardState({
    ...base,
    failure: { access: "forbidden", message: "forbidden" },
    hasSnapshot: true,
    itemCount: 1,
  }), "error");
});

test("Scheduled 主资源、辅助权限、访问失效和副作用 journal 使用独立边界", async () => {
  const [
    resource,
    commands,
    directory,
    board,
    history,
    dialog,
    schedulePanel,
    api,
    journalSource,
  ] = await Promise.all([
    read("src/features/capability/scheduled/controller/use-scheduled-tasks-resource.ts"),
    read("src/features/capability/scheduled/controller/use-scheduled-task-commands.ts"),
    read("src/features/capability/scheduled/scheduled-tasks-directory.tsx"),
    read("src/features/capability/scheduled/board/scheduled-task-board.tsx"),
    read("src/features/capability/scheduled/history/view/scheduled-task-run-history-content.tsx"),
    read("src/features/capability/scheduled/dialog/use-task-dialog-controller.ts"),
    read("src/features/capability/scheduled/dialog/schedule/task-schedule-panel.tsx"),
    read("src/lib/api/capability/scheduled-task-api.ts"),
    read("src/features/capability/scheduled/controller/scheduled-task-mutation-journal.ts"),
  ]);

  assert.doesNotMatch(resource, /Promise\.allSettled\(\[/);
  assert.match(resource, /void refreshPermissionRequests\(\)\.catch/);
  assert.match(resource, /return \{ items: tasks \};/);
  assert.match(resource, /invalidateAccess/);
  assert.match(resource, /permissionFailure/);
  assert.match(resource, /mergePermissionRequests\(previous\.items, \[\]\)/);
  assert.match(resource, /if \(current\.failure\?\.access\) \{[\s\S]*return current;/);
  assert.match(resource, /permissionSnapshotRef\.current = \{ hasSnapshot: false, requests: \[\] \}/);
  assert.match(resource, /accessInvalidatedRef/);
  assert.match(resource, /activeScopeKeyRef\.current === requestScopeKey/);

  assert.match(commands, /unconfirmedRef\.current\.get\(command\)\?\.has\(targetId\)/);
  assert.match(commands, /if \(projection\.blocksRepeat\)/);
  assert.match(commands, /phase: "pending"/);
  assert.match(commands, /phase: "unconfirmed"/);
  assert.match(commands, /allowScheduledTaskRepeat/);
  assert.match(commands, /refresh\(\{ includePermissions: false \}\)/);
  assert.doesNotMatch(commands, /latestPermissionRequestsRef/);
  assert.match(commands, /commands: new Set\(\["recover", "run"\]\)/);
  assert.match(commands, /requestId = `web-run:\$\{generateUuid\(\)\}`/);
  assert.match(commands, /scheduledTaskRunCommandTarget\(task\.job_id, requestId\)/);
  assert.match(commands, /scheduledTaskConfigurationCommandTarget\(/);
  assert.match(commands, /scheduledTaskDeliveryCommandTarget\(/);
  assert.match(
    commands,
    /runScheduledTaskApi\(\s*runExpectation\.jobId,\s*runExpectation\.baseConfigurationVersion,\s*runExpectation\.requestId,/,
  );
  assert.match(
    commands,
    /被 401\/403 拒绝[\s\S]*if \(projection\.access\) \{[\s\S]*invalidateAccess\([\s\S]*return;/,
  );
  assert.match(commands, /scheduled_mutation_review_unlock_action/);
  assert.doesNotMatch(commands, /稍后会自动同步|自动重试/);

  assert.match(directory, /owner:local-system/);
  assert.match(directory, /isOpen=\{!scopeUnavailable[\s\S]*!editingTaskUnavailable[\s\S]*dialog\.kind !== "closed"\}/);
  assert.match(directory, /isOpen=\{!scopeUnavailable && !accessBlocked && visibleHistoryTask !== null\}/);
  assert.match(directory, /isOpen=\{!scopeUnavailable[\s\S]*!deleteTargetUnavailable[\s\S]*deleteTarget !== null\}/);
  assert.match(directory, /const authoritativeTask = resource\.items\.find/);
  assert.match(directory, /commands\.reconcile\(\)/);
  assert.doesNotMatch(directory, /disabled=\{resource\.isLoading \|\| resource\.isPermissionLoading\}/);
  assert.match(board, /permissionFailure/);
  assert.match(history, /scheduledTaskCommandTarget\(task\.job_id, run\.run_id\)/);
  assert.match(dialog, /request_id: createRequestId/);
  assert.match(dialog, /saveScheduledTaskCreateRequestId\(scopeKey, createRequestId\)/);
  assert.match(dialog, /scheduled_dialog_create_restored_message/);
  assert.match(dialog, /isRestoredCreateIntent/);
  assert.match(dialog, /mutationFailureRef\.current\?\.blocksRepeat/);
  assert.match(dialog, /updateMutationFailure\(projection\)/);
  assert.match(dialog, /scheduled\.journal_unavailable/);
  assert.match(dialog, /getScheduledTaskCreateRequestApi\(requestId\)/);
  assert.match(dialog, /void reconcileCreateRequest\(restoredRequestId\)/);
  assert.match(dialog, /scheduled_dialog_create_not_found/);
  assert.match(dialog, /startNewCreateIntent/);
  assert.match(
    dialog,
    /clearScheduledTaskCreateRequestId\(scopeKey, createRequestId\);[\s\S]*loadScheduledTaskCreateRequestId\(scopeKey\)[\s\S]*restoreCreateIntent\(nextRequestId\)/,
  );
  assert.match(dialog, /await onReconcile\?\.\(\)/);
  assert.match(dialog, /setIsMutationReviewed\(true\)/);
  assert.match(
    dialog,
    /const targetId = updateTargetId \?\? initialTask\.job_id;[\s\S]*onConfirmMutationReviewed\("update", targetId\)/,
  );
  assert.match(schedulePanel, /scheduled_mutation_not_applied_impact/);
  assert.match(schedulePanel, /scheduled_journal_unavailable_impact/);
  assert.match(schedulePanel, /scheduled_mutation_not_applied_next_step/);
  assert.match(api, /deleteScheduledTaskApi\([\s\S]*expectedConfigurationVersion/);
  assert.match(api, /method: "DELETE",[\s\S]*expected_configuration_version: expectedConfigurationVersion/);
  assert.match(api, /confirmScheduledTaskDeletionStoppedApi/);
  assert.match(api, /\/deletion\/confirm-stopped/);
  assert.match(api, /method: "POST",[\s\S]*expected_configuration_version: expectedConfigurationVersion/);
  assert.match(api, /request_id: requestId/);
  assert.match(api, /SCHEDULED_TASKS_API_BASE_URL\}\/create-requests\/\$\{encodeURIComponent\(requestId\)\}/);
  assert.doesNotMatch(journalSource, /MAX_ENTRIES|slice\(-/);
  assert.match(journalSource, /window\.localStorage/);
  assert.doesNotMatch(journalSource, /window\.sessionStorage/);
  assert.match(journalSource, /window\.addEventListener\("storage"/);
  assert.match(journalSource, /navigator\.locks/);
  assert.match(journalSource, /ifAvailable: true/);
  assert.match(journalSource, /ScheduledTaskMutationCoordinationUnavailableError/);
  assert.doesNotMatch(
    journalSource,
    /if \(!scopeKey \|\| !jobId\.trim\(\) \|\| !lockManager\) \{\s*return execute\(\)/,
  );
  assert.match(commands, /withScheduledTaskMutationGate\(/);
  assert.match(dialog, /withScheduledTaskMutationGate\(/);
});

test("Scheduled 访问重验和运行历史都以 owner scope 为硬栅栏", async () => {
  const [
    resource,
    directory,
    historyResource,
    historyDialog,
    historyActions,
    commands,
  ] = await Promise.all([
    read("src/features/capability/scheduled/controller/use-scheduled-tasks-resource.ts"),
    read("src/features/capability/scheduled/scheduled-tasks-directory.tsx"),
    read("src/features/capability/scheduled/history/use-scheduled-task-run-history-resource.ts"),
    read("src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx"),
    read("src/features/capability/scheduled/history/use-scheduled-task-run-history-actions.ts"),
    read("src/features/capability/scheduled/controller/use-scheduled-task-commands.ts"),
  ]);

  const revalidation = resource.slice(resource.indexOf("const revalidateAccess"));
  assert.match(revalidation, /accessInvalidatedRef\.current/);
  assert.match(revalidation, /hasSnapshot: false,[\s\S]*items: \[\]/);
  assert.match(revalidation, /tasks = await listScheduledTasksApi\(\)/);
  assert.ok(
    revalidation.indexOf("isCurrentRequest(version)")
      < revalidation.indexOf("accessInvalidatedRef.current = false"),
    "access fence must only clear after the current owner request is verified",
  );
  assert.match(
    directory,
    /accessBlockedRef\.current[\s\S]*resource\.revalidateAccess\(\)[\s\S]*commands\.reconcile\(\)/,
  );

  assert.match(historyResource, /runHistoryResourceKey\(scopeKey, taskJobId\)/);
  assert.match(historyResource, /activeResourceKeyRef\.current !== resourceKey/);
  const historyRead = historyResource.slice(
    historyResource.indexOf("const runs = await listScheduledTaskRunsApi"),
  );
  assert.ok(
    historyRead.indexOf("throw new ScheduledTaskRunHistoryRefreshSupersededError")
      < historyRead.indexOf("return runs"),
    "superseded history must not be returned to reconciliation callers",
  );
  assert.match(historyDialog, /useScheduledTaskRunHistoryResource\([\s\S]*scopeKey/);
  assert.match(historyDialog, /useScheduledTaskRunHistoryActions\([\s\S]*scopeKey/);
  assert.match(historyActions, /runHistoryTaskKey\(scopeKey, taskJobId\)/);
  assert.match(
    commands,
    /confirmRunHistoryReconciled = useCallback\([\s\S]*activeScopeKeyRef\.current !== scopeKey[\s\S]*return;/,
  );
});

test("Scheduled 只按 authoritative evidence 清 exact mutation", async () => {
  const {
    allowScheduledTaskRepeat,
    isScheduledTaskMutationBlocked,
    reconcileScheduledTaskUnconfirmed,
    scheduledTaskCommandKey,
    scheduledTaskPermissionCommandTarget,
  } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/controller/scheduled-task-directory-model.ts",
  );
  const empty = commandState();
  const pendingDelete = commandState({ delete: ["job-1"] });
  assert.equal(isScheduledTaskMutationBlocked(pendingDelete, empty, "job-1"), true);
  assert.equal(isScheduledTaskMutationBlocked(pendingDelete, empty, "job-2"), false);

  const decisionRequest = permissionRequest("decision-request", "job-permission", "run-p");
  const resumeRequest = {
    ...permissionRequest("resume-request", "job-resume", "run-resume"),
    decision: "allow_once",
    status: "approved",
  };
  const decisionTarget = scheduledTaskPermissionCommandTarget(
    decisionRequest.job_id,
    decisionRequest,
  );
  const resumeTarget = scheduledTaskPermissionCommandTarget(
    resumeRequest.job_id,
    resumeRequest,
  );
  const uncertain = commandState({
    confirmDeletionStopped: ["job-confirm-stopped"],
    delete: [
      "job-delete",
      "job-delete-claimed",
      "job-delete-review",
      "job-delete-stale",
      "job-delete-same",
    ],
    permission: [decisionTarget, resumeTarget],
    recover: ["job-recover:run-recover"],
    retryDelivery: ["job-delivery:run-1", "job-other:run-2"],
    run: ["job-run"],
    toggle: ["job-toggle"],
    update: ["job-update"],
  });
  const expectations = new Map([
    [scheduledTaskCommandKey("confirmDeletionStopped", "job-confirm-stopped"), {
      baseConfigurationVersion: 8,
      jobId: "job-confirm-stopped",
      kind: "confirm_deletion_stopped",
    }],
    [scheduledTaskCommandKey("delete", "job-delete"), {
      baseConfigurationVersion: 4,
      jobId: "job-delete",
      kind: "delete",
    }],
    [scheduledTaskCommandKey("delete", "job-delete-stale"), {
      baseConfigurationVersion: 5,
      jobId: "job-delete-stale",
      kind: "delete",
    }],
    [scheduledTaskCommandKey("delete", "job-delete-claimed"), {
      baseConfigurationVersion: 4,
      jobId: "job-delete-claimed",
      kind: "delete",
    }],
    [scheduledTaskCommandKey("delete", "job-delete-review"), {
      baseConfigurationVersion: 4,
      jobId: "job-delete-review",
      kind: "delete",
    }],
    [scheduledTaskCommandKey("delete", "job-delete-same"), {
      baseConfigurationVersion: 4,
      jobId: "job-delete-same",
      kind: "delete",
    }],
    [scheduledTaskCommandKey("permission", decisionTarget), {
      decision: "allow_once",
      jobId: "job-permission",
      kind: "permission_decision",
      originalStatus: "pending",
      policyRevision: 3,
      requestId: "decision-request",
      runId: "run-p",
    }],
    [scheduledTaskCommandKey("permission", resumeTarget), {
      jobId: "job-resume",
      kind: "permission_resume",
      policyRevision: 3,
      requestId: "resume-request",
      runId: "run-resume",
    }],
    [scheduledTaskCommandKey("recover", "job-recover:run-recover"), {
      jobId: "job-recover",
      kind: "recover",
      runId: "run-recover",
    }],
    [scheduledTaskCommandKey("run", "job-run"), {
      baseConfigurationVersion: 4,
      jobId: "job-run",
      kind: "run",
      requestId: "web-run:request-1",
    }],
    [scheduledTaskCommandKey("toggle", "job-toggle"), {
      baseConfigurationVersion: 5,
      expectedEnabled: true,
      jobId: "job-toggle",
      kind: "toggle",
    }],
    [scheduledTaskCommandKey("update", "job-update"), {
      baseConfigurationVersion: 7,
      jobId: "job-update",
      kind: "update",
    }],
  ]);
  const taskRefresh = reconcileScheduledTaskUnconfirmed(uncertain, {
    commands: new Set([
      "confirmDeletionStopped",
      "delete",
      "recover",
      "toggle",
      "update",
    ]),
    expectations,
    items: [
      task("job-toggle", { configuration_version: 5, enabled: false }),
      task("job-update", { configuration_version: 8 }),
      task("job-confirm-stopped", {
        configuration_version: 9,
        deletion_state: "review_required",
        enabled: false,
      }),
      task("job-delete-claimed", {
        configuration_version: 5,
        deletion_state: "deleting",
        enabled: false,
      }),
      task("job-delete-review", {
        configuration_version: 5,
        deletion_state: "review_required",
        enabled: false,
      }),
      task("job-delete-stale", { configuration_version: 4 }),
      task("job-delete-same", { configuration_version: 4 }),
      task("job-recover", { running_run_id: null }),
      task("job-resume", { pending_permission_request_id: null }),
    ],
    permissionRequests: null,
    runs: null,
  });
  assert.deepEqual(
    [...taskRefresh.get("delete")],
    ["job-delete-claimed", "job-delete-review", "job-delete-stale", "job-delete-same"],
  );
  assert.deepEqual(
    [...taskRefresh.get("confirmDeletionStopped")],
    ["job-confirm-stopped"],
  );
  assert.deepEqual([...taskRefresh.get("recover")], ["job-recover:run-recover"]);
  assert.deepEqual([...taskRefresh.get("toggle")], ["job-toggle"]);
  assert.equal(taskRefresh.get("update").size, 0);
  assert.deepEqual([...taskRefresh.get("run")], ["job-run"]);

  const unchangedPermissionRefresh = reconcileScheduledTaskUnconfirmed(taskRefresh, {
    commands: new Set(["permission"]),
    expectations,
    items: null,
    permissionRequests: [decisionRequest],
    runs: null,
  });
  assert.deepEqual(
    [...unchangedPermissionRefresh.get("permission")],
    [decisionTarget, resumeTarget],
  );

  const permissionRefresh = reconcileScheduledTaskUnconfirmed(unchangedPermissionRefresh, {
    commands: new Set(["permission"]),
    expectations,
    items: null,
    permissionRequests: [],
    runs: null,
  });
  assert.deepEqual([...permissionRefresh.get("permission")], [resumeTarget]);

  const historyRefresh = reconcileScheduledTaskUnconfirmed(permissionRefresh, {
    commands: new Set(["recover", "run"]),
    expectations,
    items: [
      task("job-recover", { running_run_id: null }),
      task("job-resume", { pending_permission_request_id: null }),
    ],
    permissionRequests: [],
    runs: [
      run("job-recover", "run-recover", "cancelled"),
      run("job-resume", "run-resume", "succeeded"),
      {
        ...run("job-run", "run-idempotent", "running"),
        client_request_id: "web-run:request-1",
      },
    ],
  });
  // 运行历史不能拿可能陈旧的权限请求快照替 permission API 作证。
  // permission_resume 保持保护，待用户核对同一次运行后显式解除。
  assert.deepEqual([...historyRefresh.get("permission")], [resumeTarget]);
  assert.equal(historyRefresh.get("recover").size, 0);
  assert.deepEqual(
    [...historyRefresh.get("retryDelivery")],
    ["job-delivery:run-1", "job-other:run-2"],
  );
  assert.equal(historyRefresh.get("run").size, 0);
  assert.deepEqual(
    [...historyRefresh.get("delete")],
    ["job-delete-claimed", "job-delete-review", "job-delete-stale", "job-delete-same"],
  );

  const deliveryAllowed = allowScheduledTaskRepeat(
    historyRefresh,
    "retryDelivery",
    "job-delivery:run-1",
  );
  assert.deepEqual([...deliveryAllowed.get("retryDelivery")], ["job-other:run-2"]);
  assert.equal(deliveryAllowed.get("run").size, 0);
  assert.deepEqual(
    [...deliveryAllowed.get("delete")],
    ["job-delete-claimed", "job-delete-review", "job-delete-stale", "job-delete-same"],
  );

  const deleteAllowed = allowScheduledTaskRepeat(
    deliveryAllowed,
    "delete",
    "job-delete-same",
  );
  assert.deepEqual(
    [...deleteAllowed.get("delete")],
    ["job-delete-claimed", "job-delete-review", "job-delete-stale"],
  );
  const confirmedTaskDisappeared = reconcileScheduledTaskUnconfirmed(deleteAllowed, {
    commands: new Set(["confirmDeletionStopped"]),
    expectations,
    items: [],
    permissionRequests: null,
    runs: null,
  });
  assert.equal(confirmedTaskDisappeared.get("confirmDeletionStopped").size, 0);
});

test("Scheduled durable 删除态只读，并区分自动收尾和管理员处理", async () => {
  const [
    boardModel,
    historyModel,
    card,
    attention,
    directory,
    commands,
    historyDialog,
    historyActions,
    realtimeRefresh,
    taskTypes,
    zhCatalog,
    enCatalog,
  ] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/scheduled/board/scheduled-task-board-model.ts",
    ),
    server.ssrLoadModule(
      "/src/features/capability/scheduled/history/scheduled-task-run-history-model.ts",
    ),
    read("src/features/capability/scheduled/board/scheduled-task-card.tsx"),
    read("src/features/capability/scheduled/board/scheduled-task-attention-dialog.tsx"),
    read("src/features/capability/scheduled/scheduled-tasks-directory.tsx"),
    read("src/features/capability/scheduled/controller/use-scheduled-task-commands.ts"),
    read("src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx"),
    read("src/features/capability/scheduled/history/use-scheduled-task-run-history-actions.ts"),
    read("src/features/capability/scheduled/use-scheduled-task-realtime-refresh.ts"),
    read("src/types/capability/scheduled-task/task.ts"),
    read("src/shared/i18n/catalog/zh/capability.ts"),
    read("src/shared/i18n/catalog/en/capability.ts"),
  ]);
  const deletingTask = completeTask("job-deleting", {
    configuration_version: 9,
    deletion_state: "deleting",
    enabled: false,
    failure_streak: 2,
    last_error: "stale permission detail",
    permission_state: "awaiting_approval",
    session_binding_state: "rebind_required",
  });
  const columns = boardModel.buildScheduledTaskBoard([deletingTask]);
  assert.deepEqual(
    columns.find((column) => column.id === "attention").items.map((item) => item.job_id),
    ["job-deleting"],
  );
  assert.equal(boardModel.isScheduledTaskDeleting({
    ...deletingTask,
    deletion_state: "future_cleanup_stage",
  }), true);
  const presentation = boardModel.getScheduledTaskCardPresentation(
    deletingTask,
    {
      isDeleting: false,
      isPermissionPending: false,
      isRunning: false,
      isToggling: false,
    },
  );
  assert.equal(presentation.deletion?.title, "任务正在删除");
  assert.match(presentation.deletion?.impact ?? "", /外部操作/);
  assert.equal(presentation.binding, null);
  assert.equal(presentation.permission, null);
  assert.equal(presentation.lastError, null);
  assert.equal(presentation.deleteDisabled, true);
  assert.equal(presentation.historyDisabled, false);
  assert.equal(presentation.runAction.disabled, true);
  assert.equal(presentation.toggleAction.disabled, true);
  assert.equal(historyModel.getTaskStatusMeta(deletingTask).label, "删除中");

  const reviewRequiredTask = completeTask("job-review-required", {
    configuration_version: 10,
    deletion_state: "review_required",
    enabled: false,
  });
  const reviewPresentation = boardModel.getScheduledTaskCardPresentation(
    reviewRequiredTask,
    {
      isDeleting: false,
      isPermissionPending: false,
      isRunning: false,
      isToggling: false,
    },
  );
  assert.equal(reviewPresentation.deletion?.title, "删除需要管理员处理");
  assert.match(reviewPresentation.deletion?.description ?? "", /无法确认.*原执行.*停止/);
  assert.match(reviewPresentation.deletion?.impact ?? "", /任务配置和运行记录仍然保留/);
  assert.match(reviewPresentation.deletion?.nextStep ?? "", /确认已停止，继续删除/);
  assert.equal(reviewPresentation.historyDisabled, false);
  assert.equal(reviewPresentation.runAction.disabled, true);
  assert.equal(reviewPresentation.toggleAction.disabled, true);
  assert.equal(historyModel.getTaskStatusMeta(reviewRequiredTask).label, "删除待处理");

  const runActions = historyModel.getRunActionPresentations({
    isRecoveryUnconfirmed: false,
    isRecovering: false,
    isRetryDeliveryUnconfirmed: false,
    isRetryUnconfirmed: false,
    isRetrying: false,
    isRetryingDelivery: false,
    run: {
      ...run("job-deleting", "run-deleting", "failed"),
      delivery_status: "failed",
    },
    task: deletingTask,
  });
  assert.ok(runActions.length > 0);
  assert.ok(runActions.every((action) => action.disabled));
  assert.ok(runActions.every((action) => action.title.includes("删除已受理")));

  assert.match(taskTypes, /deletion_state\?: ScheduledTaskDeletionState \| string/);
  assert.match(taskTypes, /ScheduledTaskDeletionState = "deleting" \| "review_required"/);
  assert.match(card, /presentation\.deletion/);
  assert.match(attention, /对已有内容的影响/);
  assert.match(attention, /现在可以做什么/);
  assert.match(attention, /deletionNeedsReview \? \(/);
  assert.match(attention, /确认已停止，继续删除/);
  assert.match(attention, /onConfirmDeletionStopped\(task\)/);
  assert.match(directory, /confirmText="确认已停止并删除"/);
  assert.match(directory, /系统尚未删除这个任务和运行历史/);
  assert.match(directory, /无法撤回任务此前已经产生的外部影响/);
  assert.match(directory, /commands\.confirmDeletionStopped\(task\)/);
  assert.match(commands, /confirmScheduledTaskDeletionStoppedApi/);
  assert.match(commands, /kind: "confirm_deletion_stopped"/);
  assert.match(commands, /ignoreUnconfirmedCommands: ORIGINAL_DELETE_REVIEW_LOCK/);
  assert.match(
    commands,
    /for \(const deleteTargetId of unconfirmedRef\.current\.get\("delete"\)[\s\S]*removeScheduledTaskMutationJournalEntry\([\s\S]*"delete",[\s\S]*deleteTargetId/,
  );
  assert.match(
    commands,
    /task\.configuration_version,\s*run\.delivery_attempts \?\? undefined,\s*options\?\.confirmUnverifiedAttempt === true/,
  );
  assert.doesNotMatch(`${card}\n${attention}\n${directory}`, /deletion_token|deletion_claimed_at/);
  assert.doesNotMatch(attention, /request\.request_id|request\.run_id/);
  assert.match(directory, /taskAcceptsMutations/);
  assert.match(directory, /isScheduledTaskDeleting\(currentTask\)/);
  assert.match(directory, /getTaskMutationBlockReason/);
  assert.match(directory, /blockedHistoryAction/);
  assert.ok(
    (directory.match(/if \(!taskAcceptsMutations\(task\)\) return;/g) ?? []).length >= 5,
  );
  assert.match(commands, /scheduled_delete_finishing_title/);
  assert.match(commands, /scheduled_delete_review_required_title/);
  assert.match(zhCatalog, /scheduled_delete_review_required_next_step/);
  assert.match(enCatalog, /scheduled_delete_review_required_next_step/);
  assert.match(historyDialog, /accessBlocked \|\| deletionBlocked/);
  assert.match(historyDialog, /!accessBlocked && !deletionBlocked/);
  assert.match(historyActions, /const result = await execute\(activeTask\)/);
  assert.match(historyActions, /result\.status === "blocked"/);
  assert.doesNotMatch(historyActions, /error instanceof Error \? error\.message/);
  assert.match(historyDialog, /actions\.feedback\.impact/);
  assert.match(historyDialog, /actions\.feedback\.nextStep/);
  assert.match(directory, /status: "blocked"/);
  assert.match(directory, /status: "completed"/);
  assert.match(realtimeRefresh, /trailingRefreshRef/);
  assert.match(realtimeRefresh, /REFRESH_COALESCE_MS/);
  assert.match(realtimeRefresh, /refreshRunningRef/);
  assert.equal((realtimeRefresh.match(/refreshTasksRef\.current\(\{ silent: true \}\)/g) ?? []).length, 1);
});

test("Scheduled 未确认投递只在有 exact attempts 时提供显式核对入口", async () => {
  const { getDeliveryStatusMeta, getRunActionPresentations } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/history/scheduled-task-run-history-model.ts",
  );
  assert.equal(getDeliveryStatusMeta("retrying")?.label, "投递结果待确认");
  const actionsWithoutAttemptEvidence = getRunActionPresentations({
    isRecoveryUnconfirmed: false,
    isRecovering: false,
    isRetryDeliveryUnconfirmed: false,
    isRetryUnconfirmed: false,
    isRetrying: false,
    isRetryingDelivery: false,
    run: {
      ...run("job-delivery", "run-delivery", "succeeded"),
      delivery_status: "retrying",
    },
    task: task("job-delivery"),
  });
  const actionWithoutAttemptEvidence = actionsWithoutAttemptEvidence.find(
    (action) => action.kind === "retry_delivery",
  );
  assert.equal(actionWithoutAttemptEvidence?.disabled, true);
  assert.equal(actionWithoutAttemptEvidence?.label, "请刷新后核对");

  const actionsWithExactAttempt = getRunActionPresentations({
    isRecoveryUnconfirmed: false,
    isRecovering: false,
    isRetryDeliveryUnconfirmed: false,
    isRetryUnconfirmed: false,
    isRetrying: false,
    isRetryingDelivery: false,
    run: {
      ...run("job-delivery", "run-delivery", "succeeded"),
      delivery_attempts: 2,
      delivery_status: "retrying",
    },
    task: task("job-delivery"),
  });
  const actionWithExactAttempt = actionsWithExactAttempt.find(
    (action) => action.kind === "retry_delivery",
  );
  assert.equal(actionWithExactAttempt?.disabled, false);
  assert.equal(actionWithExactAttempt?.label, "我已核对，重新投递");

  const reviewRequiredActions = getRunActionPresentations({
    isRecoveryUnconfirmed: false,
    isRecovering: false,
    isRetryDeliveryUnconfirmed: false,
    isRetryUnconfirmed: false,
    isRetrying: false,
    isRetryingDelivery: false,
    run: {
      ...run("job-delivery", "run-delivery", "succeeded"),
      delivery_attempts: 2,
      delivery_status: "retrying",
    },
    task: task("job-delivery", { deletion_state: "review_required" }),
  });
  const reviewRequiredAction = reviewRequiredActions.find(
    (action) => action.kind === "retry_delivery",
  );
  assert.equal(reviewRequiredAction?.disabled, true);
  assert.equal(reviewRequiredAction?.label, "删除待处理");
});

test("Scheduled persistent journal 按 owner 隔离并跨 App 重启恢复 exact pending identity", async () => {
  const fakeStorage = createMemoryStorage();
  globalThis.window = { localStorage: fakeStorage };
  const journal = await server.ssrLoadModule(
    "/src/features/capability/scheduled/controller/scheduled-task-mutation-journal.ts",
  );
  if (!globalThis.navigator?.locks) {
    let executedWithoutCoordination = false;
    await assert.rejects(
      journal.withScheduledTaskMutationLock("owner:a", "job-a", async () => {
        executedWithoutCoordination = true;
      }),
      (error) => error?.name === "ScheduledTaskMutationCoordinationUnavailableError",
    );
    assert.equal(executedWithoutCoordination, false);
  }
  const navigatorDescriptor = Object.getOwnPropertyDescriptor(
    globalThis,
    "navigator",
  );
  let activeLock = false;
  let releaseFirstLock;
  let firstLockStarted;
  const firstStarted = new Promise((resolve) => {
    firstLockStarted = resolve;
  });
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: {
      locks: {
        request: async (name, options, callback) => {
          assert.equal(name, "nexus:scheduled-task:owner%3Aa:job-a");
          assert.equal(options.ifAvailable, true);
          if (activeLock) {
            return callback(null);
          }
          activeLock = true;
          try {
            return await callback({ mode: "exclusive", name });
          } finally {
            activeLock = false;
          }
        },
      },
    },
  });
  let executeCount = 0;
  const firstLock = journal.withScheduledTaskMutationGate(
    "owner:a",
    "job-a",
    async () => {
      executeCount += 1;
      firstLockStarted();
      await new Promise((resolve) => {
        releaseFirstLock = resolve;
      });
    },
  );
  await firstStarted;
  await assert.rejects(
    journal.withScheduledTaskMutationGate("owner:a", "job-a", async () => {
      executeCount += 1;
    }),
    (error) => error?.name === "ScheduledTaskMutationLockUnavailableError",
  );
  releaseFirstLock();
  await firstLock;
  assert.equal(executeCount, 1);
  if (navigatorDescriptor) {
    Object.defineProperty(globalThis, "navigator", navigatorDescriptor);
  } else {
    delete globalThis.navigator;
  }

  journal.upsertScheduledTaskMutationJournalEntry("owner:a", {
    command: "run",
    phase: "pending",
    targetId: "job-a",
    updatedAt: 1,
  });
  journal.upsertScheduledTaskMutationJournalEntry("owner:b", {
    command: "retryDelivery",
    phase: "unconfirmed",
    targetId: "job-b:run-b",
    updatedAt: 2,
  });
  journal.saveScheduledTaskCreateRequestId("owner:a", "web-create:intent-a");
  journal.saveScheduledTaskCreateRequestId("owner:a", "web-create:intent-a2");
  for (let index = 0; index < 120; index += 1) {
    journal.upsertScheduledTaskMutationJournalEntry("owner:c", {
      command: "run",
      phase: "unconfirmed",
      targetId: `job-${index}`,
      updatedAt: index,
    });
  }

  assert.deepEqual(
    journal.loadScheduledTaskMutationJournal("owner:a").map((entry) => (
      [entry.command, entry.targetId, entry.phase]
    )),
    [["run", "job-a", "pending"]],
  );
  assert.deepEqual(
    journal.loadScheduledTaskMutationJournal("owner:b").map((entry) => entry.targetId),
    ["job-b:run-b"],
  );
  assert.equal(
    journal.loadScheduledTaskCreateRequestId("owner:a"),
    "web-create:intent-a",
  );
  assert.deepEqual(
    journal.loadScheduledTaskCreateRequestIds("owner:a"),
    ["web-create:intent-a", "web-create:intent-a2"],
  );
  journal.clearScheduledTaskCreateRequestId("owner:a", "web-create:intent-a");
  assert.deepEqual(
    journal.loadScheduledTaskCreateRequestIds("owner:a"),
    ["web-create:intent-a2"],
  );
  assert.equal(journal.loadScheduledTaskMutationJournal("owner:c").length, 120);
  journal.clearScheduledTaskMutationJournal("owner:a");
  assert.equal(journal.loadScheduledTaskMutationJournal("owner:a").length, 0);
  assert.equal(journal.loadScheduledTaskMutationJournal("owner:b").length, 1);
  globalThis.window = {
    localStorage: {
      getItem: () => null,
      removeItem: () => undefined,
      setItem: () => {
        throw new Error("quota exceeded");
      },
    },
  };
  assert.equal(journal.upsertScheduledTaskMutationJournalEntry("owner:d", {
    command: "delete",
    phase: "pending",
    targetId: "job-d",
    updatedAt: 3,
  }), false);
  assert.equal(
    journal.saveScheduledTaskCreateRequestId("owner:d", "web-create:intent-d"),
    false,
  );
  delete globalThis.window;
});

function failureCore(effect) {
  return {
    category: "conflict",
    code: "automation.test_failure",
    effect,
    version: 1,
  };
}

function commandState(values = {}) {
  return new Map([
    ["confirmDeletionStopped", new Set(values.confirmDeletionStopped ?? [])],
    ["delete", new Set(values.delete ?? [])],
    ["permission", new Set(values.permission ?? [])],
    ["recover", new Set(values.recover ?? [])],
    ["retryDelivery", new Set(values.retryDelivery ?? [])],
    ["run", new Set(values.run ?? [])],
    ["toggle", new Set(values.toggle ?? [])],
    ["update", new Set(values.update ?? [])],
  ]);
}

function task(jobId, overrides = {}) {
  return {
    configuration_version: 1,
    enabled: true,
    job_id: jobId,
    pending_permission_request_id: null,
    running_run_id: null,
    ...overrides,
  };
}

function completeTask(jobId, overrides = {}) {
  return {
    ...task(jobId),
    agent_id: "agent-main",
    delivery: { mode: "none" },
    execution_kind: "agent",
    expires_at: null,
    failure_streak: 0,
    instruction: "Summarize updates",
    last_error: null,
    last_run_at: null,
    last_run_status: null,
    name: "Daily summary",
    next_run_at: null,
    running: false,
    running_started_at: null,
    schedule: { interval_seconds: 3600, kind: "every" },
    session_binding_state: "ready",
    session_target: { kind: "isolated" },
    source: { kind: "user_page" },
    ...overrides,
  };
}

function run(jobId, runId, status) {
  return {
    block_state: null,
    blocked_request_id: null,
    job_id: jobId,
    run_id: runId,
    status,
  };
}

function permissionRequest(requestId, jobId, runId) {
  return {
    decision: null,
    job_id: jobId,
    policy_revision: 3,
    request_id: requestId,
    run_id: runId,
    status: "pending",
  };
}

function createMemoryStorage() {
  const values = new Map();
  return {
    get length() {
      return values.size;
    },
    getItem: (key) => values.get(key) ?? null,
    key: (index) => [...values.keys()][index] ?? null,
    removeItem: (key) => values.delete(key),
    setItem: (key, value) => values.set(key, String(value)),
  };
}

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
