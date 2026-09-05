// INPUT: Select Menu 选项、当前值、键盘方向和锚点几何。
// OUTPUT: 当前选项数据、下一个可用值、面板高度与定位结果。
// POS: Select Menu 纯状态/几何模型；不返回视觉类或访问 React 生命周期。

import {
  MENU_ITEM_GAP_PX,
  MENU_SURFACE_VERTICAL_PADDING_PX,
} from "@/shared/ui/menu/menu-styles";

import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayPlacement,
  type UiAnchoredOverlayPosition,
} from "../overlay/anchored-overlay-model";

export type UiSelectMenuPlacement = UiAnchoredOverlayPlacement;
export type UiSelectMenuSize = "xs" | "sm" | "md" | "lg";
export type UiSelectMenuSurface = "surface" | "dialog";
export type UiSelectMenuSelectionDirection = -1 | 1;

export interface UiSelectMenuOption {
  value: string;
  label: string;
  badge?: string | null;
  disabled?: boolean;
}

export interface SelectMenuModel {
  activeLabel: string;
  activeBadge: string | null;
}

const SELECT_MENU_MAX_HEIGHT = 280;

export const SELECT_MENU_SEARCH_ROW_HEIGHT = 44;

const UNKNOWN_SELECTION_INDEX_BY_DIRECTION: Record<UiSelectMenuSelectionDirection, number> = {
  [-1]: 0,
  [1]: -1,
};

export function buildSelectMenuModel({
  options,
  placeholder,
  value,
}: {
  options: UiSelectMenuOption[];
  placeholder: string;
  value: string;
}): SelectMenuModel {
  const activeOption = options.find((option) => option.value === value);
  return {
    activeBadge: activeOption?.badge ?? null,
    activeLabel: activeOption?.label ?? placeholder,
  };
}

/** 未命中当前值时，以方向对应的边界作为游标，确保首次移动不会跳过选项。 */
export function resolveNextSelectMenuValue({
  direction,
  options,
  value,
}: {
  direction: UiSelectMenuSelectionDirection;
  options: UiSelectMenuOption[];
  value: string;
}): string | null {
  const enabledOptions = options.filter((option) => !option.disabled);
  if (enabledOptions.length === 0) {
    return null;
  }

  const selectedIndex = enabledOptions.findIndex((option) => option.value === value);
  const currentIndex = selectedIndex >= 0
    ? selectedIndex
    : UNKNOWN_SELECTION_INDEX_BY_DIRECTION[direction];
  const nextIndex = (currentIndex + direction + enabledOptions.length) % enabledOptions.length;
  return enabledOptions[nextIndex].value;
}

export function estimateSelectMenuHeight(
  optionCount: number,
  optionHeight: number,
  extraHeight = MENU_SURFACE_VERTICAL_PADDING_PX,
): number {
  return Math.min(
    SELECT_MENU_MAX_HEIGHT,
    Math.max(
      optionHeight + 8,
      optionCount * optionHeight
        + Math.max(0, optionCount - 1) * MENU_ITEM_GAP_PX
        + extraHeight,
    ),
  );
}

export function resolveSelectMenuPosition({
  button,
  estimatedHeight,
  estimatedOptionHeight,
  menuMinWidth,
  placement,
}: {
  button: HTMLButtonElement;
  estimatedHeight: number;
  estimatedOptionHeight: number;
  placement: UiSelectMenuPlacement;
  menuMinWidth?: number;
}): UiAnchoredOverlayPosition {
  return resolveAnchoredOverlayPosition({
    anchor: button,
    estimatedHeight,
    maxHeight: SELECT_MENU_MAX_HEIGHT,
    minHeight: estimatedOptionHeight + 8,
    minWidth: menuMinWidth,
    placement,
  });
}
