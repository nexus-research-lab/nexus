"use client";

import { Bot, Loader2, Square } from "lucide-react";
import { memo, useCallback, useMemo } from "react";

import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-primitives";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  AssistantMessage,
  ResultSummary,
  RoomPendingAgentSlotState,
} from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import type { AgentRoundStatus } from "../round/round-agent-model";
import {
  buildGroupAgentStatusModel,
  type AgentStatusSummaryTone,
} from "./group-round-card-model";
import { ThreadActionButton } from "./thread-action-button";

interface GroupAgentStatusCardProps {
  agentAvatar?: string | null;
  agentId: string;
  agentName: string;
  isThreadActive: boolean;
  messages: AssistantMessage[];
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onStopMessage?: () => void;
  pendingPermissions?: PendingPermission[];
  pendingSlot?: RoomPendingAgentSlotState;
  resultSummary?: ResultSummary;
  status: AgentRoundStatus;
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
  onStopMessage,
  pendingPermissions = [],
  pendingSlot,
  resultSummary,
  status,
}: GroupAgentStatusCardProps) {
  const { locale, t } = useI18n();
  const statusModel = useMemo(() => buildGroupAgentStatusModel({
    labels: {
      failed: t("room.agent_status_failed"),
      preparing: t("room.agent_status_preparing"),
      replying: t("room.agent_status_replying"),
      stopped: t("room.agent_status_stopped"),
      waitingPermission: t("room.agent_status_waiting_permission"),
    },
    messages,
    pendingPermissions,
    pendingSlot,
    resultSummary,
    status,
  }), [messages, pendingPermissions, pendingSlot, resultSummary, status, t]);
  const {
    isActive,
    isQuestionPending,
    isWaitingPermission,
    model,
    preview,
    primaryPendingPermission,
    shouldRenderMarkdownSummary,
    summaryText,
    summaryTone,
    timestamp,
  } = statusModel;
  const contactLabel = t("room.agent_contact_open", { name: agentName });

  const handleStop = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    onStopMessage?.();
  }, [onStopMessage]);
  const handleAllow = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    if (isQuestionPending || !primaryPendingPermission || !onPermissionResponse) {
      onClickThread();
      return;
    }
    onPermissionResponse({
      request_id: primaryPendingPermission.request_id,
      decision: "allow",
    });
  }, [isQuestionPending, onClickThread, onPermissionResponse, primaryPendingPermission]);
  const handleDeny = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    if (!primaryPendingPermission || !onPermissionResponse) {
      onClickThread();
      return;
    }
    onPermissionResponse({
      request_id: primaryPendingPermission.request_id,
      decision: "deny",
    });
  }, [onClickThread, onPermissionResponse, primaryPendingPermission]);
  const handleToggleThread = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
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
    if (ACTIVATION_KEYS.has(event.key)) {
      onClickThread();
    }
  }, [onClickThread]);

  return (
    <div
      className={cn(
        "group/card grid min-w-0 cursor-pointer grid-cols-[40px_minmax(0,1fr)] gap-3 px-2 py-3 transition-colors duration-(--motion-duration-normal)",
        isThreadActive
          ? "bg-primary/5"
          : "hover:bg-(--interaction-hover-background)",
      )}
      onClick={onClickThread}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
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
        <div className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-sm font-bold text-(--text-strong)">
            {agentName}
          </span>
          {isActive && !isWaitingPermission ? (
            <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />
          ) : null}
          <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
            {timestamp ? formatTime(timestamp, locale) : "--:--"}
          </span>
          {model ? (
            <span className="min-w-0 truncate text-xs text-(--text-soft)">
              {model}
            </span>
          ) : null}
          <div className="min-w-0 flex-1" />

          <ThreadActionButton
            active={isThreadActive}
            onClick={handleToggleThread}
          />

          {isWaitingPermission ? (
            <>
              <button
                className="rounded-md border border-(--divider-subtle-color) bg-transparent px-2 py-1 text-[11px] font-medium text-(--text-default) transition-colors hover:bg-(--interaction-hover-background)"
                onClick={handleDeny}
                type="button"
              >
                {t("room.permission_deny")}
              </button>
              <button
                className="rounded-md bg-primary px-2 py-1 text-[11px] font-medium text-white transition-colors hover:bg-primary/88"
                onClick={handleAllow}
                type="button"
              >
                {t(isQuestionPending
                  ? "room.permission_answer"
                  : "room.permission_allow")}
              </button>
            </>
          ) : null}

          {onStopMessage && isActive ? (
            <button
              aria-label={t("room.agent_stop")}
              className="flex h-6 items-center gap-1 rounded px-1.5 text-xs text-(--icon-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--icon-default)"
              onClick={handleStop}
              title={t("room.agent_stop")}
              type="button"
            >
              <Square className="h-3 w-3 fill-current" />
            </button>
          ) : null}
        </div>

        <div className="min-w-0 pt-1">
          {shouldRenderMarkdownSummary ? (
            <MarkdownRendererContent
              className="line-clamp-1 text-(--text-strong)"
              content={preview}
              variant="summary"
              workspaceAgentId={agentId}
            />
          ) : (
            <p
              className={cn(
                "truncate text-[15px] leading-7",
                SUMMARY_TONE_CLASS[summaryTone],
              )}
            >
              {summaryText}
            </p>
          )}
        </div>
      </div>
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
