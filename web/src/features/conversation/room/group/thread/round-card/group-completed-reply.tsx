/**
 * INPUT: 已完成 agent round 卡片、待确认权限、身份和 Thread 操作。
 * OUTPUT: 以精确 entry_id 隔离 MessageItem 状态，并继续暴露尚未解决的 Room 权限。
 * POS: Room 主 Feed 的完成态 Agent 回复与权限视图。
 */
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import { PendingPermissionList } from "@/features/conversation/shared/message/item/view/assistant/pending-permission-list";
import { CONVERSATION_ASSISTANT_CONTENT_WIDTH_CLASS_NAME } from "@/features/conversation/shared/conversation-panel-styles";
import { cn } from "@/shared/ui/class-name";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import {
  getRoomInlinePermissions,
  type GroupRoundAgentCardModel,
} from "./group-round-card-model";
import { ThreadActionButton } from "./thread-action-button";

interface GroupCompletedReplyProps {
  entry: GroupRoundAgentCardModel;
  isThreadActive: boolean;
  onClickThread: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  roundId: string;
  agentMentionDirectory?: AgentMentionDirectory;
}

export function GroupCompletedReply({
  entry,
  isThreadActive,
  onClickThread,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  roundId,
  agentMentionDirectory,
}: GroupCompletedReplyProps) {
  const inlinePermissions = getRoomInlinePermissions(entry.pendingPermissions);
  return (
    <>
      <MessageItem
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
        roundId={`${roundId}:${entry.entry_id}`}
        workspaceAgentId={entry.agent_id}
      />
      {inlinePermissions.length > 0 ? (
        <div className="nexus-chat-message-section w-full px-2 sm:px-3">
          <div
            className={cn(
              "nexus-chat-assistant-grid-expanded grid w-full grid-cols-[40px_minmax(0,1fr)] gap-3",
              CONVERSATION_ASSISTANT_CONTENT_WIDTH_CLASS_NAME,
            )}
          >
            <div aria-hidden="true" />
            <div data-room-permission-surface>
              <PendingPermissionList
                canRespond
                mode="room_result"
                onResponse={onPermissionResponse}
                permissions={inlinePermissions}
                workspaceAgentId={entry.agent_id}
              />
            </div>
          </div>
        </div>
      ) : null}
      <hr aria-hidden="true" className="conversation-round-divider" />
    </>
  );
}
