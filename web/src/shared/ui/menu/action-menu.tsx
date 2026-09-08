// INPUT: 外部控制的打开态、锚点、菜单项与选择/关闭命令。
// OUTPUT: 完整动作/说明与唯一激活入口；按内容测高、可见后聚焦、重定位保留焦点及统一退出。
// POS: Action Menu 交互 pattern；不持有业务值或决定命令是否允许。
"use client";

import {
  type ReactNode,
  type Ref,
  type RefObject,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
} from "react";
import { createPortal } from "react-dom";
import { Check } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { focusFirstMenuItem, handleMenuKeyDown } from "./menu-keyboard";

import {
  getMenuItemLayout,
  getMenuContentHeight,
  MENU_LIST_CLASS_NAME,
  MENU_SEPARATOR_CLASS_NAME,
} from "./menu-styles";
import {
  UiMenuActionRow,
  type UiMenuActionRowDensity,
} from "./menu-action-row";
import { useAnchoredOverlayLayer } from "../overlay/anchored-overlay-layer";
import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayAlignment,
  type UiAnchoredOverlayPlacement,
} from "../overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "../overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "../overlay/overlay-styles";

export interface UiActionMenuItem {
  value: string;
  label: ReactNode;
  description?: ReactNode;
  icon?: ReactNode;
  trailing?: ReactNode;
  active?: boolean;
  /** Controlled checked state; labels, icons and trailing content must be non-interactive. */
  checked?: boolean;
  disabled?: boolean;
  tone?: "default" | "primary" | "danger";
}

export type UiActionMenuDensity = UiMenuActionRowDensity;

export interface UiActionMenuContentProps {
  ref?: Ref<HTMLDivElement>;
  density?: UiActionMenuDensity;
  disabled?: boolean;
  footerItems?: UiActionMenuItem[];
  items: UiActionMenuItem[];
  onSelect: (value: string) => void;
}

type UiActionMenuPlacement = UiAnchoredOverlayPlacement;

interface UiActionMenuProps {
  align?: UiAnchoredOverlayAlignment;
  anchorRef: RefObject<HTMLElement | null>;
  ariaLabel: string;
  density?: UiActionMenuDensity;
  footerItems?: UiActionMenuItem[];
  isOpen: boolean;
  items: UiActionMenuItem[];
  minWidth?: number;
  placement?: UiActionMenuPlacement;
  onClose: () => void;
  onSelect: (value: string) => void;
}

const ACTION_MENU_MAX_HEIGHT = 320;
const EMPTY_ACTION_MENU_ITEMS: UiActionMenuItem[] = [];

function estimateActionMenuHeight({
  density = "default",
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  items,
}: {
  density?: UiActionMenuDensity;
  footerItems?: UiActionMenuItem[];
  items: UiActionMenuItem[];
}): number {
  return getMenuContentHeight(
    [...items, ...footerItems].map((item) => getMenuItemLayout({ density, hasDescription: Boolean(item.description) }).height),
    footerItems.length > 0 && items.length > 0 ? 1 : 0,
  );
}

function resolveActionMenuPosition({
  align,
  anchor,
  density,
  items,
  footerItems,
  minWidth,
  measuredContentHeight,
  placement,
}: {
  align: UiAnchoredOverlayAlignment;
  anchor: HTMLElement;
  density: UiActionMenuDensity;
  items: UiActionMenuItem[];
  footerItems: UiActionMenuItem[];
  minWidth: number;
  measuredContentHeight: number | null;
  placement: UiActionMenuPlacement;
}) {
  const contentHeight = measuredContentHeight ?? estimateActionMenuHeight({
    density,
    footerItems,
    items,
  });
  const estimatedHeight = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    Math.max(getMenuItemLayout({ density }).height, contentHeight),
  );
  return resolveAnchoredOverlayPosition({
    align,
    anchor,
    estimatedHeight,
    maxHeight: ACTION_MENU_MAX_HEIGHT,
    minHeight: getMenuItemLayout({ density }).height,
    minWidth,
    placement,
  });
}

export function UiActionMenu({
  align = "start",
  anchorRef,
  ariaLabel,
  density = "default",
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  isOpen,
  items,
  minWidth = 220,
  placement = "auto",
  onClose,
  onSelect,
}: UiActionMenuProps) {
  const contentRef = useRef<HTMLDivElement>(null);
  const estimatePosition = useCallback(
    (anchor: HTMLElement) => resolveActionMenuPosition({
      align,
      anchor,
      density,
      footerItems,
      items,
      minWidth,
      measuredContentHeight: measureActionMenuHeight(contentRef.current),
      placement,
    }),
    [align, density, footerItems, items, minWidth, placement],
  );
  const {
    overlayPosition: menuPosition,
    overlayRef: menuRef,
    overlayStyle: menuStyle,
    portalContainer,
    updateOverlayPosition,
  } = useAnchoredOverlayLayer({
    anchorRef,
    disabled: false,
    estimatePosition,
    isOpen,
    onClose,
  });
  // 只观察未限高的内容；不能用已被 maxHeight 裁过的菜单壳测量，否则无法重新长高。
  useLayoutEffect(() => {
    const content = contentRef.current;
    if (!isOpen || !portalContainer || !content) return;
    updateOverlayPosition();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(updateOverlayPosition);
    observer.observe(content);
    return () => observer.disconnect();
  }, [isOpen, menuPosition?.width, portalContainer, updateOverlayPosition]);
  const isMenuPositioned = menuPosition !== null;

  useEffect(() => {
    if (!isOpen || !portalContainer || !isMenuPositioned) {
      return;
    }
    focusFirstMenuItem(menuRef.current);
  }, [isMenuPositioned, isOpen, menuRef, portalContainer]);

  if (!isOpen) {
    return null;
  }
  if (!portalContainer) {
    return null;
  }
  const closeAndRestoreFocus = () => {
    onClose();
    anchorRef.current?.focus();
  };
  const select = (value: string) => {
    onSelect(value);
    closeAndRestoreFocus();
  };

  return createPortal(
    <div
      ref={menuRef}
      aria-label={ariaLabel}
      className={cn(
        "fixed ui-layer-action-menu overflow-y-auto p-1",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
      )}
      data-placement={menuPosition?.placement ?? "bottom"}
      data-state="open"
      onKeyDown={(event) => handleMenuKeyDown(event, closeAndRestoreFocus)}
      role="menu"
      style={menuStyle}
      tabIndex={-1}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      <UiActionMenuContent
        ref={contentRef}
        density={density}
        footerItems={footerItems}
        items={items}
        onSelect={select}
      />
    </div>,
    portalContainer,
  );
}

export function UiActionMenuContent({
  ref,
  density = "default",
  disabled = false,
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  items,
  onSelect,
}: UiActionMenuContentProps) {
  return (
    <div ref={ref} className={MENU_LIST_CLASS_NAME} role="none">
      {items.map((item) => (
        <ActionMenuItem
          density={density}
          disabled={disabled}
          item={item}
          key={item.value}
          onSelect={onSelect}
        />
      ))}
      {footerItems.length > 0 ? (
        <>
          {items.length > 0 ? <div className={MENU_SEPARATOR_CLASS_NAME} role="separator" /> : null}
          {footerItems.map((item) => (
            <ActionMenuItem
              density={density}
              disabled={disabled}
              item={item}
              key={item.value}
              onSelect={onSelect}
            />
          ))}
        </>
      ) : null}
    </div>
  );
}

function ActionMenuItem({
  density,
  disabled,
  item,
  onSelect,
}: {
  density: UiActionMenuDensity;
  disabled: boolean;
  item: UiActionMenuItem;
  onSelect: (value: string) => void;
}) {
  const select = () => {
    if (disabled || item.disabled) {
      return;
    }
    onSelect(item.value);
  };
  return (
    <UiMenuActionRow
      active={item.active}
      checked={item.checked}
      contentSized
      density={density}
      disabled={disabled || item.disabled}
      hasDescription={Boolean(item.description)}
      onClick={select}
      tone={item.tone}
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        {item.icon ? (
          <span aria-hidden="true" className="flex h-4 w-4 shrink-0 items-center justify-center">
            {item.icon}
          </span>
        ) : null}
        <span className="min-w-0 flex-1">
          <span className="block whitespace-normal wrap-anywhere">
            {item.label}
          </span>
          {item.description ? (
            <span className={cn("block whitespace-normal wrap-anywhere", getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "regular" }))}>
              {item.description}
            </span>
          ) : null}
        </span>
      </span>
      {item.trailing ? (
        <span className="flex shrink-0 items-center">
          {item.trailing}
        </span>
      ) : null}
      {item.checked !== undefined ? (
        <span aria-hidden="true" className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          {item.checked ? <Check className="h-3.5 w-3.5" /> : null}
        </span>
      ) : null}
    </UiMenuActionRow>
  );
}

/** 内容层不带 padding/border；外框尺寸读取实际 recipe，避免复制另一组边框数字。 */
function measureActionMenuHeight(content: HTMLDivElement | null): number | null {
  if (!content || content.scrollHeight <= 0 || !content.parentElement) return null;
  const style = window.getComputedStyle(content.parentElement);
  return content.scrollHeight + [style.paddingTop, style.paddingBottom, style.borderTopWidth, style.borderBottomWidth]
    .reduce((height, value) => height + (Number.parseFloat(value) || 0), 0);
}
