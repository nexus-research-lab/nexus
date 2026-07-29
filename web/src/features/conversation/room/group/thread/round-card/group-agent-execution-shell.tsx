"use client";

/**
 * INPUT: Room Agent 执行身份、消息、人工介入状态、局部说话人边界与用户动作。
 * OUTPUT: 从 pending 到 terminal 始终复用 MessageItem 的稳定 Agent 执行外壳；首次 handoff 只做一次不重挂内容的轻量淡入。
 * POS: Room 主 Feed 单个 agent_round 的唯一 Assistant 展示面。
 */
import { Square } from "lucide-react";
import { memo, useCallback, useMemo } from "react";

import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
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
  hasRoomAgentTerminalEvidence,
  isRoomAgentNoPublicReply,
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
  messages: AssistantMessage[];
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
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
  messages,
  onClickThread,
  onOpenAgentContact,
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
  const noPublicReply = isRoomAgentNoPublicReply(
    messages,
    resultSummary,
    status,
  );
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
  const handleStopMessage = useCallback(() => {
    onStopAgentRound?.();
  }, [onStopAgentRound]);
  const showFallbackStop = isActive
    && messages.length === 0
    && Boolean(onStopAgentRound);

  return (
    <div
      data-room-agent-execution-shell={roundId}
      className={cn(
        "room-agent-execution-shell w-full min-w-0",
        isActive && messages.length === 0 && "room-agent-execution-enter",
      )}
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
        assistantEmptyState={noPublicReply ? (
          <p className="text-base leading-7 text-(--text-muted)">
            {t("room.agent_status_no_reply")}
          </p>
        ) : undefined}
        assistantHeaderAction={(
          <div className="flex items-center gap-1.5">
            <ThreadActionButton
              active={isThreadActive}
              onClick={onClickThread}
            />
            {showFallbackStop ? (
              <button
                aria-label={t("room.agent_stop")}
                className="flex h-7 items-center gap-1 rounded-md px-2 text-xs text-(--text-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--text-default)"
                onClick={onStopAgentRound}
                title={t("room.agent_stop")}
                type="button"
              >
                <Square className="h-3 w-3 fill-current" />
                <span className="hidden sm:inline">
                  {t("room.agent_stop")}
                </span>
              </button>
            ) : null}
          </div>
        )}
        currentAgentAvatar={agentAvatar}
        currentAgentName={agentName}
        activityState={activityState}
        isLastRound
        isLoading={isLoading}
        messages={projectedMessages}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onPermissionResponse={onPermissionResponse}
        onStopMessage={
          isActive && messages.length > 0 && onStopAgentRound
            ? handleStopMessage
            : undefined
        }
        pendingPermissions={pendingPermissions}
        roundId={roundId}
        workspaceAgentId={agentId}
      />
    </div>
  );
}

export const GroupAgentExecutionShell = memo(GroupAgentExecutionShellInner);
