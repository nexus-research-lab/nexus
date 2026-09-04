/**
 * INPUT: Room 会话目录、当前会话和既有创建/选择/删除/重命名命令。
 * OUTPUT: 带固定高度列表、批量操作，以及未确认删除项 Problem/Impact/Recovery 的历史菜单。
 * POS: Room Header 历史交互层；只按命令结果展示恢复事实，不猜测底层提交状态。
 */

"use client";

import { createPortal } from "react-dom";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ChevronDown,
  History,
  Loader2,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { cn } from "@/shared/ui/class-name";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiListSectionDivider } from "@/shared/ui/list/list-section-divider";
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
import {
  buildRoomHistoryEntries,
  groupRoomHistoryEntries,
  type RoomHistoryEntry,
} from "./room-history-model";
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

interface RoomHistoryBulkDeleteFailure {
  failedCount: number;
  totalCount: number;
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
  const [bulkDeleteFailure, setBulkDeleteFailure] = useState<RoomHistoryBulkDeleteFailure | null>(null);
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
  const entryGroups = useMemo(() => groupRoomHistoryEntries(entries), [entries]);
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
  const triggerLabel = isBulkDeleting
    ? t("room.history_batch_deleting")
    : t("room.history");

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
    setBulkDeleteFailure(null);
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
      setBulkDeleteFailure({
        failedCount: failedConversationIds.length,
        totalCount: conversationIds.length,
      });
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
    setBulkDeleteFailure(null);
    if (isSelecting) {
      clearSelection();
      return;
    }
    startSelection();
  }, [clearSelection, isSelecting, startSelection]);
  const renderHistoryEntry = (entry: RoomHistoryEntry) => (
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
  );

  return (
    <>
      <UiIconButton
        aria-controls={isOpen ? menuId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={triggerLabel}
        className={cn(
          triggerVariant === "session"
            ? "workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-history focus-visible:z-10"
            : "workspace-surface-header-control-segment workspace-surface-history-trigger",
        )}
        data-tour-anchor={CONVERSATION_TOUR_ANCHORS.history_menu}
        disabled={isBulkDeleting}
        onClick={toggleMenu}
        onKeyDown={handleTriggerKeyDown}
        ref={buttonRef}
        size={triggerVariant === "session" ? "md" : "lg"}
        tooltip={triggerLabel}
        variant="ghost"
      >
        {isBulkDeleting ? (
          <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
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
      </UiIconButton>

      {isOpen && portalContainer ? createPortal(
        <div
          ref={menuRef}
          aria-labelledby={historyTitleId}
          className={cn(
            "fixed ui-layer-action-menu flex flex-col overflow-hidden",
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
          <header className="flex shrink-0 items-center border-b border-(--divider-subtle-color) px-3.5 py-2.5">
            <h2
              className="truncate text-compact font-semibold text-(--text-strong)"
              id={historyTitleId}
            >
              {t("room.history")}
            </h2>
          </header>

          {isSelecting ? (
            <div className="shrink-0 border-b border-(--divider-subtle-color) px-2.5 py-1.5">
              <div className="flex items-center justify-between gap-3">
                <label
                  className={cn(
                    "inline-flex h-6 min-w-0 items-center gap-2 radius-control-xs px-1.5 text-xs font-medium text-(--text-default) transition-colors hover:bg-(--surface-interactive-hover-background)",
                    !hasSelectableEntries && "cursor-not-allowed opacity-(--disabled-opacity)",
                  )}
                >
                  <UiCheckbox
                    checked={selectionState === "all"}
                    checkboxSize="small"
                    disabled={!hasSelectableEntries}
                    indeterminate={selectionState === "mixed"}
                    onChange={toggleAllSelection}
                  />
                  <span className="truncate">{t("room.history_select_all")}</span>
                </label>
                <span className="shrink-0 text-2xs text-(--text-soft)">
                  {t("room.history_selection_count", {
                    count: selectedIds.size,
                  })}
                </span>
              </div>
              {bulkDeleteFailure ? (
                <div
                  aria-atomic="true"
                  aria-live="polite"
                  className="space-y-0.5 px-1.5 pt-1 text-2xs leading-4"
                  role="status"
                >
                  <p className="font-medium text-(--destructive)">
                    {t("room.history_batch_delete_failed", {
                      count: bulkDeleteFailure.failedCount,
                    })}
                  </p>
                  <p className="text-(--text-muted)">
                    {t("room.history_batch_delete_impact", {
                      completed: bulkDeleteFailure.totalCount - bulkDeleteFailure.failedCount,
                      pending: bulkDeleteFailure.failedCount,
                    })}
                  </p>
                  <p className="font-medium text-(--text-default)">
                    {t("room.history_batch_delete_next_step")}
                  </p>
                </div>
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
                {entryGroups.history.length > 0 ? (
                  <div className="space-y-1" data-room-history-section="history">
                    {entryGroups.history.map(renderHistoryEntry)}
                  </div>
                ) : null}
                {entryGroups.im.length > 0 ? (
                  <>
                    <UiListSectionDivider aria-label="IM" label="IM" />
                    <div className="space-y-1" data-room-history-section="im">
                      {entryGroups.im.map(renderHistoryEntry)}
                    </div>
                  </>
                ) : null}
              </div>
            ) : (
              <div className="flex h-full min-h-[150px] items-center justify-center px-5 py-8 text-center">
                <p className="text-sm text-(--text-soft)">
                  {t("room.no_conversations")}
                </p>
              </div>
            )}
          </div>

          {hasSelectableEntries || isSelecting ? (
            <footer className="flex shrink-0 items-center justify-between gap-2 border-t border-(--divider-subtle-color) px-2.5 py-1.5">
              {hasSelectableEntries ? (
                <UiButton
                  className="shrink-0"
                  onClick={toggleSelectionMode}
                  size="xs"
                  variant="text"
                >
                  {isSelecting
                    ? t("room.history_cancel_selection")
                    : t("room.history_select")}
                </UiButton>
              ) : <span />}
              {isSelecting ? (
                <UiButton
                  className="shrink-0"
                  disabled={selectedIds.size === 0}
                  onClick={requestBulkDelete}
                  size="xs"
                  tone="danger"
                  variant="text"
                >
                  {selectionState === "all"
                    ? t("room.history_clear")
                    : t("room.history_batch_delete")}
                </UiButton>
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
