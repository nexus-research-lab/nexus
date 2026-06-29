"use client";

import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, Clock3, MessageSquarePlus, Pencil, Trash2, X } from "lucide-react";

import { get_session_channel_label } from "@/features/conversation/external-session-labels";
import { cn, format_relative_time } from "@/lib/utils";
import {
  ConversationDeleteState,
  resolve_room_conversation_delete_state,
} from "@/lib/conversation/room-conversation-delete";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
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

function get_conversation_ids(conversations: RoomConversationView[]): string[] {
  return conversations.map((conversation) => conversation.conversation_id);
}

function are_conversation_ids_equal(left_ids: string[], right_ids: string[]): boolean {
  if (left_ids.length !== right_ids.length) {
    return false;
  }
  return left_ids.every((id, index) => id === right_ids[index]);
}

function string_option(options: Record<string, unknown>, key: string): string | null {
  const value = options[key];
  return typeof value === "string" ? value : null;
}

function get_external_session_label(conversation: RoomConversationView): string | null {
  if (conversation.options?.external_session !== true) {
    return null;
  }
  return get_session_channel_label(
    string_option(conversation.options, "channel_type"),
    conversation.session_key,
  );
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
  const incoming_conversation_ids = useMemo(
    () => get_conversation_ids(conversations),
    [conversations],
  );
  const [conversation_order_ids, set_conversation_order_ids] = useState<string[]>(() => incoming_conversation_ids);
  const [pending_delete_conversation, set_pending_delete_conversation] = useState<RoomConversationView | null>(null);
  const conversations_by_id = useMemo(
    () => new Map(conversations.map((conversation) => [conversation.conversation_id, conversation])),
    [conversations],
  );
  const ordered_conversation_ids = useMemo(() => {
    const live_ids = new Set(incoming_conversation_ids);
    const existing_ids = conversation_order_ids.filter((id) => live_ids.has(id));
    const existing_id_set = new Set(existing_ids);
    const added_ids = incoming_conversation_ids.filter((id) => !existing_id_set.has(id));
    return [...added_ids, ...existing_ids];
  }, [conversation_order_ids, incoming_conversation_ids]);
  const ordered_conversations = useMemo(
    () => ordered_conversation_ids
      .map((id) => conversations_by_id.get(id))
      .filter((conversation): conversation is RoomConversationView => Boolean(conversation)),
    [conversations_by_id, ordered_conversation_ids],
  );

  useEffect(() => {
    // 中文注释：历史面板保持浏览时的视觉顺序，避免活跃会话更新时间后整列重排。
    set_conversation_order_ids((current_ids) => (
      are_conversation_ids_equal(current_ids, ordered_conversation_ids)
        ? current_ids
        : ordered_conversation_ids
    ));
  }, [ordered_conversation_ids]);

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
    <>
      <WorkspaceSurfaceView
        action={action}
        body_class_name="px-4 py-3.5 sm:px-5 xl:px-6"
        content_class_name="space-y-1.5"
        eyebrow={t("room.history")}
        max_width_class_name="max-w-none"
        show_eyebrow={false}
        title={current_room_type === "dm" ? t("room.history_view_title_dm") : t("room.history_view_title")}
      >
        {ordered_conversations.length > 0 ? (
          <div className="space-y-1.5">
            {ordered_conversations.map((conversation) => {
              const is_external_session = conversation.options?.external_session === true;
              const delete_state = is_external_session
                ? { enabled: false, reason: "外部会话由 IM 通道生成" }
                : resolve_room_conversation_delete_state(
                  conversation,
                  ordered_conversations.length,
                  can_manage_conversations,
                  t,
                );
              return (
                <ConversationHistoryItem
                  key={conversation.conversation_id}
                  can_rename={!is_external_session && can_manage_conversations && on_update_conversation_title !== undefined}
                  conversation={conversation}
                  delete_state={delete_state}
                  is_active={conversation.conversation_id === conversation_id}
                  on_delete={() => set_pending_delete_conversation(conversation)}
                  on_rename={(title) => void on_update_conversation_title?.(conversation.conversation_id, title)}
                  on_select={() => on_select_conversation(conversation.conversation_id)}
                />
              );
            })}
          </div>
        ) : (
          <div className="rounded-[12px] border border-(--divider-subtle-color) px-6 py-10 text-center">
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

      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={Boolean(pending_delete_conversation)}
        message={t("room.delete_conversation_message", {
          title: pending_delete_conversation?.title?.trim() || t("room.untitled_conversation"),
        })}
        on_cancel={() => set_pending_delete_conversation(null)}
        on_confirm={() => {
          const target = pending_delete_conversation;
          set_pending_delete_conversation(null);
          if (target) {
            void on_delete_conversation(target.conversation_id);
          }
        }}
        title={t("room.delete_conversation_title")}
        variant="danger"
      />
    </>
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
  const edit_input_ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (is_editing) {
      edit_input_ref.current?.focus();
    }
  }, [is_editing]);
  const show_actions = !is_editing && (can_rename || delete_state.enabled);
  const external_session_label = get_external_session_label(conversation);

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
        "group relative w-full overflow-hidden rounded-[14px] border px-3 py-2.5 text-left transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast) ease-out",
        is_active
          ? "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
          : "border-transparent bg-transparent hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_64%,transparent)] hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_72%,transparent)]",
      )}
      style={is_active
        ? {
          background: "color-mix(in srgb, var(--surface-interactive-active-background) 46%, transparent)",
          boxShadow: "inset 0 1px 0 rgba(255,255,255,0.56)",
        }
        : undefined}
    >
      {is_active ? (
        <span
          aria-hidden="true"
          className="absolute left-0 top-2.5 bottom-2.5 w-px rounded-full bg-(--primary)"
        />
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          {is_editing ? (
            <div className="flex items-center gap-1.5">
              <input
                aria-label="编辑对话标题"
                className="min-w-0 flex-1 rounded-[10px] border border-(--input-shell-border) bg-transparent px-2.5 py-1.5 text-[13px] font-semibold text-(--text-strong) outline-none transition focus:border-(--surface-interactive-active-border)"
                ref={edit_input_ref}
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
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--primary) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)"
                onClick={confirm_edit}
                type="button"
              >
                <Check className="h-3.5 w-3.5" />
              </button>
              <button
                aria-label="取消"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
                onClick={cancel_edit}
                type="button"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ) : (
            <button
              className="block w-full rounded-[10px] text-left outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]"
              onClick={on_select}
              type="button"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex min-w-0 items-center gap-2">
                    <p className="min-w-0 truncate text-[13px] font-semibold text-(--text-strong)">
                      {conversation.title?.trim() || t("room.untitled_conversation")}
                    </p>
                    {external_session_label ? (
                      <span className="inline-flex shrink-0 items-center rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-1.5 py-0.5 text-[9.5px] font-medium text-(--primary)">
                        IM · {external_session_label}
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10.5px] text-(--text-soft)">
                    <span className="inline-flex items-center gap-1.5">
                      <Clock3 className="h-3 w-3 shrink-0" />
                      <span>{format_relative_time(conversation.last_activity_at)}</span>
                    </span>
                  </div>
                </div>
                <span
                  aria-hidden={!is_active}
                  className={cn(
                    "inline-flex shrink-0 items-center rounded-[6px] border px-1.5 py-0.5 text-[9.5px] font-medium transition-[border-color,color] duration-(--motion-duration-fast)",
                    is_active
                      ? "border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] text-(--primary)"
                      : "invisible border-transparent text-transparent",
                  )}
                >
                  {t("room.current_conversation")}
                </span>
              </div>
            </button>
          )}

          {is_editing ? (
            <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10.5px] text-(--text-soft)">
              <span className="inline-flex items-center gap-1.5">
                <Clock3 className="h-3 w-3 shrink-0" />
                <span>{format_relative_time(conversation.last_activity_at)}</span>
              </span>
            </div>
          ) : null}
        </div>

        {show_actions ? (
          <div className="flex shrink-0 items-center gap-1">
            {can_rename ? (
              <button
                aria-label="重命名"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--icon-default) opacity-0 transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong) focus-visible:opacity-100 group-hover:opacity-100"
                onClick={start_edit}
                type="button"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            ) : null}

            {delete_state.enabled ? (
              <button
                aria-label="删除对话"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--destructive) opacity-0 transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] focus-visible:opacity-100 group-hover:opacity-100"
                onClick={(event) => {
                  event.stopPropagation();
                  on_delete();
                }}
                type="button"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    </article>
  );
}
