/**
 * INPUT: 同一个 agent round 的运行摘要、终态结果、人工介入与操作。
 * OUTPUT: 保留运行态摘要，并在原槽位直接切换终态正文的回复面。
 * POS: Room 主 Feed 从状态摘要切换到公开结果的连续性边界。
 */
"use client";

import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type { GroupRoundAgentCardModel } from "./group-round-card-model";
import { GroupAgentStatusCard } from "./group-agent-status-card";
import { ThreadActionButton } from "./thread-action-button";
import { isAgentRoundActive } from "../../round/round-agent-model";

interface GroupAgentReplyProps {
  entry: GroupRoundAgentCardModel;
  isThreadActive: boolean;
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopAgentRound?: () => void;
  roundId: string;
  agentMentionDirectory?: AgentMentionDirectory;
}

export function GroupAgentReply({
  entry,
  isThreadActive,
  onClickThread,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  onStopAgentRound,
  roundId,
  agentMentionDirectory,
}: GroupAgentReplyProps) {
  const hasTerminalMessage = Boolean(
    entry.result_summary
    || entry.assistant_messages.some((message) => (
      message.is_complete
      || message.stop_reason
      || message.stream_status === "cancelled"
      || message.stream_status === "done"
      || message.stream_status === "error"
    )),
  );
  const shouldKeepStatusCard = (
    isAgentRoundActive(entry.status)
    || !hasTerminalMessage
  );

  if (shouldKeepStatusCard) {
    return (
      <>
        <GroupAgentStatusCard
          agentAvatar={entry.agentAvatar}
          agentId={entry.agent_id}
          agentName={entry.agentName}
          isThreadActive={isThreadActive}
          messages={entry.assistant_messages}
          onClickThread={onClickThread}
          onOpenAgentContact={onOpenAgentContact}
          onPermissionResponse={onPermissionResponse}
          onStopAgentRound={onStopAgentRound}
          pendingPermissions={entry.pendingPermissions}
          resultSummary={entry.result_summary}
          status={entry.status}
          timestamp={entry.timestamp}
        />
        <hr aria-hidden="true" className="conversation-round-divider" />
      </>
    );
  }

  return (
    <>
      <MessageItem
        animateEntry={false}
        assistantContentMode="room_result"
        assistantHeaderAction={(
          <ThreadActionButton
            active={isThreadActive}
            onClick={onClickThread}
          />
        )}
        currentAgentAvatar={entry.agentAvatar}
        currentAgentName={entry.agentName}
        agentMentionDirectory={agentMentionDirectory}
        isLastRound={false}
        isLoading={false}
        messages={entry.assistant_messages}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onPermissionResponse={onPermissionResponse}
        pendingPermissions={entry.pendingPermissions}
        roundId={`${roundId}:${entry.entry_id}`}
        workspaceAgentId={entry.agent_id}
      />
      <hr aria-hidden="true" className="conversation-round-divider" />
    </>
  );
}
