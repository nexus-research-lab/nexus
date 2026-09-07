/**
 * INPUT: exact 子智能体任务、只读 transcript 资源和任务控制结果。
 * OUTPUT: 公共头像/排版/状态构成的任务详情，文件沿精确来源打开，控制保持 capability 边界。
 * POS: 子智能体详情纯视图；任务展示身份不当作工作区，停止结果未知时禁止普通重复停止。
 */
"use client";

import { Loader2, MessageSquareMore, Square } from "lucide-react";

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import { getSeededAvatarDataUrl } from "@/lib/seeded-avatar";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { Message } from "@/types/conversation/message/entity";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
} from "@/types/conversation/subagent-task";

import {
  canSendSubagentTaskMessage,
  isSubagentTaskActive,
  subagentTaskAvatarSeed,
  subagentTaskTitle,
} from "../subagent-task-model";
import type { SubagentTaskThreadError } from "./subagent-task-thread-model";
import type {
  SubagentTaskActionFailure,
  SubagentTaskActions,
} from "./use-subagent-task-actions";

interface SubagentTaskThreadViewModel {
	actions: SubagentTaskActions;
  detail: SubagentTaskMessagesResponse | null;
  error: SubagentTaskThreadError | null;
  isLoading: boolean;
  messages: Message[];
  onRetry: () => void;
	onSendRequest: () => void;
	onStopRequest: () => void;
  rounds: ConversationThreadRound[];
  sessionKey: string;
  task: SubagentTask;
}

interface SubagentTaskThreadViewProps {
  layout: "desktop" | "mobile";
  model: SubagentTaskThreadViewModel;
  onBack: () => void;
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
}

export function SubagentTaskThreadView({
  layout,
  model,
  onBack,
  onOpenWorkspaceFile,
}: SubagentTaskThreadViewProps) {
  const { t } = useI18n();
  const taskTitle = subagentTaskTitle(model.task, t);

  return (
    <ConversationThreadPanel
      agentAvatar={getSeededAvatarDataUrl(subagentTaskAvatarSeed(model.task))}
      agentId={model.task.agent_id ?? model.task.task_id}
      agentName={taskTitle}
      emptyContent={(
        <ThreadEmptyContent
          detail={model.detail}
          hasError={model.error !== null}
          isLoading={model.isLoading}
          task={model.task}
        />
      )}
			footer={(
				<SubagentTaskControls
					actions={model.actions}
					onRefresh={model.onRetry}
					onSendRequest={model.onSendRequest}
					onStopRequest={model.onStopRequest}
					task={model.task}
				/>
			)}
      headerAvatar={(
        <UiSeededAvatar
          seed={subagentTaskAvatarSeed(model.task)}
          size="xs"
          state={isSubagentTaskActive(model.task) ? "running" : "default"}
          title={taskTitle}
        />
      )}
      headerSubtitle={null}
      isLoading={isSubagentTaskActive(model.task)}
      layout={layout}
      messages={model.messages}
      navigation="back"
      notice={<ThreadNotice error={model.error} onRetry={model.onRetry} />}
      onClose={onBack}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      roundId={model.task.round_id ?? model.task.task_id}
      rounds={model.rounds}
      sessionKey={model.sessionKey}
      workspaceAgentId={model.task.host_agent_id?.trim() || null}
    />
  );
}

function SubagentTaskControls({
	actions,
	onRefresh,
	onSendRequest,
	onStopRequest,
	task,
}: {
	actions: SubagentTaskActions;
	onRefresh: () => void;
	onSendRequest: () => void;
	onStopRequest: () => void;
	task: SubagentTask;
}) {
	const { t } = useI18n();
	const active = isSubagentTaskActive(task);
	const canSend = canSendSubagentTaskMessage(task);
	const canStop = active && task.capabilities.stop;
	const stopResultUnconfirmed = actions.error?.action === "stop"
		&& actions.error.effect !== "not_applied";
	const pending = actions.pendingAction !== null;
	const unsupportedKey = task.status.trim().toLowerCase() === "deleted"
		? "subagents.deleted_unsupported"
		: active
		? "subagents.controls_unsupported"
		: "subagents.resume_unsupported";
	return (
		<footer className="shrink-0 border-t border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-2.5">
			{actions.error ? (
				<SubagentActionFailureState
					failure={actions.error}
					onRefresh={onRefresh}
				/>
			) : actions.feedback ? (
				<p className={cn("mb-2 px-1", getUiTypographyClassName({ role: "metadata", tone: "muted" }))} role="status">
					{t(actions.feedback)}
				</p>
			) : null}
			<div className="flex flex-wrap items-center justify-between gap-2">
				<p className={cn("min-w-0 basis-40 grow", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
					{canSend || canStop
						? t(active ? "subagents.controls_active_hint" : "subagents.controls_resume_hint")
						: t(unsupportedKey)}
				</p>
				<div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-1.5">
					{canStop ? (
						<UiButton
							aria-busy={actions.pendingAction === "stop"}
							disabled={pending || stopResultUnconfirmed}
							onClick={onStopRequest}
							size="sm"
							tone="danger"
							variant="ghost"
						>
							{actions.pendingAction === "stop" ? (
								<Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "sm" })} />
							) : (
								<Square aria-hidden="true" className="h-3.5 w-3.5" />
							)}
							{t("subagents.stop")}
						</UiButton>
					) : null}
					{canSend ? (
						<UiButton
							aria-busy={actions.pendingAction === "send"}
							disabled={pending}
							onClick={onSendRequest}
							size="sm"
							tone="primary"
							variant="surface"
						>
							{actions.pendingAction === "send" ? (
								<Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "sm" })} />
							) : (
								<MessageSquareMore aria-hidden="true" className="h-3.5 w-3.5" />
							)}
							{t(active ? "subagents.send_message" : "subagents.resume")}
						</UiButton>
					) : null}
				</div>
			</div>
		</footer>
	);
}

function ThreadNotice({
  error,
  onRetry,
}: {
  error: SubagentTaskThreadError | null;
  onRetry: () => void;
}) {
  const { t } = useI18n();
  if (!error) {
    return null;
  }
  return (
    <div className="shrink-0 border-b border-(--divider-subtle-color) px-3 py-2">
      <UiResourceState
        className="min-h-0 py-3"
        impact={t("subagents.transcript_load_failed_impact")}
        {...(error.retryable
          ? {
              primaryAction: {
                label: t("subagents.retry"),
                onClick: onRetry,
              },
            }
          : { nextStep: t("subagents.transcript_load_failed_next_step") })}
        size="sm"
        state="error"
        title={t("subagents.transcript_load_failed_title")}
        urgency="polite"
        variant="card"
      />
    </div>
  );
}

function SubagentActionFailureState({
  failure,
  onRefresh,
}: {
  failure: SubagentTaskActionFailure;
  onRefresh: () => void;
}) {
  const { t } = useI18n();
  const operation = failure.action === "stop"
    ? t("subagents.stop")
    : t("subagents.send_message");
  const isNotApplied = failure.effect === "not_applied";
  const title = isNotApplied
    ? t("subagents.action_not_applied_title", { operation })
    : failure.effect === "accepted"
      ? t("subagents.action_accepted_title", { operation })
      : failure.effect === "committed"
        ? t("subagents.action_committed_title", { operation })
        : t("subagents.action_unknown_title", { operation });
  return (
    <UiResourceState
      className="mb-2 min-h-0 py-3"
      impact={t(isNotApplied
        ? "subagents.action_not_applied_impact"
        : "subagents.action_unknown_impact", { operation })}
      {...(isNotApplied
        ? { nextStep: t("subagents.action_not_applied_next_step") }
        : {
            primaryAction: {
              label: t("subagents.refresh_task"),
              onClick: onRefresh,
            },
          })}
      size="sm"
      state="error"
      title={title}
      urgency="polite"
      variant="card"
    />
  );
}

function ThreadEmptyContent({
  detail,
  hasError,
  isLoading,
  task,
}: {
  detail: SubagentTaskMessagesResponse | null;
  hasError: boolean;
  isLoading: boolean;
  task: SubagentTask;
}) {
  const { t } = useI18n();
  if (isLoading && !detail) {
    return (
      <UiResourceState
        icon={<Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "md", tone: "muted" })} />}
        size="sm"
        state="loading"
        title={t("subagents.transcript_loading")}
        variant="plain"
      />
    );
  }
  if (!task.capabilities.transcript) {
    return (
      <UiResourceState
        description={t("subagents.transcript_unsupported_description")}
        icon={false}
        size="sm"
        state="empty"
        title={t("subagents.transcript_unsupported")}
        variant="plain"
      />
    );
  }
  if (detail?.output?.trim()) {
    return (
      <pre className={cn("whitespace-pre-wrap [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "code", tone: "default" }))}>
        {detail.output}
      </pre>
    );
  }
  if (hasError) {
    return null;
  }
  return (
    <UiResourceState
      description={t("subagents.transcript_empty_description")}
      icon={false}
      size="sm"
      state="empty"
      title={t("subagents.transcript_empty")}
      variant="plain"
    />
  );
}
