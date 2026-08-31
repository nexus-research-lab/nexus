/**
 * INPUT: exact 子智能体任务、只读 transcript 资源和任务控制结果。
 * OUTPUT: 线程内容、精确控制，以及完整说明发生事项、数据影响和恢复动作的原位异常面。
 * POS: 子智能体详情纯视图；不根据文案推断写入结果，停止结果未知时禁止普通重复停止。
 */
"use client";

import { Loader2, MessageSquareMore, Square } from "lucide-react";

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import { getSeededAvatarDataUrl } from "@/lib/seeded-avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { Message } from "@/types/conversation/message/entity";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
} from "@/types/conversation/subagent-task";

import { SubagentTaskAvatar } from "../subagent-task-list";
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
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
}

export function SubagentTaskThreadView({
  layout,
  model,
  onBack,
  onOpenWorkspaceFile,
}: SubagentTaskThreadViewProps) {
  const taskTitle = subagentTaskTitle(model.task);
  const handleOpenWorkspaceFile = onOpenWorkspaceFile
    ? (path: string) => onOpenWorkspaceFile(path, model.task.host_agent_id ?? null)
    : undefined;

  return (
    <ConversationThreadPanel
      agentAvatar={getSeededAvatarDataUrl(subagentTaskAvatarSeed(model.task))}
      agentId={model.task.agent_id ?? model.task.task_id}
      agentName={taskTitle}
      emptyContent={(
        <ThreadEmptyContent
          detail={model.detail}
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
        <SubagentTaskAvatar
          className="mt-0 h-7 w-7"
          isActive={isSubagentTaskActive(model.task)}
          name={taskTitle}
          seed={subagentTaskAvatarSeed(model.task)}
        />
      )}
      headerSubtitle={null}
      isLoading={isSubagentTaskActive(model.task)}
      layout={layout}
      messages={model.messages}
      navigation="back"
      notice={<ThreadNotice error={model.error} onRetry={model.onRetry} />}
      onClose={onBack}
      onOpenWorkspaceFile={handleOpenWorkspaceFile}
      roundId={model.task.round_id ?? model.task.task_id}
      rounds={model.rounds}
      sessionKey={model.sessionKey}
      workspaceAgentId={model.task.host_agent_id ?? null}
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
		<footer className="shrink-0 border-t border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_88%,transparent)] px-3 py-2.5 backdrop-blur-[14px]">
			{actions.error ? (
				<SubagentActionFailureState
					failure={actions.error}
					onRefresh={onRefresh}
				/>
			) : actions.feedback ? (
				<p className="mb-2 px-1 text-xs leading-5 text-(--text-muted)" role="status">
					{t(actions.feedback)}
				</p>
			) : null}
			<div className="flex items-center justify-between gap-2">
				<p className="min-w-0 flex-1 text-[11px] leading-4 text-(--text-soft)">
					{canSend || canStop
						? t(active ? "subagents.controls_active_hint" : "subagents.controls_resume_hint")
						: t(unsupportedKey)}
				</p>
				<div className="flex shrink-0 items-center gap-1.5">
					{canStop ? (
						<button
							className={getUiButtonClassName({ size: "sm", tone: "danger", variant: "ghost" })}
							disabled={pending || stopResultUnconfirmed}
							onClick={onStopRequest}
							type="button"
						>
							{actions.pendingAction === "stop" ? (
								<Loader2 className="h-3.5 w-3.5 animate-spin" />
							) : (
								<Square className="h-3.5 w-3.5" />
							)}
							{t("subagents.stop")}
						</button>
					) : null}
					{canSend ? (
						<button
							className={getUiButtonClassName({ size: "sm", tone: "primary", variant: "surface" })}
							disabled={pending}
							onClick={onSendRequest}
							type="button"
						>
							{actions.pendingAction === "send" ? (
								<Loader2 className="h-3.5 w-3.5 animate-spin" />
							) : (
								<MessageSquareMore className="h-3.5 w-3.5" />
							)}
							{t(active ? "subagents.send_message" : "subagents.resume")}
						</button>
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
        description={error.message}
        impact={t("subagents.transcript_load_failed_impact")}
        nextStep={t("subagents.transcript_load_failed_next_step")}
        primaryAction={error.retryable ? {
          label: t("subagents.retry"),
          onClick: onRetry,
        } : undefined}
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
      description={failure.message}
      impact={t(isNotApplied
        ? "subagents.action_not_applied_impact"
        : "subagents.action_unknown_impact", { operation })}
      nextStep={t(isNotApplied
        ? "subagents.action_not_applied_next_step"
        : failure.action === "stop"
          ? "subagents.stop_unknown_next_step"
          : "subagents.message_unknown_next_step")}
      primaryAction={isNotApplied ? undefined : {
        label: t("subagents.refresh_task"),
        onClick: onRefresh,
      }}
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
  isLoading,
  task,
}: {
  detail: SubagentTaskMessagesResponse | null;
  isLoading: boolean;
  task: SubagentTask;
}) {
  const { t } = useI18n();
  if (isLoading && !detail) {
    return (
      <div className="flex min-h-36 items-center justify-center gap-2 text-sm text-(--text-muted)">
        <Loader2 className="h-4 w-4 animate-spin" />
        {t("subagents.transcript_loading")}
      </div>
    );
  }
  if (!task.capabilities.transcript) {
    return (
      <ThreadEmptyState
        description={t("subagents.transcript_unsupported_description")}
        title={t("subagents.transcript_unsupported")}
      />
    );
  }
  if (detail?.output?.trim()) {
    return (
      <pre className="whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
        {detail.output}
      </pre>
    );
  }
  return (
    <ThreadEmptyState
      description={t("subagents.transcript_empty_description")}
      title={t("subagents.transcript_empty")}
    />
  );
}

function ThreadEmptyState({
  description,
  title,
}: {
  description: string;
  title: string;
}) {
  return (
    <div className="flex min-h-36 flex-col items-center justify-center px-4 text-center">
      <p className="text-sm font-medium text-(--text-strong)">{title}</p>
      <p className="mt-1 max-w-sm text-xs leading-5 text-(--text-soft)">{description}</p>
    </div>
  );
}
