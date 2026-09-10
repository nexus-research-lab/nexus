// INPUT: 已投影的 Workbook、当前工作表索引与切换命令。
// OUTPUT: 共享工作表选择条和可键盘滚动的只读虚拟表格，保留行列位置与合并语义。
// POS: Spreadsheet 视图；行投影与内容样式归本目录，Tabs、元数据排版和滚动配方归 shared/ui。
"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { useCallback, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import { cn } from "@/shared/ui/class-name";
import { UI_PREVIEW_VIEWPORT_CLASS_NAME } from "@/shared/ui/layout/preview-viewport-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { createSpreadsheetCellStyle } from "./spreadsheet-cell-style";
import {
  columnIndexToLabel,
  createColumnSizeTable,
  createRenderedSpreadsheetRows,
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

const coordinateTypography = getUiTypographyClassName({ role: "caption", tone: "muted", weight: "medium" });

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
    <div className="flex h-full min-h-0 min-w-0 flex-col">
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
  const renderedRows = createRenderedSpreadsheetRows(
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
      className={cn("grid min-h-0 min-w-0 flex-1 bg-[var(--surface-panel-subtle-background)]", getUiTypographyClassName({ role: "caption" }))}
      style={{
        gridTemplateColumns:
          `${SPREADSHEET_GRID_DIMENSIONS.rowHeaderWidth}px minmax(0, 1fr)`,
        gridTemplateRows:
          `${SPREADSHEET_GRID_DIMENSIONS.columnHeaderHeight}px minmax(0, 1fr)`,
      }}
    >
      <div aria-hidden="true" className="z-30 border-r border-b border-(--divider-subtle-color) bg-(--surface-panel-background)" />
      <div aria-hidden="true" className="relative overflow-hidden border-b border-(--divider-subtle-color) bg-(--surface-panel-background)">
        <div
          className="relative h-full"
          style={{
            transform: `translateX(${-scrollOffset.left}px)`,
            width: columnSizes.total,
          }}
        >
          {virtualColumns.map((column) => (
            <div
              className={cn("absolute top-0 flex h-full items-center justify-center border-r border-(--divider-subtle-color) px-2 tabular-nums", coordinateTypography)}
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
      <div aria-hidden="true" className="relative overflow-hidden border-r border-(--divider-subtle-color) bg-(--surface-panel-background)">
        <div
          className="relative w-full"
          style={{
            height: rowSizes.total,
            transform: `translateY(${-scrollOffset.top}px)`,
          }}
        >
          {virtualRows.map((row) => (
            <div
              className={cn("absolute left-0 flex w-full items-center justify-end border-b border-(--divider-subtle-color) px-2 tabular-nums", coordinateTypography)}
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
        aria-label={sheet.name}
        className={cn("bg-(--surface-paper-background) text-(--surface-paper-foreground)", UI_PREVIEW_VIEWPORT_CLASS_NAME)}
        onScroll={handleScroll}
        ref={scrollRef}
        role="region"
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Named read-only viewport needs native keyboard scrolling.
        tabIndex={0}
      >
        <div
          aria-label={sheet.name}
          aria-colcount={sheet.column_count}
          aria-rowcount={sheet.row_count}
          className="relative"
          role="table"
          style={{ height: rowSizes.total, width: columnSizes.total }}
        >
          {renderedRows.map((row) => (
            <div
              aria-rowindex={row.index + 1}
              className="absolute left-0 top-0 w-full"
              key={row.index}
              role="row"
              style={{ height: row.height, transform: `translateY(${row.start}px)` }}
            >
              {row.cells.map((cell) => (
                <UiTooltip label={cell.cell?.text || undefined} key={cell.columnIndex}><div
                  aria-colindex={cell.columnIndex + 1}
                  aria-colspan={cell.columnSpan > 1 ? cell.columnSpan : undefined}
                  aria-rowspan={cell.rowSpan > 1 ? cell.rowSpan : undefined}
                  className="absolute left-0 top-0 overflow-hidden border-r border-b border-(--surface-paper-border) px-2 py-0.5"

                  role="cell"
                  style={{
                    ...createSpreadsheetCellStyle(sheet.styles, cell.cell?.style),
                    height: cell.height,
                    transform: `translateX(${cell.columnStart}px)`,
                    width: cell.width,
                  }}

                >
                  <span className="min-w-0 shrink-0">{cell.cell?.text || ""}</span>
                </div></UiTooltip>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
