"use client";

import { useEffect, useMemo, useRef } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useSessionLoader } from "@/hooks/use-session-loader";
import { useExtractTodos } from "@/hooks/use-extract-todos";
import { useFollowScroll } from "@/hooks/use-follow-scroll";
import { SessionSnapshotPayload } from "@/types/conversation";
import { TodoItem } from "@/types/todo";

import { ComposerPanel } from "@/features/conversation-shared/composer-panel";
import { ConversationFeed } from "@/features/conversation-shared/conversation-feed";
import { ScrollToLatestButton } from "@/features/conversation-shared/scroll-to-latest-button";
import { groupMessagesByRound, get_latest_reply_timestamp } from "@/features/conversation-shared/utils";
import { DmConversationHeader } from "./dm-conversation-header";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { Conversation } from "@/types/conversation";

export interface DmChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  session_key: string | null;
  session_title?: string | null;
  conversations?: Conversation[];
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  hide_header?: boolean;
  /** Controlled tab — caller manages which surface tab is active */
  active_tab?: RoomSurfaceTabKey;
  on_change_tab?: (tab: RoomSurfaceTabKey) => void;
  is_detail_panel_open?: boolean;
  on_toggle_detail_panel?: () => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: SessionSnapshotPayload) => void;
  on_create_conversation?: (title?: string) => void | Promise<string | null>;
}

export function DmChatPanel({
  agent_id,
  current_agent_name,
  session_key,
  session_title,
  conversations = [],
  layout = "desktop",
  initial_draft = null,
  hide_header = false,
  active_tab = "chat",
  on_change_tab,
  is_detail_panel_open = false,
  on_toggle_detail_panel,
  on_open_workspace_file,
  on_todos_change,
  on_loading_change,
  on_conversation_snapshot_change,
  on_create_conversation,
}: DmChatPanelProps) {
  const is_mobile_layout = layout === "mobile";

  const {
    error,
    messages,
    is_loading,
    pending_permission,
    send_message,
    stop_generation,
    load_session,
    send_permission_response,
  } = useAgentConversation({
    agent_id,
    on_error: (err) => {
      console.error("DM conversation error:", err);
    },
  });

  const {
    scroll_ref,
    feed_ref,
    bottom_anchor_ref,
    show_scroll_to_bottom,
    scroll_to_bottom,
    on_scroll,
    on_wheel,
    on_touch_start,
    on_touch_move,
    on_touch_end,
  } = useFollowScroll({
    trigger_deps: [messages, is_loading] as const,
    session_key,
  });

  const todos = useExtractTodos(messages, session_key);
  const last_snapshot_key_ref = useRef<string | null>(null);

  useEffect(() => { on_todos_change?.(todos); }, [on_todos_change, todos]);
  useEffect(() => { on_loading_change?.(is_loading); }, [is_loading, on_loading_change]);

  useEffect(() => {
    if (!session_key || messages.length === 0) return;
    const last = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    const snapshot = {
      session_key,
      message_count: messages.length,
      ...(latest_reply_timestamp ? { last_activity_at: latest_reply_timestamp } : {}),
      session_id: last?.session_id ?? null,
    };
    const snapshot_key = JSON.stringify(snapshot);

    // 中文注释：DM 与 Room 共用流式消息模式，这里同样需要阻断重复快照回写。
    if (last_snapshot_key_ref.current === snapshot_key) {
      return;
    }

    last_snapshot_key_ref.current = snapshot_key;
    on_conversation_snapshot_change?.(snapshot);
  }, [session_key, messages, on_conversation_snapshot_change]);

  useSessionLoader({
    session_key,
    load_session,
    debug_name: "DmChatPanel",
  });

  const message_groups = useMemo(() => groupMessagesByRound(messages), [messages]);
  const round_ids = Array.from(message_groups.keys());

  const handle_send_message = async (content: string) => {
    if (!content.trim() || is_loading) return;
    scroll_to_bottom("auto");
    try {
      await send_message(content);
    } catch (err) {
      console.error("Failed to send message:", err);
    }
  };

  const handle_stop = () => stop_generation();

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

      {error && error.includes("服务器") ? (
        <div className="absolute left-1/2 top-4 z-50 max-w-md -translate-x-1/2">
          <div className="rounded-2xl border border-destructive/20 bg-destructive/8 p-3">
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

      {!is_mobile_layout && !hide_header ? (
        <DmConversationHeader
          active_tab={active_tab}
          conversation_count={conversations.length}
          current_agent_name={current_agent_name ?? null}
          is_detail_panel_open={is_detail_panel_open}
          is_loading={is_loading}
          on_change_tab={on_change_tab ?? (() => {})}
          on_toggle_detail_panel={on_toggle_detail_panel ?? (() => {})}
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
        onScroll={on_scroll}
        onTouchEnd={on_touch_end}
        onTouchMove={on_touch_move}
        onTouchStart={on_touch_start}
        onWheel={on_wheel}
      >
        <ConversationFeed
          bottom_anchor_ref={bottom_anchor_ref}
          feed_ref={feed_ref}
          scroll_ref={scroll_ref}
          current_agent_name={current_agent_name ?? null}
          is_last_round_pending_permission={pending_permission}
          is_loading={is_loading}
          is_mobile_layout={is_mobile_layout}
          message_groups={message_groups}
          on_open_workspace_file={on_open_workspace_file}
          on_permission_response={send_permission_response}
          round_ids={round_ids}
        />
      </div>

      {show_scroll_to_bottom ? (
        <ScrollToLatestButton
          is_loading={is_loading}
          is_mobile_layout={is_mobile_layout}
          on_click={() => scroll_to_bottom("smooth")}
        />
      ) : null}

      <ComposerPanel
        compact={is_mobile_layout}
        current_agent_name={current_agent_name ?? null}
        initial_draft={initial_draft}
        is_loading={is_loading}
        on_send_message={handle_send_message}
        on_stop={handle_stop}
      />
    </div>
  );
}
