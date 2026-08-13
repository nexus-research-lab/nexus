/**
 * INPUT: 同一个 agent round 的消息、人工介入、身份、相邻说话人边界与操作。
 * OUTPUT: pending、streaming、waiting 与 terminal 共用的稳定 Agent 执行外壳。
 * POS: Room 主 Feed 把 Agent entry 绑定到唯一 Assistant 展示面的薄装配层。
 */
"use client";

import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type { GroupRoundAgentCardModel } from "./group-round-card-model";
import { isRoomAgentNoPublicReply } from "./group-agent-execution-model";
import { GroupAgentExecutionShell } from "./group-agent-execution-shell";

interface GroupAgentReplyProps {
  entry: GroupRoundAgentCardModel;
  isThreadActive: boolean;
  isStopping?: boolean;
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopAgentRound?: () => void;
  roundId: string;
  showAgentBoundary?: boolean;
  agentMentionDirectory?: AgentMentionDirectory;
}

export function GroupAgentReply({
  entry,
  isThreadActive,
  isStopping = false,
  onClickThread,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  onPermissionResponse,
  onStopAgentRound,
  roundId,
  showAgentBoundary,
  agentMentionDirectory,
}: GroupAgentReplyProps) {
  if (isRoomAgentNoPublicReply(
    entry.assistant_messages,
    entry.result_summary,
    entry.status,
  )) {
    return null;
  }

  return (
    <GroupAgentExecutionShell
      agentAvatar={entry.agentAvatar}
      agentId={entry.agent_id}
      agentMentionDirectory={agentMentionDirectory}
      agentName={entry.agentName}
      isThreadActive={isThreadActive}
      isStopping={isStopping}
      messages={entry.assistant_messages}
      onClickThread={onClickThread}
      onOpenAgentContact={onOpenAgentContact}
      onOpenSubagentTask={onOpenSubagentTask}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      onPermissionResponse={onPermissionResponse}
      onStopAgentRound={onStopAgentRound}
      pendingPermissions={entry.pendingPermissions}
      resultSummary={entry.result_summary}
      roundId={`${roundId}:${entry.entry_id}`}
      showAgentBoundary={showAgentBoundary}
      status={entry.status}
      timestamp={entry.timestamp}
    />
  );
}
