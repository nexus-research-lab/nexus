import { useEffect, useMemo, useRef, type PointerEvent } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/lib/utils";
import { DIALOG_ICON_BUTTON_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";

interface PreviewDragState {
  pointerId: number;
  scrollLeft: number;
  scrollTop: number;
  startX: number;
  startY: number;
}

interface MermaidPreviewDialogProps {
  isOpen: boolean;
  onClose: () => void;
  svg: string;
}

export function MermaidPreviewDialog({
  isOpen,
  onClose,
  svg,
}: MermaidPreviewDialogProps) {
  const imageUrl = useMemo(() => buildSvgDataUrl(svg), [svg]);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<PreviewDragState | null>(null);
  const [isDragging, setIsDragging] = useResettableState(
    false,
    `${isOpen ? "open" : "closed"}\x1f${svg}`,
  );

  useEffect(() => {
    dragStateRef.current = null;
  }, [isOpen, svg]);

  useEffect(() => {
    if (!isOpen) {
      return undefined;
    }
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
    if (!isOpen || typeof document === "undefined") {
      return undefined;
    }
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [isOpen]);

  const handlePointerDown = (event: PointerEvent<HTMLDivElement>) => {
    const scrollElement = scrollRef.current;
    if (event.button !== 0 || !scrollElement) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    dragStateRef.current = {
      pointerId: event.pointerId,
      scrollLeft: scrollElement.scrollLeft,
      scrollTop: scrollElement.scrollTop,
      startX: event.clientX,
      startY: event.clientY,
    };
    setIsDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    const scrollElement = scrollRef.current;
    if (!dragState || !scrollElement || dragState.pointerId !== event.pointerId) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    scrollElement.scrollLeft = dragState.scrollLeft - (event.clientX - dragState.startX);
    scrollElement.scrollTop = dragState.scrollTop - (event.clientY - dragState.startY);
  };

  const finishDrag = (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId) {
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
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/click-events-have-key-events -- 背景层只负责点击关闭与滚轮隔离，键盘关闭由全局监听统一处理。
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
      onWheel={(event) => {
        if (event.target === event.currentTarget) {
          event.preventDefault();
        }
      }}
      role="dialog"
    >
      <section className="dialog-shell surface-radius-md relative flex h-[88vh] w-[94vw] max-w-7xl flex-col overflow-hidden overscroll-contain animate-in zoom-in-95 duration-(--motion-duration-fast)">
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
          onPointerCancel={finishDrag}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={finishDrag}
          onWheel={(event) => event.stopPropagation()}
          ref={scrollRef}
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

function buildSvgDataUrl(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}
