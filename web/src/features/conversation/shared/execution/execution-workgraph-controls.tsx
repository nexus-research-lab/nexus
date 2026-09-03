/**
 * INPUT: WorkGraph 本地 viewport、折叠与搜索状态，以及可选的大图打开动作。
 * OUTPUT: 复用共享 Button、IconButton、Popover 与 Typography 的紧凑画布控制条。
 * POS: WorkGraph 主画布的可访问导航层；只影响当前用户视图，不私有化标准控件状态。
 */
"use client";

import { useEffect, useRef, useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  ChevronsDownUp,
  ChevronsUpDown,
  LocateFixed,
  Maximize2,
  Minus,
  Plus,
  Scan,
  Search,
  X,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface ExecutionWorkGraphControlsProps {
  collapsibleCount: number;
  collapsedCount: number;
  currentResultIndex: number;
  onCollapseAll: () => void;
  onExpandAll: () => void;
  onFit: () => void;
  onLocateCurrent: () => void;
  onNextResult: () => void;
  onOpenExpanded?: () => void;
  onPreviousResult: () => void;
  onQueryChange: (query: string) => void;
  onResetZoom: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  query: string;
  resultCount: number;
  zoom: number;
}

export function ExecutionWorkGraphControls({
  collapsibleCount,
  collapsedCount,
  currentResultIndex,
  onCollapseAll,
  onExpandAll,
  onFit,
  onLocateCurrent,
  onNextResult,
  onOpenExpanded,
  onPreviousResult,
  onQueryChange,
  onResetZoom,
  onZoomIn,
  onZoomOut,
  query,
  resultCount,
  zoom,
}: ExecutionWorkGraphControlsProps) {
  const { t } = useI18n();
  const [searchOpen, setSearchOpen] = useState(false);
  const searchRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (searchOpen) {
      searchRef.current?.focus();
    }
  }, [searchOpen]);

  const closeSearch = () => {
    setSearchOpen(false);
    onQueryChange("");
  };
  return (
    <div
      className="pointer-events-none absolute left-3 top-3 z-40 flex max-w-[calc(100%-1.5rem)] flex-col items-start gap-2"
      data-execution-workgraph-controls
    >
      <div className="surface-popover surface-radius-sm pointer-events-auto flex items-center gap-0.5 p-1 backdrop-blur-xl">
        <UiIconButton
          aria-label={t("execution.search_graph")}
          aria-pressed={searchOpen}
          onClick={() => setSearchOpen((value) => !value)}
          size="sm"
          tooltip={t("execution.search_graph")}
          variant="ghost"
        >
          <Search className="h-3.5 w-3.5" />
        </UiIconButton>
        <span aria-hidden="true" className="mx-0.5 h-4 w-px bg-(--divider-subtle-color)" />
        <UiIconButton
          aria-label={t("execution.zoom_out")}
          onClick={onZoomOut}
          size="sm"
          tooltip={t("execution.zoom_out")}
          variant="ghost"
        >
          <Minus className="h-3.5 w-3.5" />
        </UiIconButton>
        <UiButton
          aria-label={t("execution.reset_zoom")}
          className="h-7 min-w-11 px-1.5 tabular-nums"
          data-execution-graph-zoom
          onClick={onResetZoom}
          size="xs"
          variant="text"
        >
          {Math.round(zoom * 100)}%
        </UiButton>
        <UiIconButton
          aria-label={t("execution.zoom_in")}
          onClick={onZoomIn}
          size="sm"
          tooltip={t("execution.zoom_in")}
          variant="ghost"
        >
          <Plus className="h-3.5 w-3.5" />
        </UiIconButton>
        <UiIconButton
          aria-label={t("execution.fit_graph")}
          onClick={onFit}
          size="sm"
          tooltip={t("execution.fit_graph")}
          variant="ghost"
        >
          <Scan className="h-3.5 w-3.5" />
        </UiIconButton>
        {onOpenExpanded ? (
          <UiIconButton
            aria-label={t("execution.open_workgraph")}
            onClick={onOpenExpanded}
            size="sm"
            tooltip={t("execution.open_workgraph")}
            variant="ghost"
          >
            <Maximize2 className="h-3.5 w-3.5" />
          </UiIconButton>
        ) : null}
        <UiIconButton
          aria-label={t("execution.locate_current")}
          onClick={onLocateCurrent}
          size="sm"
          tooltip={t("execution.locate_current")}
          variant="ghost"
        >
          <LocateFixed className="h-3.5 w-3.5" />
        </UiIconButton>
        {collapsibleCount > 0 ? (
          <>
            <span aria-hidden="true" className="mx-0.5 h-4 w-px bg-(--divider-subtle-color)" />
            {collapsedCount > 0 ? (
              <UiIconButton
                aria-label={t("execution.expand_all")}
                onClick={onExpandAll}
                size="sm"
                tooltip={t("execution.expand_all")}
                variant="ghost"
              >
                <ChevronsUpDown className="h-3.5 w-3.5" />
              </UiIconButton>
            ) : (
              <UiIconButton
                aria-label={t("execution.collapse_all")}
                onClick={onCollapseAll}
                size="sm"
                tooltip={t("execution.collapse_all")}
                variant="ghost"
              >
                <ChevronsDownUp className="h-3.5 w-3.5" />
              </UiIconButton>
            )}
          </>
        ) : null}
      </div>

      {searchOpen ? (
        <div className="surface-popover surface-radius-sm pointer-events-auto flex w-[min(22rem,calc(100vw-2rem))] items-center gap-1 p-1.5">
          <Search aria-hidden="true" className="ml-1 h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <input
            aria-label={t("execution.search_graph")}
            className={cn(
              "h-7 min-w-0 flex-1 bg-transparent px-1 outline-none placeholder:text-(--text-soft)",
              getUiTypographyClassName({ role: "control", tone: "strong" }),
            )}
            data-execution-graph-search
            onChange={(event) => onQueryChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                closeSearch();
              } else if (event.key === "Enter") {
                event.preventDefault();
                if (event.shiftKey) {
                  onPreviousResult();
                } else {
                  onNextResult();
                }
              }
            }}
            placeholder={t("execution.search_placeholder")}
            ref={searchRef}
            value={query}
          />
          {query ? (
            <span className={cn(
              "shrink-0 tabular-nums",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}>
              {resultCount > 0
                ? t("execution.search_results", {
                    current: Math.max(1, currentResultIndex + 1),
                    total: resultCount,
                  })
                : t("execution.search_no_results")}
            </span>
          ) : null}
          <UiIconButton
            aria-label={t("execution.previous_result")}
            disabled={resultCount === 0}
            onClick={onPreviousResult}
            size="sm"
            tooltip={t("execution.previous_result")}
            variant="ghost"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </UiIconButton>
          <UiIconButton
            aria-label={t("execution.next_result")}
            disabled={resultCount === 0}
            onClick={onNextResult}
            size="sm"
            tooltip={t("execution.next_result")}
            variant="ghost"
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </UiIconButton>
          <UiIconButton
            aria-label={t("execution.close_search")}
            onClick={closeSearch}
            size="sm"
            tooltip={t("execution.close_search")}
            variant="ghost"
          >
            <X className="h-3.5 w-3.5" />
          </UiIconButton>
        </div>
      ) : null}
    </div>
  );
}
