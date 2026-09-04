// INPUT: Menu action 的原生按钮属性、密度、活动态与语义 tone。
// OUTPUT: 统一的 button/role=menuitem DOM、禁用语义、命中几何与视觉状态。
// POS: Shared Menu action row primitive；不管理菜单定位、开关、命令或业务内容。

import type { ButtonHTMLAttributes } from "react";

import { cn } from "@/shared/ui/class-name";

import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
  type UiMenuItemTone,
} from "./menu-styles";

export type UiMenuActionRowDensity = "compact" | "default";

interface UiMenuActionRowProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "aria-disabled" | "className" | "role" | "type"
> {
  active?: boolean;
  className?: string;
  density?: UiMenuActionRowDensity;
  hasDescription?: boolean;
  tone?: UiMenuItemTone;
}

const MENU_ACTION_ROW_CLASS_NAME = {
  compact: {
    described: "h-10 gap-2 px-2 py-0.5 text-compact",
    plain: "h-8 gap-2 px-2 text-compact",
  },
  default: {
    described: "h-11 gap-3 px-2.5 py-1 text-sm",
    plain: "h-9 gap-3 px-2.5 text-sm",
  },
} as const;

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
        "flex cursor-pointer items-center text-left",
        MENU_ACTION_ROW_CLASS_NAME[density][hasDescription ? "described" : "plain"],
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
