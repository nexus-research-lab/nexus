import {
  AlertTriangle,
  LoaderCircle,
  Maximize2,
} from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

import {
  getMermaidBodyClassName,
  getMermaidSvgClassName,
} from "./mermaid-view-layout";

export function MermaidModeButton({
  active,
  children,
  onClick,
}: {
  active: boolean;
  children: ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      aria-selected={active}
      className={cn(
        "inline-flex h-6 items-center gap-1 rounded-[6px] px-2 text-[11px] font-medium transition-colors",
        active
          ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
          : "text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-strong)",
      )}
      data-active={active}
      onClick={onClick}
      role="tab"
      type="button"
    >
      {children}
    </button>
  );
}

export function MermaidSourceView({
  chart,
  compact,
  constrainHeight,
}: {
  chart: string;
  compact: boolean;
  constrainHeight: boolean;
}) {
  return (
    <div
      className={cn(
        "soft-scrollbar min-w-0 overflow-auto bg-(--surface-panel-background)",
        getMermaidBodyClassName(compact, constrainHeight),
      )}
    >
      <pre className="message-code-font min-w-full whitespace-pre px-3 py-2.5 text-[12px] leading-[1.5] text-(--text-strong)">
        {chart}
      </pre>
    </div>
  );
}

export function MermaidRenderedPreview({
  compact,
  constrainHeight,
  error,
  isRendering,
  isStreaming,
  onOpenPreview,
  svg,
}: {
  compact: boolean;
  constrainHeight: boolean;
  error: string | null;
  isRendering: boolean;
  isStreaming: boolean;
  onOpenPreview: () => void;
  svg: string;
}) {
  const minimumHeightClassName = compact ? "min-h-24" : "min-h-56";
  if (isRendering && !svg) {
    return (
      <div className={cn("flex items-center justify-center text-(--text-muted)", minimumHeightClassName)}>
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
      <div className={cn("flex items-center justify-center text-(--text-muted)", minimumHeightClassName)}>
        {isStreaming ? "等待完整图表" : "暂无图表预览"}
      </div>
    );
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    onOpenPreview();
  };

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
        onClick={onOpenPreview}
        onKeyDown={handleKeyDown}
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
}
