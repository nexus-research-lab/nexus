"use client";

import { Bot, Loader2, Square } from "lucide-react";
import { memo, useCallback, useMemo } from "react";

import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
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
  onStopMessage?: () => void;
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
  onStopMessage,
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
      waitingPermission: t("room.agent_status_waiting_permission"),
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
    onStopMessage?.();
  }, [onStopMessage]);
  const handleAllow = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    if (
      statusModel.isQuestionPending ||
      !statusModel.primaryPendingPermission
    ) {
      onClickThread();
      return;
    }
    onPermissionResponse({
      request_id: statusModel.primaryPendingPermission.request_id,
      decision: "allow",
    });
  }, [onClickThread, onPermissionResponse, statusModel]);
  const handleDeny = useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    if (!statusModel.primaryPendingPermission) {
      onClickThread();
      return;
    }
    onPermissionResponse({
      request_id: statusModel.primaryPendingPermission.request_id,
      decision: "deny",
    });
  }, [onClickThread, onPermissionResponse, statusModel]);
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
        <GroupAgentStatusHeader
          actions={{
            allow: handleAllow,
            deny: handleDeny,
            stop: onStopMessage ? handleStop : undefined,
            toggleThread: handleToggleThread,
          }}
          agentName={agentName}
          isThreadActive={isThreadActive}
          labels={{
            permissionAllow: t(
              statusModel.isQuestionPending
                ? "room.permission_answer"
                : "room.permission_allow",
            ),
            permissionDeny: t("room.permission_deny"),
            stop: t("room.agent_stop"),
          }}
          locale={locale}
          model={statusModel}
        />
        <GroupAgentStatusSummary agentId={agentId} model={statusModel} />
      </div>
    </div>
  );
}

interface GroupAgentStatusActions {
  allow: React.MouseEventHandler<HTMLButtonElement>;
  deny: React.MouseEventHandler<HTMLButtonElement>;
  stop?: React.MouseEventHandler<HTMLButtonElement>;
  toggleThread: React.MouseEventHandler<HTMLButtonElement>;
}

interface GroupAgentStatusLabels {
  permissionAllow: string;
  permissionDeny: string;
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
      <span className="shrink-0 text-sm font-bold text-(--text-strong)">
        {agentName}
      </span>
      {model.isActive && !model.isWaitingPermission ? (
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
      <GroupAgentPermissionActions
        allowLabel={labels.permissionAllow}
        denyLabel={labels.permissionDeny}
        isWaiting={model.isWaitingPermission}
        onAllow={actions.allow}
        onDeny={actions.deny}
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

interface GroupAgentPermissionActionsProps {
  allowLabel: string;
  denyLabel: string;
  isWaiting: boolean;
  onAllow: React.MouseEventHandler<HTMLButtonElement>;
  onDeny: React.MouseEventHandler<HTMLButtonElement>;
}

function GroupAgentPermissionActions({
  allowLabel,
  denyLabel,
  isWaiting,
  onAllow,
  onDeny,
}: GroupAgentPermissionActionsProps) {
  if (!isWaiting) {
    return null;
  }
  return (
    <>
      <button
        className="rounded-md border border-(--divider-subtle-color) bg-transparent px-2 py-1 text-[11px] font-medium text-(--text-default) transition-colors hover:bg-(--interaction-hover-background)"
        onClick={onDeny}
        type="button"
      >
        {denyLabel}
      </button>
      <button
        className="rounded-md bg-primary px-2 py-1 text-[11px] font-medium text-white transition-colors hover:bg-primary/88"
        onClick={onAllow}
        type="button"
      >
        {allowLabel}
      </button>
    </>
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
            "truncate text-[15px] leading-7",
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
