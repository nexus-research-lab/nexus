// INPUT: 已投影的 Workbook、当前工作表索引与切换命令。
// OUTPUT: 共享底线式工作表选择条和只读虚拟化表格。
// POS: Spreadsheet 预览视图；不解析 ExcelJS，不拥有全局 Tabs 外观。
"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiTabs } from "@/shared/ui/navigation/tabs";

import { createSpreadsheetCellStyle } from "./spreadsheet-cell-style";
import {
  columnIndexToLabel,
  createColumnSizeTable,
  createRenderedSpreadsheetCells,
  createRowSizeTable,
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
  const { t } = useI18n();
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
        <UiTabs
          activeValue={String(resolvedSheetIndex)}
          ariaLabel={t("workspace_file.spreadsheet_loaded", {
            count: workbook.sheets.length,
          })}
          className="shrink-0 border-b divider-subtle bg-(--surface-panel-background) px-3 py-1"
          density="compact"
          itemClassName="max-w-[180px] overflow-hidden text-ellipsis"
          onChange={(value) => onSelectSheet(Number(value))}
          options={workbook.sheets.map((sheet, index) => ({
            label: sheet.name,
            title: sheet.name,
            value: String(index),
          }))}
        />
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
              className="absolute top-0 flex h-full items-center justify-center border-r border-(--divider-subtle-color) px-2 text-2xs font-semibold text-(--text-muted)"
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
              className="absolute left-0 flex w-full items-center justify-end border-b border-(--divider-subtle-color) px-2 text-2xs font-medium text-(--text-muted)"
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
                ...createSpreadsheetCellStyle(
                  sheet.styles,
                  cell.cell?.style,
                ),
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
