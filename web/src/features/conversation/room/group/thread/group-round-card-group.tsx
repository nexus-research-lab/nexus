"use client";

import { memo, useCallback, useMemo } from "react";
import { MessageItem } from "@/features/conversation/shared/message";

import { cn } from "@/lib/utils";
import { AssistantMessage, Message, RoomPendingAgentSlotState, } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";
import {
  build_room_agent_round_entries,
  is_agent_round_active,
  is_automation_trigger_user_message,
} from "@/features/conversation/shared/utils";
import { GroupAgentStatusCard } from "./group-agent-status-card";
import { useGroupThread } from "./group-thread-state";

interface GroupRoundCardGroupProps {
  round_id: string;
  messages: Message[];
  pending_permissions?: PendingPermission[];
  pending_slots?: RoomPendingAgentSlotState[];
  agent_name_map?: Record<string, string>;
  agent_avatar_map?: Record<string, string | null>;
  current_user_avatar?: string | null;
  is_last_round: boolean;
  is_loading: boolean;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_stop_message?: (msgId: string) => void;
  on_open_agent_contact?: (agentId: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

function getUserAttachmentWorkspaceAgentId(message: Message | undefined) {
  if (!message || message.role !== "user") {
    return null;
  }
  return message.attachments?.[0]?.workspace_agent_id ?? null;
}

function GroupCompletedReply(
  {
    round_id: roundId,
    agent_id: agentId,
    agent_name: agentName,
    agent_avatar: agentAvatar,
    assistant_messages: assistantMessages,
    is_thread_active: isThreadActive,
    on_click_thread: onClickThread,
    on_open_agent_contact: onOpenAgentContact,
    on_open_workspace_file: onOpenWorkspaceFile,
  }: {
    round_id: string;
    agent_id: string;
    agent_name: string;
    agent_avatar: string | null;
    assistant_messages: AssistantMessage[];
    is_thread_active: boolean;
    on_click_thread: () => void;
    on_open_agent_contact?: (agentId: string) => void;
    on_open_workspace_file?: (path: string) => void;
  }) {
  const messagesForRender = useMemo<Message[]>(
    () => [...assistantMessages],
    [assistantMessages],
  );

  return (
    <div className="border-b border-(--divider-subtle-color)">
      <MessageItem
        current_agent_name={agentName}
        current_agent_avatar={agentAvatar}
        workspace_agent_id={agentId}
        round_id={`${roundId}:${agentId}`}
        messages={messagesForRender}
        assistant_content_mode="room_result"
        is_last_round={false}
        is_loading={false}
        on_open_agent_contact={onOpenAgentContact}
        on_open_workspace_file={onOpenWorkspaceFile}
        assistant_header_action={(
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
        class_name="border-b-0"
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
    round_id: roundId,
    messages,
    pending_permissions: pendingPermissions = [],
    pending_slots: pendingSlots = [],
    agent_name_map: agentNameMap,
    agent_avatar_map: agentAvatarMap,
    current_user_avatar: currentUserAvatar,
    on_permission_response: onPermissionResponse,
    can_respond_to_permissions: canRespondToPermissions = true,
    permission_read_only_reason: permissionReadOnlyReason,
    on_stop_message: onStopMessage,
    on_open_agent_contact: onOpenAgentContact,
    on_open_workspace_file: onOpenWorkspaceFile,
  }: GroupRoundCardGroupProps) {
  const {active_thread: activeThread, close_thread: closeThread, open_thread: openThread} = useGroupThread();

  const userMessage = useMemo(
    () => messages.find((message) => message.role === "user" && !is_automation_trigger_user_message(message)),
    [messages],
  );

  const agentEntries = useMemo(() => {
    return build_room_agent_round_entries(messages, pendingSlots).map((entry) => ({
      ...entry,
      agent_name: agentNameMap?.[entry.agent_id] ?? entry.agent_id,
      agent_avatar: agentAvatarMap?.[entry.agent_id] ?? null,
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
    if (activeThread?.round_id === roundId && activeThread.agent_id === agentId) {
      closeThread();
      return;
    }

    openThread(roundId, agentId);
  }, [activeThread, closeThread, openThread, roundId]);

  return (
    <div className="w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 duration-300">
      {userMessage ? (
        <div className="border-b border-(--divider-subtle-color)">
          {/* 仅复用用户消息样式，传入 is_loading 避免渲染空的助手区域。 */}
          <MessageItem
            round_id={roundId}
            messages={[userMessage]}
            workspace_agent_id={getUserAttachmentWorkspaceAgentId(userMessage)}
            current_user_avatar={currentUserAvatar}
            is_last_round={false}
            is_loading
            on_open_workspace_file={onOpenWorkspaceFile}
            class_name="border-b-0"
          />
        </div>
      ) : null}

      {completedEntries.map((entry) => {
        const isThreadActive = activeThread?.round_id === roundId && activeThread.agent_id === entry.agent_id;

        return (
          <GroupCompletedReply
            key={entry.agent_id}
            round_id={roundId}
            agent_id={entry.agent_id}
            agent_name={entry.agent_name}
            agent_avatar={entry.agent_avatar}
            assistant_messages={entry.assistant_messages}
            is_thread_active={isThreadActive}
            on_click_thread={() => toggleThread(entry.agent_id)}
            on_open_agent_contact={onOpenAgentContact}
            on_open_workspace_file={onOpenWorkspaceFile}
          />
        );
      })}

      {pendingEntries.length > 0 ? (
        <>
          {pendingEntries.map((entry) => {
            const isThreadActive = activeThread?.round_id === roundId && activeThread.agent_id === entry.agent_id;
            const entryPendingPermissions = pendingPermissions.filter(
              (permission) => permission.agent_id === entry.agent_id,
            );

            return (
              <div key={entry.agent_id} className="border-b border-(--divider-subtle-color)">
                <div className="w-full px-2 sm:px-3">
                  <div className="mx-auto w-full max-w-[980px]">
                    <GroupAgentStatusCard
                      agent_id={entry.agent_id}
                      agent_name={entry.agent_name}
                      agent_avatar={entry.agent_avatar}
                      messages={entry.assistant_messages}
                      result_summary={entry.result_summary}
                      pending_slot={entry.pending_slot}
                      status={entry.status}
                      pending_permissions={entryPendingPermissions}
                      is_thread_active={isThreadActive}
                      on_click_thread={() => toggleThread(entry.agent_id)}
                      on_permission_response={onPermissionResponse}
                      can_respond_to_permissions={canRespondToPermissions}
                      permission_read_only_reason={permissionReadOnlyReason}
                      on_open_agent_contact={onOpenAgentContact}
                      on_stop_message={
                        entry.pending_slot && onStopMessage && is_agent_round_active(entry.status)
                          ? () => onStopMessage(entry.pending_slot!.msg_id)
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
