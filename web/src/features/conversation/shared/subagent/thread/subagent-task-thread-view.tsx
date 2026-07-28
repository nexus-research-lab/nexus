"use client";

import { Loader2 } from "lucide-react";

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Message } from "@/types/conversation/message/entity";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
} from "@/types/conversation/subagent-task";

import { SubagentTaskAvatar } from "../subagent-task-list";
import {
  isSubagentTaskActive,
  subagentTaskAvatarDataUrl,
  subagentTaskTitle,
} from "../subagent-task-model";
import type { SubagentTaskThreadError } from "./subagent-task-thread-model";

interface SubagentTaskThreadViewModel {
  detail: SubagentTaskMessagesResponse | null;
  error: SubagentTaskThreadError | null;
  isLoading: boolean;
  messages: Message[];
  onRetry: () => void;
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
      agentAvatar={subagentTaskAvatarDataUrl(model.task.task_id)}
      agentId={model.task.agent_id ?? model.task.task_id}
      agentName={taskTitle}
      emptyContent={(
        <ThreadEmptyContent
          detail={model.detail}
          isLoading={model.isLoading}
          task={model.task}
        />
      )}
      footer={null}
      headerAvatar={(
        <SubagentTaskAvatar
          className="mt-0 h-7 w-7"
          isActive={isSubagentTaskActive(model.task)}
          name={taskTitle}
          taskId={model.task.task_id}
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
