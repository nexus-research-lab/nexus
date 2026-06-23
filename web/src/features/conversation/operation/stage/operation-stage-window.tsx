import type { CSSProperties, MouseEvent, PointerEvent, ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { Maximize2, Minimize2, Minus, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import { resolve_operation_window_keyboard_action } from "./operation-stage-window-actions";
import { build_stage_window_titlebar_state } from "./operation-stage-window-titlebar";

interface OperationStageWindowProps {
  title: string;
  icon: LucideIcon;
  children: ReactNode;
  position_class_name: string;
  app_label?: string;
  delay_ms?: number;
  focus?: boolean;
  launch_origin?: "active" | "desktop" | "dock";
  maximized?: boolean;
  minimized?: boolean;
  dimmed?: boolean;
  drag_offset?: { x: number; y: number };
  resize_size?: { height: number; width: number };
  mobile_hidden?: boolean;
  content_mode?: "flush" | "inset";
  preview_mode?: "stage-manager";
  restore_token?: number;
  z_index?: number;
  tone?: "default" | "terminal";
  on_close?: () => void;
  on_drag?: (offset: { x: number; y: number }) => void;
  on_focus?: () => void;
  on_minimize?: () => void;
  on_resize?: (size: { height: number; width: number }) => void;
  on_zoom?: () => void;
  on_cycle_focus?: (direction: "next" | "previous") => void;
}

export function OperationStageWindow({
  title,
  icon: Icon,
  children,
  position_class_name,
  app_label,
  delay_ms = 0,
  focus = false,
  launch_origin = "active",
  maximized = false,
  minimized = false,
  dimmed = false,
  drag_offset = { x: 0, y: 0 },
  resize_size,
  mobile_hidden = false,
  content_mode = "inset",
  preview_mode,
  restore_token,
  z_index,
  tone = "default",
  on_close,
  on_drag,
  on_focus,
  on_minimize,
  on_resize,
  on_zoom,
  on_cycle_focus,
}: OperationStageWindowProps) {
  const window_ref = useRef<HTMLDivElement | null>(null);
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
  const titlebar = build_stage_window_titlebar_state({
    app_label,
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
      on_focus?.();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    on_focus?.();
    drag_state_ref.current = {
      pointer_id,
      start_x: event.clientX,
      start_y: event.clientY,
      origin_x: drag_offset.x,
      origin_y: drag_offset.y,
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
      on_drag?.({
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
    if (!restore_token) {
      return;
    }
    set_is_restoring(true);
    const timeout = window.setTimeout(() => set_is_restoring(false), 360);
    return () => window.clearTimeout(timeout);
  }, [restore_token]);

  const move_drag = (event: PointerEvent<HTMLDivElement>) => {
    const drag_state = drag_state_ref.current;
    if (!drag_state || drag_state.pointer_id !== event.pointerId) {
      return;
    }
    event.preventDefault();
    on_drag?.({
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
    event: PointerEvent<HTMLDivElement>,
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
    on_focus?.();
    resize_state_ref.current = {
      edge,
      origin_height: resize_size?.height ?? rect.height,
      origin_width: resize_size?.width ?? rect.width,
      pointer_id: event.pointerId,
      start_x: event.clientX,
      start_y: event.clientY,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    set_is_dragging(true);
  };

  const move_resize = (event: PointerEvent<HTMLDivElement>) => {
    const resize_state = resize_state_ref.current;
    if (!resize_state || resize_state.pointer_id !== event.pointerId) {
      return;
    }
    event.preventDefault();
    on_resize?.({
      height: resize_state.edge === "right"
        ? resize_state.origin_height
        : resize_state.origin_height + event.clientY - resize_state.start_y,
      width: resize_state.edge === "bottom"
        ? resize_state.origin_width
        : resize_state.origin_width + event.clientX - resize_state.start_x,
    });
  };

  const end_resize = (event: PointerEvent<HTMLDivElement>) => {
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

  const show_resize_handles = !maximized && !minimized && preview_mode !== "stage-manager";

  return (
    <div
      ref={window_ref}
      aria-label={titlebar.aria_label}
      aria-roledescription="window"
      className={cn(
        "operation-stage-window absolute flex min-h-0 min-w-0 cursor-default flex-col overflow-hidden rounded-[14px] border backdrop-blur-xl outline-none transition-[left,top,width,height,opacity,filter,box-shadow,border-radius] duration-300 ease-[cubic-bezier(.2,.82,.2,1)] focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.42)] focus-visible:ring-offset-2 focus-visible:ring-offset-transparent max-md:!relative max-md:!inset-auto max-md:!h-auto max-md:!min-h-[180px] max-md:!w-full max-md:max-w-full",
        tone === "terminal"
          ? "border-white/14 bg-[#0d151e]/95 text-[#d8e8e2] shadow-[0_30px_76px_rgba(0,8,16,0.34)]"
          : "border-white/60 bg-[rgba(250,252,253,0.96)] text-(--text-strong) shadow-[0_28px_72px_rgba(18,28,42,0.24)]",
        focus && "operation-stage-window-focus",
        launch_origin === "dock" && "operation-stage-window-launch-dock",
        launch_origin === "desktop" && "operation-stage-window-launch-desktop",
        maximized && "operation-stage-window-maximized rounded-[18px]",
        dimmed && "opacity-[0.62] saturate-[0.82]",
        is_dragging && "operation-stage-window-dragging select-none",
        is_restoring && "operation-stage-window-restoring",
        preview_mode === "stage-manager" && "operation-stage-window-stage-manager rounded-[18px]",
        minimized && "min-h-0",
        mobile_hidden && "max-md:hidden",
        position_class_name,
      )}
      onKeyDown={(keyboard_event) => {
        if (keyboard_event.currentTarget !== keyboard_event.target) {
          return;
        }
        const action = resolve_operation_window_keyboard_action(keyboard_event);
        if (!action) {
          return;
        }
        keyboard_event.preventDefault();
        if (action === "focus") {
          on_focus?.();
        } else if (action === "close") {
          on_close?.();
        } else if (action === "minimize") {
          on_minimize?.();
        } else if (action === "cycle_next") {
          on_cycle_focus?.("next");
        } else if (action === "cycle_previous") {
          on_cycle_focus?.("previous");
        } else {
          on_zoom?.();
        }
      }}
      onMouseDown={on_focus}
      role="group"
      style={{
        "--operation-delay": `${delay_ms}ms`,
        "--operation-window-drag-x": `${drag_offset.x}px`,
        "--operation-window-drag-y": `${drag_offset.y}px`,
        height: resize_size && !maximized ? `${resize_size.height}px` : undefined,
        zIndex: z_index,
        translate: `${drag_offset.x}px ${drag_offset.y}px`,
        width: resize_size && !maximized ? `${resize_size.width}px` : undefined,
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
          on_zoom?.();
        }}
        onMouseDown={start_mouse_drag}
        onPointerDown={start_pointer_drag}
        onPointerMove={move_drag}
        onPointerUp={end_drag}
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
              on_close?.();
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
              on_minimize?.();
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
              on_zoom?.();
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
          : content_mode === "flush"
            ? "overflow-hidden p-0"
            : "overflow-auto p-4",
        minimized && "hidden",
      )}>
        {preview_mode === "stage-manager" ? (
          <button
            aria-label={`切换到 ${titlebar.title_label}`}
            className="group h-full w-full overflow-hidden bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(239,244,249,0.68))] p-2 text-left outline-none"
            onClick={(event) => {
              event.stopPropagation();
              on_focus?.();
            }}
            type="button"
          >
            <StageManagerWindowPreview
              app_label={app_label ?? "Nexus"}
              icon={Icon}
              title={titlebar.title_label}
              tone={tone}
            />
          </button>
        ) : tone !== "terminal" && content_mode !== "flush" ? (
          <div className="pointer-events-none absolute bottom-3 right-3 flex h-8 w-8 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-white/72 text-(--icon-muted) opacity-30">
            <Icon className="h-3.5 w-3.5" />
          </div>
        ) : null}
        {preview_mode === "stage-manager" ? null : children}
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
          <div
            aria-label="调整窗口大小"
            className="absolute bottom-1 right-1 z-30 h-5 w-5 cursor-nwse-resize rounded-[8px] border border-white/50 bg-white/42 opacity-0 transition-opacity hover:opacity-80 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]"
            onPointerCancel={end_resize}
            onLostPointerCapture={end_resize}
            onPointerDown={(event) => start_resize(event, "corner")}
            onPointerMove={move_resize}
            onPointerUp={end_resize}
            role="separator"
            tabIndex={0}
          />
        </>
      ) : null}
    </div>
  );
}

function StageManagerWindowPreview({
  app_label,
  icon: Icon,
  title,
  tone,
}: {
  app_label: string;
  icon: LucideIcon;
  title: string;
  tone: "default" | "terminal";
}) {
  const skin = stage_manager_preview_skin(app_label, tone);
  return (
    <div className={cn(
      "flex h-full min-h-0 flex-col overflow-hidden rounded-[12px] border shadow-[inset_0_1px_0_rgba(255,255,255,0.62)] transition group-hover:brightness-[1.03]",
      skin.frame,
    )}>
      <div className={cn("flex h-5 shrink-0 items-center gap-1.5 border-b px-2", skin.titlebar)}>
        <span className="h-1.5 w-1.5 rounded-full bg-[rgba(223,93,98,0.72)]" />
        <span className="h-1.5 w-1.5 rounded-full bg-[rgba(223,157,46,0.72)]" />
        <span className="h-1.5 w-1.5 rounded-full bg-[rgba(47,184,132,0.72)]" />
        <span className="ml-auto flex min-w-0 items-center gap-1 text-[7px] font-black">
          <Icon className="h-2.5 w-2.5 shrink-0" />
          <span className="truncate">{app_label}</span>
        </span>
      </div>
      <div className="min-h-0 flex-1 p-2">
        <p className={cn("truncate text-[9px] font-black leading-3", skin.title)}>{title}</p>
        <div className={cn("mt-1.5 space-y-1", skin.body)}>
          {skin.lines.map((line, index) => (
            <span
              className={cn("block h-1.5 rounded-full", line)}
              key={`${app_label}-${index}`}
            />
          ))}
        </div>
      </div>
      <div className={cn("h-3 shrink-0 border-t px-2 py-0.5", skin.status)}>
        <span className="block h-1.5 w-8 rounded-full bg-current opacity-30" />
      </div>
    </div>
  );
}

function stage_manager_preview_skin(app_label: string, tone: "default" | "terminal") {
  if (tone === "terminal" || app_label === "终端") {
    return {
      body: "bg-[#070c12]",
      frame: "border-white/12 bg-[#0a1118] text-[#8de0ad]",
      lines: ["w-10 bg-[#8de0ad]/35", "w-16 bg-white/18", "w-12 bg-[#8de0ad]/28"],
      status: "border-white/8 bg-[#070c12] text-[#8de0ad]",
      title: "text-[#d8e8e2]",
      titlebar: "border-white/8 bg-white/[0.04] text-[#8aa09b]",
    };
  }
  if (app_label === "Safari") {
    return {
      body: "bg-white/60",
      frame: "border-white/70 bg-[#eef5fb] text-(--text-soft)",
      lines: ["w-full bg-[rgba(91,114,255,0.16)]", "w-3/4 bg-[rgba(47,184,132,0.14)]", "w-1/2 bg-[rgba(117,131,149,0.14)]"],
      status: "border-white/54 bg-white/50 text-(--text-soft)",
      title: "text-(--text-strong)",
      titlebar: "border-white/60 bg-white/64 text-(--text-soft)",
    };
  }
  if (app_label === "Code") {
    return {
      body: "bg-[#101820]",
      frame: "border-white/14 bg-[#0c141c] text-[#8aa0ad]",
      lines: ["w-14 bg-[#8de0ad]/25", "w-full bg-white/12", "w-2/3 bg-white/10"],
      status: "border-white/8 bg-[#0a1118] text-[#8de0ad]",
      title: "text-[#dce8ee]",
      titlebar: "border-white/8 bg-[#151f29] text-[#8aa0ad]",
    };
  }
  return {
    body: "bg-white/54",
    frame: "border-white/68 bg-white/58 text-(--text-soft)",
    lines: ["w-full bg-[rgba(91,114,255,0.14)]", "w-4/5 bg-[rgba(117,131,149,0.14)]", "w-1/2 bg-[rgba(47,184,132,0.14)]"],
    status: "border-white/58 bg-white/46 text-(--text-soft)",
    title: "text-(--text-strong)",
    titlebar: "border-white/58 bg-white/58 text-(--text-soft)",
  };
}
