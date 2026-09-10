/**
 * INPUT: 同一个 agent round 的结构 entry、当前身份目录/语言、相邻说话人边界与操作。
 * OUTPUT: pending、streaming、waiting 与 terminal 共用的稳定 Agent 执行外壳。
 * POS: Room 主 Feed 把 Agent entry 绑定到唯一 Assistant 展示面的薄装配层。
 */
"use client";

import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import { getAgentDisplayName } from "@/lib/agent-display-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type { GroupRoundAgentCardModel } from "./group-round-card-model";
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
  agentMentionDirectory: Required<AgentMentionDirectory>;
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
  const { t } = useI18n();
  return (
    <GroupAgentExecutionShell
      agentAvatar={agentMentionDirectory.avatars[entry.agent_id] ?? null}
      agentId={entry.agent_id}
      agentMentionDirectory={agentMentionDirectory}
      agentName={getAgentDisplayName(agentMentionDirectory.names[entry.agent_id], t)}
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
