// INPUT: Scheduled exact command identity、durable task/run/permission 快照与未确认操作。
// OUTPUT: 按 exact job/run/request 对账的操作锁；删除收尾/人工处理任务在真正消失前保持删除保护。
// POS: Scheduled 目录的纯状态机；不发请求、不以显示文案或 HTTP trace ID 推测结果。

import {
  setPendingCommand,
  type PendingCommandState,
} from "./pending-command-model";
import type { AutomationPermissionRequest } from "@/types/capability/scheduled-task/permission";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

export const SCHEDULED_TASK_COMMAND_KINDS = [
  "confirmDeletionStopped",
  "delete",
  "permission",
  "recover",
  "retryDelivery",
  "run",
  "toggle",
  "update",
] as const;
export type ScheduledTaskCommandKind = typeof SCHEDULED_TASK_COMMAND_KINDS[number];

export interface ScheduledTaskFeedback {
  action?: {
    label: string;
    onClick: () => void;
  };
  impact?: string;
  message: string;
  nextStep?: string;
  title: string;
  tone: "success" | "warning" | "error";
}

export type ScheduledTaskPendingCommands = PendingCommandState<ScheduledTaskCommandKind>;
export type ScheduledTaskUnconfirmedCommands = PendingCommandState<ScheduledTaskCommandKind>;

export function scheduledTaskCommandTarget(jobId: string, runId?: string): string {
  return runId ? `${jobId}:${runId}` : jobId;
}

export function scheduledTaskConfigurationCommandTarget(
  jobId: string,
  configurationVersion: number,
): string {
  return `${jobId}:configuration:${configurationVersion}`;
}

export function scheduledTaskRunCommandTarget(
  jobId: string,
  requestId: string,
): string {
  return `${jobId}:run-request:${encodeURIComponent(requestId)}`;
}

export function scheduledTaskDeliveryCommandTarget(
  jobId: string,
  runId: string,
  deliveryAttempts: number | null | undefined,
): string {
  return [
    jobId,
    "delivery",
    encodeURIComponent(runId),
    String(deliveryAttempts ?? 0),
  ].join(":");
}

export function scheduledTaskPermissionCommandTarget(
  jobId: string,
  request: Pick<
    AutomationPermissionRequest,
    "policy_revision" | "request_id" | "run_id"
  >,
): string {
  return [
    jobId,
    "permission",
    encodeURIComponent(request.request_id),
    encodeURIComponent(request.run_id ?? ""),
    String(request.policy_revision),
  ].join(":");
}

export function scheduledTaskCommandKey(
  command: ScheduledTaskCommandKind,
  targetId: string,
): string {
  return `${command}:${targetId}`;
}

export function scheduledTaskCommandTargetsJob(targetId: string, jobId: string): boolean {
  return targetId === jobId || targetId.startsWith(`${jobId}:`);
}

export function hasScheduledTaskCommandForJob(
  state: PendingCommandState<ScheduledTaskCommandKind>,
  command: ScheduledTaskCommandKind,
  jobId: string,
): boolean {
  return [...(state.get(command) ?? [])].some((targetId) => (
    scheduledTaskCommandTargetsJob(targetId, jobId)
  ));
}

export function isScheduledTaskMutationBlocked(
  pending: ScheduledTaskPendingCommands,
  unconfirmed: ScheduledTaskUnconfirmedCommands,
  jobId: string,
): boolean {
  return SCHEDULED_TASK_COMMAND_KINDS.some((command) => (
    [pending, unconfirmed].some((state) => (
      hasScheduledTaskCommandForJob(state, command, jobId)
    ))
  ));
}

export type ScheduledTaskReconcileExpectation =
  | {
      baseConfigurationVersion: number;
      jobId: string;
      kind: "confirm_deletion_stopped";
    }
  | {
      baseConfigurationVersion: number;
      jobId: string;
      kind: "delete";
    }
  | {
      baseConfigurationVersion: number;
      expectedEnabled: boolean;
      jobId: string;
      kind: "toggle";
    }
  | {
      baseConfigurationVersion: number;
      jobId: string;
      kind: "update";
    }
  | {
      baseConfigurationVersion: number;
      jobId: string;
      kind: "run";
      requestId: string;
    }
  | {
      decision: AutomationPermissionRequest["decision"];
      jobId: string;
      kind: "permission_decision";
      originalStatus: AutomationPermissionRequest["status"];
      policyRevision: number;
      requestId: string;
      runId: string | null;
    }
  | {
      jobId: string;
      kind: "permission_resume";
      policyRevision: number;
      requestId: string;
      runId: string;
    }
  | { jobId: string; kind: "recover"; runId: string };

export function reconcileScheduledTaskUnconfirmed(
  current: ScheduledTaskUnconfirmedCommands,
  snapshot: {
    commands?: ReadonlySet<
      | "confirmDeletionStopped"
      | "delete"
      | "permission"
      | "recover"
      | "run"
      | "toggle"
      | "update"
    >;
    expectations: ReadonlyMap<string, ScheduledTaskReconcileExpectation>;
    items: ScheduledTaskItem[] | null;
    permissionRequests: AutomationPermissionRequest[] | null;
    runs: ScheduledTaskRunItem[] | null;
  },
): ScheduledTaskUnconfirmedCommands {
  const next = new Map(current);
  const itemsById = snapshot.items
    ? new Map(snapshot.items.map((item) => [item.job_id, item]))
    : null;
  const permissionRequestsById = snapshot.permissionRequests
    ? new Map(snapshot.permissionRequests.map((request) => [request.request_id, request]))
    : null;
  const runsById = snapshot.runs
    ? new Map(snapshot.runs.map((run) => [run.run_id, run]))
    : null;
  const runsByClientRequestId = snapshot.runs
    ? new Map(snapshot.runs.flatMap((run) => (
        run.client_request_id?.trim()
          ? [[run.client_request_id.trim(), run] as const]
          : []
      )))
    : null;
  ([
    "confirmDeletionStopped",
    "delete",
    "permission",
    "recover",
    "run",
    "toggle",
    "update",
  ] as const).forEach((command) => {
    if (snapshot.commands && !snapshot.commands.has(command)) {
      return;
    }
    next.set(command, new Set(
      [...(current.get(command) ?? [])].filter((targetId) => {
        const expectation = snapshot.expectations.get(
          scheduledTaskCommandKey(command, targetId),
        );
        if (command === "delete" || command === "confirmDeletionStopped") {
          const expectedKind = command === "delete"
            ? "delete"
            : "confirm_deletion_stopped";
          if (expectation?.kind !== expectedKind || itemsById === null) {
            return true;
          }
          const task = itemsById.get(expectation.jobId);
          // 任一 deletion_state 都是服务端 durable 删除阶段的权威事实：
          // 版本已推进，但任务仍在收尾或等待人工处理，不得解锁同一删除或其他修改。
          // 只有任务定义消失才证明删除完成；无删除态的更高版本
          // 仍可证明旧 CAS 已失效。同版本继续保护在途请求。
          return Boolean(
            task
            && (
              Boolean(task.deletion_state?.trim())
              || task.configuration_version <= expectation.baseConfigurationVersion
            ),
          );
        }
        if (command === "permission") {
          if (expectation?.kind === "permission_decision") {
            if (permissionRequestsById === null) {
              return true;
            }
            const request = permissionRequestsById.get(expectation.requestId);
            if (!request) {
              return false;
            }
            if (
              request.job_id !== expectation.jobId
              || request.policy_revision !== expectation.policyRevision
              || (request.run_id ?? null) !== expectation.runId
            ) {
              return true;
            }
            // 同一请求仍是原始状态时，在途决策仍可能随后提交；
            // 只有请求已转入其他状态或不再 actionable 才能解除旧 intent。
            return request.status === expectation.originalStatus;
          }
          if (expectation?.kind === "permission_resume") {
            if (
              permissionRequestsById === null
              || itemsById === null
              || runsById === null
            ) {
              return true;
            }
            const task = itemsById.get(expectation.jobId);
            const run = runsById.get(expectation.runId);
            const requestStillActionable = permissionRequestsById.has(
              expectation.requestId,
            );
            const taskStillBlocked = task?.pending_permission_request_id
              === expectation.requestId;
            const runStillBlocked = !run
              || run.block_state === "ready_to_retry"
              || run.blocked_request_id === expectation.requestId;
            return requestStillActionable || taskStillBlocked || runStillBlocked;
          }
          return true;
        }
        if (command === "recover") {
          const task = expectation?.kind === "recover"
            ? itemsById?.get(expectation.jobId)
            : null;
          const run = expectation?.kind === "recover"
            ? runsById?.get(expectation.runId)
            : null;
          return expectation?.kind !== "recover"
            || itemsById === null
            || runsById === null
            || !run
            || !["cancelled", "failed", "skipped", "succeeded"].includes(run.status)
            || Boolean(task?.running_run_id === expectation.runId);
        }
        if (command === "run") {
          const run = expectation?.kind === "run"
            ? runsByClientRequestId?.get(expectation.requestId)
            : null;
          return expectation?.kind !== "run"
            || runsByClientRequestId === null
            || !run
            || run.job_id !== expectation.jobId;
        }
        if (command === "update") {
          const task = expectation?.kind === "update"
            ? itemsById?.get(expectation.jobId)
            : null;
          return expectation?.kind !== "update"
            || itemsById === null
            || !task
            || task.configuration_version <= expectation.baseConfigurationVersion;
        }
        const task = expectation?.kind === "toggle"
          ? itemsById?.get(expectation.jobId)
          : null;
        return expectation?.kind !== "toggle"
          || itemsById === null
          || !task
          || task.configuration_version <= expectation.baseConfigurationVersion;
      }),
    ));
  });
  return next;
}

export function allowScheduledTaskRepeat(
  current: ScheduledTaskUnconfirmedCommands,
  command: ScheduledTaskCommandKind,
  targetId: string,
): ScheduledTaskUnconfirmedCommands {
  return setPendingCommand(current, command, targetId, false);
}
