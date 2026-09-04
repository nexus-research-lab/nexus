// INPUT: Mermaid 文本、流式状态、尺寸模式与可选标题栏。
// OUTPUT: 使用共享 IconButton/SegmentedControl 的源码、图表和全屏预览组合。
// POS: Markdown Mermaid 视图编排；渲染状态和 SVG 清理归相邻 Hook/模型。

"use client";

import { useEffect, useId, useRef, useState } from "react";
import {
  Check,
  Code2,
  Copy,
  Eye,
} from "lucide-react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";

import { MermaidPreviewDialog } from "./mermaid-preview-dialog";
import {
  getMermaidContainerClassName,
  getMermaidContentClassName,
} from "./mermaid-view-layout";
import {
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
  const { t } = useI18n();
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
  const copySourceLabel = t(
    copied ? "markdown.mermaid.copied_source" : "markdown.mermaid.copy_source",
  );

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
          <div className="message-code-font min-w-0 truncate text-xs uppercase text-(--text-muted)">
            Mermaid
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {viewMode === "source" ? (
              <UiIconButton
                aria-label={copySourceLabel}
                onClick={() => {
                  void copySource();
                }}
                size="xs"
                variant="ghost"
              >
                {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </UiIconButton>
            ) : null}
            <UiSegmentedControl
              density="compact"
              onChange={setViewMode}
              options={[
                { icon: Eye, label: t("markdown.mermaid.preview"), value: "preview" },
                { icon: Code2, label: t("markdown.mermaid.source"), value: "source" },
              ]}
              title={t("markdown.mermaid.display_mode")}
              value={viewMode}
            />
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
