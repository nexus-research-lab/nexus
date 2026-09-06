// INPUT: Menu action 的原生按钮属性、密度、活动态与语义 tone。
// OUTPUT: 统一的 button/role=menuitem DOM 与原生 ref、禁用语义、命中几何和视觉状态。
// POS: Shared Menu action row primitive；不管理菜单定位、开关、命令或业务内容。

import type { ComponentPropsWithRef } from "react";

import { cn } from "@/shared/ui/class-name";

import {
  getMenuItemStateClassName,
  getMenuItemLayout,
  MENU_ITEM_BASE_CLASS_NAME,
  type UiMenuItemDensity,
  type UiMenuItemTone,
} from "./menu-styles";

export type UiMenuActionRowDensity = UiMenuItemDensity;

interface UiMenuActionRowProps extends Omit<
  ComponentPropsWithRef<"button">,
  "aria-disabled" | "className" | "role" | "type"
> {
  active?: boolean;
  className?: string;
  density?: UiMenuActionRowDensity;
  hasDescription?: boolean;
  tone?: UiMenuItemTone;
}

/** Action menus and contextual menus share this native menu-item button. */
export function UiMenuActionRow({
  active = false,
  children,
  className,
  density = "default",
  disabled = false,
  hasDescription = false,
  tone = "default",
  ...props
}: UiMenuActionRowProps) {
  return (
    <button
      {...props}
      aria-disabled={disabled || undefined}
      className={cn(
        MENU_ITEM_BASE_CLASS_NAME,
        "flex shrink-0 cursor-pointer items-center text-left",
        getMenuItemLayout({ density, hasDescription }).className,
        disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
        getMenuItemStateClassName({ active, tone }),
        className,
      )}
      data-active={active ? "true" : undefined}
      disabled={disabled}
      role="menuitem"
      type="button"
    >
      {children}
    </button>
  );
}
