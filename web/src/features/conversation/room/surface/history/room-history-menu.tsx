"use client";

import { createPortal } from "react-dom";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock3,
  History,
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

import { RoomHistoryItem } from "./room-history-item";
import {
  buildRoomHistoryEntries,
  paginateRoomHistoryEntries,
  ROOM_HISTORY_PAGE_SIZE,
} from "./room-history-model";

interface RoomHistoryMenuProps {
  canManageConversations?: boolean;
  conversationId: string | null;
  conversations: RoomConversationView[];
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
  triggerVariant?: "history" | "session";
}

const HISTORY_MENU_MAX_HEIGHT = 560;
const HISTORY_MENU_HEADER_ESTIMATED_HEIGHT = 64;
const HISTORY_MENU_FOOTER_ESTIMATED_HEIGHT = 44;
const HISTORY_MENU_MIN_WIDTH = 380;
const HISTORY_ITEM_ESTIMATED_HEIGHT = 42;
const HISTORY_MENU_MIN_HEIGHT = 190;

export function RoomHistoryMenu({
  canManageConversations = true,
  conversationId,
  conversations,
  onDeleteConversation,
  onSelectConversation,
  onUpdateConversationTitle,
  triggerVariant = "history",
}: RoomHistoryMenuProps) {
  const { t } = useI18n();
  const [pageNumber, setPageNumber] = useState(0);
  const [pendingDeleteConversation, setPendingDeleteConversation] = useState<RoomConversationView | null>(null);
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
  const page = useMemo(
    () => paginateRoomHistoryEntries(entries, pageNumber),
    [entries, pageNumber],
  );
  const estimatedItemCount = Math.min(entries.length, ROOM_HISTORY_PAGE_SIZE);
  const estimatedHeight = Math.max(
    HISTORY_MENU_MIN_HEIGHT,
    HISTORY_MENU_HEADER_ESTIMATED_HEIGHT
      + estimatedItemCount * HISTORY_ITEM_ESTIMATED_HEIGHT
      + (page.pageCount > 1 ? HISTORY_MENU_FOOTER_ESTIMATED_HEIGHT : 0),
  );
  const estimatePosition = useCallback((button: HTMLButtonElement) => (
    resolveAnchoredOverlayPosition({
      anchor: button,
      estimatedHeight,
      maxHeight: HISTORY_MENU_MAX_HEIGHT,
      minHeight: HISTORY_MENU_MIN_HEIGHT,
      minWidth: HISTORY_MENU_MIN_WIDTH,
      placement: "auto",
    })
  ), [estimatedHeight]);
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
    disabled: false,
    estimatePosition,
  });
  const historyTitleId = `${menuId}-title`;

  useEffect(() => {
    setPageNumber((current) => Math.min(current, page.pageCount - 1));
  }, [page.pageCount]);
  useEffect(() => {
    setPageNumber(0);
  }, [conversationId, entries.length]);

  const selectConversation = useCallback((id: string) => {
    onSelectConversation(id);
    closeMenu();
    buttonRef.current?.focus();
  }, [buttonRef, closeMenu, onSelectConversation]);
  const requestDelete = useCallback((conversation: RoomConversationView) => {
    setPendingDeleteConversation(conversation);
    closeMenu();
  }, [closeMenu]);

  return (
    <>
      <button
        aria-controls={isOpen ? menuId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={t("room.history")}
        className={cn(
          "inline-flex shrink-0 items-center justify-center bg-transparent text-(--icon-default) transition-[background-color,color] duration-(--motion-duration-fast) hover:text-(--text-strong) focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset",
          triggerVariant === "session"
            ? "workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-history h-8 w-8 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_42%,transparent)]"
            : "workspace-surface-header-control-segment workspace-surface-history-trigger h-9 w-9 rounded-[8px] focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]",
          isOpen && "text-(--text-strong)",
        )}
        data-tour-anchor={CONVERSATION_TOUR_ANCHORS.history_menu}
        onClick={toggleMenu}
        onKeyDown={handleTriggerKeyDown}
        ref={buttonRef}
        title={t("room.history")}
        type="button"
      >
        {triggerVariant === "session" ? (
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

          <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-1.5">
            {page.entries.length > 0 ? (
              <div className="space-y-1">
                {page.entries.map((entry) => (
                  <RoomHistoryItem
                    entry={entry}
                    key={entry.conversation.conversation_id}
                    onDelete={() => requestDelete(entry.conversation)}
                    onRename={(title) => {
                      void onUpdateConversationTitle?.(
                        entry.conversation.conversation_id,
                        title,
                      );
                    }}
                    onSelect={() => selectConversation(entry.conversation.conversation_id)}
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

          {page.pageCount > 1 ? (
            <footer className="flex shrink-0 items-center justify-between border-t border-(--divider-subtle-color) px-2.5 py-1.5">
              <button
                aria-label={t("room.history_page_previous")}
                className="inline-flex h-7 w-7 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                disabled={page.page === 0}
                onClick={() => setPageNumber((current) => Math.max(0, current - 1))}
                type="button"
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
              <span className="text-xs text-(--text-soft)">
                {t("room.history_page_status", {
                  current: page.page + 1,
                  total: page.pageCount,
                })}
              </span>
              <button
                aria-label={t("room.history_page_next")}
                className="inline-flex h-7 w-7 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                disabled={page.page >= page.pageCount - 1}
                onClick={() => setPageNumber((current) => Math.min(page.pageCount - 1, current + 1))}
                type="button"
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </footer>
          ) : null}
        </div>,
        portalContainer,
      ) : null}

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
