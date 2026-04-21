"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { AgentConversationIdentity, get_session_control_status_text } from "@/types/agent/agent-conversation";
import { SessionSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";

import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import { prepare_workspace_text_attachments } from "@/features/conversation/shared/composer-attachments";
import { ConversationFeed } from "@/features/conversation/shared/conversation-feed";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { group_messages_by_round, get_latest_reply_timestamp } from "@/features/conversation/shared/utils";

const HISTORY_LOAD_THRESHOLD_PX = 120;

export interface DmChatPanelProps {
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  session_identity: AgentConversationIdentity | null;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
  on_conversation_snapshot_change?: (snapshot: SessionSnapshotPayload) => void;
}

export function DmChatPanel({
  current_agent_name,
  current_agent_avatar,
  session_identity,
  layout = "desktop",
  initial_draft = null,
  on_initial_draft_consumed,
  on_open_workspace_file,
  on_todos_change,
  on_room_event,
  on_loading_change,
  on_conversation_snapshot_change,
}: DmChatPanelProps) {
  const is_mobile_layout = layout === "mobile";
  const session_key = session_identity?.session_key ?? null;

  const {
    error,
    messages,
    is_loading,
    is_history_loading,
    has_more_history,
    history_prepend_token,
    session_control_state,
    session_observer_count,
    pending_permissions,
    send_message,
    stop_generation,
    load_session,
    load_older_messages,
    send_permission_response,
    runtime_phase,
    live_round_ids,
  } = useAgentConversation({
    identity: session_identity,
    on_error: (err) => {
      console.error("DM conversation error:", err);
    },
    on_room_event,
  });

  const {
    scroll_ref,
    feed_ref,
    bottom_anchor_ref,
    show_scroll_to_bottom,
    scroll_to_bottom,
    prepare_history_prepend_restore,
    cancel_history_prepend_restore,
    on_scroll,
    on_wheel,
    on_touch_start,
    on_touch_move,
    on_touch_end,
  } = useFollowScroll({
    trigger_deps: [messages, is_loading] as const,
    session_key,
    history_prepend_token,
  });

  const todos = useExtractTodos(messages, session_key);
  const last_snapshot_key_ref = useRef<string | null>(null);
  const consumed_initial_draft_ref = useRef<string | null>(null);
  const can_control_session = session_control_state !== "observer";
  const observer_read_only_reason = "当前窗口是观察视图，控制权在另一窗口";
  const session_control_text = useMemo(
    () => get_session_control_status_text(session_control_state, session_observer_count),
    [session_control_state, session_observer_count],
  );

  useEffect(() => { on_todos_change?.(todos); }, [on_todos_change, todos]);
  useEffect(() => { on_loading_change?.(is_loading); }, [is_loading, on_loading_change]);

  useEffect(() => {
    if (!session_key || messages.length === 0) return;
    const last = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    const snapshot = {
      session_key,
      agent_id: session_identity?.agent_id ?? null,
      room_id: session_identity?.room_id ?? null,
      conversation_id: session_identity?.conversation_id ?? null,
      room_session_id: session_identity?.room_session_id ?? null,
      message_count: messages.length,
      ...(latest_reply_timestamp ? { last_activity_at: latest_reply_timestamp } : {}),
      session_id: last?.session_id ?? null,
    };
    const snapshot_key = JSON.stringify(snapshot);

    // DM 与 Room 共用流式消息模式，这里同样需要阻断重复快照回写。
    if (last_snapshot_key_ref.current === snapshot_key) {
      return;
    }

    last_snapshot_key_ref.current = snapshot_key;
    on_conversation_snapshot_change?.(snapshot);
  }, [session_identity, session_key, messages, on_conversation_snapshot_change]);

  useSessionLoader({
    session_key,
    load_session,
    debug_name: "DmChatPanel",
  });

  const message_groups = useMemo(() => group_messages_by_round(messages), [messages]);
  const round_ids = Array.from(message_groups.keys());

  const maybe_load_older_messages = useCallback(async () => {
    const container = scroll_ref.current;
    if (
      !container
      || !has_more_history
      || is_history_loading
      || container.scrollTop > HISTORY_LOAD_THRESHOLD_PX
    ) {
      return;
    }

    prepare_history_prepend_restore();
    const did_prepend = await load_older_messages();
    if (!did_prepend) {
      cancel_history_prepend_restore();
    }
  }, [
    cancel_history_prepend_restore,
    has_more_history,
    is_history_loading,
    load_older_messages,
    prepare_history_prepend_restore,
    scroll_ref,
  ]);

  const handle_scroll = useCallback(() => {
    on_scroll();
    void maybe_load_older_messages();
  }, [maybe_load_older_messages, on_scroll]);

  useEffect(() => {
    const container = scroll_ref.current;
    if (
      !container
      || !has_more_history
      || is_history_loading
      || is_loading
      || container.scrollHeight > container.clientHeight + 24
    ) {
      return;
    }
    void maybe_load_older_messages();
  }, [has_more_history, is_history_loading, is_loading, maybe_load_older_messages, messages.length, scroll_ref]);

  const handle_send_message = async (content: string) => {
    if (!content.trim() || is_loading) return;
    scroll_to_bottom("auto");
    await send_message(content);
  };

  const handle_stop = () => stop_generation();

  const handle_prepare_attachments = async (files: File[]) => {
    const target_agent_id = session_identity?.agent_id;
    if (!target_agent_id) {
      throw new Error("当前会话尚未准备好，暂时无法附加文件。");
    }
    return prepare_workspace_text_attachments(target_agent_id, files);
  };

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (!session_key || !normalized_draft || is_loading || !can_control_session) {
      return;
    }

    const initial_draft_signature = `${session_key}:${normalized_draft}`;
    if (consumed_initial_draft_ref.current === initial_draft_signature) {
      return;
    }

    consumed_initial_draft_ref.current = initial_draft_signature;
    scroll_to_bottom("auto");
    void send_message(normalized_draft)
      .then(() => {
        on_initial_draft_consumed?.();
      })
      .catch((error) => {
        consumed_initial_draft_ref.current = null;
        console.error("Failed to auto send initial DM prompt:", error);
      });
  }, [can_control_session, initial_draft, is_loading, on_initial_draft_consumed, scroll_to_bottom, send_message, session_key]);

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

      <div
        ref={scroll_ref}
        className={
          is_mobile_layout
            ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2"
            : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
        }
        style={{ overflowAnchor: "none" }}
        onScroll={handle_scroll}
        onTouchEnd={on_touch_end}
        onTouchMove={on_touch_move}
        onTouchStart={on_touch_start}
        onWheel={on_wheel}
      >
        {is_history_loading ? (
          <div className="mx-auto mb-3 flex w-full max-w-[980px] items-center justify-center text-xs text-muted-foreground">
            正在加载更早消息...
          </div>
        ) : null}
        <ConversationFeed
          bottom_anchor_ref={bottom_anchor_ref}
          feed_ref={feed_ref}
          scroll_ref={scroll_ref}
          current_agent_name={current_agent_name ?? null}
          current_agent_avatar={current_agent_avatar ?? null}
          is_last_round_pending_permissions={pending_permissions}
          is_loading={is_loading}
          runtime_phase={runtime_phase}
          live_round_ids={live_round_ids}
          is_mobile_layout={is_mobile_layout}
          message_groups={message_groups}
          on_open_workspace_file={on_open_workspace_file}
          on_permission_response={send_permission_response}
          can_respond_to_permissions={can_control_session}
          permission_read_only_reason={observer_read_only_reason}
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
        control_status_text={session_control_text}
        is_loading={is_loading}
        runtime_phase={runtime_phase}
        on_prepare_attachments={handle_prepare_attachments}
        on_send_message={handle_send_message}
        on_stop={handle_stop}
        disabled={!can_control_session}
      />
    </div>
  );
}
