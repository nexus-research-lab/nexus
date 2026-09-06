// INPUT: 真实输入锚点、打开状态、候选和筛选文本与选择/关闭命令。
// OUTPUT: 公共 listbox、实时锚定定位与当前候选关联，输入保持焦点及 IME 所有权。
// POS: Mention 交互 pattern；浮层生命周期/几何归 Overlay，行尺寸/DOM 归 Menu。
"use client";

import { memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, type RefObject } from "react";
import { createPortal } from "react-dom";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import {
  getMenuItemStateClassName,
  getMenuItemLayout,
  MENU_ITEM_GAP_PX,
  MENU_LIST_CLASS_NAME,
  MENU_SURFACE_VERTICAL_PADDING_PX,
} from "@/shared/ui/menu/menu-styles";
import { SelectMenuOptionRow, SelectMenuPanel } from "@/shared/ui/menu/select-menu-primitives";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { isTopAnchoredOverlay } from "@/shared/ui/overlay/overlay-dismissal-runtime";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import {
  filterMentionTargets,
  getMentionKeyboardAction,
  type MentionKeyboardAction,
  type MentionTargetItem,
} from "./mention-target-model";

interface MentionTargetPopoverProps {
  anchorRef: RefObject<HTMLInputElement | HTMLTextAreaElement | null>;
  filter: string;
  items: MentionTargetItem[];
  onClose: () => void;
  onSelect: (item: MentionTargetItem) => void;
  isOpen?: boolean;
}

export const MentionTargetPopover = memo(function MentionTargetPopover({
  anchorRef,
  filter,
  items,
  onClose,
  onSelect,
  isOpen = true,
}: MentionTargetPopoverProps) {
  const { t } = useI18n();
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
  const open = isOpen && filteredItems.length > 0;
  const estimatePosition = useCallback((anchor: HTMLElement) => resolveUiAnchoredOverlayPosition({
    anchor,
    placement: "auto",
    preset: "reference-list",
    estimatedContentHeight: MENU_SURFACE_VERTICAL_PADDING_PX
      + filteredItems.reduce((height, item) => height + getMenuItemLayout({ hasDescription: Boolean(item.subtitle) }).height, 0)
      + MENU_ITEM_GAP_PX * Math.max(0, filteredItems.length - 1),
  }), [filteredItems]);
  const { overlayId, overlayPosition, overlayRef, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef,
    captureEscape: true,
    disabled: false,
    estimatePosition,
    isOpen: open,
    onClose,
    restoreFocus: false,
  });
  const activeOptionId = activeItem ? `${overlayId}-option-${visibleActiveIndex}` : undefined;

  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    if (!open || !anchor || !activeOptionId) return;
    // The editor owns value/events; this attached list owns only its temporary ARIA relationship.
    const attributes = { "aria-controls": overlayId, "aria-activedescendant": activeOptionId, "aria-autocomplete": "list" };
    const previous = new Map(Object.keys(attributes).map((name) => [name, anchor.getAttribute(name)]));
    for (const [name, value] of Object.entries(attributes)) anchor.setAttribute(name, value);
    return () => {
      for (const [name, value] of Object.entries(attributes)) {
        if (anchor.getAttribute(name) !== value) continue;
        const old = previous.get(name);
        if (old == null) anchor.removeAttribute(name);
        else anchor.setAttribute(name, old);
      }
    };
  }); // Reconcile after each commit: a stable ref can point to a replacement editor node.

  useEffect(() => {
    if (isOpen && filteredItems.length === 0) {
      onClose();
    }
  }, [filteredItems.length, isOpen, onClose]);

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    const target = event.target;
    if (event.defaultPrevented || isImeKeyboardEvent(event)
      || !isTopAnchoredOverlay(overlayRef.current)
      || !(target instanceof Node)
      || !(anchorRef.current?.contains(target) || overlayRef.current?.contains(target))) {
      return;
    }
    const action = getMentionKeyboardAction(event.key);
    if (!action || action === "close" || filteredItems.length === 0) {
      return;
    }
    const commands: Readonly<Record<Exclude<MentionKeyboardAction, "close">, () => void>> = {
      next: () => setActiveIndex((current) => (current + 1) % filteredItems.length),
      previous: () => setActiveIndex((current) =>
        (current - 1 + filteredItems.length) % filteredItems.length),
      select: () => activeItem && onSelect(activeItem),
    };
    event.preventDefault();
    event.stopPropagation();
    commands[action]();
  }, [activeItem, anchorRef, filteredItems.length, onSelect, overlayRef, setActiveIndex]);

  useEffect(() => {
    if (!open || !overlayPosition) {
      return;
    }
    document.addEventListener("keydown", handleKeyDown, true);
    return () => document.removeEventListener("keydown", handleKeyDown, true);
  }, [handleKeyDown, open, overlayPosition]);

  useEffect(() => {
    const activeElement = listRef.current?.children[visibleActiveIndex] as HTMLElement | undefined;
    activeElement?.scrollIntoView({ block: "nearest" });
  }, [open, visibleActiveIndex]);

  if (!open || !portalContainer) {
    return null;
  }
  return createPortal(
    <SelectMenuPanel
      ariaLabel={t("common.mention_suggestions")}
      id={overlayId}
      layoutClassName="soft-scrollbar overflow-y-auto p-1"
      panelRef={overlayRef}
      placement={overlayPosition?.placement}
      style={overlayStyle}
      surface="surface"
    >
      <div className={MENU_LIST_CLASS_NAME} ref={listRef} role="none">
        {filteredItems.map((item, index) => (
          <SelectMenuOptionRow
            active={index === visibleActiveIndex}
            className={cn(
              "flex items-center",
              getMenuItemLayout({ hasDescription: Boolean(item.subtitle) }).className,
              getMenuItemStateClassName({
                active: index === visibleActiveIndex,
              }),
            )}
            key={item.id}
            id={`${overlayId}-option-${index}`}
            onMouseDown={(event) => {
              event.preventDefault();
            }}
            onClick={() => onSelect(item)}
            onMouseEnter={() => setActiveIndex(index)}
            tabIndex={-1}
            title={item.subtitle ? `${item.label} — ${item.subtitle}` : item.label}
          >
            <span
              aria-hidden="true"
              className={cn("flex h-6 w-6 shrink-0 items-center justify-center radius-control-sm bg-(--surface-avatar-background) text-(--surface-avatar-foreground)", getUiTypographyClassName({ role: "metadata" }))}
            >
              {item.marker}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate">{item.label}</span>
              {item.subtitle ? (
                <span className={cn("block truncate", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
                  {item.subtitle}
                </span>
              ) : null}
            </span>
          </SelectMenuOptionRow>
        ))}
      </div>
    </SelectMenuPanel>,
    portalContainer,
  );
});
