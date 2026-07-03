"use client";

import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, Clock3, MessageSquarePlus, Pencil, Trash2, X } from "lucide-react";

import { getSessionChannelLabel } from "@/features/conversation/external-session-labels";
import { cn, formatRelativeTime } from "@/lib/utils";
import {
  ConversationDeleteState,
  resolveRoomConversationDeleteState,
} from "@/lib/conversation/room-conversation-delete";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { RoomConversationView } from "@/types/conversation/conversation";

interface RoomHistorySurfaceProps {
  canManageConversations?: boolean;
  conversations: RoomConversationView[];
  conversationId: string | null;
  currentRoomType: string;
  headerAction?: ReactNode;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
}

function compareConversationsByRecentActivity(
  left: RoomConversationView,
  right: RoomConversationView,
): number {
  if (left.last_activity_at !== right.last_activity_at) {
    return right.last_activity_at - left.last_activity_at;
  }
  if (left.created_at !== right.created_at) {
    return right.created_at - left.created_at;
  }
  return left.conversation_id.localeCompare(right.conversation_id);
}

function stringOption(options: Record<string, unknown>, key: string): string | null {
  const value = options[key];
  return typeof value === "string" ? value : null;
}

function getExternalSessionLabel(conversation: RoomConversationView): string | null {
  if (conversation.options?.external_session !== true) {
    return null;
  }
  return getSessionChannelLabel(
    stringOption(conversation.options, "channel_type"),
    conversation.session_key,
  );
}

export function RoomHistorySurface({
  canManageConversations: canManageConversations = true,
  conversations,
  conversationId: conversationId,
  currentRoomType: currentRoomType,
  headerAction: headerAction,
  onCreateConversation: onCreateConversation,
  onDeleteConversation: onDeleteConversation,
  onSelectConversation: onSelectConversation,
  onUpdateConversationTitle: onUpdateConversationTitle,
}: RoomHistorySurfaceProps) {
  const { t } = useI18n();
  const [pendingDeleteConversation, setPendingDeleteConversation] = useState<RoomConversationView | null>(null);
  const orderedConversations = useMemo(
    () => [...conversations].sort(compareConversationsByRecentActivity),
    [conversations],
  );

  const createAction = canManageConversations ? (
    <WorkspaceSurfaceToolbarAction
      onClick={() => {
        void onCreateConversation();
      }}
      tone="primary"
    >
      <MessageSquarePlus className="h-3.5 w-3.5" />
      {t("room.new_conversation")}
    </WorkspaceSurfaceToolbarAction>
  ) : null;

  const action = createAction || headerAction ? (
    <div className="flex items-center gap-3">
      {createAction}
      {headerAction}
    </div>
  ) : null;

  return (
    <>
      <WorkspaceSurfaceView
        action={action}
        bodyClassName="px-4 py-3.5 sm:px-5 xl:px-6"
        contentClassName="space-y-1.5"
        eyebrow={t("room.history")}
        maxWidthClassName="max-w-none"
        showEyebrow={false}
        title={currentRoomType === "dm" ? t("room.history_view_title_dm") : t("room.history_view_title")}
      >
        {orderedConversations.length > 0 ? (
          <div className="space-y-1.5">
            {orderedConversations.map((conversation) => {
              const isExternalSession = conversation.options?.external_session === true;
              const deleteState = isExternalSession
                ? { enabled: false, reason: "外部会话由 IM 通道生成" }
                : resolveRoomConversationDeleteState(
                  conversation,
                  orderedConversations.length,
                  canManageConversations,
                  t,
                );
              return (
                <ConversationHistoryItem
                  key={conversation.conversation_id}
                  canRename={!isExternalSession && canManageConversations && onUpdateConversationTitle !== undefined}
                  conversation={conversation}
                  deleteState={deleteState}
                  isActive={conversation.conversation_id === conversationId}
                  onDelete={() => setPendingDeleteConversation(conversation)}
                  onRename={(title) => void onUpdateConversationTitle?.(conversation.conversation_id, title)}
                  onSelect={() => onSelectConversation(conversation.conversation_id)}
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
            {canManageConversations ? (
              <button
                className="mt-4 inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--primary) transition duration-(--motion-duration-fast) ease-out hover:text-[color:color-mix(in_srgb,var(--primary)_84%,var(--foreground)_16%)]"
                onClick={() => {
                  void onCreateConversation();
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
        confirmText={t("common.delete")}
        isOpen={Boolean(pendingDeleteConversation)}
        message={t("room.delete_conversation_message", {
          title: pendingDeleteConversation?.title?.trim() || t("room.untitled_conversation"),
        })}
        onCancel={() => setPendingDeleteConversation(null)}
        onConfirm={() => {
          const target = pendingDeleteConversation;
          setPendingDeleteConversation(null);
          if (target) {
            void onDeleteConversation(target.conversation_id);
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
  canRename: canRename,
  conversation,
  deleteState: deleteState,
  isActive: isActive,
  onDelete: onDelete,
  onRename: onRename,
  onSelect: onSelect,
}: {
  canRename: boolean;
  conversation: RoomConversationView;
  deleteState: ConversationDeleteState;
  isActive: boolean;
  onDelete: () => void;
  onRename: (title: string) => void;
  onSelect: () => void;
}) {
  const { t } = useI18n();
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isEditing) {
      editInputRef.current?.focus();
    }
  }, [isEditing]);
  const showActions = !isEditing && (canRename || deleteState.enabled);
  const externalSessionLabel = getExternalSessionLabel(conversation);

  const startEdit = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setEditValue(conversation.title?.trim() || "");
    setIsEditing(true);
  }, [conversation.title]);

  const confirmEdit = useCallback(() => {
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== conversation.title?.trim()) {
      onRename(trimmed);
    }
    setIsEditing(false);
  }, [editValue, conversation.title, onRename]);

  const cancelEdit = useCallback(() => {
    setIsEditing(false);
  }, []);

  return (
    <article
      className={cn(
        "group relative w-full overflow-hidden rounded-[14px] border px-3 py-2.5 text-left transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast) ease-out",
        isActive
          ? "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
          : "border-transparent bg-transparent hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_64%,transparent)] hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_72%,transparent)]",
      )}
      style={isActive
        ? {
          background: "color-mix(in srgb, var(--surface-interactive-active-background) 46%, transparent)",
          boxShadow: "inset 0 1px 0 rgba(255,255,255,0.56)",
        }
        : undefined}
    >
      {isActive ? (
        <span
          aria-hidden="true"
          className="absolute left-0 top-2.5 bottom-2.5 w-px rounded-full bg-(--primary)"
        />
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          {isEditing ? (
            <div className="flex items-center gap-1.5">
              <input
                aria-label="编辑对话标题"
                className="min-w-0 flex-1 rounded-[10px] border border-(--input-shell-border) bg-transparent px-2.5 py-1.5 text-[13px] font-semibold text-(--text-strong) outline-none transition focus:border-(--surface-interactive-active-border)"
                ref={editInputRef}
                maxLength={64}
                onChange={(e) => setEditValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") confirmEdit();
                  if (e.key === "Escape") cancelEdit();
                }}
                value={editValue}
              />
              <button
                aria-label="确认"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--primary) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)"
                onClick={confirmEdit}
                type="button"
              >
                <Check className="h-3.5 w-3.5" />
              </button>
              <button
                aria-label="取消"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
                onClick={cancelEdit}
                type="button"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ) : (
            <button
              className="block w-full rounded-[10px] text-left outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]"
              onClick={onSelect}
              type="button"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex min-w-0 items-center gap-2">
                    <p className="min-w-0 truncate text-[13px] font-semibold text-(--text-strong)">
                      {conversation.title?.trim() || t("room.untitled_conversation")}
                    </p>
                    {externalSessionLabel ? (
                      <span className="inline-flex shrink-0 items-center rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-1.5 py-0.5 text-[9.5px] font-medium text-(--primary)">
                        IM · {externalSessionLabel}
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10.5px] text-(--text-soft)">
                    <span className="inline-flex items-center gap-1.5">
                      <Clock3 className="h-3 w-3 shrink-0" />
                      <span>{formatRelativeTime(conversation.last_activity_at)}</span>
                    </span>
                  </div>
                </div>
                <span
                  aria-hidden={!isActive}
                  className={cn(
                    "inline-flex shrink-0 items-center rounded-[6px] border px-1.5 py-0.5 text-[9.5px] font-medium transition-[border-color,color] duration-(--motion-duration-fast)",
                    isActive
                      ? "border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] text-(--primary)"
                      : "invisible border-transparent text-transparent",
                  )}
                >
                  {t("room.current_conversation")}
                </span>
              </div>
            </button>
          )}

          {isEditing ? (
            <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10.5px] text-(--text-soft)">
              <span className="inline-flex items-center gap-1.5">
                <Clock3 className="h-3 w-3 shrink-0" />
                <span>{formatRelativeTime(conversation.last_activity_at)}</span>
              </span>
            </div>
          ) : null}
        </div>

        {showActions ? (
          <div className="flex shrink-0 items-center gap-1">
            {canRename ? (
              <button
                aria-label="重命名"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--icon-default) opacity-0 transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong) focus-visible:opacity-100 group-hover:opacity-100"
                onClick={startEdit}
                type="button"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            ) : null}

            {deleteState.enabled ? (
              <button
                aria-label="删除对话"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--destructive) opacity-0 transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] focus-visible:opacity-100 group-hover:opacity-100"
                onClick={(event) => {
                  event.stopPropagation();
                  onDelete();
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
