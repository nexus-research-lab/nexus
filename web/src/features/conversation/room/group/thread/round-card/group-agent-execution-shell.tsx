"use client";

/**
 * INPUT: Room Agent 执行身份、消息、stopping/人工介入状态、局部说话人边界与用户动作。
 * OUTPUT: 始终复用 MessageItem 的稳定执行外壳与共享 xs 动作；精确停止/Thread 命令、忙碌和终态沿同一控制条投影。
 * POS: Room 主 Feed 单个 agent_round 的唯一 Assistant 展示面。
 */
import { Square } from "lucide-react";
import { memo, useMemo, type ReactNode } from "react";

import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
          <RoomAgentExecutionActions>
            {terminalLabel ? (
              <span className={cn("px-2", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
                {terminalLabel}
              </span>
            ) : null}
            {showStop ? (
              <RoomAgentStopButton isStopping={isStopping} onClick={onStopAgentRound!} />
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
                agentName={agentName}
                onClick={onClickThread}
              />
            ) : null}
          </RoomAgentExecutionActions>
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

export function RoomAgentStopButton({isStopping = false, onClick}: {isStopping?: boolean; onClick: () => void}) {
  const {t} = useI18n();
  const label = t(isStopping ? "room.agent_stopping" : "room.agent_stop_action");
  return <UiButton aria-busy={isStopping || undefined} aria-label={label} title={label}
    data-room-agent-action="stop" disabled={isStopping} onClick={onClick} size="xs" tone="danger" variant="text">
    <Square aria-hidden="true" className="h-3.5 w-3.5 fill-current" />
    <span className="hidden sm:inline">{t(isStopping ? "room.agent_stopping" : "room.agent_stop")}</span>
  </UiButton>;
}

/** 本地执行和在线投递复用同一动作条；动作权限仍由各自的数据源决定。 */
export function RoomAgentExecutionActions({children}: {children: ReactNode}) {
  const { t } = useI18n();
  return <div aria-label={t("room.agent_actions")}
    className="radius-control-sm inline-flex min-h-8 items-center bg-(--surface-control-field-background) p-0.5"
    data-room-agent-execution-actions role="group">{children}</div>;
}
