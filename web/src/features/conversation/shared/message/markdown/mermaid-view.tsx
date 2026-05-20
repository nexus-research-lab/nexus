"use client";

import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import {
  AlertTriangle,
  Check,
  Code2,
  Copy,
  Eye,
  LoaderCircle,
  Maximize2,
  X,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { DIALOG_ICON_BUTTON_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";
import { useMermaidSvg } from "./use-mermaid-svg";

interface MermaidViewProps {
  chart: string;
  compact?: boolean;
  class_name?: string;
  constrain_height?: boolean;
  is_streaming?: boolean;
  show_header?: boolean;
}

type MermaidViewMode = "preview" | "source";

const MERMAID_COMPACT_MAX_HEIGHT_CLASS_NAME = "max-h-[320px]";
const MERMAID_MARKDOWN_MAX_HEIGHT_CLASS_NAME = "max-h-[420px]";

function MermaidModeButton({
  active,
  children,
  on_click,
}: {
  active: boolean;
  children: ReactNode;
  on_click: () => void;
}) {
  return (
    <button
      className={cn(
        "inline-flex h-6 items-center gap-1 rounded-[6px] px-2 text-[11px] font-medium transition-colors",
        active
          ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
          : "text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-strong)",
      )}
      aria-selected={active}
      data-active={active}
      onClick={on_click}
      role="tab"
      type="button"
    >
      {children}
    </button>
  );
}

function get_mermaid_body_class_name(compact: boolean, constrain_height: boolean) {
  if (compact) {
    return MERMAID_COMPACT_MAX_HEIGHT_CLASS_NAME;
  }
  if (constrain_height) {
    return MERMAID_MARKDOWN_MAX_HEIGHT_CLASS_NAME;
  }
  return "min-h-0 flex-1";
}

function get_mermaid_svg_class_name(compact: boolean, constrain_height: boolean) {
  if (compact) {
    return "[&>svg]:!h-auto [&>svg]:!max-h-[288px] [&>svg]:!max-w-full [&>svg]:!w-auto";
  }
  if (constrain_height) {
    return "[&>svg]:!h-auto [&>svg]:!max-h-[388px] [&>svg]:!max-w-full [&>svg]:!w-auto";
  }
  return "[&>svg]:!h-auto [&>svg]:!max-w-full [&>svg]:!w-auto";
}

function MermaidSourceView({
  chart,
  compact,
  constrain_height,
}: {
  chart: string;
  compact: boolean;
  constrain_height: boolean;
}) {
  return (
    <div
      className={cn(
        "soft-scrollbar min-w-0 overflow-auto bg-(--surface-panel-background)",
        get_mermaid_body_class_name(compact, constrain_height),
      )}
    >
      <pre className="message-cjk-code-font min-w-full whitespace-pre px-4 py-3.5 text-[13px] leading-[1.6] text-(--text-strong)">
        {chart}
      </pre>
    </div>
  );
}

function build_svg_data_url(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

interface MermaidPreviewDragState {
  pointer_id: number;
  start_x: number;
  start_y: number;
  scroll_left: number;
  scroll_top: number;
}

function MermaidImagePreviewDialog({
  is_open,
  svg,
  on_close,
}: {
  is_open: boolean;
  svg: string;
  on_close: () => void;
}) {
  const image_url = useMemo(() => build_svg_data_url(svg), [svg]);
  const preview_scroll_ref = useRef<HTMLDivElement | null>(null);
  const drag_state_ref = useRef<MermaidPreviewDragState | null>(null);
  const [is_dragging, set_is_dragging] = useState(false);

  useEffect(() => {
    if (is_open) {
      drag_state_ref.current = null;
      set_is_dragging(false);
    }
  }, [is_open, svg]);

  useEffect(() => {
    if (!is_open) return;

    const handle_key_down = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        on_close();
      }
    };

    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_close]);

  useEffect(() => {
    if (!is_open || typeof document === "undefined") return;

    const original_overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = original_overflow;
    };
  }, [is_open]);

  const handle_preview_pointer_down = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return;
    }

    const scroll_el = preview_scroll_ref.current;
    if (!scroll_el) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    drag_state_ref.current = {
      pointer_id: event.pointerId,
      start_x: event.clientX,
      start_y: event.clientY,
      scroll_left: scroll_el.scrollLeft,
      scroll_top: scroll_el.scrollTop,
    };
    set_is_dragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handle_preview_pointer_move = (event: PointerEvent<HTMLDivElement>) => {
    const drag_state = drag_state_ref.current;
    const scroll_el = preview_scroll_ref.current;
    if (!drag_state || !scroll_el || drag_state.pointer_id !== event.pointerId) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    scroll_el.scrollLeft = drag_state.scroll_left - (event.clientX - drag_state.start_x);
    scroll_el.scrollTop = drag_state.scroll_top - (event.clientY - drag_state.start_y);
  };

  const finish_preview_drag = (event: PointerEvent<HTMLDivElement>) => {
    const drag_state = drag_state_ref.current;
    if (!drag_state || drag_state.pointer_id !== event.pointerId) {
      return;
    }

    drag_state_ref.current = null;
    set_is_dragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  if (!is_open || !svg || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <div
      aria-labelledby="mermaid-image-preview-title"
      aria-modal="true"
      className="dialog-backdrop z-[10000] overscroll-contain animate-in fade-in duration-(--motion-duration-fast)"
      onClick={on_close}
      onWheel={(event) => {
        if (event.target === event.currentTarget) {
          event.preventDefault();
        }
      }}
      role="dialog"
    >
      <section
        className="dialog-shell radius-shell-xl relative flex h-[88vh] w-[94vw] max-w-7xl flex-col overflow-hidden overscroll-contain animate-in zoom-in-95 duration-(--motion-duration-fast)"
        onClick={(event) => event.stopPropagation()}
      >
        <h2 className="sr-only" id="mermaid-image-preview-title">
          Mermaid 预览
        </h2>
        <button
          aria-label="关闭"
          className={cn(
            DIALOG_ICON_BUTTON_CLASS_NAME,
            "absolute right-3 top-3 z-10 border border-black/8 bg-white/88 shadow-sm backdrop-blur",
          )}
          onClick={on_close}
          type="button"
        >
          <X className="h-5 w-5" />
        </button>
        <div
          className={cn(
            "soft-scrollbar min-h-0 flex-1 select-none overflow-auto overscroll-contain bg-white",
            is_dragging ? "cursor-grabbing" : "cursor-grab",
          )}
          onPointerCancel={finish_preview_drag}
          onPointerDown={handle_preview_pointer_down}
          onPointerMove={handle_preview_pointer_move}
          onPointerUp={finish_preview_drag}
          onWheel={(event) => event.stopPropagation()}
          ref={preview_scroll_ref}
        >
          <div className="flex min-h-full min-w-full items-start justify-start p-6">
            <img
              alt="Mermaid 图表预览"
              className="max-h-none max-w-none object-contain"
              draggable={false}
              src={image_url}
            />
          </div>
        </div>
      </section>
    </div>,
    document.body,
  );
}

export function MermaidView({
  chart,
  compact = false,
  class_name,
  constrain_height = true,
  is_streaming = false,
  show_header = true,
}: MermaidViewProps) {
  const render_id_prefix = `mermaid-${useId().replace(/:/g, "")}`;
  const copy_reset_timer_ref = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { error, is_rendering, svg } = useMermaidSvg(chart, is_streaming, render_id_prefix);
  const [view_mode, set_view_mode] = useState<MermaidViewMode>("preview");
  const [copied, set_copied] = useState(false);
  const [is_image_preview_open, set_is_image_preview_open] = useState(false);

  useEffect(() => {
    return () => {
      if (copy_reset_timer_ref.current) {
        clearTimeout(copy_reset_timer_ref.current);
      }
    };
  }, []);

  const handle_copy_source = async () => {
    try {
      await navigator.clipboard.writeText(chart);
    } catch {
      return;
    }

    set_copied(true);
    if (copy_reset_timer_ref.current) {
      clearTimeout(copy_reset_timer_ref.current);
    }
    copy_reset_timer_ref.current = setTimeout(() => set_copied(false), 1600);
  };

  const handle_open_image_preview = () => {
    if (svg) {
      set_is_image_preview_open(true);
    }
  };

  const handle_preview_key_down = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    handle_open_image_preview();
  };

  const render_preview = () => {
    if (is_rendering && !svg) {
      return (
        <div className={cn("flex items-center justify-center text-(--text-muted)", compact ? "min-h-24" : "min-h-56")}>
          <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
          {is_streaming ? "等待完整图表" : "正在渲染图表"}
        </div>
      );
    }

    if (error) {
      return (
        <div className="m-3 rounded-[8px] border border-destructive/20 bg-destructive/6 px-3 py-2 text-sm text-destructive">
          <div className="flex items-center gap-2 font-medium">
            <AlertTriangle className="h-4 w-4" />
            Mermaid 渲染失败
          </div>
          <pre className="mt-2 whitespace-pre-wrap break-words text-xs leading-5">{error}</pre>
        </div>
      );
    }

    if (!svg) {
      return (
        <div className={cn("flex items-center justify-center text-(--text-muted)", compact ? "min-h-24" : "min-h-56")}>
          {is_streaming ? "等待完整图表" : "暂无图表预览"}
        </div>
      );
    }

    return (
      <div className={cn("group relative min-h-0 w-full", !compact && "flex flex-1")}>
        <div
          className={cn(
            "mermaid-view soft-scrollbar relative flex min-w-0 w-full cursor-zoom-in items-center justify-center overflow-auto bg-white p-4 outline-none transition-[box-shadow] focus-visible:ring-2 focus-visible:ring-primary/28",
            get_mermaid_body_class_name(compact, constrain_height),
            get_mermaid_svg_class_name(compact, constrain_height),
          )}
          dangerouslySetInnerHTML={{ __html: svg }}
          onClick={handle_open_image_preview}
          onKeyDown={handle_preview_key_down}
          role="button"
          tabIndex={0}
          title="放大预览"
        />
        <div className="pointer-events-none absolute bottom-2 right-2 flex h-7 w-7 items-center justify-center rounded-full border border-black/8 bg-white/86 text-slate-600 opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
          <Maximize2 className="h-3.5 w-3.5" />
        </div>
        {is_rendering ? (
          <div className="pointer-events-none absolute right-2 top-2 inline-flex items-center rounded-full border border-black/8 bg-white/86 px-2 py-1 text-[11px] text-slate-500 shadow-sm">
            <LoaderCircle className="mr-1.5 h-3 w-3 animate-spin" />
            更新中
          </div>
        ) : null}
      </div>
    );
  };

  return (
    <div
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-[8px] border border-(--divider-subtle-color)",
        compact ? "my-2 max-h-[360px]" : constrain_height ? "my-3 max-h-[460px]" : "min-h-0",
        class_name,
      )}
      data-mermaid-streaming={is_streaming}
    >
      {show_header ? (
        <div className="flex shrink-0 items-center justify-between gap-2 border-b border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 py-1.5">
          <div className="message-cjk-code-font min-w-0 truncate text-[11px] uppercase text-(--text-muted)">
            Mermaid
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {view_mode === "source" ? (
              <button
                className="inline-flex h-6 w-6 items-center justify-center rounded-[6px] text-(--text-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
                onClick={() => void handle_copy_source()}
                title={copied ? "已复制源码" : "复制源码"}
                type="button"
              >
                {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </button>
            ) : null}
            <div
              aria-label="Mermaid 显示模式"
              className="inline-flex items-center rounded-[7px] border border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) p-0.5"
              role="tablist"
            >
              <MermaidModeButton
                active={view_mode === "preview"}
                on_click={() => set_view_mode("preview")}
              >
                <Eye className="h-3.5 w-3.5" />
                预览
              </MermaidModeButton>
              <MermaidModeButton
                active={view_mode === "source"}
                on_click={() => set_view_mode("source")}
              >
                <Code2 className="h-3.5 w-3.5" />
                源码
              </MermaidModeButton>
            </div>
          </div>
        </div>
      ) : null}
      <div
        className={cn(
          "min-w-0",
          compact
            ? MERMAID_COMPACT_MAX_HEIGHT_CLASS_NAME
            : constrain_height
              ? MERMAID_MARKDOWN_MAX_HEIGHT_CLASS_NAME
              : "flex min-h-0 flex-1",
        )}
      >
        {view_mode === "source" ? (
          <MermaidSourceView chart={chart} compact={compact} constrain_height={constrain_height} />
        ) : (
          render_preview()
        )}
      </div>
      <MermaidImagePreviewDialog
        is_open={is_image_preview_open}
        svg={svg}
        on_close={() => set_is_image_preview_open(false)}
      />
    </div>
  );
}
