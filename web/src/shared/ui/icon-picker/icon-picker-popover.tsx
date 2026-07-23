"use client";

import {
  useCallback,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

import type { AvatarIconFamily } from "@/lib/avatar";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";

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
  triggerClassName: string;
  value?: string;
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
  triggerClassName,
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
        className={triggerClassName}
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
          className="fixed z-[140] overflow-y-auto rounded-[18px] border border-(--surface-popover-border) bg-(--surface-popover-background) p-3 shadow-[0_18px_42px_color-mix(in_srgb,var(--shadow-color)_18%,transparent)] backdrop-blur-xl animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1"
          data-placement={overlayPosition?.placement ?? "bottom"}
          role="dialog"
          style={overlayStyle}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <div className="mb-3 flex items-center justify-between gap-3 px-0.5">
            <span className="text-[13px] font-semibold text-(--text-strong)">
              {ariaLabel}
            </span>
            <span className="text-[11px] text-(--text-soft)">
              {maxIcons}
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
