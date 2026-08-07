/**
 * INPUT: 一个 Room feed 节点的 root/agent_round 子集与交互回调。
 * OUTPUT: global user、目标 agent round 紧前的定向 guided user、对应回复卡片。
 * POS: Group round 卡片的渲染顺序真相源。
 */
"use client";

import { Fragment, memo, useCallback, useMemo } from "react";

import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import type { Message } from "@/types/conversation/message/entity";
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import { GroupAgentReply } from "./group-agent-reply";
import {
  buildGroupRoundCardModel,
  type GroupRoundUserMessageModel,
} from "./group-round-card-model";
import { useGroupThread } from "../group-thread-state";

interface GroupRoundCardGroupProps {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  messages: Message[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopAgentRound: (agentRoundId: string) => void;
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
  roomAgentExecutionStates: RoomAgentExecutionState[];
  roundId: string;
  stoppingAgentRoundIds: string[];
}

function GroupRoundCardGroupInner({
  agentAvatarMap,
  agentNameMap,
  messages,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  onPermissionResponse,
  onStopAgentRound,
  pendingPermissions,
  pendingSlots,
  roomAgentExecutionStates,
  roundId,
  stoppingAgentRoundIds = [],
}: GroupRoundCardGroupProps) {
  const { activeThread, closeThread, openThread } = useGroupThread();
  const model = useMemo(
    () => buildGroupRoundCardModel({
      agentAvatarMap,
      agentNameMap,
      executionStates: roomAgentExecutionStates,
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
      roomAgentExecutionStates,
    ],
  );
  const toggleThread = useCallback((
    agentId: string,
    agentRoundId: string | null,
  ) => {
    if (
      activeThread?.roundId === roundId
      && activeThread.agentId === agentId
      && activeThread.agentRoundId === agentRoundId
    ) {
      closeThread();
      return;
    }
    openThread(roundId, agentId, agentRoundId);
  }, [activeThread, closeThread, openThread, roundId]);

  return (
    <div className="w-full min-w-0">
      {model.userMessages.map((item) => (
        <GroupUserMessage
          agentAvatarMap={agentAvatarMap}
          agentNameMap={agentNameMap}
          item={item}
          onOpenAgentContact={onOpenAgentContact}
          key={item.message.message_id}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          roundId={roundId}
        />
      ))}

      {model.entries.map((entry, entryIndex) => {
        const isThreadActive = activeThread?.roundId === roundId
          && activeThread.agentId === entry.agent_id
          && activeThread.agentRoundId === entry.agent_round_id;
        const toggleEntryThread = () => toggleThread(
          entry.agent_id,
          entry.agent_round_id,
        );
        const stopAgentRoundId = entry.stopAgentRoundId;
        return (
          <Fragment key={entry.entry_id}>
            {entry.guidedUserMessages.map((item) => (
              <GroupUserMessage
                agentAvatarMap={agentAvatarMap}
                agentNameMap={agentNameMap}
                item={item}
                onOpenAgentContact={onOpenAgentContact}
                key={item.message.message_id}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                roundId={roundId}
              />
            ))}
            <GroupAgentReply
              entry={entry}
              isThreadActive={isThreadActive}
              isStopping={Boolean(
                entry.agent_round_id
                && stoppingAgentRoundIds.includes(entry.agent_round_id)
              )}
              onClickThread={toggleEntryThread}
              onOpenAgentContact={onOpenAgentContact}
              onOpenSubagentTask={onOpenSubagentTask}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              onPermissionResponse={onPermissionResponse}
              onStopAgentRound={
                stopAgentRoundId
                  ? () => onStopAgentRound(stopAgentRoundId)
                  : undefined
              }
              agentMentionDirectory={{ avatars: agentAvatarMap, names: agentNameMap }}
              roundId={roundId}
              showAgentBoundary={
                entryIndex > 0 && entry.guidedUserMessages.length === 0
              }
            />
          </Fragment>
        );
      })}
    </div>
  );
}

function GroupUserMessage({
  agentAvatarMap,
  agentNameMap,
  item,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  roundId,
}: {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  item: GroupRoundUserMessageModel;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  roundId: string;
}) {
  return (
    <div className="border-b border-(--divider-subtle-color)">
      {/* 用户消息沿用通用样式，但不渲染尚未出现的助手区域。 */}
      <MessageItem
        animateEntry={false}
        className="border-b-0"
        agentMentionDirectory={{ avatars: agentAvatarMap, names: agentNameMap }}
        isLastRound={false}
        messages={[item.message]}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        roundId={roundId}
        workspaceAgentId={item.workspaceAgentId}
      />
    </div>
  );
}

export const GroupRoundCardGroup = memo(GroupRoundCardGroupInner);
