// INPUT: 单个可聚焦触发器、短标签、可选快捷键与锚定方向。
// OUTPUT: 具延迟 hover、即时 focus、ARIA 关联、Portal 定位和焦点归还的共享提示。
// POS: Tooltip primitive；不承担业务点击动作或长内容 Popover。
"use client";

import {
  cloneElement,
  type ReactElement,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/ui/class-name";

import { useAnchoredOverlayLayer } from "./anchored-overlay-layer";
import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayPlacement,
} from "./anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "./overlay-contract";
import { ANCHORED_OVERLAY_MOTION_CLASS_NAME } from "./overlay-styles";

interface TooltipTriggerProps {
  "aria-describedby"?: string;
}

interface UiTooltipProps {
  children: ReactElement<TooltipTriggerProps>;
  label: string;
  placement?: UiAnchoredOverlayPlacement;
  shortcut?: string;
}

const TOOLTIP_OPEN_DELAY_MS = 260;

export function UiTooltip({
  children,
  label,
  placement = "auto",
  shortcut,
}: UiTooltipProps) {
  const anchorRef = useRef<HTMLSpanElement>(null);
  const measuredSizeRef = useRef({ height: 40, width: 0 });
  const openTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const tooltipLabel = label.trim();
  const clearOpenTimer = useCallback(() => {
    if (openTimerRef.current) {
      clearTimeout(openTimerRef.current);
      openTimerRef.current = null;
    }
  }, []);
  const close = useCallback(() => {
    clearOpenTimer();
    setIsOpen(false);
  }, [clearOpenTimer]);
  const openNow = useCallback(() => {
    clearOpenTimer();
    setIsOpen(true);
  }, [clearOpenTimer]);
  const scheduleOpen = useCallback(() => {
    clearOpenTimer();
    openTimerRef.current = setTimeout(openNow, TOOLTIP_OPEN_DELAY_MS);
  }, [clearOpenTimer, openNow]);
  const restoreTriggerFocus = useCallback(() => {
    const container = anchorRef.current;
    const trigger = container?.firstElementChild;
    if (trigger instanceof HTMLElement) {
      trigger.focus();
      return;
    }
    container?.focus();
  }, []);
  const estimatePosition = useCallback(
    (container: HTMLSpanElement) => {
      const anchor = container.firstElementChild instanceof HTMLElement
        ? container.firstElementChild
        : container;
      return resolveAnchoredOverlayPosition({
        align: "center",
        anchor,
        estimatedHeight: measuredSizeRef.current.height,
        gap: 8,
        maxHeight: measuredSizeRef.current.height,
        minHeight: Math.min(40, measuredSizeRef.current.height),
        minWidth: measuredSizeRef.current.width,
        placement,
      });
    },
    [placement],
  );
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
    updateOverlayPosition,
  } = useAnchoredOverlayLayer({
    anchorRef,
    disabled: !tooltipLabel,
    estimatePosition,
    isOpen,
    onClose: close,
    restoreFocus: restoreTriggerFocus,
  });

  useEffect(() => clearOpenTimer, [clearOpenTimer]);
  useLayoutEffect(() => {
    if (!isOpen || !overlayRef.current) {
      return;
    }
    measuredSizeRef.current = {
      height: overlayRef.current.offsetHeight,
      width: overlayRef.current.offsetWidth,
    };
    updateOverlayPosition();
  }, [isOpen, overlayRef, shortcut, tooltipLabel, updateOverlayPosition]);

  const describedBy = [
    children.props["aria-describedby"],
    isOpen ? overlayId : null,
  ].filter(Boolean).join(" ") || undefined;

  return (
    <>
      <span
        ref={anchorRef}
        className="contents"
        data-ui-tooltip-trigger="true"
        onBlurCapture={close}
        onFocusCapture={openNow}
        onMouseEnter={scheduleOpen}
        onMouseLeave={close}
        onPointerDownCapture={close}
      >
        {cloneElement(children, { "aria-describedby": describedBy })}
      </span>
      {isOpen && tooltipLabel && portalContainer
        ? createPortal(
            <div
              ref={overlayRef}
              className={cn(
                "ui-tooltip pointer-events-none fixed left-0 top-0 ui-layer-tooltip flex w-max max-w-[calc(100vw-24px)] items-center gap-2",
                ANCHORED_OVERLAY_MOTION_CLASS_NAME,
              )}
              data-placement={overlayPosition?.placement ?? placement}
              id={overlayId}
              role="tooltip"
              style={{
                ...overlayStyle,
                maxHeight: "min(160px, calc(100vh - 24px))",
                width: "max-content",
              }}
              {...OPEN_OVERLAY_DATA_ATTRIBUTES}
            >
              <span className="min-w-0 break-words">{tooltipLabel}</span>
              {shortcut ? <kbd className="shrink-0">{shortcut}</kbd> : null}
            </div>,
            portalContainer,
          )
        : null}
    </>
  );
}
