"use client";

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useConversationLoader } from "@/hooks/use-conversation-loader";
import { useExtractTodos } from "@/hooks/use-extract-todos";
import { Message } from "@/types/message";
import { ConversationSnapshotPayload } from "@/types/conversation";
import { TodoItem } from "@/types/todo";

import { RoomComposerPanel } from "./room-composer-panel";
import { RoomConversationEmptyState } from "./room-conversation-empty-state";
import { RoomConversationFeed } from "./room-conversation-feed";
import { RoomConversationHeader } from "./room-conversation-header";
import { RoomScrollToLatestButton } from "./room-scroll-to-latest-button";

interface RoomChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  current_room_title?: string | null;
  session_key: string | null;
  session_title?: string | null;
  on_create_conversation?: (title?: string) => void | Promise<string | null>;
  layout?: "desktop" | "mobile";
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: ConversationSnapshotPayload) => void;
  hide_header?: boolean;
}

const BOTTOM_THRESHOLD_PX = 80;

function groupMessagesByRound(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();

  for (const message of messages) {
    const round_id = message.round_id || message.message_id;
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(message);
  }

  return groups;
}

export function RoomChatPanel({
  agent_id,
  current_agent_name,
  current_room_title,
  session_key: external_session_key,
  session_title,
  on_create_conversation,
  layout = "desktop",
  on_open_workspace_file,
  on_todos_change,
  on_loading_change,
  on_conversation_snapshot_change,
  hide_header = false,
}: RoomChatPanelProps) {
  const is_mobile_layout = layout === "mobile";
  const scroll_ref = useRef<HTMLDivElement>(null);
  const feed_ref = useRef<HTMLDivElement>(null);
  const bottom_anchor_ref = useRef<HTMLDivElement>(null);
  const should_follow_latest_ref = useRef(true);
  const last_scroll_top_ref = useRef(0);
  const pending_scroll_frame_ref = useRef<number | null>(null);
  const pending_scroll_inner_frame_ref = useRef<number | null>(null);
  const touch_start_y_ref = useRef<number | null>(null);
  const [show_scroll_to_bottom, setShowScrollToBottom] = useState(false);

  const {
    error,
    messages,
    session_key,
    is_loading,
    pending_permission,
    send_message,
    stop_generation,
    load_conversation,
    send_permission_response,
    delete_round,
    regenerate,
  } = useAgentConversation({
    agent_id,
    on_error: (err) => {
      console.error("Conversation error:", err);
    },
  });

  const todos = useExtractTodos(messages, external_session_key);

  useEffect(() => {
    on_todos_change?.(todos);
  }, [on_todos_change, todos]);

  useEffect(() => {
    on_loading_change?.(is_loading);
  }, [is_loading, on_loading_change]);

  useEffect(() => {
    if (!external_session_key) {
      return;
    }

    const last_message = messages[messages.length - 1];
    on_conversation_snapshot_change?.({
      conversation_id: external_session_key,
      message_count: messages.length,
      last_activity_at: last_message?.timestamp ?? Date.now(),
      session_id: last_message?.session_id ?? null,
    });
  }, [external_session_key, messages, on_conversation_snapshot_change]);

  useConversationLoader({
    conversation_id: external_session_key,
    load_conversation,
    debug_name: "RoomChatPanel",
  });

  const message_groups = useMemo(() => groupMessagesByRound(messages), [messages]);

  const update_follow_state = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    const distance_to_bottom = container.scrollHeight - container.scrollTop - container.clientHeight;
    const is_near_bottom = distance_to_bottom <= BOTTOM_THRESHOLD_PX;
    should_follow_latest_ref.current = is_near_bottom;
    setShowScrollToBottom(!is_near_bottom);
  }, []);

  const cancel_pending_scroll = useCallback(() => {
    if (pending_scroll_frame_ref.current !== null) {
      cancelAnimationFrame(pending_scroll_frame_ref.current);
      pending_scroll_frame_ref.current = null;
    }
    if (pending_scroll_inner_frame_ref.current !== null) {
      cancelAnimationFrame(pending_scroll_inner_frame_ref.current);
      pending_scroll_inner_frame_ref.current = null;
    }
  }, []);

  const schedule_scroll_to_bottom = useCallback((behavior: ScrollBehavior = "smooth") => {
    cancel_pending_scroll();

    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    // 流式输出时直接贴到底部，避免等待两帧后再修正位置导致换行抖动。
    if (behavior === "auto") {
      container.scrollTop = container.scrollHeight;
      last_scroll_top_ref.current = container.scrollTop;
      return;
    }

    pending_scroll_frame_ref.current = requestAnimationFrame(() => {
      const nextContainer = scroll_ref.current;
      if (!nextContainer) {
        return;
      }

      nextContainer.scrollTo({
        top: nextContainer.scrollHeight,
        behavior,
      });

      last_scroll_top_ref.current = nextContainer.scrollTop;
    });
  }, [cancel_pending_scroll]);

  const scroll_to_bottom = useCallback((behavior: ScrollBehavior = "smooth") => {
    should_follow_latest_ref.current = true;
    setShowScrollToBottom(false);
    schedule_scroll_to_bottom(behavior);
  }, [schedule_scroll_to_bottom]);

  useLayoutEffect(() => {
    if (!should_follow_latest_ref.current) {
      update_follow_state();
      return;
    }

    schedule_scroll_to_bottom(is_loading ? "auto" : "smooth");
  }, [is_loading, messages, schedule_scroll_to_bottom, update_follow_state]);

  useEffect(() => {
    const feed = feed_ref.current;
    if (!feed || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      if (!should_follow_latest_ref.current) {
        update_follow_state();
        return;
      }

      schedule_scroll_to_bottom("auto");
    });

    observer.observe(feed);

    return () => {
      observer.disconnect();
    };
  }, [schedule_scroll_to_bottom, update_follow_state]);

  useEffect(() => {
    update_follow_state();
    last_scroll_top_ref.current = scroll_ref.current?.scrollTop || 0;
  }, [update_follow_state, external_session_key]);

  useEffect(() => {
    return () => {
      cancel_pending_scroll();
    };
  }, [cancel_pending_scroll]);

  const handle_scroll = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    const current_scroll_top = container.scrollTop;
    const is_scrolling_up = current_scroll_top < last_scroll_top_ref.current;
    last_scroll_top_ref.current = current_scroll_top;

    if (is_scrolling_up) {
      cancel_pending_scroll();
    }

    update_follow_state();
  }, [cancel_pending_scroll, update_follow_state]);

  const handle_wheel = useCallback((event: React.WheelEvent<HTMLDivElement>) => {
    if (event.deltaY < 0) {
      cancel_pending_scroll();
    }
  }, [cancel_pending_scroll]);

  const handle_touch_start = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    touch_start_y_ref.current = event.touches[0]?.clientY ?? null;
  }, []);

  const handle_touch_move = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    const current_y = event.touches[0]?.clientY;
    if (current_y === undefined || touch_start_y_ref.current === null) {
      return;
    }
    if (current_y > touch_start_y_ref.current) {
      cancel_pending_scroll();
    }
  }, [cancel_pending_scroll]);

  const handle_touch_end = useCallback(() => {
    touch_start_y_ref.current = null;
  }, []);

  const handle_jump_to_bottom = useCallback(() => {
    scroll_to_bottom("smooth");
  }, [scroll_to_bottom]);

  const handle_send_message = async (content: string) => {
    if (!content.trim() || is_loading) {
      return;
    }
    should_follow_latest_ref.current = true;
    setShowScrollToBottom(false);
    scroll_to_bottom("auto");

    try {
      await send_message(content);
    } catch (error) {
      console.error("Failed to send message:", error);
    }
  };

  const handle_stop = () => {
    stop_generation();
  };

  const round_ids = Array.from(message_groups.keys());

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

      {error && error.includes("服务器") ? (
        <div className="absolute left-1/2 top-4 z-50 max-w-md -translate-x-1/2">
          <div className="rounded-2xl border border-destructive/20 bg-destructive/8 p-3 backdrop-blur-sm">
            <div className="flex items-start gap-3">
              <svg className="mt-0.5 h-5 w-5 shrink-0 text-destructive" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                />
              </svg>
              <div className="flex-1">
                <p className="text-sm font-medium text-destructive">无法连接到后端服务</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  请确保后端服务正在运行 (端口 8010)
                </p>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {!external_session_key ? (
        <RoomConversationEmptyState on_create_conversation={on_create_conversation ?? (() => {})} />
      ) : (
        <>
          {!is_mobile_layout && !hide_header ? (
            <RoomConversationHeader
              active_tab="chat"
              conversation_count={1}
              current_agent_name={current_agent_name ?? null}
              current_conversation_id={session_key}
              current_room_type="room"
              current_room_title={current_room_title ?? null}
              current_conversation_title={session_title ?? null}
              is_loading={is_loading}
              member_count={1}
              on_change_tab={() => {}}
            />
          ) : null}

          <div
            ref={scroll_ref}
            className={
              is_mobile_layout
                ? "soft-scrollbar relative z-0 min-w-0 flex-1 space-y-4 overflow-x-hidden overflow-y-auto px-1 py-2"
                : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto bg-[#fcfcfd] px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
            }
            style={{ overflowAnchor: "none" }}
            onScroll={handle_scroll}
            onTouchEnd={handle_touch_end}
            onTouchMove={handle_touch_move}
            onTouchStart={handle_touch_start}
            onWheel={handle_wheel}
          >
            <RoomConversationFeed
              bottom_anchor_ref={bottom_anchor_ref}
              feed_ref={feed_ref}
              current_agent_name={current_agent_name ?? null}
              is_last_round_pending_permission={pending_permission}
              is_loading={is_loading}
              is_mobile_layout={is_mobile_layout}
              message_groups={message_groups}
              on_delete_round={delete_round}
              on_open_workspace_file={on_open_workspace_file}
              on_permission_response={send_permission_response}
              on_regenerate_round={regenerate}
              round_ids={round_ids}
            />
          </div>

          {show_scroll_to_bottom ? (
            <RoomScrollToLatestButton
              is_loading={is_loading}
              is_mobile_layout={is_mobile_layout}
              on_click={handle_jump_to_bottom}
            />
          ) : null}

          <RoomComposerPanel
            compact={is_mobile_layout}
            current_agent_name={current_agent_name ?? null}
            is_loading={is_loading}
            on_send_message={handle_send_message}
            on_stop={handle_stop}
          />
        </>
      )}
    </div>
  );
}
