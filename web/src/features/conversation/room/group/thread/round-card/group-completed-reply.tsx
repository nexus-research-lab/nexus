/**
 * INPUT: 已完成 agent round 卡片、身份和 Thread 操作。
 * OUTPUT: 以精确 entry_id 隔离 MessageItem 状态的 Room 最终回复。
 * POS: Room 主 Feed 的完成态 Agent 回复视图。
 */
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";

import type { GroupRoundAgentCardModel } from "./group-round-card-model";
import { ThreadActionButton } from "./thread-action-button";

interface GroupCompletedReplyProps {
  entry: GroupRoundAgentCardModel;
  isThreadActive: boolean;
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  roundId: string;
}

export function GroupCompletedReply({
  entry,
  isThreadActive,
  onClickThread,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  roundId,
}: GroupCompletedReplyProps) {
  return (
    <div className="border-b border-(--divider-subtle-color)">
      <MessageItem
        assistantContentMode="room_result"
        assistantHeaderAction={(
          <ThreadActionButton
            active={isThreadActive}
            onClick={onClickThread}
          />
        )}
        className="border-b-0"
        currentAgentAvatar={entry.agentAvatar}
        currentAgentName={entry.agentName}
        isLastRound={false}
        isLoading={false}
        messages={entry.assistant_messages}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        roundId={`${roundId}:${entry.entry_id}`}
        workspaceAgentId={entry.agent_id}
      />
    </div>
  );
}
