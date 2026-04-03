"use client";

import { memo, useCallback, useMemo } from "react";
import { MessageItem } from "@/features/conversation-shared/message";

import { cn } from "@/lib/utils";
import { AssistantMessage, Message, ResultMessage } from "@/types/message";
import {
  buildRoomAgentRoundEntries,
  RoomAgentRoundEntry,
  isAgentRoundActive,
} from "@/features/conversation-shared/utils";
import { AgentStatusCard } from "./agent-status-card";
import { useRoomThread } from "./room-thread-context";

interface RoomRoundCardGroupProps {
  round_id: string;
  messages: Message[];
  agent_name_map?: Record<string, string>;
  is_last_round: boolean;
  is_loading: boolean;
  on_stop_message?: (msg_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

interface RoomAgentEntry extends RoomAgentRoundEntry {
  agent_name: string;
}

function RoomCompletedReply({
  round_id,
  agent_id,
  agent_name,
  assistant_messages,
  result_message,
  is_thread_active,
  on_click_thread,
  on_open_workspace_file,
}: {
  round_id: string;
  agent_id: string;
  agent_name: string;
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
    <div className="border-b border-slate-200/75">
      <MessageItem
        current_agent_name={agent_name}
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
                ? "border-[#cfe0ff] bg-[#eff6ff] text-[#27539d]"
                : "border-slate-200 bg-white text-slate-500 hover:bg-slate-50 hover:text-slate-700",
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
function RoomRoundCardGroupInner({
  round_id,
  messages,
  agent_name_map,
  is_last_round,
  is_loading,
  on_stop_message,
  on_open_workspace_file,
}: RoomRoundCardGroupProps) {
  const { active_thread, close_thread, open_thread } = useRoomThread();

  const user_message = useMemo(
    () => messages.find((message) => message.role === "user"),
    [messages],
  );

  const agent_entries = useMemo(() => {
    return buildRoomAgentRoundEntries(messages).map((entry) => ({
      ...entry,
      agent_name: agent_name_map?.[entry.agent_id] ?? entry.agent_id,
    }));
  }, [agent_name_map, messages]);

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

    open_thread(round_id, agent_id, { auto_close_on_finish });
  }, [active_thread, close_thread, open_thread, round_id]);

  return (
    <div className="w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 duration-300">
      {user_message ? (
        <div className="border-b border-slate-200/75">
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
          <RoomCompletedReply
            key={entry.agent_id}
            round_id={round_id}
            agent_id={entry.agent_id}
            agent_name={entry.agent_name}
            assistant_messages={entry.assistant_messages}
            result_message={entry.result_message}
            is_thread_active={is_thread_active}
            on_click_thread={() => toggle_thread(entry.agent_id)}
            on_open_workspace_file={on_open_workspace_file}
          />
        );
      })}

      {pending_entries.length > 0 ? (
        <div className="border-b border-slate-200/75 py-3">
          <div className="w-full px-2 sm:px-3">
            <div className="mx-auto w-full max-w-[980px]">
              <div className="grid min-w-0 grid-cols-[40px_minmax(0,1fr)] gap-3">
                <div />
                <div className="rounded-[24px] border border-dashed border-slate-200 bg-[linear-gradient(180deg,rgba(252,252,253,0.96),rgba(246,248,250,0.94))] px-3 py-3">
                  <div className="mb-2 flex items-center justify-between gap-3 text-[11px] font-medium text-slate-500">
                    <span>{is_last_round && is_loading ? "协作进行中" : "待处理回复"}</span>
                    <span>{pending_entries.length} 个占位卡片</span>
                  </div>

                  <div className="flex flex-col gap-2">
                    {pending_entries.map((entry) => {
                      const is_thread_active = active_thread?.round_id === round_id && active_thread.agent_id === entry.agent_id;
                      const stoppable_message = entry.assistant_messages.find((message) => (
                        message.stream_status === "pending" || message.stream_status === "streaming"
                      ));

                      return (
                        <AgentStatusCard
                          key={entry.agent_id}
                          agent_id={entry.agent_id}
                          agent_name={entry.agent_name}
                          messages={entry.assistant_messages}
                          result_message={entry.result_message}
                          is_thread_active={is_thread_active}
                          on_click_thread={() => toggle_thread(entry.agent_id, true)}
                          on_stop_message={
                            stoppable_message && on_stop_message && isAgentRoundActive(entry.status)
                              ? () => on_stop_message(stoppable_message.message_id)
                              : undefined
                          }
                        />
                      );
                    })}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export const RoomRoundCardGroup = memo(RoomRoundCardGroupInner);
