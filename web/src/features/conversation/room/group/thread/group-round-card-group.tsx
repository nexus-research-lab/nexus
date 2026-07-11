"use client";

import { memo, useCallback, useMemo } from "react";

import { MessageItem } from "@/features/conversation/shared/message";
import type { Message, RoomPendingAgentSlotState } from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import { GroupAgentStatusCard } from "./group-agent-status-card";
import { GroupCompletedReply } from "./group-completed-reply";
import { buildGroupRoundCardModel } from "./group-round-card-model";
import { useGroupThread } from "./group-thread-state";

interface GroupRoundCardGroupProps {
  agentAvatarMap?: Record<string, string | null>;
  agentNameMap?: Record<string, string>;
  currentUserAvatar?: string | null;
  messages: Message[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onStopMessage?: (msgId: string) => void;
  pendingPermissions?: PendingPermission[];
  pendingSlots?: RoomPendingAgentSlotState[];
  roundId: string;
}

function GroupRoundCardGroupInner({
  agentAvatarMap,
  agentNameMap,
  currentUserAvatar,
  messages,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  onStopMessage,
  pendingPermissions = [],
  pendingSlots = [],
  roundId,
}: GroupRoundCardGroupProps) {
  const { activeThread, closeThread, openThread } = useGroupThread();
  const model = useMemo(
    () => buildGroupRoundCardModel({
      agentAvatarMap,
      agentNameMap,
      messages,
      pendingPermissions,
      pendingSlots,
    }),
    [
      agentAvatarMap,
      agentNameMap,
      messages,
      pendingPermissions,
      pendingSlots,
    ],
  );
  const activeAgentId = activeThread?.roundId === roundId
    ? activeThread.agentId
    : null;
  const toggleThread = useCallback((agentId: string) => {
    if (activeAgentId === agentId) {
      closeThread();
      return;
    }
    openThread(roundId, agentId);
  }, [activeAgentId, closeThread, openThread, roundId]);

  return (
    <div className="w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 duration-300">
      {model.userMessage ? (
        <div className="border-b border-(--divider-subtle-color)">
          {/* 用户消息沿用通用样式，但不渲染尚未出现的助手区域。 */}
          <MessageItem
            className="border-b-0"
            currentUserAvatar={currentUserAvatar}
            isLastRound={false}
            isLoading
            messages={[model.userMessage]}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            roundId={roundId}
            workspaceAgentId={model.userWorkspaceAgentId}
          />
        </div>
      ) : null}

      {model.completedEntries.map((entry) => (
        <GroupCompletedReply
          entry={entry}
          isThreadActive={activeAgentId === entry.agent_id}
          key={entry.agent_id}
          onClickThread={() => toggleThread(entry.agent_id)}
          onOpenAgentContact={onOpenAgentContact}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          roundId={roundId}
        />
      ))}

      {model.pendingEntries.map((entry) => {
        const stopMessageId = entry.stopMessageId;
        return (
          <div
            className="border-b border-(--divider-subtle-color)"
            key={entry.agent_id}
          >
            <div className="w-full px-2 sm:px-3">
              <div className="mx-auto w-full max-w-[980px]">
                <GroupAgentStatusCard
                  agentAvatar={entry.agentAvatar}
                  agentId={entry.agent_id}
                  agentName={entry.agentName}
                  isThreadActive={activeAgentId === entry.agent_id}
                  messages={entry.assistant_messages}
                  onClickThread={() => toggleThread(entry.agent_id)}
                  onOpenAgentContact={onOpenAgentContact}
                  onPermissionResponse={onPermissionResponse}
                  onStopMessage={
                    stopMessageId && onStopMessage
                      ? () => onStopMessage(stopMessageId)
                      : undefined
                  }
                  pendingPermissions={entry.pendingPermissions}
                  pendingSlot={entry.pending_slot}
                  resultSummary={entry.result_summary}
                  status={entry.status}
                />
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export const GroupRoundCardGroup = memo(GroupRoundCardGroupInner);
