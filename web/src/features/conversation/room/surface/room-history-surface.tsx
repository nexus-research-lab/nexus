"use client";

import { ReactNode, useCallback, useState } from "react";
import { Check, Clock3, MessageSquarePlus, Pencil, TextCursorInput, Trash2, X } from "lucide-react";

import { cn, format_relative_time } from "@/lib/utils";
import { I18nContextValue, useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { RoomConversationView } from "@/types/conversation/conversation";

interface RoomHistorySurfaceProps {
  can_manage_conversations?: boolean;
  conversations: RoomConversationView[];
  conversation_id: string | null;
  current_room_type: string;
  header_action?: ReactNode;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_update_conversation_title?: (conversation_id: string, title: string) => Promise<void>;
}

interface ConversationDeleteState {
  enabled: boolean;
  reason: string | null;
}

export function RoomHistorySurface({
  can_manage_conversations = true,
  conversations,
  conversation_id,
  current_room_type,
  header_action,
  on_create_conversation,
  on_delete_conversation,
  on_select_conversation,
  on_update_conversation_title,
}: RoomHistorySurfaceProps) {
  const { t } = useI18n();
  const create_action = can_manage_conversations ? (
    <WorkspaceSurfaceToolbarAction
      onClick={() => {
        void on_create_conversation();
      }}
      tone="primary"
    >
      <MessageSquarePlus className="h-3.5 w-3.5" />
      {t("room.new_conversation")}
    </WorkspaceSurfaceToolbarAction>
  ) : null;

  const action = create_action || header_action ? (
    <div className="flex items-center gap-3">
      {create_action}
      {header_action}
    </div>
  ) : null;

  return (
    <WorkspaceSurfaceView
      action={action}
      body_class_name="px-4 py-5 sm:px-5 xl:px-6"
      content_class_name="space-y-3"
      eyebrow={t("room.history")}
      max_width_class_name="max-w-[820px]"
      show_eyebrow={false}
      title={current_room_type === "dm" ? t("room.history_view_title_dm") : t("room.history_view_title")}
    >
      {conversations.length > 0 ? (
        <div className="space-y-3">
          {conversations.map((conversation) => {
            const delete_state = resolve_conversation_delete_state(
              conversation,
              conversations.length,
              can_manage_conversations,
              t,
            );
            return (
              <ConversationHistoryItem
                key={conversation.conversation_id}
                can_rename={can_manage_conversations && on_update_conversation_title !== undefined}
                conversation={conversation}
                delete_state={delete_state}
                is_active={conversation.conversation_id === conversation_id}
                on_delete={() => void on_delete_conversation(conversation.conversation_id)}
                on_rename={(title) => void on_update_conversation_title?.(conversation.conversation_id, title)}
                on_select={() => on_select_conversation(conversation.conversation_id)}
              />
            );
          })}
        </div>
      ) : (
        <div className="rounded-[24px] border border-(--divider-subtle-color) px-6 py-10 text-center">
          <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
            <Clock3 className="h-4 w-4" />
          </div>
          <p className="mt-4 text-[15px] font-semibold text-(--text-strong)">
            {t("room.no_conversations")}
          </p>
          <p className="mt-1 text-[12px] leading-6 text-(--text-soft)">
            {t("room.history_empty_hint")}
          </p>
          {can_manage_conversations ? (
            <button
              className="mt-4 inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--primary) transition duration-(--motion-duration-fast) ease-out hover:text-[color:color-mix(in_srgb,var(--primary)_84%,var(--foreground)_16%)]"
              onClick={() => {
                void on_create_conversation();
              }}
              type="button"
            >
              <MessageSquarePlus className="h-3.5 w-3.5" />
              {t("room.new_conversation")}
            </button>
          ) : null}
        </div>
      )}
    </WorkspaceSurfaceView>
  );
}

/** 中文注释：历史条目需要支持整卡切换与内联重命名，因此动作区和主体区分开处理。 */
function ConversationHistoryItem({
  can_rename,
  conversation,
  delete_state,
  is_active,
  on_delete,
  on_rename,
  on_select,
}: {
  can_rename: boolean;
  conversation: RoomConversationView;
  delete_state: ConversationDeleteState;
  is_active: boolean;
  on_delete: () => void;
  on_rename: (title: string) => void;
  on_select: () => void;
}) {
  const { t } = useI18n();
  const [is_editing, set_is_editing] = useState(false);
  const [edit_value, set_edit_value] = useState("");

  const start_edit = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    set_edit_value(conversation.title?.trim() || "");
    set_is_editing(true);
  }, [conversation.title]);

  const confirm_edit = useCallback(() => {
    const trimmed = edit_value.trim();
    if (trimmed && trimmed !== conversation.title?.trim()) {
      on_rename(trimmed);
    }
    set_is_editing(false);
  }, [edit_value, conversation.title, on_rename]);

  const cancel_edit = useCallback(() => {
    set_is_editing(false);
  }, []);

  return (
    <article
      className={cn(
        "group relative w-full rounded-[22px] border p-4 text-left transition duration-(--motion-duration-fast) ease-out",
        is_active
          ? "border-[color:color-mix(in_srgb,var(--primary)_32%,var(--divider-subtle-color))]"
          : "border-(--divider-subtle-color) hover:-translate-y-[1px] hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
      )}
      style={is_active
        ? {
          background: "color-mix(in srgb, var(--surface-interactive-active-background) 86%, transparent)",
          boxShadow: "0 14px 30px color-mix(in srgb, var(--primary) 7%, transparent)",
        }
        : undefined}
    >
      {is_active ? (
        <span
          aria-hidden="true"
          className="absolute left-0 top-4 bottom-4 w-[2px] rounded-full bg-(--primary)"
        />
      ) : null}

      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          {is_editing ? (
            <div className="flex items-center gap-1.5">
              <input
                autoFocus
                className="min-w-0 flex-1 rounded-[12px] border border-(--input-shell-border) bg-transparent px-3 py-2 text-[13px] font-semibold text-(--text-strong) outline-none transition focus:border-(--surface-interactive-active-border)"
                maxLength={64}
                onChange={(e) => set_edit_value(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") confirm_edit();
                  if (e.key === "Escape") cancel_edit();
                }}
                value={edit_value}
              />
              <button
                aria-label="确认"
                className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--primary) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)"
                onClick={confirm_edit}
                type="button"
              >
                <Check className="h-3.5 w-3.5" />
              </button>
              <button
                aria-label="取消"
                className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
                onClick={cancel_edit}
                type="button"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ) : (
            <button
              className="block w-full rounded-[14px] text-left outline-none transition duration-(--motion-duration-fast) ease-out focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]"
              onClick={on_select}
              type="button"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-[14px] font-semibold text-(--text-strong)">
                    {conversation.title?.trim() || t("room.untitled_conversation")}
                  </p>
                  <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-(--text-soft)">
                    <span className="inline-flex items-center gap-1.5">
                      <Clock3 className="h-3.5 w-3.5 shrink-0" />
                      <span>{format_relative_time(conversation.last_activity_at)}</span>
                    </span>
                    <span className="inline-flex items-center gap-1.5">
                      <TextCursorInput className="h-3.5 w-3.5 shrink-0" />
                      <span>{t("room.message_count_label", { count: conversation.message_count ?? 0 })}</span>
                    </span>
                  </div>
                </div>
                {is_active ? (
                  <span className="mt-0.5 inline-flex shrink-0 items-center rounded-full border border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-(--primary)">
                    {t("room.current_conversation")}
                  </span>
                ) : null}
              </div>
            </button>
          )}

          {is_editing ? (
            <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-(--text-soft)">
              <span className="inline-flex items-center gap-1.5">
                <Clock3 className="h-3.5 w-3.5 shrink-0" />
                <span>{format_relative_time(conversation.last_activity_at)}</span>
              </span>
              <span className="inline-flex items-center gap-1.5">
                <TextCursorInput className="h-3.5 w-3.5 shrink-0" />
                <span>{t("room.message_count_label", { count: conversation.message_count ?? 0 })}</span>
              </span>
            </div>
          ) : null}
        </div>

        <div className="flex shrink-0 items-start gap-1">
          {!is_editing && can_rename ? (
            <button
              aria-label="重命名"
              className={cn(
                "inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
                is_active ? "opacity-100" : "opacity-0 group-hover:opacity-100",
              )}
              onClick={start_edit}
              type="button"
            >
              <Pencil className="h-3.5 w-3.5" />
            </button>
          ) : null}

          {!is_editing ? (
            delete_state.enabled ? (
              <button
                aria-label="删除对话"
                className={cn(
                  "inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--destructive) transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)]",
                  is_active ? "opacity-100" : "opacity-0 group-hover:opacity-100",
                )}
                onClick={on_delete}
                type="button"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            ) : (
              <div className="group/delete-hint relative flex shrink-0 items-center">
                <button
                  aria-label="当前对话不可删除"
                  className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-[color:color-mix(in_srgb,var(--destructive)_40%,transparent)]"
                  disabled
                  type="button"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
                {delete_state.reason ? (
                  <div
                    className={cn(
                      "pointer-events-none absolute bottom-full right-0 z-10 mb-2 w-max max-w-52 rounded-xl border px-2.5 py-1.5 text-[11px] leading-5 text-(--text-strong) shadow-lg",
                      "whitespace-normal break-words",
                      "translate-y-1 opacity-0 transition duration-(--motion-duration-fast) ease-out group-hover/delete-hint:translate-y-0 group-hover/delete-hint:opacity-100",
                    )}
                    style={{
                      background: "var(--surface-popover-background)",
                      borderColor: "var(--surface-panel-subtle-border)",
                    }}
                  >
                    {delete_state.reason}
                  </div>
                ) : null}
              </div>
            )
          ) : null}
        </div>
      </div>
    </article>
  );
}

function resolve_conversation_delete_state(
  conversation: RoomConversationView,
  conversation_count: number,
  can_manage_conversations: boolean,
  t: I18nContextValue["t"],
): ConversationDeleteState {
  if (!can_manage_conversations) {
    return { enabled: false, reason: t("room.delete_no_permission") };
  }

  if (conversation.conversation_type !== "topic") {
    return { enabled: false, reason: t("room.delete_main_locked") };
  }

  if (conversation_count <= 1) {
    return { enabled: false, reason: t("room.delete_keep_one") };
  }

  return { enabled: true, reason: null };
}
