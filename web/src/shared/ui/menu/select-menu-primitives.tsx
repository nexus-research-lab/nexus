// INPUT: Select trigger 的开关/禁用事实、既有样式投影、内容与原生事件，以及 listbox/选项数据。
// OUTPUT: 稳定的触发器、选择面板和 option button 语义 DOM。
// POS: Select Menu 视图原语；不管理开关、选值或定位计算。

import type {
  ButtonHTMLAttributes,
  CSSProperties,
  ReactNode,
  RefObject,
} from "react";
import { ChevronDown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type { UiAnchoredOverlayPosition } from "../overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "../overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "../overlay/overlay-styles";
import type { UiSelectMenuSurface } from "./select-menu-model";
import { MENU_ITEM_BASE_CLASS_NAME } from "./menu-styles";
import {
  getSelectMenuButtonClassName,
  type SelectMenuStyleProjection,
} from "./select-menu-styles";

interface SelectMenuTriggerProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "aria-controls" | "aria-disabled" | "aria-expanded" | "aria-haspopup" | "aria-label" | "type"
> {
  ariaLabel: string;
  buttonRef: RefObject<HTMLButtonElement | null>;
  children: ReactNode;
  disabled: boolean;
  isOpen: boolean;
  menuId: string;
  styles: Pick<SelectMenuStyleProjection, "roundedClassName" | "textClassName">;
  surface: UiSelectMenuSurface;
}

interface SelectMenuOptionRowProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "aria-selected" | "className" | "role" | "type"
> {
  active: boolean;
  className?: string;
}

/** 单选与领域多选共用的触发器 DOM；开关和键盘决策仍由上游控制器负责。 */
export function SelectMenuTrigger({
  ariaLabel,
  buttonRef,
  className,
  disabled,
  isOpen,
  menuId,
  styles,
  surface,
  ...props
}: SelectMenuTriggerProps) {
  return (
    <button
      {...props}
      ref={buttonRef}
      aria-controls={isOpen ? menuId : undefined}
      aria-disabled={disabled}
      aria-expanded={isOpen}
      aria-haspopup="listbox"
      aria-label={ariaLabel}
      className={getSelectMenuButtonClassName({
        roundedClassName: styles.roundedClassName,
        surface,
        textClassName: styles.textClassName,
        className,
      })}
      disabled={disabled}
      type="button"
    />
  );
}

export function SelectMenuTriggerContent({
  children,
  isOpen,
  label,
  leading,
}: {
  children: ReactNode;
  isOpen: boolean;
  label?: ReactNode;
  leading?: ReactNode;
}) {
  return (
    <>
      <span className="flex min-w-0 flex-1 items-center gap-2">
        {leading ? (
          <span className="shrink-0 text-(--icon-default)">{leading}</span>
        ) : null}
        {label ? (
          <>
            <span className="shrink-0 text-compact font-medium text-(--text-muted)">
              {label}
            </span>
            <span className="h-3.5 w-px shrink-0 bg-(--divider-subtle-color)" />
          </>
        ) : null}
        {children}
      </span>
      <ChevronDown
        className={cn(
          "h-4 w-4 shrink-0 text-(--icon-muted) transition-transform",
          isOpen && "rotate-180",
        )}
      />
    </>
  );
}

export function SelectMenuPanel({
  ariaLabel,
  children,
  id,
  layoutClassName,
  panelRef,
  placement,
  style,
  surface,
}: {
  ariaLabel: string;
  children: ReactNode;
  id: string;
  layoutClassName: string;
  panelRef: RefObject<HTMLDivElement | null>;
  placement?: UiAnchoredOverlayPosition["placement"];
  style: CSSProperties;
  surface: UiSelectMenuSurface;
}) {
  return (
    <div
      ref={panelRef}
      aria-label={ariaLabel}
      className={cn(
        "fixed ui-layer-select-menu",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
        layoutClassName,
      )}
      data-placement={placement ?? "bottom"}
      data-state="open"
      data-surface={surface}
      id={id}
      role="listbox"
      style={style}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      {children}
    </div>
  );
}

/** Listbox 选项统一持有原生 button、选中语义和菜单交互底面。 */
export function SelectMenuOptionRow({
  active,
  className,
  ...props
}: SelectMenuOptionRowProps) {
  return (
    <button
      {...props}
      aria-selected={active}
      className={cn(MENU_ITEM_BASE_CLASS_NAME, className)}
      data-active={active ? "true" : undefined}
      role="option"
      type="button"
    />
  );
}
