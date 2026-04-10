"use client";

import { CSSProperties, useEffect, useMemo } from "react";
import { AlertCircle, RotateCcw, X, } from "lucide-react";

import {
  HeroActionOrbShell,
  HeroActionPillShell,
  HeroSidePanelShell,
} from "@/features/launcher/launcher-glass-shell";
import { ConversationFeed } from "@/features/conversation-shared/conversation-feed";
import { ScrollToLatestButton } from "@/features/conversation-shared/scroll-to-latest-button";
import { useFollowScroll } from "@/hooks/use-follow-scroll";
import { cn } from "@/lib/utils";
import { Message } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { WebSocketState } from "@/types/websocket";

interface LauncherAppConversationPanelProps {
  app_conversation_messages: Message[];
  error: string | null;
  is_info_mode?: boolean;
  is_loading: boolean;
  session_key: string | null;
  ws_state: WebSocketState;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_clear_session: () => void;
  on_close: () => void;
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  pending_permissions: PendingPermission[];
}

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
  app_conversation_messages,
  error,
  is_info_mode = false,
  is_loading,
  session_key,
  ws_state,
  can_respond_to_permissions = true,
  permission_read_only_reason,
  on_clear_session,
  on_close,
  on_permission_response,
  pending_permissions,
}: LauncherAppConversationPanelProps) {
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
    trigger_deps: [app_conversation_messages, is_loading] as const,
    session_key,
  });
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
  const launcher_message_theme_style = useMemo(
    () => ({
      "--surface-canvas-background": "transparent",
      "--surface-panel-subtle-background": "rgba(255,255,255,0.18)",
      "--surface-panel-subtle-border": "rgba(255,255,255,0.14)",
      "--surface-avatar-background": "rgba(255,255,255,0.38)",
      "--surface-avatar-border": "rgba(255,255,255,0.26)",
      "--surface-avatar-foreground": "rgba(54,66,88,0.74)",
      "--text-strong": "rgba(24,33,49,0.92)",
      "--text-default": "rgba(38,48,68,0.82)",
      "--text-muted": "rgba(88,100,122,0.62)",
      "--text-soft": "rgba(98,110,130,0.58)",
      "--icon-default": "rgba(62,74,95,0.76)",
      "--icon-muted": "rgba(102,114,135,0.56)",
      "--divider-subtle-color": "rgba(255,255,255,0.12)",
    }) as CSSProperties,
    [],
  );

  useEffect(() => {
    const feed = feed_ref.current;
    if (!feed || app_conversation_messages.length === 0 || typeof ResizeObserver === "undefined") {
      return;
    }

    let is_bootstrapping = true;
    let first_frame_id: number | null = null;
    let second_frame_id: number | null = null;
    let bootstrap_timeout_id: number | null = null;

    const sync_to_bottom = () => {
      if (!is_bootstrapping) {
        return;
      }

      if (first_frame_id !== null) {
        cancelAnimationFrame(first_frame_id);
      }
      if (second_frame_id !== null) {
        cancelAnimationFrame(second_frame_id);
      }

      // 中文注释：Launcher 右侧面板打开时，历史消息可能已经准备好，
      // 但消息内容层还会继续完成排版与高度结算。这里只在“打开引导期”内
      // 监听内容层尺寸并强制贴底，确保既有历史一出现就落在最底部。
      first_frame_id = requestAnimationFrame(() => {
        second_frame_id = requestAnimationFrame(() => {
          scroll_to_bottom("auto");
          first_frame_id = null;
          second_frame_id = null;
        });
      });
    };

    sync_to_bottom();
    const observer = new ResizeObserver(() => {
      sync_to_bottom();
    });
    observer.observe(feed);
    bootstrap_timeout_id = window.setTimeout(() => {
      is_bootstrapping = false;
    }, 720);

    return () => {
      is_bootstrapping = false;
      observer.disconnect();
      if (first_frame_id !== null) {
        cancelAnimationFrame(first_frame_id);
      }
      if (second_frame_id !== null) {
        cancelAnimationFrame(second_frame_id);
      }
      if (bootstrap_timeout_id !== null) {
        window.clearTimeout(bootstrap_timeout_id);
      }
    };
  }, [app_conversation_messages.length, feed_ref, scroll_to_bottom, session_key]);

  return (
    <HeroSidePanelShell class_name="h-full min-h-[620px] w-full max-w-[512px]">
      <div className="flex h-full min-h-0 flex-col mx-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <HeroActionPillShell class_name="w-fit">
              <span
                className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-foreground/72">
                <span className={cn("h-3 w-3 rounded-full", connection_meta.dot_class_name)} />
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
                <RotateCcw className="h-4 w-4 text-foreground/76" />
              </HeroActionOrbShell>
            </button>
            <button
              aria-label="关闭 Nexus 对话"
              className="transition-transform duration-300 hover:-translate-y-0.5"
              onClick={on_close}
              type="button"
            >
              <HeroActionOrbShell class_name="h-[54px] w-[54px]">
                <X className="h-4 w-4 text-foreground/76" />
              </HeroActionOrbShell>
            </button>
          </div>
        </div>

        <div
          className="relative mt-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-[28px] bg-[rgba(244,247,252,0.44)] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.26),0_18px_36px_rgba(112,128,154,0.08)] backdrop-blur-[14px]"
          style={launcher_message_theme_style}
        >
          {error ? (
            <div
              className="mx-3 mt-3 flex items-start gap-2 rounded-[20px] bg-[rgba(255,243,243,0.74)] px-3 py-2 text-xs leading-5 text-red-900/84 shadow-[inset_0_0_0_1px_rgba(255,120,120,0.14)]">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{error}</span>
            </div>
          ) : null}

          <div
            ref={scroll_ref}
            className="soft-scrollbar relative z-0 min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 pb-3 pt-4 sm:px-4 sm:pb-3 sm:pt-4"
            style={{ overflowAnchor: "none" }}
            onScroll={on_scroll}
            onTouchEnd={on_touch_end}
            onTouchMove={on_touch_move}
            onTouchStart={on_touch_start}
            onWheel={on_wheel}
          >
            {app_conversation_messages.length ? (
              <ConversationFeed
                bottom_anchor_ref={bottom_anchor_ref}
                feed_ref={feed_ref}
                compact
                current_agent_name="Nexus"
                is_last_round_pending_permissions={pending_permissions}
                is_loading={is_loading}
                is_mobile_layout
                message_groups={message_groups}
                on_permission_response={on_permission_response}
                can_respond_to_permissions={can_respond_to_permissions}
                permission_read_only_reason={permission_read_only_reason}
                scroll_ref={scroll_ref}
                round_ids={round_ids}
              />
            ) : (
              <div className="flex min-h-80 flex-col justify-center px-5 py-6">
                <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-[rgba(90,104,128,0.56)]">
                  {is_info_mode ? "Nexus Status" : "Nexus Chat"}
                </p>
                <p className="mt-3 text-[17px] font-semibold leading-7 text-[rgba(24,33,49,0.88)]">
                  {is_info_mode ? "这里会显示 Nexus 当前整理出的协作信息" : "告诉 Nexus 你要推进什么"}
                </p>
                <p className="mt-2 max-w-[22rem] text-sm leading-6 text-[rgba(84,97,120,0.68)]">
                  {is_info_mode
                    ? "右侧只负责承接当前协作摘要，不再承担输入。"
                    : "从中间 Hero 输入框发起后，Nexus 会在这里持续整理反馈。"}
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
      </div>
    </HeroSidePanelShell>
  );
}
