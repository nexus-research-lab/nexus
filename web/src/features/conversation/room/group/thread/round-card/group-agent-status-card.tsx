"use client";

/**
 * INPUT: Room Agent 执行卡片、待处理人工介入请求与用户动作。
 * OUTPUT: 与轮次分隔线对齐的选择面、Agent 状态摘要、Thread 入口及完整公区用户交互。
 * POS: Room 主 Feed 中活动 Agent 卡片的唯一交互视图。
 */
import { Bot, Loader2, Square } from "lucide-react";
import { memo, useCallback, useMemo } from "react";

import { CONVERSATION_ASSISTANT_CONTENT_WIDTH_CLASS_NAME } from "@/features/conversation/shared/conversation-panel-styles";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { PendingHumanInteractionList } from "@/features/conversation/shared/message/item/view/assistant/pending-human-interaction-list";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-avatar";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  AssistantMessage,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { AgentRoundStatus } from "../../round/round-agent-model";
import {
  buildGroupAgentStatusModel,
  type AgentStatusSummaryTone,
  type GroupAgentStatusModel,
} from "./group-round-card-model";
import { ThreadActionButton } from "./thread-action-button";

interface GroupAgentStatusCardProps {
  agentAvatar: string | null;
  agentId: string;
  agentName: string;
  isThreadActive: boolean;
  messages: AssistantMessage[];
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopAgentRound?: () => void;
  pendingPermissions: PendingPermission[];
  resultSummary?: ResultSummary;
  status: AgentRoundStatus;
  timestamp: number;
}

const ACTIVATION_KEYS = new Set(["Enter", " "]);
const SUMMARY_TONE_CLASS: Record<AgentStatusSummaryTone, string> = {
  default: "text-(--text-strong)",
  error: "text-(--destructive)",
  stopped: "text-(--text-soft) italic",
  waiting: "text-(--text-default)",
};

function GroupAgentStatusCardInner({
  agentAvatar,
  agentId,
  agentName,
  isThreadActive,
  messages,
  onClickThread,
  onOpenAgentContact,
  onPermissionResponse,
  onStopAgentRound,
  pendingPermissions,
  resultSummary,
  status,
  timestamp,
}: GroupAgentStatusCardProps) {
  const { locale, t } = useI18n();
  const statusModel = useMemo(() => buildGroupAgentStatusModel({
    labels: {
      failed: t("room.agent_status_failed"),
      stopped: t("room.agent_status_stopped"),
      waitingForUser: t("room.agent_status_waiting_user"),
    },
    messages,
    pendingPermissions,
    resultSummary,
    status,
    timestamp,
  }), [messages, pendingPermissions, resultSummary, status, t, timestamp]);
  const contactLabel = t("room.agent_contact_open", { name: agentName });

  const handleStop = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    onStopAgentRound?.();
  }, [onStopAgentRound]);
  const handleToggleThread = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    onClickThread();
  }, [onClickThread]);
  const handleCardClick = useCallback((
    event: React.MouseEvent<HTMLDivElement>,
  ) => {
    const target = event.target as HTMLElement;
    if (target.closest("[data-human-interaction-surface]")) {
      return;
    }
    onClickThread();
  }, [onClickThread]);
  const handleOpenAgentContact = useCallback(
    (event: React.MouseEvent<HTMLButtonElement>) => {
      event.stopPropagation();
      onOpenAgentContact?.(agentId);
    },
    [agentId, onOpenAgentContact],
  );
  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (
      event.target === event.currentTarget
      && ACTIVATION_KEYS.has(event.key)
    ) {
      onClickThread();
    }
  }, [onClickThread]);

  return (
    <div
      className="group/card relative w-full min-w-0 cursor-pointer"
      onClick={handleCardClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
    >
      <div
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute inset-x-1.5 inset-y-0 radius-control-lg transition-colors duration-(--motion-duration-normal)",
          isThreadActive
            ? "bg-primary/5"
            : "group-hover/card:bg-(--interaction-hover-background)",
        )}
      />
      <div className="nexus-chat-message-section relative w-full px-2 sm:px-3">
        <div
          className={cn(
            "nexus-chat-message-round-expanded nexus-chat-assistant-grid-expanded grid w-full grid-cols-[40px_minmax(0,1fr)] gap-3 py-3",
            CONVERSATION_ASSISTANT_CONTENT_WIDTH_CLASS_NAME,
          )}
        >
          <MessageAvatar
            ariaLabel={contactLabel}
            avatarUrl={agentAvatar}
            className="shrink-0"
            onClick={onOpenAgentContact ? handleOpenAgentContact : undefined}
            size="full"
            title={contactLabel}
          >
            {!agentAvatar && <Bot className="h-4 w-4" />}
          </MessageAvatar>

          <div className="min-w-0 flex-1">
            <GroupAgentStatusHeader
              actions={{
                stop: onStopAgentRound ? handleStop : undefined,
                toggleThread: handleToggleThread,
              }}
              agentName={agentName}
              isThreadActive={isThreadActive}
              labels={{
                stop: t("room.agent_stop"),
              }}
              locale={locale}
              model={statusModel}
            />
            <GroupAgentStatusSummary agentId={agentId} model={statusModel} />
            {pendingPermissions.length > 0 ? (
              <PendingHumanInteractionList
                canRespond
                mode="room_result"
                onResponse={onPermissionResponse}
                permissions={pendingPermissions}
                workspaceAgentId={agentId}
              />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

interface GroupAgentStatusActions {
  stop?: React.MouseEventHandler<HTMLButtonElement>;
  toggleThread: React.MouseEventHandler<HTMLButtonElement>;
}

interface GroupAgentStatusLabels {
  stop: string;
}

interface GroupAgentStatusHeaderProps {
  actions: GroupAgentStatusActions;
  agentName: string;
  isThreadActive: boolean;
  labels: GroupAgentStatusLabels;
  locale: "zh" | "en";
  model: GroupAgentStatusModel;
}

function GroupAgentStatusHeader({
  actions,
  agentName,
  isThreadActive,
  labels,
  locale,
  model,
}: GroupAgentStatusHeaderProps) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <span className="shrink-0 text-sm font-semibold text-(--text-strong)">
        {agentName}
      </span>
      {model.isActive && !model.isWaitingForUser ? (
        <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />
      ) : null}
      <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
        {model.timestamp ? formatTime(model.timestamp, locale) : "--:--"}
      </span>
      {model.model ? (
        <span className="min-w-0 truncate text-xs text-(--text-soft)">
          {model.model}
        </span>
      ) : null}
      <div className="min-w-0 flex-1" />
      <ThreadActionButton
        active={isThreadActive}
        onClick={actions.toggleThread}
      />
      {actions.stop && model.isActive ? (
        <button
          aria-label={labels.stop}
          className="flex h-6 items-center gap-1 rounded px-1.5 text-xs text-(--icon-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--icon-default)"
          onClick={actions.stop}
          title={labels.stop}
          type="button"
        >
          <Square className="h-3 w-3 fill-current" />
        </button>
      ) : null}
    </div>
  );
}

function GroupAgentStatusSummary({
  agentId,
  model,
}: {
  agentId: string;
  model: GroupAgentStatusModel;
}) {
  if (!model.shouldRenderMarkdownSummary && !model.summaryText) {
    return null;
  }

  return (
    <div className="min-w-0 pt-1">
      {model.shouldRenderMarkdownSummary ? (
        <UiMarkdownContent
          className="line-clamp-1 text-(--text-strong)"
          content={model.preview}
          variant="summary"
          workspaceAgentId={agentId}
        />
      ) : (
        <p
          className={cn(
            "truncate text-base leading-7",
            SUMMARY_TONE_CLASS[model.summaryTone],
          )}
        >
          {model.summaryText}
        </p>
      )}
    </div>
  );
}

export const GroupAgentStatusCard = memo(GroupAgentStatusCardInner);

function formatTime(timestamp: number, locale: "zh" | "en"): string {
  return new Intl.DateTimeFormat(locale === "zh" ? "zh-CN" : "en-US", {
    hour: "2-digit",
    hour12: false,
    minute: "2-digit",
  }).format(timestamp);
}
