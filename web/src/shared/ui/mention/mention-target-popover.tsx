"use client";

import { memo, useCallback, useEffect, useMemo, useRef } from "react";
import { createPortal } from "react-dom";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
  MENU_LIST_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";

import {
  filterMentionTargets,
  getMentionKeyboardAction,
  getMentionPopoverLayout,
  type MentionKeyboardAction,
  type MentionPlacement,
  type MentionTargetItem,
} from "./mention-target-model";

interface MentionTargetPopoverProps {
  anchorRect: DOMRect | null;
  filter: string;
  items: MentionTargetItem[];
  onClose: () => void;
  onSelect: (item: MentionTargetItem) => void;
  placement?: MentionPlacement;
}

export const MentionTargetPopover = memo(function MentionTargetPopover({
  anchorRect,
  filter,
  items,
  onClose,
  onSelect,
  placement = "auto",
}: MentionTargetPopoverProps) {
  const [activeIndex, setActiveIndex] = useResettableState(0, filter);
  const listRef = useRef<HTMLDivElement>(null);
  const filteredItems = useMemo(
    () => filterMentionTargets(items, filter),
    [filter, items],
  );
  const visibleActiveIndex = Math.min(
    activeIndex,
    Math.max(filteredItems.length - 1, 0),
  );
  const activeItem = filteredItems[visibleActiveIndex];

  useEffect(() => {
    if (filteredItems.length === 0) {
      onClose();
    }
  }, [filteredItems.length, onClose]);

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    const action = getMentionKeyboardAction(event.key);
    if (!action || filteredItems.length === 0) {
      return;
    }
    const commands: Readonly<Record<MentionKeyboardAction, () => void>> = {
      next: () => setActiveIndex((current) => (current + 1) % filteredItems.length),
      previous: () => setActiveIndex((current) =>
        (current - 1 + filteredItems.length) % filteredItems.length),
      select: () => activeItem && onSelect(activeItem),
      close: onClose,
    };
    event.preventDefault();
    event.stopPropagation();
    commands[action]();
  }, [activeItem, filteredItems.length, onClose, onSelect, setActiveIndex]);

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown, true);
    return () => document.removeEventListener("keydown", handleKeyDown, true);
  }, [handleKeyDown]);

  useEffect(() => {
    const activeElement = listRef.current?.children[visibleActiveIndex] as HTMLElement | undefined;
    activeElement?.scrollIntoView({ block: "nearest" });
  }, [visibleActiveIndex]);

  if (!anchorRect || filteredItems.length === 0) {
    return null;
  }
  const layout = getMentionPopoverLayout(anchorRect, filteredItems.length, placement);

  return createPortal(
    <div
      className={cn(
        "fixed ui-layer-dialog max-h-48 overflow-y-auto",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      style={layout}
    >
      <div className={cn(MENU_LIST_CLASS_NAME, "p-1")} ref={listRef}>
        {filteredItems.map((item, index) => (
          <button
            className={cn(
              MENU_ITEM_BASE_CLASS_NAME,
              "flex items-center gap-2 px-2.5 py-2 text-sm",
              getMenuItemStateClassName({
                active: index === visibleActiveIndex,
              }),
            )}
            key={item.id}
            onMouseDown={(event) => {
              event.preventDefault();
              onSelect(item);
            }}
            onMouseEnter={() => setActiveIndex(index)}
            type="button"
          >
            <span
              className="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
              style={{
                background: "var(--surface-avatar-background)",
                boxShadow: "var(--surface-avatar-shadow)",
                color: "var(--surface-avatar-foreground)",
              }}
            >
              {item.marker}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{item.label}</span>
              {item.subtitle ? (
                <span className="block truncate text-xs text-(--text-soft)">
                  {item.subtitle}
                </span>
              ) : null}
            </span>
          </button>
        ))}
      </div>
    </div>,
    document.body,
  );
});
