// INPUT: Select Menu 尺寸、标签换行、表面和选中状态。
// OUTPUT: 触发器、选项与标签布局使用的共享视觉 recipe。
// POS: Select Menu 唯一视觉投影；不决定当前值、键盘遍历或浮层位置。

import { cn } from "@/shared/ui/class-name";
import { getMenuItemStateClassName } from "@/shared/ui/menu/menu-styles";

import type {
  UiSelectMenuSize,
  UiSelectMenuSurface,
} from "./select-menu-model";

export interface SelectMenuStyleProjection {
  estimatedOptionHeight: number;
  heightClassName: string;
  optionButtonLayoutClassName: string;
  optionHeightClassName: string;
  optionLabelClassName: string;
  roundedClassName: string;
  textClassName: string;
  triggerLabelClassName: string;
}

const SELECT_MENU_SIZE_CONFIG: Record<UiSelectMenuSize, {
  estimatedOptionHeight: number;
  heightClassName: string;
  optionHeightClassName: string;
  roundedClassName: string;
  textClassName: string;
}> = {
  md: {
    estimatedOptionHeight: 32,
    heightClassName: "h-10",
    optionHeightClassName: "min-h-8 text-sm",
    roundedClassName: "radius-control-md",
    textClassName: "text-sm",
  },
  sm: {
    estimatedOptionHeight: 32,
    heightClassName: "h-9",
    optionHeightClassName: "min-h-8 text-sm",
    roundedClassName: "radius-control-sm",
    textClassName: "text-compact",
  },
  xs: {
    estimatedOptionHeight: 28,
    heightClassName: "h-7",
    optionHeightClassName: "min-h-7 text-compact",
    roundedClassName: "radius-control-xs",
    textClassName: "text-xs",
  },
};

const SELECT_MENU_LABEL_LAYOUT_CONFIG = {
  singleLine: {
    minimumOptionHeight: 0,
    optionButtonLayoutClassName: "items-center",
    optionLabelClassName: "truncate",
    triggerLabelClassName: "truncate leading-normal",
  },
  wrap: {
    minimumOptionHeight: 46,
    optionButtonLayoutClassName: "items-start py-2",
    optionLabelClassName: "whitespace-normal break-words leading-snug",
    triggerLabelClassName: "whitespace-normal break-words text-left leading-snug",
  },
} as const;

const SELECT_MENU_BUTTON_SURFACE_CLASS_NAMES: Record<UiSelectMenuSurface, string> = {
  dialog: "dialog-input shadow-none hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--modal-input-border))] hover:bg-[color:color-mix(in_srgb,var(--modal-input-focus-background)_72%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]",
  surface: "border border-(--surface-control-border) bg-(--surface-control-field-background) shadow-(--surface-control-field-shadow) hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
};

export function getSelectMenuSizeConfig(size: UiSelectMenuSize) {
  return SELECT_MENU_SIZE_CONFIG[size];
}

export function getSelectMenuStyleProjection({
  allowLabelWrap,
  size,
}: {
  allowLabelWrap: boolean;
  size: UiSelectMenuSize;
}): SelectMenuStyleProjection {
  const sizeConfig = getSelectMenuSizeConfig(size);
  const labelLayout = SELECT_MENU_LABEL_LAYOUT_CONFIG[
    allowLabelWrap ? "wrap" : "singleLine"
  ];
  return {
    ...sizeConfig,
    estimatedOptionHeight: Math.max(
      sizeConfig.estimatedOptionHeight,
      labelLayout.minimumOptionHeight,
    ),
    optionButtonLayoutClassName: labelLayout.optionButtonLayoutClassName,
    optionLabelClassName: labelLayout.optionLabelClassName,
    triggerLabelClassName: labelLayout.triggerLabelClassName,
  };
}

export function getSelectMenuButtonClassName({
  roundedClassName,
  surface,
  textClassName,
  className,
}: {
  roundedClassName: string;
  surface: UiSelectMenuSurface;
  textClassName: string;
  className?: string;
}) {
  return cn(
    "flex h-full w-full items-center justify-between gap-2 px-3 transition-[background,border-color,box-shadow] duration-(--motion-duration-fast) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
    SELECT_MENU_BUTTON_SURFACE_CLASS_NAMES[surface],
    roundedClassName,
    textClassName,
    className,
  );
}

export function getSelectMenuOptionStateClassName(isActive: boolean): string {
  return getMenuItemStateClassName({ active: isActive });
}
