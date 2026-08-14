"use client";

import { Loader2, MessageSquareMore, Square } from "lucide-react";

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import { getSeededAvatarDataUrl } from "@/lib/seeded-avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
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
import type { SubagentTaskActions } from "./use-subagent-task-actions";

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
	onSendRequest,
	onStopRequest,
	task,
}: {
	actions: SubagentTaskActions;
	onSendRequest: () => void;
	onStopRequest: () => void;
	task: SubagentTask;
}) {
	const { t } = useI18n();
	const active = isSubagentTaskActive(task);
	const canSend = canSendSubagentTaskMessage(task);
	const canStop = active && task.capabilities.stop;
	const pending = actions.pendingAction !== null;
	const unsupportedKey = task.status.trim().toLowerCase() === "deleted"
		? "subagents.deleted_unsupported"
		: active
		? "subagents.controls_unsupported"
		: "subagents.resume_unsupported";
	return (
		<footer className="shrink-0 border-t border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_88%,transparent)] px-3 py-2.5 backdrop-blur-[14px]">
			{actions.error ? (
				<p className="mb-2 rounded-[8px] border border-[color:color-mix(in_srgb,var(--destructive)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_6%,transparent)] px-2.5 py-1.5 text-xs leading-5 text-(--destructive)" role="alert">
					{actions.error}
				</p>
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
							disabled={pending}
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
    <div className="flex shrink-0 items-start gap-3 border-b border-(--divider-subtle-color) px-4 py-2 text-xs leading-5 text-(--destructive)">
      <p className="min-w-0 flex-1">{error.message}</p>
      {error.retryable ? (
        <button
          className="shrink-0 font-semibold hover:underline"
          onClick={onRetry}
          type="button"
        >
          {t("subagents.retry")}
        </button>
      ) : null}
    </div>
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
