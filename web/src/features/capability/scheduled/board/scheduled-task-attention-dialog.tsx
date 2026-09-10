/**
 * INPUT: 定时任务 durable 删除、绑定、权限或运行注意事项与后续动作。
 * OUTPUT: 以任务名为标题、复用 Badge/Panel/Typography 的处理面，已知错误按当前语言解释且保留完整诊断。
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
import { useI18n } from "@/shared/i18n/i18n-context";
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
  const { t } = useI18n();
  if (!isOpen) {
    return null;
  }
  const request = task.pending_permission_request;
  const capabilityLabel = request
    ? getScheduledPermissionCapabilityLabel(request, t)
    : null;
  const resourceSummary = request
    ? getScheduledPermissionResourceSummary(request)
    : null;
  const errorCopy = getScheduledTaskErrorCopy(task.running ? null : task.last_error, t);
  const errorEchoesPermission = Boolean(
    request?.capability.tool_name
      && task.last_error?.includes(request.capability.tool_name),
  );
  const hasPermissionAttention = hasScheduledTaskPermissionAttention(task);
  const requestStatusLabel = request?.status === "approved"
    ? t("capability.scheduled_board_approved_retry")
    : t("capability.scheduled_board_waiting");
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
                  {title || (deletionNeedsReview ? t("capability.scheduled_delete_review_required_title") : t("capability.scheduled_board_deleting_title"))}
                </h3>
                <p className={cn(
                  "mt-2",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}>
                  {description || (deletionNeedsReview
                    ? t("capability.scheduled_board_delete_review_description")
                    : t("capability.scheduled_board_deleting_description"))}
                </p>
                <UiPanel className="mt-4 space-y-4" padding="sm" radius="sm">
                  <div>
                    <h4 className={getUiTypographyClassName({
                      role: "caption",
                      tone: "strong",
                      weight: "semibold",
                    })}>
                      {t("capability.scheduled_board_impact_title")}
                    </h4>
                    <p className={cn(
                      "mt-1",
                      getUiTypographyClassName({ role: "metadata", tone: "default" }),
                    )}>
                      {deletionImpact || (deletionNeedsReview
                        ? t("capability.scheduled_board_delete_review_impact")
                        : t("capability.scheduled_board_deleting_impact"))}
                    </p>
                  </div>
                  <div>
                    <h4 className={getUiTypographyClassName({
                      role: "caption",
                      tone: "strong",
                      weight: "semibold",
                    })}>
                      {t("capability.scheduled_board_next_step_title")}
                    </h4>
                    <p className={cn(
                      "mt-1",
                      getUiTypographyClassName({ role: "metadata", tone: "default" }),
                    )}>
                      {deletionNextStep || (deletionNeedsReview
                        ? t("capability.scheduled_board_delete_review_next")
                        : t("capability.scheduled_board_deleting_next"))}
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
                  {title || t("capability.scheduled_board_rebind")}
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
                    {title || capabilityLabel || t("capability.scheduled_board_permission_request")}
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
                    })}>{t("capability.scheduled_board_capability")}</dt>
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
                      })}>{t("capability.scheduled_board_target")}</dt>
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
                    })}>{t("capability.scheduled_board_after_approval")}</dt>
                    <dd className={getUiTypographyClassName({
                      role: "caption",
                      tone: "default",
                    })}>
                      {request.status === "approved"
                        ? t("capability.scheduled_board_approved_next")
                        : request.resume_safe
                        ? t("capability.scheduled_board_safe_resume")
                        : t("capability.scheduled_board_unsafe_resume")}
                    </dd>
                  </div>
                </dl>

                <UiPanel className="mt-3" padding="sm" radius="sm">
                  <p className={getUiTypographyClassName({
                    role: "metadata",
                    tone: "default",
                    weight: "medium",
                  })}>{t("capability.scheduled_board_permission_scope")}</p>
                  <p className={cn(
                    "mt-1",
                    getUiTypographyClassName({ role: "metadata", tone: "muted" }),
                  )}>
                    {t("capability.scheduled_board_permission_reconcile")}
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
                  {title || t("capability.scheduled_board_attention_title")}
                </h3>
                <p className={cn(
                  "mt-2",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}>
                  {description || t("capability.scheduled_board_permission_changed")}
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
                  {hasPermissionAttention ? t("capability.scheduled_board_additional_diagnostic") : t("capability.scheduled_board_recent_diagnostic")}
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
                {t("capability.scheduled_board_history")}
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
                    {t("capability.scheduled_board_refresh")}
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
                        ? t("capability.scheduled_board_confirmation_unknown")
                        : t("capability.scheduled_board_confirm_stopped")}
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
                  {t("capability.scheduled_board_rebind")}
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
