"use client";

import { Loader2, Send, Square } from "lucide-react";
import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  GroupThreadDetailPanel,
  type GroupThreadRound,
} from "@/features/conversation/room/group/thread/group-thread-detail-panel";
import { groupMessagesByRound } from "@/features/conversation/shared/timeline/timeline-model";
import {
  getSubagentTaskMessagesApi,
  sendSubagentTaskMessageApi,
  stopSubagentTaskApi,
} from "@/lib/api/subagent-task-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import { SubagentTaskAvatar } from "./subagent-task-list";
import {
  canSendSubagentTaskMessage,
  canStopSubagentTask,
  isSubagentTaskActive,
  normalizeSubagentTask,
  subagentTaskAvatarDataUrl,
  subagentTaskSourceKey,
  subagentTaskTitle,
} from "./subagent-task-model";

interface SubagentTaskThreadProps {
  layout?: "desktop" | "mobile";
  onBack: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onRefreshTasks: () => Promise<void>;
  source: SubagentTaskSource;
  task: SubagentTask;
}

export function SubagentTaskThread({
  layout = "desktop",
  onBack,
  onOpenWorkspaceFile,
  onRefreshTasks,
  source,
  task,
}: SubagentTaskThreadProps) {
  const { t } = useI18n();
  const [detail, setDetail] = useState<SubagentTaskMessagesResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isStopping, setIsStopping] = useState(false);
  const [isSending, setIsSending] = useState(false);
  const [message, setMessage] = useState("");
  const sourceKey = subagentTaskSourceKey(source);
  const sourceRef = useRef(source);
  sourceRef.current = source;
  const taskRef = useRef(task);
  taskRef.current = task;
  const requestSequenceRef = useRef(0);
  const taskId = task.task_id;
  const transcriptAvailable = task.capabilities.transcript;

  const loadDetail = useCallback(async (silent = false) => {
    const currentSource = sourceRef.current;
    const currentTask = taskRef.current;
    if (
      subagentTaskSourceKey(currentSource) !== sourceKey
      || currentTask.task_id !== taskId
      || !transcriptAvailable
    ) {
      setDetail(null);
      return;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    if (!silent) {
      setIsLoading(true);
    }
    try {
      const result = await getSubagentTaskMessagesApi(currentSource, taskId);
      if (requestSequenceRef.current !== requestSequence) {
        return;
      }
      setDetail({
        ...result,
        task: normalizeSubagentTask(
          result.task,
          currentTask.runtime_kind,
          currentTask.capabilities,
        ),
      });
      setError(null);
    } catch (requestError) {
      if (requestSequenceRef.current !== requestSequence) {
        return;
      }
      setError(requestError instanceof Error ? requestError.message : String(requestError));
    } finally {
      if (!silent && requestSequenceRef.current === requestSequence) {
        setIsLoading(false);
      }
    }
  }, [sourceKey, taskId, transcriptAvailable]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    setDetail(null);
    setError(null);
    setMessage("");
    void loadDetail();
  }, [loadDetail, sourceKey, taskId]);

  const taskIsActive = isSubagentTaskActive(task);
  useEffect(() => {
    if (!transcriptAvailable || !taskIsActive) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void loadDetail(true);
    }, 3_000);
    return () => window.clearInterval(intervalId);
  }, [loadDetail, taskIsActive, transcriptAvailable]);

  const effectiveTask = detail?.task ?? task;
  const messages = useMemo(() => detail?.messages ?? [], [detail?.messages]);
  const messageGroups = useMemo(() => groupMessagesByRound(messages), [messages]);
  const rounds = useMemo<GroupThreadRound[]>(
    () => Array.from(messageGroups, ([roundId, roundMessages]) => ({
      roundId,
      messages: roundMessages,
    })),
    [messageGroups],
  );
  const canStop = canStopSubagentTask(effectiveTask);
  const canSend = canSendSubagentTaskMessage(effectiveTask);
  const isResume = canSend && !isSubagentTaskActive(effectiveTask);
  const threadKey = `${sourceKey}:${effectiveTask.task_id}`;
  const agentName = subagentTaskTitle(effectiveTask);
  const agentId = effectiveTask.agent_id ?? effectiveTask.task_id;
  const workspaceAgentId = effectiveTask.host_agent_id ?? null;
  const handleOpenTaskWorkspaceFile = useCallback((path: string) => {
    onOpenWorkspaceFile?.(path, effectiveTask.host_agent_id ?? null);
  }, [effectiveTask.host_agent_id, onOpenWorkspaceFile]);

  const handleStop = useCallback(async () => {
    if (!canStop || isStopping) {
      return;
    }
    setIsStopping(true);
    setError(null);
    try {
      await stopSubagentTaskApi(sourceRef.current, effectiveTask.task_id);
      await onRefreshTasks();
      await loadDetail(true);
    } catch (actionError) {
      setError(actionError instanceof Error ? actionError.message : String(actionError));
    } finally {
      setIsStopping(false);
    }
  }, [canStop, effectiveTask.task_id, isStopping, loadDetail, onRefreshTasks]);

  const handleSend = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = message.trim();
    if (!canSend || !value || isSending) {
      return;
    }
    setIsSending(true);
    setError(null);
    try {
      await sendSubagentTaskMessageApi(sourceRef.current, effectiveTask.task_id, value);
      setMessage("");
      await onRefreshTasks();
      await loadDetail(true);
    } catch (actionError) {
      setError(actionError instanceof Error ? actionError.message : String(actionError));
    } finally {
      setIsSending(false);
    }
  }, [canSend, effectiveTask.task_id, isSending, loadDetail, message, onRefreshTasks]);

  const notice = error ? (
    <div className="flex shrink-0 items-start gap-3 border-b border-(--divider-subtle-color) px-4 py-2 text-xs leading-5 text-(--destructive)">
      <p className="min-w-0 flex-1">{error}</p>
      <button
        className="shrink-0 font-semibold hover:underline"
        onClick={() => void loadDetail()}
        type="button"
      >
        {t("subagents.retry")}
      </button>
    </div>
  ) : null;
  const emptyContent = resolveThreadEmptyContent({
    detail,
    isLoading,
    task: effectiveTask,
    t,
  });
  const footer = (
    <SubagentThreadControls
      canSend={canSend}
      canStop={canStop}
      isResume={isResume}
      isSending={isSending}
      isStopping={isStopping}
      message={message}
      onMessageChange={setMessage}
      onSend={handleSend}
      onStop={() => void handleStop()}
      runtimeKind={effectiveTask.runtime_kind}
    />
  );

  return (
    <GroupThreadDetailPanel
      agentId={agentId}
      agentAvatar={subagentTaskAvatarDataUrl(effectiveTask.task_id)}
      agentName={agentName}
      emptyContent={emptyContent}
      footer={footer}
      headerAvatar={(
        <SubagentTaskAvatar
          className="mt-0 h-7 w-7"
          isActive={isSubagentTaskActive(effectiveTask)}
          name={agentName}
          taskId={effectiveTask.task_id}
        />
      )}
      headerSubtitle={null}
      isLoading={isSubagentTaskActive(effectiveTask)}
      layout={layout}
      messages={messages}
      navigation="back"
      notice={notice}
      onClose={onBack}
      onOpenWorkspaceFile={onOpenWorkspaceFile ? handleOpenTaskWorkspaceFile : undefined}
      roundId={effectiveTask.round_id ?? effectiveTask.task_id}
      rounds={rounds}
      sessionKey={threadKey}
      workspaceAgentId={workspaceAgentId}
    />
  );
}

function SubagentThreadControls({
  canSend,
  canStop,
  isResume,
  isSending,
  isStopping,
  message,
  onMessageChange,
  onSend,
  onStop,
  runtimeKind,
}: {
  canSend: boolean;
  canStop: boolean;
  isResume: boolean;
  isSending: boolean;
  isStopping: boolean;
  message: string;
  onMessageChange: (value: string) => void;
  onSend: (event: FormEvent<HTMLFormElement>) => void;
  onStop: () => void;
  runtimeKind: SubagentTask["runtime_kind"];
}) {
  const { t } = useI18n();

  if (!canSend) {
    return (
      <div className="flex shrink-0 items-center gap-3 border-t border-(--divider-subtle-color) px-4 py-2.5">
        <p className="min-w-0 flex-1 text-[11.5px] leading-5 text-(--text-soft)">
          {runtimeKind === "claude"
            ? t("subagents.cc_follow_up_unavailable")
            : t("subagents.follow_up_unavailable")}
        </p>
        {canStop ? (
          <StopTaskButton disabled={isStopping} onClick={onStop} />
        ) : null}
      </div>
    );
  }

  return (
    <form
      className="shrink-0 border-t border-(--divider-subtle-color) px-3 py-2"
      onSubmit={onSend}
    >
      <div className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2.5 pb-1.5 pt-1.5 transition-colors focus-within:border-(--surface-interactive-hover-border)">
        <textarea
          className="max-h-28 min-h-10 w-full resize-none bg-transparent py-1 text-[12.5px] leading-5 text-(--text-default) outline-none placeholder:text-(--text-soft)"
          disabled={isSending}
          onChange={(event) => onMessageChange(event.target.value)}
          placeholder={isResume
            ? t("subagents.resume_placeholder")
            : t("subagents.send_placeholder")}
          rows={2}
          value={message}
        />
        <div className="flex min-h-7 items-center justify-between gap-3">
          {canStop ? (
            <StopTaskButton disabled={isStopping} onClick={onStop} />
          ) : <span />}
          <div className="flex min-w-0 items-center gap-2">
            {isResume ? (
              <span className="hidden truncate text-[10.5px] text-(--text-soft) sm:inline">
                {t("subagents.continue_same_thread")}
              </span>
            ) : null}
            <button
              aria-label={t("subagents.send")}
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-(--primary) text-(--primary-foreground) transition-colors disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
              disabled={isSending || !message.trim()}
              title={t("subagents.send")}
              type="submit"
            >
              {isSending
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
  onClick,
}: {
  disabled: boolean;
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
      {disabled
        ? <Loader2 className="h-3 w-3 animate-spin" />
        : <Square className="h-2.5 w-2.5 fill-current" />}
      {t("subagents.stop")}
    </button>
  );
}

function resolveThreadEmptyContent({
  detail,
  isLoading,
  task,
  t,
}: {
  detail: SubagentTaskMessagesResponse | null;
  isLoading: boolean;
  task: SubagentTask;
  t: ReturnType<typeof useI18n>["t"];
}) {
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
