/**
 * INPUT: WorkGraph 本地 viewport、折叠与搜索状态，以及可选的大图打开/关闭动作。
 * OUTPUT: 复用共享 Button、IconButton、SearchInput 与 Typography 的独立控制行：导航在左，缩放在右，搜索与窄屏换行不遮挡画布。
 * POS: WorkGraph 主画布的可访问导航层；只影响当前用户视图，不私有化标准控件状态。
 */
"use client";

import { useLayoutEffect, useRef, useState } from "react";
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
import { UiDialogCloseButton } from "@/shared/ui/dialog/dialog";
import { cn } from "@/shared/ui/class-name";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface ExecutionWorkGraphControlsProps {
  collapsibleCount: number;
  collapsedCount: number;
  currentResultIndex: number;
  onCloseExpanded?: () => void;
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
  onCloseExpanded,
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
  const searchTriggerRef = useRef<HTMLButtonElement | null>(null);
  const restoreSearchFocusRef = useRef(false);

  useLayoutEffect(() => {
    if (searchOpen) {
      searchRef.current?.focus();
    } else if (restoreSearchFocusRef.current) {
      restoreSearchFocusRef.current = false;
      searchTriggerRef.current?.focus();
    }
  }, [searchOpen]);

  const closeSearch = () => {
    // The IconButton's tooltip wrapper returns when search closes. Restore
    // focus after that DOM commit, rather than focusing the outgoing trigger.
    restoreSearchFocusRef.current = true;
    setSearchOpen(false);
    onQueryChange("");
  };
  return (
    <div
      className="flex shrink-0 flex-col gap-1 border-b border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 py-1.5"
      data-execution-workgraph-controls
    >
      <div aria-label={t("execution.graph_controls")} className="flex flex-wrap items-center justify-between gap-x-2 gap-y-1" role="group">
        <div className="flex shrink-0 items-center gap-0.5">
          <UiIconButton
            ref={searchTriggerRef}
            aria-label={t("execution.search_graph")}
            aria-pressed={searchOpen}
            onClick={() => searchOpen ? closeSearch() : setSearchOpen(true)}
            size="sm"
            tooltip={searchOpen ? null : t("execution.search_graph")}
            variant="ghost"
          >
            <Search className="h-3.5 w-3.5" />
          </UiIconButton>
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
        <div className="flex shrink-0 items-center gap-0.5">
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
            className="min-w-11 tabular-nums"
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
          {onCloseExpanded ? <UiDialogCloseButton onClose={onCloseExpanded} /> : null}
        </div>
      </div>

      {searchOpen ? (
        <div className="flex min-w-0 flex-wrap items-center gap-1 pb-1">
          <UiSearchInput
            aria-label={t("execution.search_graph")}
            className="min-w-28 flex-1"
            data-execution-graph-search
            onChange={onQueryChange}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                event.preventDefault();
                event.stopPropagation();
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
            variant="toolbar"
          />
          <div className="ml-auto flex shrink-0 items-center gap-1">
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
        </div>
      ) : null}
    </div>
  );
}
