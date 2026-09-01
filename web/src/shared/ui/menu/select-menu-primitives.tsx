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
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "../overlay/overlay-styles";
import type { UiSelectMenuSurface } from "./select-menu-model";

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
        "fixed z-[120]",
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
