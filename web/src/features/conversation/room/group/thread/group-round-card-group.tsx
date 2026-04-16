"use client";

import { memo, useCallback, useMemo } from "react";
import { MessageItem } from "@/features/conversation/shared/message";

import { cn } from "@/lib/utils";
import { AssistantMessage, Message, ResultMessage, RoomPendingAgentSlotState, } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";
import { build_room_agent_round_entries, is_agent_round_active, } from "@/features/conversation/shared/utils";
import { GroupAgentStatusCard } from "./group-agent-status-card";
import { useGroupThread } from "./group-thread-state";

interface GroupRoundCardGroupProps {
  round_id: string;
  messages: Message[];
  pending_permissions?: PendingPermission[];
  pending_slots?: RoomPendingAgentSlotState[];
  agent_name_map?: Record<string, string>;
  agent_avatar_map?: Record<string, string | null>;
  is_last_round: boolean;
  is_loading: boolean;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_stop_message?: (msg_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

function GroupCompletedReply(
  {
    round_id,
    agent_id,
    agent_name,
    agent_avatar,
    assistant_messages,
    result_message,
    is_thread_active,
    on_click_thread,
    on_open_workspace_file,
  }: {
    round_id: string;
    agent_id: string;
    agent_name: string;
    agent_avatar: string | null;
    assistant_messages: AssistantMessage[];
    result_message?: ResultMessage;
    is_thread_active: boolean;
    on_click_thread: () => void;
    on_open_workspace_file?: (path: string) => void;
  }) {
  const messages_for_render = useMemo(() => {
    const next_messages: Message[] = [...assistant_messages];
    if (result_message) {
      next_messages.push(result_message);
    }
    return next_messages;
  }, [assistant_messages, result_message]);

  return (
    <div className="border-b border-(--divider-subtle-color)">
      <MessageItem
        current_agent_name={agent_name}
        current_agent_avatar={agent_avatar}
        round_id={`${round_id}:${agent_id}`}
        messages={messages_for_render}
        assistant_content_mode="room_result"
        is_last_round={false}
        is_loading={false}
        on_open_workspace_file={on_open_workspace_file}
        assistant_header_action={(
          <button
            className={cn(
              "rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
              is_thread_active
                ? "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)"
                : "border-(--divider-subtle-color) bg-(--material-chip-background) text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-default)",
            )}
            onClick={on_click_thread}
            type="button"
          >
            {is_thread_active ? "关闭 Thread" : "查看 Thread"}
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
    round_id,
    messages,
    pending_permissions = [],
    pending_slots = [],
    agent_name_map,
    agent_avatar_map,
    on_permission_response,
    can_respond_to_permissions = true,
    permission_read_only_reason,
    on_stop_message,
    on_open_workspace_file,
  }: GroupRoundCardGroupProps) {
  const {active_thread, close_thread, open_thread} = useGroupThread();

  const user_message = useMemo(
    () => messages.find((message) => message.role === "user"),
    [messages],
  );

  const agent_entries = useMemo(() => {
    return build_room_agent_round_entries(messages, pending_slots).map((entry) => ({
      ...entry,
      agent_name: agent_name_map?.[entry.agent_id] ?? entry.agent_id,
      agent_avatar: agent_avatar_map?.[entry.agent_id] ?? null,
    }));
  }, [agent_avatar_map, agent_name_map, messages, pending_slots]);

  const completed_entries = useMemo(
    () => agent_entries
      .filter((entry) => entry.status === "done")
      .sort((left, right) => left.timestamp - right.timestamp),
    [agent_entries],
  );

  const pending_entries = useMemo(
    () => agent_entries.filter((entry) => entry.status !== "done"),
    [agent_entries],
  );

  const toggle_thread = useCallback((agent_id: string, auto_close_on_finish = false) => {
    if (active_thread?.round_id === round_id && active_thread.agent_id === agent_id) {
      close_thread();
      return;
    }

    open_thread(round_id, agent_id, {auto_close_on_finish});
  }, [active_thread, close_thread, open_thread, round_id]);

  return (
    <div className="w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 duration-300">
      {user_message ? (
        <div className="border-b border-(--divider-subtle-color)">
          {/* 仅复用用户消息样式，传入 is_loading 避免渲染空的助手区域。 */}
          <MessageItem
            round_id={round_id}
            messages={[user_message]}
            is_last_round={false}
            is_loading
            class_name="border-b-0"
          />
        </div>
      ) : null}

      {completed_entries.map((entry) => {
        const is_thread_active = active_thread?.round_id === round_id && active_thread.agent_id === entry.agent_id;

        return (
          <GroupCompletedReply
            key={entry.agent_id}
            round_id={round_id}
            agent_id={entry.agent_id}
            agent_name={entry.agent_name}
            agent_avatar={entry.agent_avatar}
            assistant_messages={entry.assistant_messages}
            result_message={entry.result_message}
            is_thread_active={is_thread_active}
            on_click_thread={() => toggle_thread(entry.agent_id)}
            on_open_workspace_file={on_open_workspace_file}
          />
        );
      })}

      {pending_entries.length > 0 ? (
        <>
          {pending_entries.map((entry) => {
            const is_thread_active = active_thread?.round_id === round_id && active_thread.agent_id === entry.agent_id;
            const entry_pending_permissions = pending_permissions.filter(
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
                      result_message={entry.result_message}
                      pending_slot={entry.pending_slot}
                      status={entry.status}
                      pending_permissions={entry_pending_permissions}
                      is_thread_active={is_thread_active}
                      on_click_thread={() => toggle_thread(entry.agent_id, true)}
                      on_permission_response={on_permission_response}
                      can_respond_to_permissions={can_respond_to_permissions}
                      permission_read_only_reason={permission_read_only_reason}
                      on_stop_message={
                        entry.pending_slot && on_stop_message && is_agent_round_active(entry.status)
                          ? () => on_stop_message(entry.pending_slot!.msg_id)
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
