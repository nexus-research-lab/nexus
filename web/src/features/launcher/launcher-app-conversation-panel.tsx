"use client";

import { KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, Bot, LoaderCircle, RotateCcw, X, } from "lucide-react";

import {
  HeroActionOrbShell,
  HeroActionPillShell,
  HeroInputShell,
  HeroSidePanelShell,
} from "@/features/launcher/launcher-glass-shell";
import { ConversationFeed } from "@/features/conversation-shared/conversation-feed";
import { ScrollToLatestButton } from "@/features/conversation-shared/scroll-to-latest-button";
import { cn } from "@/lib/utils";
import { useTextareaHeight } from "@/hooks/use-textarea-height";
import { Message } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { WebSocketState } from "@/types/websocket";

interface LauncherAppConversationPanelProps {
  app_conversation_draft: string;
  app_conversation_messages: Message[];
  error: string | null;
  is_loading: boolean;
  ws_state: WebSocketState;
  on_clear_session: () => void;
  on_change_draft: (next_value: string) => void;
  on_close: () => void;
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  on_stop_generation: () => void;
  on_submit: (next_prompt: string) => void;
  pending_permissions: PendingPermission[];
}

const BOTTOM_THRESHOLD_PX = 80;

function group_messages_by_round(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();

  for (const message of messages) {
    const round_id = message.round_id || message.message_id;
    const current_group = groups.get(round_id) ?? [];
    current_group.push(message);
    groups.set(round_id, current_group);
  }

  return groups;
}

export function LauncherAppConversationPanel({
                                               app_conversation_draft,
                                               app_conversation_messages,
                                               error,
                                               is_loading,
                                               ws_state,
                                               on_clear_session,
                                               on_change_draft,
                                               on_close,
                                               on_permission_response,
                                               on_stop_generation,
                                               on_submit,
                                               pending_permissions,
                                             }: LauncherAppConversationPanelProps) {
  const scroll_ref = useRef<HTMLDivElement>(null);
  const bottom_anchor_ref = useRef<HTMLDivElement>(null);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const is_composing_ref = useRef(false);
  const should_follow_latest_ref = useRef(true);
  const [show_scroll_to_bottom, set_show_scroll_to_bottom] = useState(false);
  const message_groups = useMemo(
    () => group_messages_by_round(app_conversation_messages),
    [app_conversation_messages],
  );
  const round_ids = useMemo(() => Array.from(message_groups.keys()), [message_groups]);

  const connection_meta = useMemo(() => {
    if (is_loading) {
      return {
        label: "回复中",
        badge_class_name: "text-emerald-900/78",
        dot_class_name: "bg-emerald-400",
      };
    }

    if (ws_state === "connected") {
      return {
        label: "已连接",
        badge_class_name: "text-slate-700/48",
        dot_class_name: "bg-[#7fe3a8]",
      };
    }

    if (ws_state === "connecting") {
      return {
        label: "连接中",
        badge_class_name: "text-slate-700/48",
        dot_class_name: "bg-slate-300",
      };
    }

    return {
      label: "正在重连",
      badge_class_name: "text-amber-900/68",
      dot_class_name: "bg-amber-300",
    };
  }, [is_loading, ws_state]);

  const can_send_message = (
    ws_state === "connected" &&
    !is_loading &&
    app_conversation_draft.trim().length > 0
  );

  const update_follow_state = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    const distance_to_bottom = (
      container.scrollHeight -
      container.scrollTop -
      container.clientHeight
    );
    const is_near_bottom = distance_to_bottom <= BOTTOM_THRESHOLD_PX;
    should_follow_latest_ref.current = is_near_bottom;
    set_show_scroll_to_bottom(!is_near_bottom);
  }, []);

  const scroll_to_bottom = useCallback((behavior: ScrollBehavior = "smooth") => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }

    should_follow_latest_ref.current = true;
    set_show_scroll_to_bottom(false);
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        container.scrollTo({
          top: container.scrollHeight,
          behavior,
        });
        bottom_anchor_ref.current?.scrollIntoView({
          behavior,
          block: "end",
        });
      });
    });
  }, []);

  useEffect(() => {
    if (!should_follow_latest_ref.current) {
      update_follow_state();
      return;
    }

    scroll_to_bottom(is_loading ? "auto" : "smooth");
  }, [app_conversation_messages, is_loading, scroll_to_bottom, update_follow_state]);

  useTextareaHeight(textarea_ref, app_conversation_draft, {
    minHeight: 28,
    maxHeight: 144,
    lineHeight: 24,
  });

  const handle_submit = useCallback(() => {
    if (!can_send_message) {
      return;
    }
    should_follow_latest_ref.current = true;
    set_show_scroll_to_bottom(false);
    on_submit(app_conversation_draft);
  }, [app_conversation_draft, can_send_message, on_submit]);

  const handle_key_down = useCallback((event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (is_composing_ref.current || event.nativeEvent.isComposing) {
      return;
    }

    if (event.key === "Escape" && is_loading) {
      event.preventDefault();
      on_stop_generation();
      return;
    }

    if (event.key !== "Enter" || event.shiftKey) {
      return;
    }

    event.preventDefault();
    handle_submit();
  }, [handle_submit, is_loading, on_stop_generation]);

  return (
    <HeroSidePanelShell class_name="h-full min-h-[620px] w-full max-w-[420px]">
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <HeroActionPillShell class_name="w-fit">
              <span
                className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-slate-800/72">
                <span className={cn("h-3 w-3 rounded-full", connection_meta.dot_class_name)}/>
                Nexus
              </span>
              <span className={cn("text-[11px] font-medium", connection_meta.badge_class_name)}>
                {connection_meta.label}
              </span>
            </HeroActionPillShell>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <button
              aria-label="清空 Nexus 对话"
              className="transition-transform duration-300 hover:-translate-y-0.5"
              onClick={on_clear_session}
              type="button"
            >
              <HeroActionOrbShell class_name="h-[46px] w-[46px]">
                <RotateCcw className="h-4 w-4 text-slate-900/76"/>
              </HeroActionOrbShell>
            </button>
            <button
              aria-label="关闭 Nexus 对话"
              className="transition-transform duration-300 hover:-translate-y-0.5"
              onClick={on_close}
              type="button"
            >
              <HeroActionOrbShell class_name="h-[54px] w-[54px]">
                <X className="h-4 w-4 text-slate-900/76"/>
              </HeroActionOrbShell>
            </button>
          </div>
        </div>

        <div className="border-b border-white/10 px-3 pt-4">
          <p className="truncate text-[22px] font-black tracking-[-0.04em] text-slate-950/90">
            Nexus
          </p>
        </div>

        <div
          className="relative mt-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-[28px] bg-[rgba(255,255,255,0.05)] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)]">
          {error ? (
            <div
              className="mx-3 mt-3 flex items-start gap-2 rounded-[20px] bg-[rgba(255,120,120,0.12)] px-3 py-2 text-xs leading-5 text-red-900/84 shadow-[inset_0_0_0_1px_rgba(255,120,120,0.14)]">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0"/>
              <span>{error}</span>
            </div>
          ) : null}

          <div
            ref={scroll_ref}
            className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-1 pb-2 pt-3"
            onScroll={update_follow_state}
          >
            {app_conversation_messages.length ? (
              <ConversationFeed
                bottom_anchor_ref={bottom_anchor_ref}
                compact
                current_agent_name="Nexus"
                is_last_round_pending_permissions={pending_permissions}
                is_loading={is_loading}
                is_mobile_layout
                message_groups={message_groups}
                on_permission_response={on_permission_response}
                round_ids={round_ids}
              />
            ) : (
              <div className="flex min-h-80 flex-col justify-center px-5 py-6">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/44">
                  Nexus Chat
                </p>
                <p className="mt-3 text-base font-semibold text-slate-950/84">
                  告诉 Nexus 你要推进什么
                </p>
              </div>
            )}
          </div>

          {show_scroll_to_bottom ? (
            <ScrollToLatestButton
              is_loading={is_loading}
              is_mobile_layout={true}
              on_click={() => scroll_to_bottom("smooth")}
            />
          ) : null}
        </div>

        <div className="mt-4">
          <HeroInputShell class_name="w-full">
            <div className="flex min-w-0 flex-col gap-3">
              <div className="flex min-w-0 items-end gap-3">
                <textarea
                  ref={textarea_ref}
                  className="max-h-36 min-h-7 flex-1 resize-none overflow-y-auto bg-transparent text-sm leading-6 text-slate-900/84 outline-none placeholder:text-slate-700/42"
                  onChange={(event) => on_change_draft(event.target.value)}
                  onCompositionEnd={() => {
                    is_composing_ref.current = false;
                  }}
                  onCompositionStart={() => {
                    is_composing_ref.current = true;
                  }}
                  onKeyDown={handle_key_down}
                  placeholder="告诉 Nexus 你要推进什么..."
                  rows={1}
                  value={app_conversation_draft}
                />
                <button
                  className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white/84 text-slate-900 shadow-[0_10px_20px_rgba(255,255,255,0.16)] transition-transform duration-300 hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0"
                  disabled={!is_loading && !can_send_message}
                  onClick={is_loading ? on_stop_generation : handle_submit}
                  type="button"
                >
                  {is_loading ? (
                    <RotateCcw className="h-4 w-4"/>
                  ) : ws_state === "connected" ? (
                    <Bot className="h-4 w-4"/>
                  ) : (
                    <LoaderCircle className="h-4 w-4 animate-spin"/>
                  )}
                </button>
              </div>
            </div>
          </HeroInputShell>
          <div className="flex items-center justify-between gap-2 px-8 pb-1 text-[11px] text-slate-700/44">
            <span>
              {ws_state === "connected"
                ? "Enter 发送，Shift + Enter 换行"
                : "正在建立 Nexus 主对话连接..."}
            </span>
            <span>{app_conversation_messages.length ? `${app_conversation_messages.length} 条消息` : ""}</span>
          </div>
        </div>
      </div>
    </HeroSidePanelShell>
  );
}
