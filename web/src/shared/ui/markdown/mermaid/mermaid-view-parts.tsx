import {
  LoaderCircle,
  Maximize2,
} from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import {
  getMermaidBodyClassName,
  getMermaidSvgClassName,
} from "./mermaid-view-layout";
import type { MermaidRenderFailure } from "./use-mermaid-svg";

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
        "inline-flex h-6 items-center gap-1 rounded-[6px] px-2 text-xs font-medium transition-colors",
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
      <pre className="message-code-font min-w-full whitespace-pre px-3 py-2.5 text-compact leading-[1.5] text-(--text-strong)">
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
  error: MermaidRenderFailure | null;
  isRendering: boolean;
  isStreaming: boolean;
  onOpenPreview: () => void;
  svg: string;
}) {
  const { t } = useI18n();
  const minimumHeightClassName = compact ? "min-h-24" : "min-h-56";
  if (isRendering && !svg) {
    return (
      <div className={cn("flex items-center justify-center text-(--text-muted)", minimumHeightClassName)}>
        <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
        {t(isStreaming ? "markdown.mermaid.waiting" : "markdown.mermaid.rendering")}
      </div>
    );
  }
  if (error) {
    return (
      <UiResourceState
        className="m-3 min-h-0 py-4"
        impact={t("markdown.mermaid.render_failed_impact")}
        nextStep={t("markdown.mermaid.render_failed_next_step")}
        size="sm"
        state="error"
        title={t(error === "invalid_syntax"
          ? "markdown.mermaid.invalid_syntax"
          : "markdown.mermaid.render_failed")}
        urgency="polite"
        variant="card"
      />
    );
  }
  if (!svg) {
    return (
      <div className={cn("flex items-center justify-center text-(--text-muted)", minimumHeightClassName)}>
        {t(isStreaming ? "markdown.mermaid.waiting" : "markdown.mermaid.no_preview")}
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
        aria-label={t("markdown.mermaid.open_preview")}
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
        title={t("markdown.mermaid.open_preview")}
      />
      <div className="pointer-events-none absolute bottom-2 right-2 flex h-7 w-7 items-center justify-center rounded-full border border-(--surface-paper-border) bg-[color:color-mix(in_srgb,var(--surface-paper-background)_86%,transparent)] text-(--surface-paper-muted) opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
        <Maximize2 className="h-3.5 w-3.5" />
      </div>
      {isRendering ? (
        <div className="pointer-events-none absolute right-2 top-2 inline-flex items-center rounded-full border border-(--surface-paper-border) bg-[color:color-mix(in_srgb,var(--surface-paper-background)_86%,transparent)] px-2 py-1 text-xs text-(--surface-paper-muted) shadow-sm">
          <LoaderCircle className="mr-1.5 h-3 w-3 animate-spin" />
          {t("markdown.mermaid.updating")}
        </div>
      ) : null}
    </div>
  );
}
