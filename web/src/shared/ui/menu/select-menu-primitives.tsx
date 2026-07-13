import type {
  CSSProperties,
  ReactNode,
  RefObject,
} from "react";
import { ChevronDown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type { UiAnchoredOverlayPosition } from "../overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "../overlay/overlay-contract";
import {
  getSelectMenuPanelSurfaceClassName,
  type UiSelectMenuSurface,
} from "./select-menu-model";

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
            <span className="shrink-0 text-[12px] font-medium text-(--text-muted)">
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
  menuClassName,
  panelRef,
  placement,
  style,
  surface,
}: {
  ariaLabel: string;
  children: ReactNode;
  id: string;
  layoutClassName: string;
  menuClassName?: string;
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
        "fixed z-[120] rounded-[14px] border animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
        layoutClassName,
        getSelectMenuPanelSurfaceClassName(surface),
        menuClassName,
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
