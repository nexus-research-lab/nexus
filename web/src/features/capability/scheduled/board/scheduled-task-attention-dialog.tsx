/**
 * INPUT: 定时任务 durable 删除、绑定、权限或运行注意事项与后续动作。
 * OUTPUT: 以任务名为标题、复用 Badge/Panel/Typography 的注意事项处理面。
 * POS: Scheduled 看板处理边界；只组合业务事实与共享 UI，不暴露内部删除 token。
 */
"use client";

import { History, RefreshCw, Trash2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
        <UiDialogShell size="md" viewport="adaptiveMax">
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
                  className={getUiTypographyClassName({
                    role: "supporting",
                    tone: "strong",
                    weight: "semibold",
                  })}
                  id={`${titleId}-deletion`}
                >
                  {title || (deletionNeedsReview ? "删除需要管理员处理" : "任务正在删除")}
                </h3>
                <p className={cn(
                  "mt-2",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}>
                  {description || (deletionNeedsReview
                    ? "系统无法确认删除前的原执行是否已经停止，因此任务数据尚未删除。"
                    : "删除请求已受理，系统正在停止任务并完成收尾。")}
                </p>
                <UiPanel className="mt-4 space-y-4" padding="sm" radius="sm">
                  <div>
                    <h4 className={getUiTypographyClassName({
                      role: "caption",
                      tone: "strong",
                      weight: "semibold",
                    })}>
                      对已有内容的影响
                    </h4>
                    <p className={cn(
                      "mt-1",
                      getUiTypographyClassName({ role: "metadata", tone: "default" }),
                    )}>
                      {deletionImpact || (deletionNeedsReview
                        ? "任务配置和运行记录仍然保留；继续确认后会删除任务和历史，但此前已经发生的外部影响无法撤回。"
                        : "已保存的运行记录不会被重写；已经发生的外部操作不会被撤销或自动重做。")}
                    </p>
                  </div>
                  <div>
                    <h4 className={getUiTypographyClassName({
                      role: "caption",
                      tone: "strong",
                      weight: "semibold",
                    })}>
                      现在可以做什么
                    </h4>
                    <p className={cn(
                      "mt-1",
                      getUiTypographyClassName({ role: "metadata", tone: "default" }),
                    )}>
                      {deletionNextStep || (deletionNeedsReview
                        ? "请先确认原执行端已经停止，再使用下方确认操作完成删除；也可以先刷新或查看运行历史。"
                        : "无需再次删除。请等待片刻后刷新，或先查看运行历史。")}
                    </p>
                  </div>
                </UiPanel>
              </section>
            ) : isBindingAttention ? (
              <section aria-labelledby={`${titleId}-binding`}>
                <h3
                  className={getUiTypographyClassName({
                    role: "supporting",
                    tone: "strong",
                    weight: "semibold",
                  })}
                  id={`${titleId}-binding`}
                >
                  {title || "重新绑定会话"}
                </h3>
                <p className={cn(
                  "mt-2",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}>
                  {description}
                </p>
              </section>
            ) : request ? (
              <section aria-labelledby={`${titleId}-request`}>
                <div className="flex items-center justify-between gap-3">
                  <h3
                    className={getUiTypographyClassName({
                      role: "supporting",
                      tone: "strong",
                      weight: "semibold",
                    })}
                    id={`${titleId}-request`}
                  >
                    {title || capabilityLabel || "权限请求"}
                  </h3>
                  <UiBadge shape="pill" size="sm" tone="warning">
                    {requestStatusLabel}
                  </UiBadge>
                </div>
                <p className={cn(
                  "mt-2",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}>
                  {description || request.description || request.reason}
                </p>

                <dl className="mt-4 overflow-hidden surface-radius-sm border border-(--divider-subtle-color)">
                  <div className="grid grid-cols-[88px_minmax(0,1fr)] border-b border-(--divider-subtle-color) px-3 py-2.5 last:border-b-0">
                    <dt className={getUiTypographyClassName({
                      role: "caption",
                      tone: "muted",
                    })}>能力</dt>
                    <dd className={cn(
                      "min-w-0",
                      getUiTypographyClassName({
                        role: "caption",
                        tone: "strong",
                        weight: "medium",
                      }),
                    )}>
                      {capabilityLabel}
                    </dd>
                  </div>
                  {resourceSummary ? (
                    <div className="grid grid-cols-[88px_minmax(0,1fr)] border-b border-(--divider-subtle-color) px-3 py-2.5 last:border-b-0">
                      <dt className={getUiTypographyClassName({
                        role: "caption",
                        tone: "muted",
                      })}>目标</dt>
                      <dd className={cn(
                        "min-w-0 break-all",
                        getUiTypographyClassName({ role: "caption", tone: "default" }),
                      )}>
                        {isWebResource(resourceSummary) ? (
                          <a
                            className="underline decoration-(--divider-subtle-color) underline-offset-2 hover:text-(--primary)"
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
                  <div className="grid grid-cols-[88px_minmax(0,1fr)] px-3 py-2.5">
                    <dt className={getUiTypographyClassName({
                      role: "caption",
                      tone: "muted",
                    })}>批准后</dt>
                    <dd className={getUiTypographyClassName({
                      role: "caption",
                      tone: "default",
                    })}>
                      {request.status === "approved"
                        ? "审批已完成；确认后会重试同一次运行"
                        : request.resume_safe
                        ? "结束当前尝试，并自动继续同一次运行"
                        : "等待你再次确认后才会重试，避免重复副作用"}
                    </dd>
                  </div>
                </dl>

                <UiPanel className="mt-3" padding="sm" radius="sm">
                  <p className={getUiTypographyClassName({
                    role: "metadata",
                    tone: "default",
                    weight: "medium",
                  })}>这项选择只处理当前任务正在等待的权限。</p>
                  <p className={cn(
                    "mt-1",
                    getUiTypographyClassName({ role: "metadata", tone: "muted" }),
                  )}>
                    页面会在提交后重新读取任务状态；如果结果无法确认，会先停止同类操作并提示你核对，不会自动重复执行。
                  </p>
                </UiPanel>
              </section>
            ) : hasPermissionAttention ? (
              <section aria-labelledby={`${titleId}-state`}>
                <h3
                  className={getUiTypographyClassName({
                    role: "supporting",
                    tone: "strong",
                    weight: "semibold",
                  })}
                  id={`${titleId}-state`}
                >
                  {title || "任务需要处理"}
                </h3>
                <p className={cn(
                  "mt-2",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}>
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
                  className={getUiTypographyClassName({
                    role: "caption",
                    tone: "danger",
                    weight: "semibold",
                  })}
                  id={`${titleId}-diagnostic`}
                >
                  {hasPermissionAttention ? "附带运行诊断" : "最近运行诊断"}
                </h3>
                <p className={cn(
                  "mt-2 whitespace-pre-wrap break-words",
                  getUiTypographyClassName({ role: "metadata", tone: "default" }),
                )}>
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
