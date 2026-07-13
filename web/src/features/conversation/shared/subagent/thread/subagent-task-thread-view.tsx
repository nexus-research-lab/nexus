"use client";

import { Loader2, Send, Square } from "lucide-react";
import type { FormEvent } from "react";

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
import type {
  SubagentTaskCommand,
  SubagentTaskThreadError,
} from "./subagent-task-thread-model";

interface SubagentTaskThreadViewModel {
  canSend: boolean;
  canStop: boolean;
  command: SubagentTaskCommand | null;
  detail: SubagentTaskMessagesResponse | null;
  draft: string;
  error: SubagentTaskThreadError | null;
  isLoading: boolean;
  isResume: boolean;
  messages: Message[];
  onRetry: () => void;
  onSend: () => void;
  onStop: () => void;
  rounds: ConversationThreadRound[];
  sessionKey: string;
  setDraft: (value: string) => void;
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
      footer={<SubagentThreadControls model={model} />}
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

function SubagentThreadControls({ model }: { model: SubagentTaskThreadViewModel }) {
  const { t } = useI18n();
  const isPending = model.command !== null;

  if (!model.canSend) {
    return (
      <div className="flex shrink-0 items-center gap-3 border-t border-(--divider-subtle-color) px-4 py-2.5">
        <p className="min-w-0 flex-1 text-[11.5px] leading-5 text-(--text-soft)">
          {model.task.runtime_kind === "claude"
            ? t("subagents.cc_follow_up_unavailable")
            : t("subagents.follow_up_unavailable")}
        </p>
        {model.canStop ? (
          <StopTaskButton
            disabled={isPending}
            isLoading={model.command === "stop"}
            onClick={model.onStop}
          />
        ) : null}
      </div>
    );
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    model.onSend();
  };

  return (
    <form
      className="shrink-0 border-t border-(--divider-subtle-color) px-3 py-2"
      onSubmit={handleSubmit}
    >
      <div className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2.5 pb-1.5 pt-1.5 transition-colors focus-within:border-(--surface-interactive-hover-border)">
        <textarea
          className="max-h-28 min-h-10 w-full resize-none bg-transparent py-1 text-[12.5px] leading-5 text-(--text-default) outline-none placeholder:text-(--text-soft)"
          disabled={isPending}
          onChange={(event) => model.setDraft(event.target.value)}
          placeholder={model.isResume
            ? t("subagents.resume_placeholder")
            : t("subagents.send_placeholder")}
          rows={2}
          value={model.draft}
        />
        <div className="flex min-h-7 items-center justify-between gap-3">
          {model.canStop ? (
            <StopTaskButton
              disabled={isPending}
              isLoading={model.command === "stop"}
              onClick={model.onStop}
            />
          ) : <span />}
          <div className="flex min-w-0 items-center gap-2">
            {model.isResume ? (
              <span className="hidden truncate text-[10.5px] text-(--text-soft) sm:inline">
                {t("subagents.continue_same_thread")}
              </span>
            ) : null}
            <button
              aria-label={t("subagents.send")}
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-(--primary) text-(--primary-foreground) transition-colors disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
              disabled={isPending || !model.draft.trim()}
              title={t("subagents.send")}
              type="submit"
            >
              {model.command === "send"
                ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                : <Send className="h-3.5 w-3.5" />}
            </button>
          </div>
        </div>
      </div>
    </form>
  );
}

function StopTaskButton({
  disabled,
  isLoading,
  onClick,
}: {
  disabled: boolean;
  isLoading: boolean;
  onClick: () => void;
}) {
  const { t } = useI18n();
  return (
    <button
      className="inline-flex h-7 shrink-0 items-center gap-1.5 rounded-[6px] px-1.5 text-[11px] font-medium text-(--text-soft) transition-colors hover:text-(--destructive) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
      disabled={disabled}
      onClick={onClick}
      type="button"
    >
      {isLoading
        ? <Loader2 className="h-3 w-3 animate-spin" />
        : <Square className="h-2.5 w-2.5 fill-current" />}
      {t("subagents.stop")}
    </button>
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
      <pre className="whitespace-pre-wrap break-words text-[13px] leading-6 text-(--text-default)">
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
