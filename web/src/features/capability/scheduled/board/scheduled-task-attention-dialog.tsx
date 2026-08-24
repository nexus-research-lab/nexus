/**
 * INPUT: 定时任务注意事项、权限事实与后续动作。
 * OUTPUT: 以任务名为标题的处理面，技术身份按需展开。
 * POS: Scheduled 看板的异常/权限处理边界，不制造状态自述卡片。
 */
"use client";

import { History } from "lucide-react";

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
  description: string | null;
  isBindingAttention: boolean;
  isOpen: boolean;
  isPending: boolean;
  onClose: () => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenConnector: (connectorId: string) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onPermissionDecision: (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ) => void;
  onPermissionResume: (task: ScheduledTaskItem) => void;
  task: ScheduledTaskItem;
  title: string | null;
}

function isWebResource(value: string): boolean {
  return value.startsWith("https://") || value.startsWith("http://");
}

export function ScheduledTaskAttentionDialog({
  description,
  isBindingAttention,
  isOpen,
  isPending,
  onClose,
  onEdit,
  onOpenConnector,
  onOpenHistory,
  onPermissionDecision,
  onPermissionResume,
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
            {isBindingAttention ? (
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

                <details className="mt-3 rounded-[8px] border border-(--divider-subtle-color) px-3 py-2.5 text-xs">
                  <summary className="cursor-pointer select-none font-medium text-(--text-muted) hover:text-(--text-strong)">
                    查看审批技术信息
                  </summary>
                  <dl className="mt-3 grid grid-cols-[88px_minmax(0,1fr)] gap-x-3 gap-y-2 border-t border-(--divider-subtle-color) pt-3">
                    <dt className="text-(--text-muted)">工具</dt>
                    <dd className="min-w-0 break-all font-mono text-(--text-default)">
                      {request.capability.tool_name}
                    </dd>
                    <dt className="text-(--text-muted)">Request</dt>
                    <dd className="min-w-0 break-all font-mono text-(--text-default)">
                      {request.request_id}
                    </dd>
                    <dt className="text-(--text-muted)">Run</dt>
                    <dd className="min-w-0 break-all font-mono text-(--text-default)">
                      {request.run_id || "—"}
                    </dd>
                    <dt className="text-(--text-muted)">策略版本</dt>
                    <dd className="font-mono text-(--text-default)">
                      {request.policy_revision}
                    </dd>
                  </dl>
                </details>
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

            {errorCopy && !errorEchoesPermission ? (
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
              {isBindingAttention ? (
                <UiButton
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
