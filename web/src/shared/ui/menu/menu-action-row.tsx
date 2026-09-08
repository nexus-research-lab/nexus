// INPUT: Menu action 的原生按钮属性、密度、活动态与语义 tone。
// OUTPUT: 统一的原生 menuitem/menuitemcheckbox 按钮与 ref、选中/禁用语义、固定或内容自适应命中几何和视觉状态。
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
  "aria-checked" | "aria-disabled" | "className" | "role" | "type"
> {
  active?: boolean;
  checked?: boolean;
  className?: string;
  density?: UiMenuActionRowDensity;
  /** 动作内容允许完整换行；固定几何的上下文/建议行保持默认。 */
  contentSized?: boolean;
  hasDescription?: boolean;
  tone?: UiMenuItemTone;
}

/** Action menus and contextual menus share this native menu-item button. */
export function UiMenuActionRow({
  active = false,
  checked,
  children,
  className,
  density = "default",
  contentSized = false,
  disabled = false,
  hasDescription = false,
  tone = "default",
  ...props
}: UiMenuActionRowProps) {
  return (
    <button
      {...props}
      aria-checked={checked}
      aria-disabled={disabled || undefined}
      className={cn(
        MENU_ITEM_BASE_CLASS_NAME,
        "flex shrink-0 cursor-pointer items-center text-left",
        getMenuItemLayout({ density, hasDescription, contentSized }).className,
        disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
        getMenuItemStateClassName({ active, tone }),
        className,
      )}
      data-active={active ? "true" : undefined}
      disabled={disabled}
      role={checked === undefined ? "menuitem" : "menuitemcheckbox"}
      type="button"
    >
      {children}
    </button>
  );
}
