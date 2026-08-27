/**
 * INPUT: 定时任务 durable 删除、绑定、权限或运行注意事项与后续动作。
 * OUTPUT: 以任务名为标题的处理面；人工复核只提供刷新、历史和显式停止确认入口。
 * POS: Scheduled 看板的注意事项处理边界；不暴露内部删除 token 或误复用旧权限语义。
 */
"use client";

import { History, RefreshCw, Trash2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { AutomationPermissionDecision } from "@/types/capability/scheduled-task/permission";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { getScheduledTaskErrorCopy } from "../scheduled-task-error-copy";
import {
  getScheduledPermissionCapabilityLabel,
  getScheduledPermissionResourceSummary,
  hasScheduledTaskPermissionActions,
  hasScheduledTaskPermissionAttention,
} from "./scheduled-task-attention-model";
import { ScheduledTaskPermissionActions } from "./scheduled-task-permission-actions";

interface ScheduledTaskAttentionDialogProps {
  deletionImpact: string | null;
  deletionNextStep: string | null;
  description: string | null;
  isBindingAttention: boolean;
  isDeletionAttention: boolean;
  isDeletionReviewPending: boolean;
  isOpen: boolean;
  isPending: boolean;
  onClose: () => void;
  onConfirmDeletionStopped: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenConnector: (connectorId: string) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onPermissionDecision: (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ) => void;
  onPermissionResume: (task: ScheduledTaskItem) => void;
  onRefresh: () => void;
  task: ScheduledTaskItem;
  title: string | null;
}

function isWebResource(value: string): boolean {
  return value.startsWith("https://") || value.startsWith("http://");
}

export function ScheduledTaskAttentionDialog({
  deletionImpact,
  deletionNextStep,
  description,
  isBindingAttention,
  isDeletionAttention,
  isDeletionReviewPending,
  isOpen,
  isPending,
  onClose,
  onConfirmDeletionStopped,
  onEdit,
  onOpenConnector,
  onOpenHistory,
  onPermissionDecision,
  onPermissionResume,
  onRefresh,
  task,
  title,
}: ScheduledTaskAttentionDialogProps) {
  if (!isOpen) {
    return null;
  }
  const request = task.pending_permission_request;
  const capabilityLabel = request
    ? getScheduledPermissionCapabilityLabel(request)
    : null;
  const resourceSummary = request
    ? getScheduledPermissionResourceSummary(request)
    : null;
  const errorCopy = getScheduledTaskErrorCopy(task.last_error);
  const errorEchoesPermission = Boolean(
    request?.capability.tool_name
      && task.last_error?.includes(request.capability.tool_name),
  );
  const hasPermissionAttention = hasScheduledTaskPermissionAttention(task);
  const requestStatusLabel = request?.status === "approved"
    ? "已批准，待重试"
    : "等待处理";
  const deletionNeedsReview = task.deletion_state?.trim() === "review_required";
  const titleId = `scheduled-task-attention-${task.job_id}`;

  const closeThen = (action: () => void) => {
    onClose();
    action();
  };
  const hasPermissionActions = hasScheduledTaskPermissionActions(task);

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        closeOnBackdrop={false}
        labelledBy={titleId}
        onClose={onClose}
      >
        <UiDialogShell className="max-h-[86vh]" size="lg">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={task.name}
            titleId={titleId}
          />

          <UiDialogBody className="min-h-0 flex-1 space-y-5" scrollable>
            {isDeletionAttention ? (
              <section aria-labelledby={`${titleId}-deletion`}>
                <h3
                  className="text-sm font-semibold text-(--text-strong)"
                  id={`${titleId}-deletion`}
                >
                  {title || (deletionNeedsReview ? "删除需要管理员处理" : "任务正在删除")}
                </h3>
                <p className="mt-2 text-sm leading-6 text-(--text-default)">
                  {description || (deletionNeedsReview
                    ? "系统无法确认删除前的原执行是否已经停止，因此任务数据尚未删除。"
                    : "删除请求已受理，系统正在停止任务并完成收尾。")}
                </p>
                <div className="mt-4 space-y-4 rounded-[8px] border border-(--divider-subtle-color) px-3 py-3">
                  <div>
                    <h4 className="text-xs font-semibold text-(--text-strong)">
                      对已有内容的影响
                    </h4>
                    <p className="mt-1 text-xs leading-5 text-(--text-default)">
                      {deletionImpact || (deletionNeedsReview
                        ? "任务配置和运行记录仍然保留；继续确认后会删除任务和历史，但此前已经发生的外部影响无法撤回。"
                        : "已保存的运行记录不会被重写；已经发生的外部操作不会被撤销或自动重做。")}
                    </p>
                  </div>
                  <div>
                    <h4 className="text-xs font-semibold text-(--text-strong)">
                      现在可以做什么
                    </h4>
                    <p className="mt-1 text-xs leading-5 text-(--text-default)">
                      {deletionNextStep || (deletionNeedsReview
                        ? "请先确认原执行端已经停止，再使用下方确认操作完成删除；也可以先刷新或查看运行历史。"
                        : "无需再次删除。请等待片刻后刷新，或先查看运行历史。")}
                    </p>
                  </div>
                </div>
              </section>
            ) : isBindingAttention ? (
              <section aria-labelledby={`${titleId}-binding`}>
                <h3
                  className="text-sm font-semibold text-(--text-strong)"
                  id={`${titleId}-binding`}
                >
                  {title || "重新绑定会话"}
                </h3>
                <p className="mt-2 text-sm leading-6 text-(--text-default)">
                  {description}
                </p>
              </section>
            ) : request ? (
              <section aria-labelledby={`${titleId}-request`}>
                <div className="flex items-center justify-between gap-3">
                  <h3
                    className="text-sm font-semibold text-(--text-strong)"
                    id={`${titleId}-request`}
                  >
                    {title || capabilityLabel || "权限请求"}
                  </h3>
                  <span className="rounded-full border border-[color:color-mix(in_srgb,var(--warning)_28%,var(--divider-subtle-color))] px-2 py-0.5 text-[11px] font-medium text-(--warning)">
                    {requestStatusLabel}
                  </span>
                </div>
                <p className="mt-2 text-sm leading-6 text-(--text-default)">
                  {description || request.description || request.reason}
                </p>

                <dl className="mt-4 overflow-hidden rounded-[8px] border border-(--divider-subtle-color)">
                  <div className="grid grid-cols-[88px_minmax(0,1fr)] border-b border-(--divider-subtle-color) px-3 py-2.5 text-xs last:border-b-0">
                    <dt className="text-(--text-muted)">能力</dt>
                    <dd className="min-w-0 font-medium text-(--text-strong)">
                      {capabilityLabel}
                    </dd>
                  </div>
                  {resourceSummary ? (
                    <div className="grid grid-cols-[88px_minmax(0,1fr)] border-b border-(--divider-subtle-color) px-3 py-2.5 text-xs last:border-b-0">
                      <dt className="text-(--text-muted)">目标</dt>
                      <dd className="min-w-0 break-all text-(--text-default)">
                        {isWebResource(resourceSummary) ? (
                          <a
                            className="underline decoration-(--divider-color) underline-offset-2 hover:text-(--primary)"
                            href={resourceSummary}
                            rel="noreferrer"
                            target="_blank"
                          >
                            {resourceSummary}
                          </a>
                        ) : resourceSummary}
                      </dd>
                    </div>
                  ) : null}
                  <div className="grid grid-cols-[88px_minmax(0,1fr)] px-3 py-2.5 text-xs">
                    <dt className="text-(--text-muted)">批准后</dt>
                    <dd className="text-(--text-default)">
                      {request.status === "approved"
                        ? "审批已完成；确认后会重试同一次运行"
                        : request.resume_safe
                        ? "结束当前尝试，并自动继续同一次运行"
                        : "等待你再次确认后才会重试，避免重复副作用"}
                    </dd>
                  </div>
                </dl>

                <div className="mt-3 rounded-[8px] border border-(--divider-subtle-color) px-3 py-2.5 text-xs leading-5 text-(--text-muted)">
                  <p className="font-medium text-(--text-default)">这项选择只处理当前任务正在等待的权限。</p>
                  <p className="mt-1">
                    页面会在提交后重新读取任务状态；如果结果无法确认，会先停止同类操作并提示你核对，不会自动重复执行。
                  </p>
                </div>
              </section>
            ) : hasPermissionAttention ? (
              <section aria-labelledby={`${titleId}-state`}>
                <h3
                  className="text-sm font-semibold text-(--text-strong)"
                  id={`${titleId}-state`}
                >
                  {title || "任务需要处理"}
                </h3>
                <p className="mt-2 text-sm leading-6 text-(--text-default)">
                  {description || "任务权限状态已经变化，请刷新后再执行下一步操作。"}
                </p>
              </section>
            ) : null}

            {errorCopy && !errorEchoesPermission && !isDeletionAttention ? (
              <section
                aria-labelledby={`${titleId}-diagnostic`}
                className="border-t border-(--divider-subtle-color) pt-4"
              >
                <h3
                  className="text-xs font-semibold text-(--destructive)"
                  id={`${titleId}-diagnostic`}
                >
                  {hasPermissionAttention ? "附带运行诊断" : "最近运行诊断"}
                </h3>
                <p className="mt-2 whitespace-pre-wrap break-words text-xs leading-5 text-(--text-default)">
                  {errorCopy.detail}
                </p>
              </section>
            ) : null}
          </UiDialogBody>

          <UiDialogFooter appearance="plain">
            <div className="flex w-full flex-wrap items-center justify-between gap-2">
              <UiButton
                onClick={() => closeThen(() => onOpenHistory(task))}
                size="sm"
                variant="text"
              >
                <History className="h-3.5 w-3.5" />
                查看运行历史
              </UiButton>
              {isDeletionAttention ? (
                <div className="flex flex-wrap items-center justify-end gap-2">
                  <UiButton
                    onClick={() => closeThen(onRefresh)}
                    size="sm"
                    tone="primary"
                    variant="surface"
                  >
                    <RefreshCw className="h-3.5 w-3.5" />
                    刷新任务状态
                  </UiButton>
                  {deletionNeedsReview ? (
                    <UiButton
                      disabled={isDeletionReviewPending}
                      onClick={() => closeThen(() => onConfirmDeletionStopped(task))}
                      size="sm"
                      tone="danger"
                      variant="surface"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      {isDeletionReviewPending
                        ? "确认结果待核对"
                        : "确认已停止，继续删除"}
                    </UiButton>
                  ) : null}
                </div>
              ) : isBindingAttention ? (
                <UiButton
                  disabled={isPending}
                  onClick={() => closeThen(() => onEdit(task))}
                  size="sm"
                  tone="primary"
                  variant="solid"
                >
                  重新绑定会话
                </UiButton>
              ) : hasPermissionActions ? (
                <div className="flex flex-wrap items-center justify-end gap-2">
                  <ScheduledTaskPermissionActions
                    isPending={isPending}
                    onEdit={(currentTask) => closeThen(() => onEdit(currentTask))}
                    onOpenConnector={(connectorId) => closeThen(() => onOpenConnector(connectorId))}
                    onPermissionDecision={(currentTask, decision) => closeThen(
                      () => onPermissionDecision(currentTask, decision),
                    )}
                    onPermissionResume={(currentTask) => closeThen(
                      () => onPermissionResume(currentTask),
                    )}
                    task={task}
                  />
                </div>
              ) : null}
            </div>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
