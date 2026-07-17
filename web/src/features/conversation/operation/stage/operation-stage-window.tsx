/**
 * INPUT: One Stage window's geometry, state, app identity, children, and desktop commands.
 * OUTPUT: A movable, resizable window that preserves mounted app state in background mode.
 * POS: Generic Agent OS window chrome; app content and background preview skin are external.
 */
/* eslint-disable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex -- A stage window is a focusable desktop composite with nested native controls. */
import type { CSSProperties, MouseEvent, PointerEvent, ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { Maximize2, Minimize2, Minus, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { resolveOperationWindowKeyboardAction } from "./operation-stage-window-actions";
import { StageManagerWindowPreview } from "./operation-stage-window-preview";
import { buildStageWindowTitlebarState } from "./operation-stage-window-titlebar";

interface OperationStageWindowProps {
  title: string;
  icon: LucideIcon;
  children: ReactNode;
  positionClassName: string;
  appLabel?: string;
  delayMs?: number;
  focus?: boolean;
  launchOrigin?: "active" | "desktop" | "dock";
  maximized?: boolean;
  minimized?: boolean;
  dimmed?: boolean;
  dragOffset?: { x: number; y: number };
  resizeSize?: { height: number; width: number };
  mobileHidden?: boolean;
  contentMode?: "flush" | "inset";
  previewMode?: "stage-manager";
  restoreToken?: number;
  zIndex?: number;
  tone?: "default" | "terminal";
  onClose?: () => void;
  onDrag?: (offset: { x: number; y: number }) => void;
  onFocus?: () => void;
  onMinimize?: () => void;
  onResize?: (size: { height: number; width: number }) => void;
  onZoom?: () => void;
  onCycleFocus?: (direction: "next" | "previous") => void;
}

export function OperationStageWindow({
  title,
  icon: Icon,
  children,
  positionClassName,
  appLabel,
  delayMs = 0,
  focus = false,
  launchOrigin = "active",
  maximized = false,
  minimized = false,
  dimmed = false,
  dragOffset = { x: 0, y: 0 },
  resizeSize,
  mobileHidden = false,
  contentMode = "inset",
  previewMode,
  restoreToken,
  zIndex,
  tone = "default",
  onClose,
  onDrag,
  onFocus,
  onMinimize,
  onResize,
  onZoom,
  onCycleFocus,
}: OperationStageWindowProps) {
  const window_ref = useRef<HTMLDialogElement | null>(null);
  const drag_state_ref = useRef<{
    pointer_id: number;
    start_x: number;
    start_y: number;
    origin_x: number;
    origin_y: number;
  } | null>(null);
  const resize_state_ref = useRef<{
    edge: "bottom" | "corner" | "right";
    origin_height: number;
    origin_width: number;
    pointer_id: number;
    start_x: number;
    start_y: number;
  } | null>(null);
  const cleanup_mouse_drag_ref = useRef<(() => void) | null>(null);
  const [is_dragging, set_is_dragging] = useState(false);
  const [is_restoring, set_is_restoring] = useState(false);
  const titlebar = buildStageWindowTitlebarState({
    app_label: appLabel,
    focused: focus,
    maximized,
    minimized,
    title,
  });

  const start_drag = (
    event: PointerEvent<HTMLDivElement> | MouseEvent<HTMLDivElement>,
    pointer_id: number,
  ) => {
    if (event.button !== 0 || minimized || drag_state_ref.current) {
      return;
    }
    if (typeof window !== "undefined" && window.matchMedia("(max-width: 767px)").matches) {
      onFocus?.();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    onFocus?.();
    drag_state_ref.current = {
      pointer_id,
      start_x: event.clientX,
      start_y: event.clientY,
      origin_x: dragOffset.x,
      origin_y: dragOffset.y,
    };
    set_is_dragging(true);
  };

  const start_pointer_drag = (event: PointerEvent<HTMLDivElement>) => {
    if (event.pointerType === "mouse") {
      return;
    }
    start_drag(event, event.pointerId);
    if (drag_state_ref.current?.pointer_id === event.pointerId) {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
  };

  const start_mouse_drag = (event: MouseEvent<HTMLDivElement>) => {
    start_drag(event, -1);
    if (drag_state_ref.current?.pointer_id !== -1) {
      return;
    }
    const move_mouse_drag = (mouse_event: globalThis.MouseEvent) => {
      const drag_state = drag_state_ref.current;
      if (!drag_state || drag_state.pointer_id !== -1) {
        return;
      }
      mouse_event.preventDefault();
      onDrag?.({
        x: drag_state.origin_x + mouse_event.clientX - drag_state.start_x,
        y: drag_state.origin_y + mouse_event.clientY - drag_state.start_y,
      });
    };
    const end_mouse_drag = () => {
      const drag_state = drag_state_ref.current;
      if (!drag_state || drag_state.pointer_id !== -1) {
        return;
      }
      cleanup_mouse_drag_ref.current?.();
      cleanup_mouse_drag_ref.current = null;
      drag_state_ref.current = null;
      set_is_dragging(false);
    };
    cleanup_mouse_drag_ref.current?.();
    document.addEventListener("mousemove", move_mouse_drag);
    document.addEventListener("mouseup", end_mouse_drag);
    cleanup_mouse_drag_ref.current = () => {
      document.removeEventListener("mousemove", move_mouse_drag);
      document.removeEventListener("mouseup", end_mouse_drag);
    };
  };

  useEffect(() => {
    return () => {
      cleanup_mouse_drag_ref.current?.();
    };
  }, []);

  useEffect(() => {
    if (!restoreToken) {
      return;
    }
    set_is_restoring(true);
    const timeout = window.setTimeout(() => set_is_restoring(false), 360);
    return () => window.clearTimeout(timeout);
  }, [restoreToken]);

  const move_drag = (event: PointerEvent<HTMLDivElement>) => {
    const drag_state = drag_state_ref.current;
    if (!drag_state || drag_state.pointer_id !== event.pointerId) {
      return;
    }
    event.preventDefault();
    onDrag?.({
      x: drag_state.origin_x + event.clientX - drag_state.start_x,
      y: drag_state.origin_y + event.clientY - drag_state.start_y,
    });
  };

  const end_drag = (event: PointerEvent<HTMLDivElement>) => {
    const drag_state = drag_state_ref.current;
    if (!drag_state || drag_state.pointer_id !== event.pointerId) {
      return;
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    drag_state_ref.current = null;
    set_is_dragging(false);
  };

  const start_resize = (
    event: PointerEvent<HTMLElement>,
    edge: "bottom" | "corner" | "right",
  ) => {
    if (event.button !== 0 || minimized || maximized || resize_state_ref.current) {
      return;
    }
    const rect = window_ref.current?.getBoundingClientRect();
    if (!rect) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    onFocus?.();
    resize_state_ref.current = {
      edge,
      origin_height: resizeSize?.height ?? rect.height,
      origin_width: resizeSize?.width ?? rect.width,
      pointer_id: event.pointerId,
      start_x: event.clientX,
      start_y: event.clientY,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    set_is_dragging(true);
  };

  const move_resize = (event: PointerEvent<HTMLElement>) => {
    const resize_state = resize_state_ref.current;
    if (!resize_state || resize_state.pointer_id !== event.pointerId) {
      return;
    }
    event.preventDefault();
    onResize?.({
      height: resize_state.edge === "right"
        ? resize_state.origin_height
        : resize_state.origin_height + event.clientY - resize_state.start_y,
      width: resize_state.edge === "bottom"
        ? resize_state.origin_width
        : resize_state.origin_width + event.clientX - resize_state.start_x,
    });
  };

  const end_resize = (event: PointerEvent<HTMLElement>) => {
    const resize_state = resize_state_ref.current;
    if (!resize_state || resize_state.pointer_id !== event.pointerId) {
      return;
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    resize_state_ref.current = null;
    set_is_dragging(false);
  };

  const show_resize_handles = !maximized && !minimized && previewMode !== "stage-manager";

  return (
    <dialog
      ref={window_ref}
      aria-label={titlebar.aria_label}
      aria-roledescription="window"
      className={cn(
        "operation-stage-window absolute m-0 flex min-h-0 min-w-0 cursor-default flex-col overflow-hidden rounded-[14px] border p-0 backdrop-blur-xl outline-none transition-[left,top,width,height,opacity,filter,box-shadow,border-radius] duration-300 ease-[cubic-bezier(.2,.82,.2,1)] focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.42)] focus-visible:ring-offset-2 focus-visible:ring-offset-transparent max-md:!relative max-md:!inset-auto max-md:!h-auto max-md:!min-h-[180px] max-md:!w-full max-md:max-w-full",
        tone === "terminal"
          ? "border-white/14 bg-[#0d151e]/95 text-[#d8e8e2] shadow-[0_30px_76px_rgba(0,8,16,0.34)]"
          : "border-white/60 bg-[rgba(250,252,253,0.96)] text-(--text-strong) shadow-[0_28px_72px_rgba(18,28,42,0.24)]",
        focus && "operation-stage-window-focus",
        launchOrigin === "dock" && "operation-stage-window-launch-dock",
        launchOrigin === "desktop" && "operation-stage-window-launch-desktop",
        maximized && "operation-stage-window-maximized rounded-[18px]",
        dimmed && "opacity-[0.62] saturate-[0.82]",
        is_dragging && "operation-stage-window-dragging select-none",
        is_restoring && "operation-stage-window-restoring",
        previewMode === "stage-manager" && "operation-stage-window-stage-manager rounded-[18px]",
        minimized && "min-h-0",
        mobileHidden && "max-md:hidden",
        positionClassName,
      )}
      onKeyDown={(keyboard_event) => {
        if (keyboard_event.currentTarget !== keyboard_event.target) {
          return;
        }
        const action = resolveOperationWindowKeyboardAction(keyboard_event);
        if (!action) {
          return;
        }
        keyboard_event.preventDefault();
        if (action === "focus") {
          onFocus?.();
        } else if (action === "close") {
          onClose?.();
        } else if (action === "minimize") {
          onMinimize?.();
        } else if (action === "cycle_next") {
          onCycleFocus?.("next");
        } else if (action === "cycle_previous") {
          onCycleFocus?.("previous");
        } else {
          onZoom?.();
        }
      }}
      onMouseDown={onFocus}
      open
      style={{
        "--operation-delay": `${delayMs}ms`,
        "--operation-window-drag-x": `${dragOffset.x}px`,
        "--operation-window-drag-y": `${dragOffset.y}px`,
        height: resizeSize && !maximized ? `${resizeSize.height}px` : undefined,
        zIndex: zIndex,
        translate: `${dragOffset.x}px ${dragOffset.y}px`,
        width: resizeSize && !maximized ? `${resizeSize.width}px` : undefined,
      } as CSSProperties}
      tabIndex={0}
    >
      <div
        className={cn(
          "flex h-8 shrink-0 cursor-grab touch-none items-center justify-between gap-2 border-b px-3 active:cursor-grabbing max-md:cursor-default",
          tone === "terminal"
            ? "border-white/10 bg-white/[0.035] text-[rgba(233,241,244,0.56)]"
            : "border-(--divider-subtle-color) bg-white/62 text-(--text-soft)",
          !focus && "opacity-[0.82]",
        )}
        onPointerCancel={end_drag}
        onLostPointerCapture={end_drag}
        onDoubleClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          onZoom?.();
        }}
        onMouseDown={start_mouse_drag}
        onPointerDown={start_pointer_drag}
        onPointerMove={move_drag}
        onPointerUp={end_drag}
        role="toolbar"
      >
        <div className="operation-window-traffic flex items-center gap-1.5">
          <button
            aria-label={titlebar.close_label}
            className={cn(
              "operation-window-traffic-button grid h-4 w-4 place-items-center rounded-full border transition hover:bg-[rgba(223,93,98,0.86)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(223,93,98,0.24)]",
              focus
                ? "border-[rgba(223,93,98,0.26)] bg-[rgba(223,93,98,0.58)]"
                : "border-white/44 bg-[rgba(117,131,149,0.20)]",
            )}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.stopPropagation();
              onClose?.();
            }}
            title="关闭窗口"
            type="button"
          >
            <X className="operation-window-traffic-icon h-2.5 w-2.5 text-[#6f2024]" />
          </button>
          <button
            aria-label={titlebar.minimize_label}
            className={cn(
              "operation-window-traffic-button grid h-4 w-4 place-items-center rounded-full border transition hover:bg-[rgba(223,157,46,0.88)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(223,157,46,0.24)]",
              focus
                ? "border-[rgba(223,157,46,0.26)] bg-[rgba(223,157,46,0.62)]"
                : "border-white/44 bg-[rgba(117,131,149,0.20)]",
            )}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.stopPropagation();
              onMinimize?.();
            }}
            title="最小化窗口"
            type="button"
          >
            <Minus className="operation-window-traffic-icon h-2.5 w-2.5 text-[#735018]" />
          </button>
          <button
            aria-label={titlebar.zoom_label}
            className={cn(
              "operation-window-traffic-button grid h-4 w-4 place-items-center rounded-full border transition hover:bg-[rgba(47,184,132,0.84)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(47,184,132,0.24)]",
              focus
                ? "border-[rgba(47,184,132,0.22)] bg-[rgba(47,184,132,0.58)]"
                : "border-white/44 bg-[rgba(117,131,149,0.20)]",
            )}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.stopPropagation();
              onZoom?.();
            }}
            title={titlebar.zoom_title}
            type="button"
          >
            {maximized ? (
              <Minimize2 className="operation-window-traffic-icon h-2.5 w-2.5 text-[#1d6048]" />
            ) : (
              <Maximize2 className="operation-window-traffic-icon h-2.5 w-2.5 text-[#1d6048]" />
            )}
          </button>
        </div>
        <div className="flex min-w-0 flex-1 justify-center px-2">
          <div className="flex min-w-0 max-w-[70%] items-center justify-center gap-1.5 text-[10px] font-semibold">
            <Icon className={cn(
              "h-3 w-3 shrink-0",
              focus ? "text-(--icon-default)" : "text-(--icon-muted)",
            )} />
            <span className="min-w-0 truncate" title={titlebar.title_label}>
              {title}
            </span>
            <span
              aria-label={titlebar.status_label}
              className={cn(
                "h-1.5 w-1.5 shrink-0 rounded-full",
                titlebar.state_dot_tone === "active" && "bg-[rgba(47,184,132,0.82)]",
                titlebar.state_dot_tone === "background" && "bg-[rgba(117,131,149,0.42)]",
                titlebar.state_dot_tone === "minimized" && "bg-[rgba(223,157,46,0.86)]",
              )}
              title={titlebar.state_dot_title}
            />
            {titlebar.proxy_label ? (
              <span className="hidden truncate text-[9px] font-bold text-(--text-soft) lg:inline">
                {titlebar.proxy_label}
              </span>
            ) : null}
          </div>
        </div>
        <span aria-hidden="true" className="h-4 w-[52px] shrink-0" />
      </div>
      <div className={cn(
        "soft-scrollbar relative min-h-0 flex-1",
        tone === "terminal"
          ? "overflow-hidden bg-[#090e14] p-0"
          : contentMode === "flush"
            ? "overflow-hidden p-0"
            : "overflow-auto p-4",
        minimized && "hidden",
      )}>
        {previewMode === "stage-manager" ? (
          <button
            aria-label={`切换到 ${titlebar.title_label}`}
            className="group h-full w-full overflow-hidden bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(239,244,249,0.68))] p-2 text-left outline-none"
            onClick={(event) => {
              event.stopPropagation();
              onFocus?.();
            }}
            type="button"
          >
            <StageManagerWindowPreview
              appLabel={appLabel ?? "Nexus"}
              icon={Icon}
              title={titlebar.title_label}
              tone={tone}
            />
          </button>
        ) : tone !== "terminal" && contentMode !== "flush" ? (
          <div className="pointer-events-none absolute bottom-3 right-3 flex h-8 w-8 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-white/72 text-(--icon-muted) opacity-30">
            <Icon className="h-3.5 w-3.5" />
          </div>
        ) : null}
        <div
          aria-hidden={previewMode === "stage-manager" ? true : undefined}
          className={cn("h-full min-h-0", previewMode === "stage-manager" && "hidden")}
        >
          {children}
        </div>
      </div>
      {show_resize_handles ? (
        <>
          <div
            aria-hidden="true"
            className="absolute bottom-0 right-1 top-8 z-20 w-2 cursor-ew-resize"
            onPointerCancel={end_resize}
            onLostPointerCapture={end_resize}
            onPointerDown={(event) => start_resize(event, "right")}
            onPointerMove={move_resize}
            onPointerUp={end_resize}
          />
          <div
            aria-hidden="true"
            className="absolute inset-x-1 bottom-0 z-20 h-2 cursor-ns-resize"
            onPointerCancel={end_resize}
            onLostPointerCapture={end_resize}
            onPointerDown={(event) => start_resize(event, "bottom")}
            onPointerMove={move_resize}
            onPointerUp={end_resize}
          />
          <button
            aria-label="调整窗口大小"
            className="absolute bottom-1 right-1 z-30 h-5 w-5 cursor-nwse-resize rounded-[8px] border border-white/50 bg-white/42 opacity-0 transition-opacity hover:opacity-80 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]"
            onKeyDown={(event) => {
              if (!onResize || !["ArrowDown", "ArrowLeft", "ArrowRight", "ArrowUp"].includes(event.key)) {
                return;
              }
              event.preventDefault();
              const step = event.shiftKey ? 24 : 12;
              const currentWidth = resizeSize?.width ?? window_ref.current?.offsetWidth ?? 0;
              const currentHeight = resizeSize?.height ?? window_ref.current?.offsetHeight ?? 0;
              onResize({
                width: currentWidth + (event.key === "ArrowRight" ? step : event.key === "ArrowLeft" ? -step : 0),
                height: currentHeight + (event.key === "ArrowDown" ? step : event.key === "ArrowUp" ? -step : 0),
              });
            }}
            onPointerCancel={end_resize}
            onLostPointerCapture={end_resize}
            onPointerDown={(event) => start_resize(event, "corner")}
            onPointerMove={move_resize}
            onPointerUp={end_resize}
            type="button"
          />
        </>
      ) : null}
    </dialog>
  );
}
