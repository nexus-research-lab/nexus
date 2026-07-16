"use client";

import { useEffect, useId, useRef, useState } from "react";
import {
  Check,
  Code2,
  Copy,
  Eye,
} from "lucide-react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { cn } from "@/shared/ui/class-name";

import { MermaidPreviewDialog } from "./mermaid-preview-dialog";
import {
  getMermaidContainerClassName,
  getMermaidContentClassName,
} from "./mermaid-view-layout";
import {
  MermaidModeButton,
  MermaidRenderedPreview,
  MermaidSourceView,
} from "./mermaid-view-parts";
import { useMermaidSvg } from "./use-mermaid-svg";

export interface MermaidViewProps {
  chart: string;
  className?: string;
  compact?: boolean;
  constrainHeight?: boolean;
  isStreaming?: boolean;
  showHeader?: boolean;
}

type MermaidViewMode = "preview" | "source";

export function MermaidView({
  chart,
  className,
  compact = false,
  constrainHeight = true,
  isStreaming = false,
  showHeader = true,
}: MermaidViewProps) {
  const renderIdPrefix = `mermaid-${useId().replace(/:/g, "")}`;
  const copyResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { error, is_rendering: isRendering, svg } = useMermaidSvg(
    chart,
    isStreaming,
    renderIdPrefix,
  );
  const [viewMode, setViewMode] = useState<MermaidViewMode>("preview");
  const [copied, setCopied] = useState(false);
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);

  useEffect(() => () => {
    if (copyResetTimerRef.current) {
      clearTimeout(copyResetTimerRef.current);
    }
  }, []);

  const copySource = async () => {
    if (!await writeTextToClipboard(chart)) {
      return;
    }
    setCopied(true);
    if (copyResetTimerRef.current) {
      clearTimeout(copyResetTimerRef.current);
    }
    copyResetTimerRef.current = setTimeout(() => setCopied(false), 1600);
  };

  const openPreview = () => {
    if (svg) {
      setIsPreviewOpen(true);
    }
  };

  return (
    <div
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-[8px] border border-(--divider-subtle-color)",
        getMermaidContainerClassName(compact, constrainHeight),
        className,
      )}
      data-mermaid-streaming={isStreaming}
    >
      {showHeader ? (
        <div className="flex shrink-0 items-center justify-between gap-2 border-b border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 py-1.5">
          <div className="message-code-font min-w-0 truncate text-[11px] uppercase text-(--text-muted)">
            Mermaid
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {viewMode === "source" ? (
              <button
                className="inline-flex h-6 w-6 items-center justify-center rounded-[6px] text-(--text-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
                onClick={() => {
                  void copySource();
                }}
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

      <div className={cn("min-w-0", getMermaidContentClassName(compact, constrainHeight))}>
        {viewMode === "source" ? (
          <MermaidSourceView
            chart={chart}
            compact={compact}
            constrainHeight={constrainHeight}
          />
        ) : (
          <MermaidRenderedPreview
            compact={compact}
            constrainHeight={constrainHeight}
            error={error}
            isRendering={isRendering}
            isStreaming={isStreaming}
            onOpenPreview={openPreview}
            svg={svg}
          />
        )}
      </div>
      <MermaidPreviewDialog
        isOpen={isPreviewOpen}
        onClose={() => setIsPreviewOpen(false)}
        svg={svg}
      />
    </div>
  );
}
