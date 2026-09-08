// INPUT: Select Menu 尺寸、标签换行、表面和选中状态。
// OUTPUT: 复用 App Typography 的字段字号、固定/随换行增高的触发器与选项布局 recipe。
// POS: Select Menu 唯一视觉投影；不决定当前值、键盘遍历或浮层位置。

import { cn } from "@/shared/ui/class-name";
import { getMenuItemStateClassName } from "@/shared/ui/menu/menu-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
  triggerLayoutClassName?: string;
  triggerLabelClassName: string;
}

const WRAPPING_TRIGGER_CLASS_NAMES: Record<UiSelectMenuSize, string> = {
  xs: "h-auto min-h-7 py-1",
  sm: "h-auto min-h-8 py-1",
  md: "h-auto min-h-9 py-1.5",
  lg: "h-auto min-h-11 py-2.5",
};

const SELECT_MENU_SIZE_CONFIG: Record<UiSelectMenuSize, {
  estimatedOptionHeight: number;
  heightClassName: string;
  optionHeightClassName: string;
  roundedClassName: string;
  textClassName: string;
}> = {
  md: {
    estimatedOptionHeight: 32,
    heightClassName: "h-9",
    optionHeightClassName: cn("min-h-8", getUiTypographyClassName({ role: "supporting" })),
    roundedClassName: "radius-control-md",
    textClassName: getUiTypographyClassName({ role: "control", weight: "regular" }),
  },
  sm: {
    estimatedOptionHeight: 32,
    heightClassName: "h-8",
    optionHeightClassName: cn("min-h-8", getUiTypographyClassName({ role: "supporting" })),
    roundedClassName: "radius-control-sm",
    textClassName: getUiTypographyClassName({ role: "supporting" }),
  },
  xs: {
    estimatedOptionHeight: 28,
    heightClassName: "h-7",
    optionHeightClassName: cn("min-h-7", getUiTypographyClassName({ role: "metadata" })),
    roundedClassName: "radius-control-xs",
    textClassName: getUiTypographyClassName({ role: "metadata" }),
  },
  lg: {
    estimatedOptionHeight: 32,
    heightClassName: "h-11",
    optionHeightClassName: cn("min-h-8", getUiTypographyClassName({ role: "supporting" })),
    roundedClassName: "radius-control-lg",
    textClassName: getUiTypographyClassName({ role: "control", weight: "regular" }),
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
    heightClassName: allowLabelWrap ? "h-auto" : sizeConfig.heightClassName,
    triggerLayoutClassName: allowLabelWrap ? WRAPPING_TRIGGER_CLASS_NAMES[size] : undefined,
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
