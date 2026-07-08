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

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/lib/utils";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { DIALOG_ICON_BUTTON_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";
import { useMermaidSvg } from "./use-mermaid-svg";

export interface MermaidViewProps {
  chart: string;
  compact?: boolean;
  className?: string;
  constrainHeight?: boolean;
  isStreaming?: boolean;
  showHeader?: boolean;
}

type MermaidViewMode = "preview" | "source";

const MERMAID_COMPACT_MAX_HEIGHT_CLASS_NAME = "max-h-[320px]";
const MERMAID_MARKDOWN_MAX_HEIGHT_CLASS_NAME = "max-h-[420px]";

function MermaidModeButton({
  active,
  children,
  onClick: onClick,
}: {
  active: boolean;
  children: ReactNode;
  onClick: () => void;
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
      onClick={onClick}
      role="tab"
      type="button"
    >
      {children}
    </button>
  );
}

function getMermaidBodyClassName(compact: boolean, constrainHeight: boolean) {
  if (compact) {
    return MERMAID_COMPACT_MAX_HEIGHT_CLASS_NAME;
  }
  if (constrainHeight) {
    return MERMAID_MARKDOWN_MAX_HEIGHT_CLASS_NAME;
  }
  return "min-h-0 flex-1";
}

function getMermaidSvgClassName(compact: boolean, constrainHeight: boolean) {
  if (compact) {
    return "[&>svg]:!h-auto [&>svg]:!max-h-[288px] [&>svg]:!max-w-full [&>svg]:!w-auto";
  }
  if (constrainHeight) {
    return "[&>svg]:!h-auto [&>svg]:!max-h-[388px] [&>svg]:!max-w-full [&>svg]:!w-auto";
  }
  return "[&>svg]:!h-auto [&>svg]:!max-w-full [&>svg]:!w-auto";
}

function MermaidSourceView({
  chart,
  compact,
  constrainHeight: constrainHeight,
}: {
  chart: string;
  compact: boolean;
  constrainHeight: boolean;
}) {
  return (
          <div
          aria-label="放大预览 Mermaid 图表"
          className={cn(
        "soft-scrollbar min-w-0 overflow-auto bg-(--surface-panel-background)",
        getMermaidBodyClassName(compact, constrainHeight),
      )}
    >
      <pre className="message-cjk-code-font min-w-full whitespace-pre px-4 py-3.5 text-[13px] leading-[1.6] text-(--text-strong)">
        {chart}
      </pre>
    </div>
  );
}

function buildSvgDataUrl(svg: string): string {
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
  isOpen: isOpen,
  svg,
  onClose: onClose,
}: {
  isOpen: boolean;
  svg: string;
  onClose: () => void;
}) {
  const imageUrl = useMemo(() => buildSvgDataUrl(svg), [svg]);
  const previewScrollRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<MermaidPreviewDragState | null>(null);
  const [isDragging, setIsDragging] = useResettableState(false, `${isOpen ? "open" : "closed"}\x1f${svg}`);

  useEffect(() => {
    if (isOpen) {
      dragStateRef.current = null;
    }
  }, [isOpen, svg]);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  useEffect(() => {
    if (!isOpen || typeof document === "undefined") return;

    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [isOpen]);

  const handlePreviewPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return;
    }

    const scrollEl = previewScrollRef.current;
    if (!scrollEl) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    dragStateRef.current = {
      pointer_id: event.pointerId,
      start_x: event.clientX,
      start_y: event.clientY,
      scroll_left: scrollEl.scrollLeft,
      scroll_top: scrollEl.scrollTop,
    };
    setIsDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePreviewPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    const scrollEl = previewScrollRef.current;
    if (!dragState || !scrollEl || dragState.pointer_id !== event.pointerId) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    scrollEl.scrollLeft = dragState.scroll_left - (event.clientX - dragState.start_x);
    scrollEl.scrollTop = dragState.scroll_top - (event.clientY - dragState.start_y);
  };

  const finishPreviewDrag = (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointer_id !== event.pointerId) {
      return;
    }

    dragStateRef.current = null;
    setIsDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  if (!isOpen || !svg || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- backdrop click-to-close + Escape is a standard modal dialog pattern
    <div
      aria-labelledby="mermaid-image-preview-title"
      aria-modal="true"
      className="dialog-backdrop z-[10000] overscroll-contain animate-in fade-in duration-(--motion-duration-fast)"
      data-modal-root="true"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          onClose();
        }
      }}
      onWheel={(event) => {
        if (event.target === event.currentTarget) {
          event.preventDefault();
        }
      }}
      role="dialog"
    >
      <section
        className="dialog-shell surface-radius-md relative flex h-[88vh] w-[94vw] max-w-7xl flex-col overflow-hidden overscroll-contain animate-in zoom-in-95 duration-(--motion-duration-fast)"
      >
        <h2 className="sr-only" id="mermaid-image-preview-title">
          Mermaid 预览
        </h2>
        <button
          aria-label="关闭"
          className={cn(
            DIALOG_ICON_BUTTON_CLASS_NAME,
            "absolute right-3 top-3 z-10 border border-(--surface-paper-border) bg-[color:color-mix(in_srgb,var(--surface-paper-background)_88%,transparent)] text-(--surface-paper-foreground) shadow-sm backdrop-blur",
          )}
          onClick={onClose}
          type="button"
        >
          <X className="h-5 w-5" />
        </button>
        <div
          aria-label="放大预览 Mermaid 图表"
          className={cn(
            "soft-scrollbar min-h-0 flex-1 select-none overflow-auto overscroll-contain bg-(--surface-paper-background)",
            isDragging ? "cursor-grabbing" : "cursor-grab",
          )}
          onPointerCancel={finishPreviewDrag}
          onPointerDown={handlePreviewPointerDown}
          onPointerMove={handlePreviewPointerMove}
          onPointerUp={finishPreviewDrag}
          onWheel={(event) => event.stopPropagation()}
          ref={previewScrollRef}
        >
          <div className="flex min-h-full min-w-full items-start justify-start p-6">
            <img
              alt="Mermaid 图表预览"
              className="max-h-none max-w-none object-contain"
              draggable={false}
              src={imageUrl}
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
  className: className,
  constrainHeight: constrainHeight = true,
  isStreaming: isStreaming = false,
  showHeader: showHeader = true,
}: MermaidViewProps) {
  const renderIdPrefix = `mermaid-${useId().replace(/:/g, "")}`;
  const copyResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { error, is_rendering: isRendering, svg } = useMermaidSvg(chart, isStreaming, renderIdPrefix);
  const [viewMode, setViewMode] = useState<MermaidViewMode>("preview");
  const [copied, setCopied] = useState(false);
  const [isImagePreviewOpen, setIsImagePreviewOpen] = useState(false);

  useEffect(() => {
    return () => {
      if (copyResetTimerRef.current) {
        clearTimeout(copyResetTimerRef.current);
      }
    };
  }, []);

  const handleCopySource = async () => {
    if (!await writeTextToClipboard(chart)) {
      return;
    }

    setCopied(true);
    if (copyResetTimerRef.current) {
      clearTimeout(copyResetTimerRef.current);
    }
    copyResetTimerRef.current = setTimeout(() => setCopied(false), 1600);
  };

  const handleOpenImagePreview = () => {
    if (svg) {
      setIsImagePreviewOpen(true);
    }
  };

  const handlePreviewKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    handleOpenImagePreview();
  };

  const renderPreview = () => {
    if (isRendering && !svg) {
      return (
        <div className={cn("flex items-center justify-center text-(--text-muted)", compact ? "min-h-24" : "min-h-56")}>
          <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
          {isStreaming ? "等待完整图表" : "正在渲染图表"}
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
          {isStreaming ? "等待完整图表" : "暂无图表预览"}
        </div>
      );
    }

    return (
      <div className={cn("group relative min-h-0 w-full", !compact && "flex flex-1")}>
        <div
          aria-label="放大预览 Mermaid 图表"
          className={cn(
            "mermaid-view soft-scrollbar relative flex min-w-0 w-full cursor-zoom-in items-center justify-center overflow-auto bg-(--surface-paper-background) p-4 text-(--surface-paper-foreground) outline-none transition-[box-shadow] focus-visible:ring-2 focus-visible:ring-primary/28",
            getMermaidBodyClassName(compact, constrainHeight),
            getMermaidSvgClassName(compact, constrainHeight),
          )}
          dangerouslySetInnerHTML={{ __html: svg }}
          onClick={handleOpenImagePreview}
          onKeyDown={handlePreviewKeyDown}
          role="button"
          tabIndex={0}
          title="放大预览"
        />
        <div className="pointer-events-none absolute bottom-2 right-2 flex h-7 w-7 items-center justify-center rounded-full border border-(--surface-paper-border) bg-[color:color-mix(in_srgb,var(--surface-paper-background)_86%,transparent)] text-(--surface-paper-muted) opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
          <Maximize2 className="h-3.5 w-3.5" />
        </div>
        {isRendering ? (
          <div className="pointer-events-none absolute right-2 top-2 inline-flex items-center rounded-full border border-(--surface-paper-border) bg-[color:color-mix(in_srgb,var(--surface-paper-background)_86%,transparent)] px-2 py-1 text-[11px] text-(--surface-paper-muted) shadow-sm">
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
        compact ? "my-2 max-h-[360px]" : constrainHeight ? "my-3 max-h-[460px]" : "min-h-0",
        className,
      )}
      data-mermaid-streaming={isStreaming}
    >
      {showHeader ? (
        <div className="flex shrink-0 items-center justify-between gap-2 border-b border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 py-1.5">
          <div className="message-cjk-code-font min-w-0 truncate text-[11px] uppercase text-(--text-muted)">
            Mermaid
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {viewMode === "source" ? (
              <button
                className="inline-flex h-6 w-6 items-center justify-center rounded-[6px] text-(--text-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
                onClick={() => void handleCopySource()}
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
                active={viewMode === "preview"}
                onClick={() => setViewMode("preview")}
              >
                <Eye className="h-3.5 w-3.5" />
                预览
              </MermaidModeButton>
              <MermaidModeButton
                active={viewMode === "source"}
                onClick={() => setViewMode("source")}
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
            : constrainHeight
              ? MERMAID_MARKDOWN_MAX_HEIGHT_CLASS_NAME
              : "flex min-h-0 flex-1",
        )}
      >
        {viewMode === "source" ? (
          <MermaidSourceView chart={chart} compact={compact} constrainHeight={constrainHeight} />
        ) : (
          renderPreview()
        )}
      </div>
      <MermaidImagePreviewDialog
        isOpen={isImagePreviewOpen}
        svg={svg}
        onClose={() => setIsImagePreviewOpen(false)}
      />
    </div>
  );
}
