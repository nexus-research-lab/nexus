/**
 * =====================================================
 * @File   : workspace-conversation-switcher.tsx
 * @Date   : 2026-04-04 21:57
 * @Author : leemysw
 * 2026-04-04 21:57   Create
 * =====================================================
 */

"use client";

import { useMemo, useState } from "react";
import { Check, ChevronDown, History, LucideIcon, MessageSquare, MessageSquarePlus } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { RoomConversationView } from "@/types/conversation";

interface WorkspaceConversationSwitcherProps {
  conversations: RoomConversationView[];
  conversation_id: string | null;
  density?: "default" | "compact";
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
  on_view_history?: () => void;
  icon?: LucideIcon;
}

const MAX_VISIBLE_CONVERSATIONS = 6;

export function WorkspaceConversationSwitcher({
  conversations,
  conversation_id,
  density = "compact",
  on_select_conversation,
  on_create_conversation,
  on_view_history,
  icon: Icon = MessageSquare,
}: WorkspaceConversationSwitcherProps) {
  const { t } = useI18n();
  const [is_open, set_is_open] = useState(false);
  const [is_creating, set_is_creating] = useState(false);
  const sorted_conversations = useMemo(
    () => [...conversations].sort((left, right) => right.last_activity_at - left.last_activity_at),
    [conversations],
  );
  const visible_conversations = useMemo(() => {
    const recent_conversations = sorted_conversations.slice(0, MAX_VISIBLE_CONVERSATIONS);
    if (!conversation_id || recent_conversations.some((conversation) => conversation.conversation_id === conversation_id)) {
      return recent_conversations;
    }

    const active_conversation = sorted_conversations.find(
      (conversation) => conversation.conversation_id === conversation_id,
    );
    if (!active_conversation) {
      return recent_conversations;
    }

    // 中文注释：当前会话太旧时仍需保留在下拉中，避免选中态完全消失。
    return [
      active_conversation,
      ...recent_conversations.filter((conversation) => conversation.conversation_id !== conversation_id),
    ].slice(0, MAX_VISIBLE_CONVERSATIONS);
  }, [conversation_id, sorted_conversations]);
  const has_more_conversations = sorted_conversations.length > MAX_VISIBLE_CONVERSATIONS;
  const trigger_style = density === "compact"
    ? (is_open
      ? {
        borderBottom: "1px solid var(--surface-popover-border)",
      }
      : {
        borderBottom: "1px solid color-mix(in srgb, var(--divider-subtle-color) 82%, transparent)",
      })
    : (is_open
      ? {
        background: "var(--surface-popover-background)",
        border: "1px solid var(--surface-popover-border)",
      }
      : {
        background: "var(--chip-default-background)",
        border: "1px solid var(--chip-default-border)",
      });

  const current_title =
    sorted_conversations.find((conversation) => conversation.conversation_id === conversation_id)?.title
    ?? t("room.choose_conversation");

  const handle_create = async () => {
    if (!on_create_conversation || is_creating) return;
    set_is_creating(true);
    set_is_open(false);
    try {
      await on_create_conversation();
    } finally {
      set_is_creating(false);
    }
  };

  return (
    <div className="relative">
      <button
        aria-expanded={is_open}
        className={cn(
          "flex max-w-[168px] items-center text-[color:var(--text-default)] transition-[background-color,border-color,color,transform] duration-[var(--motion-duration-fast)] ease-out",
          density === "compact" ? "h-6 gap-1 px-0 pb-0.5 text-[12px]" : "h-8 gap-1 rounded-full px-3 text-[12px]",
        )}
        style={trigger_style}
        onClick={() => set_is_open((prev) => !prev)}
        type="button"
      >
        <span className={cn("truncate font-medium", density === "compact" ? "max-w-[120px]" : "max-w-[132px]")}>{current_title}</span>
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          <ChevronDown className={cn("h-3 w-3 text-[color:var(--icon-muted)] transition-transform duration-[var(--motion-duration-fast)]", is_open && "rotate-180 text-[color:var(--icon-default)]")} />
        </span>
      </button>

      {is_open ? (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => set_is_open(false)}
          />
          <div
            className="surface-panel radius-shell-lg absolute left-0 top-[calc(100%+8px)] z-50 w-[min(18.5rem,calc(100vw-24px))] overflow-hidden"
          >
            {sorted_conversations.length > 0 ? (
              <>
                <div className="p-1.5">
                  {visible_conversations.map((conversation) => {
                    const is_active = conversation.conversation_id === conversation_id;
                    return (
                      <button
                        aria-pressed={is_active}
                        key={conversation.conversation_id}
                        className={cn(
                          "group flex w-full items-center gap-2.5 rounded-[14px] border px-3.5 py-2.5 text-left text-[11.5px] font-medium transition-[background-color,border-color,color,opacity] duration-[var(--motion-duration-fast)] ease-out",
                          is_active
                            ? "bg-[var(--surface-interactive-active-background)] text-[color:var(--text-strong)] hover:brightness-[0.985]"
                            : "border-transparent text-[color:var(--text-default)] hover:border-[var(--surface-interactive-hover-border)] hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)]",
                        )}
                        onClick={() => {
                          on_select_conversation(conversation.conversation_id);
                          set_is_open(false);
                        }}
                        type="button"
                      >
                        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
                          <Icon className={cn(
                            "h-3.5 w-3.5 transition-colors duration-[var(--motion-duration-fast)]",
                            is_active
                              ? "text-[color:var(--icon-default)]"
                              : "text-[color:var(--icon-muted)] group-hover:text-[color:var(--icon-default)]",
                          )} />
                        </span>
                        <span className="min-w-0 flex-1 truncate">
                          {conversation.title || t("room.untitled_conversation")}
                        </span>
                        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
                          <Check className={cn("h-3.5 w-3.5 text-[color:var(--success)] transition-opacity duration-[var(--motion-duration-fast)]", is_active ? "opacity-100" : "opacity-0")} />
                        </span>
                      </button>
                    );
                  })}
                </div>

                {(has_more_conversations && on_view_history) || on_create_conversation ? (
                  <div className="mx-3 border-t divider-subtle" />
                ) : null}

                {has_more_conversations && on_view_history ? (
                  <div className="p-1.5 pt-1">
                    <button
                      className="flex w-full items-center gap-2.5 rounded-[14px] border border-transparent px-3.5 py-2.5 text-left text-[11.5px] font-medium text-[color:var(--text-default)] transition duration-[var(--motion-duration-fast)] ease-out hover:border-[var(--surface-interactive-hover-border)] hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)]"
                      onClick={() => {
                        set_is_open(false);
                        on_view_history();
                      }}
                      type="button"
                    >
                      <History className="h-3.5 w-3.5 shrink-0 text-[color:var(--icon-muted)]" />
                      <span className="min-w-0 flex-1">
                        {t("room.more_conversations")}
                      </span>
                    </button>
                  </div>
                ) : null}

                {on_create_conversation ? (
                  <div className="p-1.5 pt-1">
                    <button
                      className="flex w-full items-center gap-2.5 rounded-[14px] border border-transparent px-3.5 py-2.5 text-left text-[11.5px] font-medium text-[color:var(--success)] transition-[background-color,border-color,color,opacity] duration-[var(--motion-duration-fast)] ease-out hover:border-[color:color-mix(in_srgb,var(--success)_15%,transparent)] hover:bg-[color:color-mix(in_srgb,var(--success)_8%,transparent)] disabled:opacity-60"
                      disabled={is_creating}
                      onClick={handle_create}
                      type="button"
                    >
                      <MessageSquarePlus className={cn("h-3.5 w-3.5 shrink-0", is_creating && "animate-spin")} />
                      <span className="min-w-0 flex-1">
                        {is_creating ? t("room.creating") : t("room.new_conversation")}
                      </span>
                    </button>
                  </div>
                ) : null}
              </>
            ) : (
              <div className="px-3 py-2 text-[11px] text-[color:var(--text-soft)]">{t("room.no_conversations")}</div>
            )}
          </div>
        </>
      ) : null}
    </div>
  );
}
