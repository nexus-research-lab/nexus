/**
 * INPUT: Room 会话目录、当前会话和既有创建/选择/删除/重命名命令。
 * OUTPUT: 带固定高度滚动列表、标题编辑、多选和全量清空确认的锚定历史菜单。
 * POS: Room Header 历史入口的交互装配层，不解释底层会话协议。
 */

"use client";

import { createPortal } from "react-dom";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Check,
  ChevronDown,
  Clock3,
  History,
  Loader2,
  Minus,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { cn } from "@/shared/ui/class-name";
import { useSelectMenuOverlay } from "@/shared/ui/menu/use-select-menu-overlay";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import type { RoomConversationView } from "@/types/conversation/conversation";
import {
  getExternalSessionTaskReferenceCount,
  isExternalSessionConversation,
} from "@/lib/conversation/external-session";

import { deleteRoomHistoryConversationBatch } from "./room-history-bulk-delete";
import { RoomHistoryItem } from "./room-history-item";
import { buildRoomHistoryEntries } from "./room-history-model";
import {
  getRoomHistorySelectionState,
  useRoomHistorySelection,
} from "./room-history-selection";

interface RoomHistoryMenuProps {
  canManageConversations?: boolean;
  conversationId: string | null;
  conversations: RoomConversationView[];
  onCreateConversation: (title?: string) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
  triggerVariant?: "history" | "session";
}

const HISTORY_MENU_HEIGHT = 280;
const HISTORY_MENU_MIN_WIDTH = 330;
const HISTORY_MENU_MIN_HEIGHT = 190;

interface PendingRoomHistoryBulkDelete {
  clearsHistory: boolean;
  conversationIds: string[];
}

export function RoomHistoryMenu({
  canManageConversations = true,
  conversationId,
  conversations,
  onCreateConversation,
  onDeleteConversation,
  onSelectConversation,
  onUpdateConversationTitle,
  triggerVariant = "history",
}: RoomHistoryMenuProps) {
  const { t } = useI18n();
  const [pendingDeleteConversation, setPendingDeleteConversation] = useState<RoomConversationView | null>(null);
  const [
    pendingBulkDelete,
    setPendingBulkDelete,
  ] = useState<PendingRoomHistoryBulkDelete | null>(null);
  const [isBulkDeleting, setIsBulkDeleting] = useState(false);
  const [bulkDeleteFailureCount, setBulkDeleteFailureCount] = useState<number | null>(null);
  const entries = useMemo(() => buildRoomHistoryEntries({
    canManageConversations,
    canUpdateConversationTitle: onUpdateConversationTitle !== undefined,
    conversations,
    currentConversationId: conversationId,
  }), [
    canManageConversations,
    conversationId,
    conversations,
    onUpdateConversationTitle,
  ]);
  const {
    clearSelection,
    hasSelectableEntries,
    isSelecting,
    restoreSelection,
    selectedIds,
    startSelection,
    toggleAllSelection,
    toggleSelection,
  } = useRoomHistorySelection({entries});
  const selectionState = getRoomHistorySelectionState(
    selectedIds,
    entries,
  );
  const estimatePosition = useCallback((button: HTMLButtonElement) => (
    resolveAnchoredOverlayPosition({
      anchor: button,
      estimatedHeight: HISTORY_MENU_HEIGHT,
      maxHeight: HISTORY_MENU_HEIGHT,
      minHeight: HISTORY_MENU_MIN_HEIGHT,
      minWidth: HISTORY_MENU_MIN_WIDTH,
      placement: "auto",
    })
  ), []);
  const {
    buttonRef,
    closeMenu,
    handleTriggerKeyDown,
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    toggleMenu,
  } = useSelectMenuOverlay({
    disabled: isBulkDeleting,
    estimatePosition,
  });
  const historyTitleId = `${menuId}-title`;

  useEffect(() => {
    if (!isOpen) {
      clearSelection();
    }
  }, [clearSelection, isOpen]);

  const selectConversation = useCallback((id: string) => {
    onSelectConversation(id);
    closeMenu();
    buttonRef.current?.focus();
  }, [buttonRef, closeMenu, onSelectConversation]);
  const requestDelete = useCallback((conversation: RoomConversationView) => {
    setPendingDeleteConversation(conversation);
    closeMenu();
  }, [closeMenu]);
  const requestBulkDelete = useCallback(() => {
    const conversationIds = entries
      .filter((entry) => selectedIds.has(entry.conversation.conversation_id))
      .map((entry) => entry.conversation.conversation_id);
    if (conversationIds.length === 0) {
      return;
    }
    setPendingBulkDelete({
      clearsHistory: selectionState === "all",
      conversationIds,
    });
    closeMenu();
  }, [closeMenu, entries, selectedIds, selectionState]);
  const cancelBulkDelete = useCallback(() => {
    const conversationIds = pendingBulkDelete?.conversationIds ?? [];
    setPendingBulkDelete(null);
    restoreSelection(conversationIds);
    toggleMenu();
  }, [
    pendingBulkDelete,
    restoreSelection,
    toggleMenu,
  ]);
  const confirmBulkDelete = useCallback(async () => {
    const pendingDelete = pendingBulkDelete;
    const conversationIds = pendingDelete?.conversationIds ?? [];
    setPendingBulkDelete(null);
    setBulkDeleteFailureCount(null);
    if (conversationIds.length === 0) {
      return;
    }

    setIsBulkDeleting(true);
    const {
      failedConversationIds,
      replacementConversationId,
    } = await deleteRoomHistoryConversationBatch(
      conversationIds,
      onDeleteConversation,
      {
        currentConversationId: conversationId,
        createReplacementConversation: pendingDelete?.clearsHistory
          ? onCreateConversation
          : undefined,
      },
    );
    setIsBulkDeleting(false);
    if (replacementConversationId) {
      onSelectConversation(replacementConversationId);
    }
    if (failedConversationIds.length > 0) {
      setBulkDeleteFailureCount(failedConversationIds.length);
      restoreSelection(failedConversationIds);
      toggleMenu();
    }
  }, [
    conversationId,
    onCreateConversation,
    onDeleteConversation,
    onSelectConversation,
    pendingBulkDelete,
    restoreSelection,
    toggleMenu,
  ]);
  const toggleSelectionMode = useCallback(() => {
    setBulkDeleteFailureCount(null);
    if (isSelecting) {
      clearSelection();
      return;
    }
    startSelection();
  }, [clearSelection, isSelecting, startSelection]);

  return (
    <>
      <button
        aria-controls={isOpen ? menuId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={isBulkDeleting
          ? t("room.history_batch_deleting")
          : t("room.history")}
        className={cn(
          "inline-flex shrink-0 items-center justify-center bg-transparent text-(--icon-default) transition-[background-color,color] duration-(--motion-duration-fast) hover:text-(--text-strong) focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset",
          triggerVariant === "session"
            ? "workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-history h-8 w-8 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_42%,transparent)]"
            : "workspace-surface-header-control-segment workspace-surface-history-trigger h-9 w-9 rounded-[8px] focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]",
          isOpen && "text-(--text-strong)",
        )}
        data-tour-anchor={CONVERSATION_TOUR_ANCHORS.history_menu}
        disabled={isBulkDeleting}
        onClick={toggleMenu}
        onKeyDown={handleTriggerKeyDown}
        ref={buttonRef}
        title={isBulkDeleting
          ? t("room.history_batch_deleting")
          : t("room.history")}
        type="button"
      >
        {isBulkDeleting ? (
          <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />
        ) : triggerVariant === "session" ? (
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 shrink-0 transition-transform duration-(--motion-duration-fast)",
              isOpen && "rotate-180",
            )}
          />
        ) : (
          <History className="h-3.5 w-3.5 shrink-0" />
        )}
      </button>

      {isOpen && portalContainer ? createPortal(
        <div
          ref={menuRef}
          aria-labelledby={historyTitleId}
          className={cn(
            "fixed z-[130] flex flex-col overflow-hidden",
            OVERLAY_SURFACE_CLASS_NAME,
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
          )}
          data-placement={menuPosition?.placement ?? "bottom"}
          data-state="open"
          id={menuId}
          role="dialog"
          style={{
            ...menuStyle,
            height: menuStyle.maxHeight,
          }}
        >
          <header className="flex shrink-0 items-center justify-between border-b border-(--divider-subtle-color) px-3.5 py-2">
            <div className="min-w-0">
              <h2
                className="truncate text-compact font-semibold text-(--text-strong)"
                id={historyTitleId}
              >
                {t("room.history")}
              </h2>
              <p className="mt-0.5 text-2xs text-(--text-soft)">
                {t("room.conversation_count", { count: entries.length })}
              </p>
            </div>
          </header>

          {isSelecting ? (
            <div className="shrink-0 border-b border-(--divider-subtle-color) px-2.5 py-1.5">
              <div className="flex items-center justify-between gap-3">
                <button
                  aria-checked={selectionState === "mixed"
                    ? "mixed"
                    : selectionState === "all"}
                  className="inline-flex h-6 min-w-0 items-center gap-2 rounded-[7px] px-1.5 text-xs font-medium text-(--text-default) transition-colors hover:bg-(--surface-interactive-hover-background) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                  disabled={!hasSelectableEntries}
                  onClick={toggleAllSelection}
                  role="checkbox"
                  type="button"
                >
                  <span className={cn(
                    "grid h-3.5 w-3.5 shrink-0 place-items-center rounded-[4px] border transition-colors",
                    selectionState === "none"
                      ? "border-(--input-shell-border) bg-transparent"
                      : "border-(--primary) bg-(--primary) text-white",
                  )}>
                    {selectionState === "all" ? (
                      <Check className="h-2.5 w-2.5" strokeWidth={3} />
                    ) : selectionState === "mixed" ? (
                      <Minus className="h-2.5 w-2.5" strokeWidth={3} />
                    ) : null}
                  </span>
                  <span className="truncate">{t("room.history_select_all")}</span>
                </button>
                <span className="shrink-0 text-2xs text-(--text-soft)">
                  {t("room.history_selection_count", {
                    count: selectedIds.size,
                  })}
                </span>
              </div>
              {bulkDeleteFailureCount ? (
                <p
                  className="px-1.5 pt-1 text-2xs text-(--destructive)"
                  role="alert"
                >
                  {t("room.history_batch_delete_failed", {
                    count: bulkDeleteFailureCount,
                  })}
                </p>
              ) : null}
            </div>
          ) : null}

          <div
            className="soft-scrollbar min-h-0 flex-1 overflow-auto overscroll-contain p-1.5"
            data-room-history-scroll-viewport
          >
            {entries.length > 0 ? (
              <div
                className="min-w-full space-y-1 pb-1"
                data-room-history-scroll-content
              >
                {entries.map((entry) => (
                  <RoomHistoryItem
                    entry={entry}
                    isSelected={selectedIds.has(entry.conversation.conversation_id)}
                    isSelecting={isSelecting}
                    key={entry.conversation.conversation_id}
                    onDelete={() => requestDelete(entry.conversation)}
                    onRename={(title) => {
                      void onUpdateConversationTitle?.(
                        entry.conversation.conversation_id,
                        title,
                      );
                    }}
                    onSelect={() => selectConversation(entry.conversation.conversation_id)}
                    onToggleSelection={() => toggleSelection(
                      entry.conversation.conversation_id,
                    )}
                  />
                ))}
              </div>
            ) : (
              <div className="flex h-full min-h-[150px] flex-col items-center justify-center px-5 py-8 text-center">
                <Clock3 className="h-4 w-4 text-(--icon-muted)" />
                <p className="mt-3 text-sm font-semibold text-(--text-strong)">
                  {t("room.no_conversations")}
                </p>
                <p className="mt-1 text-xs leading-5 text-(--text-soft)">
                  {t("room.history_empty_hint")}
                </p>
              </div>
            )}
          </div>

          {hasSelectableEntries || isSelecting ? (
            <footer className="flex shrink-0 items-center justify-between gap-2 border-t border-(--divider-subtle-color) px-2.5 py-1.5">
              {hasSelectableEntries ? (
                <button
                  className="inline-flex h-7 shrink-0 items-center rounded-[7px] px-1.5 text-xs font-medium text-(--text-soft) outline-none transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:ring-1 focus-visible:ring-[color:color-mix(in_srgb,var(--text-default)_20%,transparent)]"
                  onClick={toggleSelectionMode}
                  type="button"
                >
                  {isSelecting
                    ? t("room.history_cancel_selection")
                    : t("room.history_select")}
                </button>
              ) : <span />}
              {isSelecting ? (
                <button
                  className="inline-flex h-7 shrink-0 items-center rounded-[8px] px-2 text-xs font-semibold text-(--destructive) transition-colors hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                  disabled={selectedIds.size === 0}
                  onClick={requestBulkDelete}
                  type="button"
                >
                  {selectionState === "all"
                    ? t("room.history_clear")
                    : t("room.history_batch_delete")}
                </button>
              ) : <span />}
            </footer>
          ) : null}
        </div>,
        portalContainer,
      ) : null}

      <ConfirmDialog
        cancelText={t("common.cancel")}
        confirmText={t("common.delete")}
        isOpen={Boolean(pendingDeleteConversation)}
        message={pendingDeleteConversation
          && isExternalSessionConversation(pendingDeleteConversation)
          && getExternalSessionTaskReferenceCount(pendingDeleteConversation) > 0
          ? t("room.delete_external_session_with_tasks_message", {
              count: getExternalSessionTaskReferenceCount(pendingDeleteConversation),
              title: pendingDeleteConversation.title?.trim() || t("room.new_conversation"),
            })
          : t("room.delete_conversation_message", {
              title: pendingDeleteConversation?.title?.trim() || t("room.new_conversation"),
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
      <ConfirmDialog
        cancelText={t("common.cancel")}
        confirmText={pendingBulkDelete?.clearsHistory
          ? t("room.history_clear")
          : t("room.history_batch_delete")}
        isOpen={Boolean(pendingBulkDelete)}
        message={pendingBulkDelete?.clearsHistory
          ? t("room.history_clear_message")
          : t("room.history_batch_delete_message", {
              count: pendingBulkDelete?.conversationIds.length ?? 0,
            })}
        onCancel={cancelBulkDelete}
        onConfirm={() => {
          void confirmBulkDelete();
        }}
        title={pendingBulkDelete?.clearsHistory
          ? t("room.history_clear_title")
          : t("room.history_batch_delete_title")}
        variant="danger"
      />
    </>
  );
}
