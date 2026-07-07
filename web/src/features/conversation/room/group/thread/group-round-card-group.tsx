"use client";

import { memo, useCallback, useMemo } from "react";
import { MessageItem } from "@/features/conversation/shared/message";

import { cn } from "@/lib/utils";
import { AssistantMessage, Message, RoomPendingAgentSlotState, } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";
import {
  buildRoomAgentRoundEntries,
  isAgentRoundActive,
  is_automation_trigger_user_message,
} from "@/features/conversation/shared/utils";
import { GroupAgentStatusCard } from "./group-agent-status-card";
import { useGroupThread } from "./group-thread-state";

interface GroupRoundCardGroupProps {
  roundId: string;
  messages: Message[];
  pendingPermissions?: PendingPermission[];
  pendingSlots?: RoomPendingAgentSlotState[];
  agentNameMap?: Record<string, string>;
  agentAvatarMap?: Record<string, string | null>;
  currentUserAvatar?: string | null;
  isLastRound: boolean;
  isLoading: boolean;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  onStopMessage?: (msgId: string) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
}

function getUserAttachmentWorkspaceAgentId(message: Message | undefined) {
  if (!message || message.role !== "user") {
    return null;
  }
  return message.attachments?.[0]?.workspace_agent_id ?? null;
}

function GroupCompletedReply(
  {
    roundId: roundId,
    agentId: agentId,
    agentName: agentName,
    agentAvatar: agentAvatar,
    assistantMessages: assistantMessages,
    isThreadActive: isThreadActive,
    onClickThread: onClickThread,
    onOpenAgentContact: onOpenAgentContact,
    onOpenWorkspaceFile: onOpenWorkspaceFile,
  }: {
    roundId: string;
    agentId: string;
    agentName: string;
    agentAvatar: string | null;
    assistantMessages: AssistantMessage[];
    isThreadActive: boolean;
    onClickThread: () => void;
    onOpenAgentContact?: (agentId: string) => void;
    onOpenWorkspaceFile?: (path: string) => void;
  }) {
  const messagesForRender = useMemo<Message[]>(
    () => [...assistantMessages],
    [assistantMessages],
  );

  return (
    <div className="border-b border-(--divider-subtle-color)">
      <MessageItem
        currentAgentName={agentName}
        currentAgentAvatar={agentAvatar}
        workspaceAgentId={agentId}
        roundId={`${roundId}:${agentId}`}
        messages={messagesForRender}
        assistantContentMode="room_result"
        isLastRound={false}
        isLoading={false}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        assistantHeaderAction={(
          <button
            className={cn(
              "rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
              isThreadActive
                ? "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)"
                : "border-(--divider-subtle-color) bg-transparent text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-default)",
            )}
            onClick={onClickThread}
            type="button"
          >
            {isThreadActive ? "关闭 Thread" : "查看 Thread"}
          </button>
        )}
        className="border-b-0"
      />
    </div>
  );
}

/**
 * Room 轮次卡片组：
 * 1. 用户消息与已完成回复沿用通用消息样式；
 * 2. 已完成的 Agent 回复直接进入主时间线；
 * 3. 未完成的 Agent 保持为底部占位卡片，点击进入 Thread 查看实时过程。
 * 4. 单 Agent / 多 Agent 的 Room 轮次统一走这一套渲染。
 */
function GroupRoundCardGroupInner(
  {
    roundId: roundId,
    messages,
    pendingPermissions: pendingPermissions = [],
    pendingSlots: pendingSlots = [],
    agentNameMap: agentNameMap,
    agentAvatarMap: agentAvatarMap,
    currentUserAvatar: currentUserAvatar,
    onPermissionResponse: onPermissionResponse,
    canRespondToPermissions: canRespondToPermissions = true,
    permissionReadOnlyReason: permissionReadOnlyReason,
    onStopMessage: onStopMessage,
    onOpenAgentContact: onOpenAgentContact,
    onOpenWorkspaceFile: onOpenWorkspaceFile,
  }: GroupRoundCardGroupProps) {
  const { activeThread, closeThread, openThread } = useGroupThread();

  const userMessage = useMemo(
    () => messages.find((message) => message.role === "user" && !is_automation_trigger_user_message(message)),
    [messages],
  );

  const agentEntries = useMemo(() => {
    return buildRoomAgentRoundEntries(messages, pendingSlots).map((entry) => ({
      ...entry,
      agentName: agentNameMap?.[entry.agent_id] ?? entry.agent_id,
      agentAvatar: agentAvatarMap?.[entry.agent_id] ?? null,
    }));
  }, [agentAvatarMap, agentNameMap, messages, pendingSlots]);

  const completedEntries = useMemo(
    () => agentEntries
      .filter((entry) => entry.status === "done")
      .sort((left, right) => left.timestamp - right.timestamp),
    [agentEntries],
  );

  const pendingEntries = useMemo(
    () => agentEntries.filter((entry) => entry.status !== "done"),
    [agentEntries],
  );

  const toggleThread = useCallback((agentId: string) => {
    if (activeThread?.roundId === roundId && activeThread.agentId === agentId) {
      closeThread();
      return;
    }

    openThread(roundId, agentId);
  }, [activeThread, closeThread, openThread, roundId]);

  return (
    <div className="w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 duration-300">
      {userMessage ? (
        <div className="border-b border-(--divider-subtle-color)">
          {/* 仅复用用户消息样式，传入 isLoading 避免渲染空的助手区域。 */}
          <MessageItem
            roundId={roundId}
            messages={[userMessage]}
            workspaceAgentId={getUserAttachmentWorkspaceAgentId(userMessage)}
            currentUserAvatar={currentUserAvatar}
            isLastRound={false}
            isLoading
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            className="border-b-0"
          />
        </div>
      ) : null}

      {completedEntries.map((entry) => {
        const isThreadActive = activeThread?.roundId === roundId && activeThread.agentId === entry.agent_id;

        return (
          <GroupCompletedReply
            key={entry.agent_id}
            roundId={roundId}
            agentId={entry.agent_id}
            agentName={entry.agentName}
            agentAvatar={entry.agentAvatar}
            assistantMessages={entry.assistant_messages}
            isThreadActive={isThreadActive}
            onClickThread={() => toggleThread(entry.agent_id)}
            onOpenAgentContact={onOpenAgentContact}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
          />
        );
      })}

      {pendingEntries.length > 0 ? (
        <>
          {pendingEntries.map((entry) => {
            const isThreadActive = activeThread?.roundId === roundId && activeThread.agentId === entry.agent_id;
            const entryPendingPermissions = pendingPermissions.filter(
              (permission) => permission.agent_id === entry.agent_id,
            );

            return (
              <div key={entry.agent_id} className="border-b border-(--divider-subtle-color)">
                <div className="w-full px-2 sm:px-3">
                  <div className="mx-auto w-full max-w-[980px]">
                    <GroupAgentStatusCard
                      agentId={entry.agent_id}
                      agentName={entry.agentName}
                      agentAvatar={entry.agentAvatar}
                      messages={entry.assistant_messages}
                      resultSummary={entry.result_summary}
                      pendingSlot={entry.pending_slot}
                      status={entry.status}
                      pendingPermissions={entryPendingPermissions}
                      isThreadActive={isThreadActive}
                      onClickThread={() => toggleThread(entry.agent_id)}
                      onPermissionResponse={onPermissionResponse}
                      canRespondToPermissions={canRespondToPermissions}
                      permissionReadOnlyReason={permissionReadOnlyReason}
                      onOpenAgentContact={onOpenAgentContact}
                      onStopMessage={
                        entry.pending_slot && onStopMessage && isAgentRoundActive(entry.status)
                          ? () => onStopMessage(entry.pending_slot!.agent_round_id)
                          : undefined
                      }
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </>
      ) : null}
    </div>
  );
}

export const GroupRoundCardGroup = memo(GroupRoundCardGroupInner);
