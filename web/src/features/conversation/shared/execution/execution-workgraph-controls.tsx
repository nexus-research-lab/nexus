/**
 * INPUT: WorkGraph 本地 viewport、折叠与搜索状态，以及可选的大图打开动作。
 * OUTPUT: 不改写 Graph 数据的紧凑画布控制条；适应当前视口与打开大图使用不同图标和语义。
 * POS: WorkGraph 主画布的可访问导航层；所有动作只影响当前用户视图。
 */
"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
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
import { cn } from "@/shared/ui/class-name";

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
      <div className="pointer-events-auto flex items-center gap-0.5 rounded-[11px] border border-(--surface-control-border) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_94%,transparent)] p-1 shadow-(--surface-control-shadow) backdrop-blur-xl">
        <GraphControlButton
          active={searchOpen}
          label={t("execution.search_graph")}
          onClick={() => setSearchOpen((value) => !value)}
        >
          <Search className="h-3.5 w-3.5" />
        </GraphControlButton>
        <span aria-hidden="true" className="mx-0.5 h-4 w-px bg-(--divider-subtle-color)" />
        <GraphControlButton label={t("execution.zoom_out")} onClick={onZoomOut}>
          <Minus className="h-3.5 w-3.5" />
        </GraphControlButton>
        <button
          aria-label={t("execution.reset_zoom")}
          className="h-7 min-w-11 rounded-[7px] px-1.5 text-[10px] font-medium tabular-nums text-(--text-soft) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
          data-execution-graph-zoom
          onClick={onResetZoom}
          title={t("execution.reset_zoom")}
          type="button"
        >
          {Math.round(zoom * 100)}%
        </button>
        <GraphControlButton label={t("execution.zoom_in")} onClick={onZoomIn}>
          <Plus className="h-3.5 w-3.5" />
        </GraphControlButton>
        <GraphControlButton label={t("execution.fit_graph")} onClick={onFit}>
          <Scan className="h-3.5 w-3.5" />
        </GraphControlButton>
        {onOpenExpanded ? (
          <GraphControlButton
            label={t("execution.open_workgraph")}
            onClick={onOpenExpanded}
          >
            <Maximize2 className="h-3.5 w-3.5" />
          </GraphControlButton>
        ) : null}
        <GraphControlButton label={t("execution.locate_current")} onClick={onLocateCurrent}>
          <LocateFixed className="h-3.5 w-3.5" />
        </GraphControlButton>
        {collapsibleCount > 0 ? (
          <>
            <span aria-hidden="true" className="mx-0.5 h-4 w-px bg-(--divider-subtle-color)" />
            {collapsedCount > 0 ? (
              <GraphControlButton label={t("execution.expand_all")} onClick={onExpandAll}>
                <ChevronsUpDown className="h-3.5 w-3.5" />
              </GraphControlButton>
            ) : (
              <GraphControlButton label={t("execution.collapse_all")} onClick={onCollapseAll}>
                <ChevronsDownUp className="h-3.5 w-3.5" />
              </GraphControlButton>
            )}
          </>
        ) : null}
      </div>

      {searchOpen ? (
        <div className="pointer-events-auto flex w-[min(22rem,calc(100vw-2rem))] items-center gap-1 rounded-[11px] border border-(--surface-control-border) bg-(--surface-panel-background) p-1.5 shadow-(--surface-control-shadow)">
          <Search aria-hidden="true" className="ml-1 h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <input
            aria-label={t("execution.search_graph")}
            className="h-7 min-w-0 flex-1 bg-transparent px-1 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
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
            <span className="shrink-0 text-[10px] tabular-nums text-(--text-soft)">
              {resultCount > 0
                ? t("execution.search_results", {
                    current: Math.max(1, currentResultIndex + 1),
                    total: resultCount,
                  })
                : t("execution.search_no_results")}
            </span>
          ) : null}
          <GraphControlButton
            disabled={resultCount === 0}
            label={t("execution.previous_result")}
            onClick={onPreviousResult}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </GraphControlButton>
          <GraphControlButton
            disabled={resultCount === 0}
            label={t("execution.next_result")}
            onClick={onNextResult}
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </GraphControlButton>
          <GraphControlButton label={t("execution.close_search")} onClick={closeSearch}>
            <X className="h-3.5 w-3.5" />
          </GraphControlButton>
        </div>
      ) : null}
    </div>
  );
}

function GraphControlButton({
  active = false,
  children,
  disabled = false,
  label,
  onClick,
}: {
  active?: boolean;
  children: ReactNode;
  disabled?: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      aria-label={label}
      aria-pressed={active || undefined}
      className={cn(
        "grid h-7 w-7 shrink-0 place-items-center rounded-[7px] text-(--icon-muted) transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)",
        active
          ? "bg-(--surface-interactive-hover-background) text-(--primary)"
          : "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
        disabled && "cursor-not-allowed opacity-40",
      )}
      disabled={disabled}
      onClick={onClick}
      title={label}
      type="button"
    >
      {children}
    </button>
  );
}
