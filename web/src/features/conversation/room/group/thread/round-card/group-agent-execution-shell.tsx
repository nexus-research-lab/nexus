"use client";

/**
 * INPUT: Room Agent 执行身份、消息、stopping/人工介入状态、局部说话人边界与用户动作。
 * OUTPUT: 从 pending、stopping 到 terminal 始终复用 MessageItem 的稳定 Agent 执行外壳与单层精确控制条，首次 handoff 也直接占据真实几何位置。
 * POS: Room 主 Feed 单个 agent_round 的唯一 Assistant 展示面。
 */
import { Square } from "lucide-react";
import { memo, useMemo } from "react";

import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
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
import { isAgentRoundActive } from "../../round/round-agent-model";
import {
  hasRoomAgentExecutionDetails,
  hasRoomAgentTerminalEvidence,
  projectRoomAgentActivityState,
  projectRoomAgentExecutionMessages,
} from "./group-agent-execution-model";
import { ThreadActionButton } from "./thread-action-button";

interface GroupAgentExecutionShellProps {
  agentAvatar: string | null;
  agentId: string;
  agentMentionDirectory?: AgentMentionDirectory;
  agentName: string;
  isThreadActive: boolean;
  isStopping?: boolean;
  messages: AssistantMessage[];
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopAgentRound?: () => void;
  pendingPermissions: PendingPermission[];
  resultSummary?: ResultSummary;
  roundId: string;
  showAgentBoundary?: boolean;
  status: AgentRoundStatus;
  timestamp: number;
}

function GroupAgentExecutionShellInner({
  agentAvatar,
  agentId,
  agentMentionDirectory,
  agentName,
  isThreadActive,
  isStopping = false,
  messages,
  onClickThread,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  onPermissionResponse,
  onStopAgentRound,
  pendingPermissions,
  resultSummary,
  roundId,
  showAgentBoundary = false,
  status,
  timestamp,
}: GroupAgentExecutionShellProps) {
  const { t } = useI18n();
  const isActive = isAgentRoundActive(status);
  const hasTerminalEvidence = useMemo(
    () => hasRoomAgentTerminalEvidence(messages, resultSummary, status),
    [messages, resultSummary, status],
  );
  const isAwaitingTerminalMessage = !isActive && !hasTerminalEvidence;
  const isLoading = isActive || isAwaitingTerminalMessage;
  const projectedMessages = useMemo(
    () => projectRoomAgentExecutionMessages({
      agentId,
      labels: {
        failed: t("room.agent_status_failed"),
        stopped: t("room.agent_status_stopped"),
      },
      messages,
      resultSummary,
      roundId,
      status,
      timestamp,
    }),
    [
      agentId,
      messages,
      resultSummary,
      roundId,
      status,
      t,
      timestamp,
    ],
  );
  const activityState = projectRoomAgentActivityState({
    messages,
    pendingPermissions,
    status,
  });
  const showStop = isActive && Boolean(onStopAgentRound);
  const showThread = isActive
    || pendingPermissions.length > 0
    || hasRoomAgentExecutionDetails(messages);
  const terminalLabel = status === "cancelled"
    ? t("room.agent_status_stopped")
    : status === "error"
      ? t("room.agent_status_failed")
      : null;
  const stopLabel = t(isStopping
    ? "room.agent_stopping"
    : "room.agent_stop");
  const stopActionLabel = t(isStopping
    ? "room.agent_stopping"
    : "room.agent_stop_action");

  return (
    <div
      data-room-agent-execution-shell={roundId}
      className="room-agent-execution-shell w-full min-w-0"
    >
      {showAgentBoundary ? (
        <div
          aria-hidden="true"
          className="conversation-agent-boundary"
          data-conversation-agent-boundary
        />
      ) : null}
      <MessageItem
        agentMentionDirectory={agentMentionDirectory}
        animateEntry={false}
        assistantContentMode="room_result"
        assistantHeaderAction={showThread || showStop || terminalLabel ? (
          <div
            aria-label={t("room.agent_actions")}
            className="inline-flex h-7 items-center rounded-lg bg-(--surface-control-field-background) p-0.5"
            data-room-agent-execution-actions
            role="group"
          >
            {terminalLabel ? (
              <span className="px-1.5 text-xs text-(--text-muted)">
                {terminalLabel}
              </span>
            ) : null}
            {showStop ? (
              <button
                aria-label={stopActionLabel}
                className="inline-flex h-6 items-center gap-1 rounded-md px-1.5 text-xs text-(--text-muted) transition-colors hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:cursor-wait disabled:opacity-70"
                data-room-agent-action="stop"
                disabled={isStopping}
                onClick={onStopAgentRound}
                title={stopActionLabel}
                type="button"
              >
                <Square className="h-3 w-3 fill-current" />
                <span className="hidden sm:inline">{stopLabel}</span>
              </button>
            ) : null}
            {(showStop || terminalLabel) && showThread ? (
              <span
                aria-hidden="true"
                className="mx-0.5 h-3.5 w-px bg-(--divider-subtle-color)"
              />
            ) : null}
            {showThread ? (
              <ThreadActionButton
                active={isThreadActive}
                onClick={onClickThread}
              />
            ) : null}
          </div>
        ) : undefined}
        currentAgentAvatar={agentAvatar}
        currentAgentName={agentName}
        activityState={activityState}
        isLastRound
        isLoading={isLoading}
        messages={projectedMessages}
        onOpenAgentContact={onOpenAgentContact}
        onOpenSubagentTask={onOpenSubagentTask}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onPermissionResponse={onPermissionResponse}
        pendingPermissions={pendingPermissions}
        roundId={roundId}
        unresolvedToolStatus={status === "cancelled"
          ? "stopped"
          : status === "error" ? "error" : undefined}
        workspaceAgentId={agentId}
      />
    </div>
  );
}

export const GroupAgentExecutionShell = memo(GroupAgentExecutionShellInner);
