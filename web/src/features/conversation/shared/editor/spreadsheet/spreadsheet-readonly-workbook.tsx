"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { cn } from "@/shared/ui/class-name";

import {
  columnIndexToLabel,
  createColumnSizeTable,
  createRenderedSpreadsheetCells,
  createRowSizeTable,
  createSpreadsheetCellStyle,
  SPREADSHEET_GRID_DIMENSIONS,
} from "./spreadsheet-grid-model";
import type {
  SpreadsheetPreviewSheetData,
  SpreadsheetPreviewWorkbookData,
} from "./spreadsheet-preview-model";

interface SpreadsheetReadonlyWorkbookProps {
  activeSheetIndex: number;
  onSelectSheet: (index: number) => void;
  workbook: SpreadsheetPreviewWorkbookData;
}

export function SpreadsheetReadonlyWorkbook({
  activeSheetIndex,
  onSelectSheet,
  workbook,
}: SpreadsheetReadonlyWorkbookProps) {
  const resolvedSheetIndex = Math.min(
    activeSheetIndex,
    workbook.sheets.length - 1,
  );
  const activeSheet = workbook.sheets[resolvedSheetIndex];
  if (!activeSheet) {
    return null;
  }
  return (
    <div className="flex h-full min-h-0 flex-col">
      {workbook.sheets.length > 1 ? (
        <div className="flex shrink-0 gap-1 overflow-x-auto border-b divider-subtle bg-(--surface-panel-background) px-3 py-2">
          {workbook.sheets.map((sheet, index) => (
            <button
              className={cn(
                "max-w-[180px] shrink-0 truncate rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                index === resolvedSheetIndex
                  ? "bg-primary text-primary-foreground"
                  : "text-(--text-muted) hover:bg-(--button-ghost-hover-background) hover:text-(--text-strong)",
              )}
              key={`${sheet.name}-${index}`}
              onClick={() => onSelectSheet(index)}
              title={sheet.name}
              type="button"
            >
              {sheet.name}
            </button>
          ))}
        </div>
      ) : null}
      <SpreadsheetReadonlySheet
        key={`${activeSheet.name}-${resolvedSheetIndex}`}
        sheet={activeSheet}
      />
    </div>
  );
}

function SpreadsheetReadonlySheet({
  sheet,
}: {
  sheet: SpreadsheetPreviewSheetData;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [scrollOffset, setScrollOffset] = useState({ left: 0, top: 0 });
  const rowSizes = useMemo(() => createRowSizeTable(sheet), [sheet]);
  const columnSizes = useMemo(() => createColumnSizeTable(sheet), [sheet]);
  const rowVirtualizer = useVirtualizer({
    count: sheet.row_count,
    estimateSize: (index) => rowSizes.sizes[index]
      ?? SPREADSHEET_GRID_DIMENSIONS.defaultRowHeight,
    getScrollElement: () => scrollRef.current,
    overscan: 8,
  });
  const columnVirtualizer = useVirtualizer({
    count: sheet.column_count,
    estimateSize: (index) => columnSizes.sizes[index]
      ?? SPREADSHEET_GRID_DIMENSIONS.defaultColumnWidth,
    getScrollElement: () => scrollRef.current,
    horizontal: true,
    overscan: 3,
  });
  const virtualRows = rowVirtualizer.getVirtualItems();
  const virtualColumns = columnVirtualizer.getVirtualItems();
  const renderedCells = createRenderedSpreadsheetCells(
    sheet,
    rowSizes,
    columnSizes,
    virtualRows,
    virtualColumns,
  );
  const handleScroll = useCallback(() => {
    const element = scrollRef.current;
    if (element) {
      setScrollOffset({
        left: element.scrollLeft,
        top: element.scrollTop,
      });
    }
  }, []);

  return (
    <div
      className="grid min-h-0 flex-1 bg-[var(--surface-panel-subtle-background)] text-xs text-(--text-default)"
      style={{
        gridTemplateColumns:
          `${SPREADSHEET_GRID_DIMENSIONS.rowHeaderWidth}px minmax(0, 1fr)`,
        gridTemplateRows:
          `${SPREADSHEET_GRID_DIMENSIONS.columnHeaderHeight}px minmax(0, 1fr)`,
      }}
    >
      <div className="z-30 border-r border-b border-(--divider-subtle-color) bg-(--surface-panel-background)" />
      <div className="relative overflow-hidden border-b border-(--divider-subtle-color) bg-(--surface-panel-background)">
        <div
          className="relative h-full"
          style={{
            transform: `translateX(${-scrollOffset.left}px)`,
            width: columnSizes.total,
          }}
        >
          {virtualColumns.map((column) => (
            <div
              className="absolute top-0 flex h-full items-center justify-center border-r border-(--divider-subtle-color) px-2 text-[10px] font-semibold text-(--text-muted)"
              key={column.key}
              style={{
                transform: `translateX(${column.start}px)`,
                width: column.size,
              }}
            >
              {columnIndexToLabel(column.index)}
            </div>
          ))}
        </div>
      </div>
      <div className="relative overflow-hidden border-r border-(--divider-subtle-color) bg-(--surface-panel-background)">
        <div
          className="relative w-full"
          style={{
            height: rowSizes.total,
            transform: `translateY(${-scrollOffset.top}px)`,
          }}
        >
          {virtualRows.map((row) => (
            <div
              className="absolute left-0 flex w-full items-center justify-end border-b border-(--divider-subtle-color) px-2 text-[10px] font-medium text-(--text-muted)"
              key={row.key}
              style={{
                height: row.size,
                transform: `translateY(${row.start}px)`,
              }}
            >
              {row.index + 1}
            </div>
          ))}
        </div>
      </div>
      <div
        className="overflow-auto bg-(--card-default-background)"
        onScroll={handleScroll}
        ref={scrollRef}
      >
        <div
          className="relative"
          role="grid"
          style={{ height: rowSizes.total, width: columnSizes.total }}
        >
          {renderedCells.map((cell) => (
            <div
              className="absolute overflow-hidden border-r border-b border-(--divider-subtle-color) px-2 py-1"
              key={`${cell.rowIndex}:${cell.columnIndex}`}
              role="gridcell"
              style={{
                ...createSpreadsheetCellStyle(sheet, cell.cell),
                height: cell.height,
                transform:
                  `translate(${cell.columnStart}px, ${cell.rowStart}px)`,
                width: cell.width,
              }}
              title={cell.cell?.text || undefined}
            >
              {cell.cell?.text || ""}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
