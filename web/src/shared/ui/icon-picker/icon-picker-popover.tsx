/**
 * INPUT: 图标族、当前值、锚点触发器与选择命令。
 * OUTPUT: 带可访问名称的紧凑锚定图标网格。
 * POS: 共享图标选择浮层；不显示无语义的图标数量标题。
 */
"use client";

import { ChevronDown } from "lucide-react";
import {
  useCallback,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

import type { AvatarIconFamily } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { IconPicker } from "./icon-picker";
import type {
  IconPickerColumns,
  IconPickerSize,
} from "./icon-picker-model";

interface IconPickerPopoverProps {
  ariaLabel: string;
  columns?: IconPickerColumns;
  disabled?: boolean;
  iconFamily: AvatarIconFamily;
  iconSize?: IconPickerSize;
  maxIcons: number;
  onSelect: (iconId: string) => void;
  renderTrigger: (isOpen: boolean) => ReactNode;
  startIconId: number;
  triggerAlign?: "center" | "start";
  triggerGap?: "compact" | "default";
  triggerRadius?: "control" | "surface";
  value?: string;
}

interface IconPickerTriggerLabelProps {
  children: ReactNode;
  isOpen: boolean;
  showChevron?: boolean;
}

const ICON_PICKER_POPOVER_HEIGHT = 356;
const ICON_PICKER_POPOVER_MIN_HEIGHT = 220;
const ICON_PICKER_POPOVER_WIDTH = 328;
const ICON_PICKER_POPOVER_VIEWPORT_GUTTER = 12;

export function IconPickerPopover({
  ariaLabel,
  columns = 5,
  disabled = false,
  iconFamily,
  iconSize = "lg",
  maxIcons,
  onSelect,
  renderTrigger,
  startIconId,
  triggerAlign = "center",
  triggerGap = "default",
  triggerRadius = "control",
  value,
}: IconPickerPopoverProps) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const closePicker = useCallback(() => setIsOpen(false), []);
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveAnchoredOverlayPosition({
      anchor,
      estimatedHeight: ICON_PICKER_POPOVER_HEIGHT,
      gap: 8,
      maxHeight: ICON_PICKER_POPOVER_HEIGHT,
      minHeight: ICON_PICKER_POPOVER_MIN_HEIGHT,
      minWidth: Math.min(
        ICON_PICKER_POPOVER_WIDTH,
        window.innerWidth - ICON_PICKER_POPOVER_VIEWPORT_GUTTER * 2,
      ),
      placement: "auto",
    })
  ), []);
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef: triggerRef,
    disabled,
    estimatePosition,
    isOpen,
    onClose: closePicker,
  });

  const selectIcon = useCallback((iconId: string) => {
    onSelect(iconId);
    closePicker();
    triggerRef.current?.focus();
  }, [closePicker, onSelect]);

  return (
    <>
      <button
        ref={triggerRef}
        aria-controls={isOpen ? overlayId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={ariaLabel}
        className={cn(
          "group group/icon-picker-trigger relative flex shrink-0 flex-col text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-(--background) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
          triggerAlign === "center" ? "items-center" : "items-start",
          triggerGap === "compact" ? "gap-1" : "gap-1.5",
          triggerRadius === "surface" ? "surface-radius-lg" : "radius-control-lg",
        )}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        type="button"
      >
        {renderTrigger(isOpen)}
      </button>

      {isOpen && portalContainer ? createPortal(
        <div
          ref={overlayRef}
          aria-label={ariaLabel}
          className={cn(
            "fixed ui-layer-popover overflow-y-auto p-3",
            OVERLAY_SURFACE_CLASS_NAME,
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
          )}
          data-placement={overlayPosition?.placement ?? "bottom"}
          role="dialog"
          style={overlayStyle}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <div className="mb-3 px-0.5">
            <span className={getUiTypographyClassName({
              role: "sectionTitle",
              tone: "strong",
            })}>
              {ariaLabel}
            </span>
          </div>
          <IconPicker
            columns={columns}
            disabled={disabled}
            iconFamily={iconFamily}
            iconSize={iconSize}
            layout="grid"
            maxIcons={maxIcons}
            onSelect={selectIcon}
            showClear={false}
            startIconId={startIconId}
            value={value}
          />
        </div>,
        portalContainer,
      ) : null}
    </>
  );
}

export function IconPickerTriggerLabel({
  children,
  isOpen,
  showChevron = true,
}: IconPickerTriggerLabelProps) {
  return (
    <span
      className={cn(
        "inline-flex min-h-7 items-center gap-1 radius-control-sm px-2 transition-[background,color] group-hover/icon-picker-trigger:bg-(--surface-interactive-hover-background) group-hover/icon-picker-trigger:text-(--text-strong)",
        getUiTypographyClassName({
          role: "metadata",
          tone: "muted",
          weight: "medium",
        }),
      )}
    >
      {children}
      {showChevron ? (
        <ChevronDown
          className={cn(
            "h-3 w-3 transition-transform duration-(--motion-duration-fast)",
            isOpen && "rotate-180",
          )}
        />
      ) : null}
    </span>
  );
}
