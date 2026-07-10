import type { CSSProperties } from "react";
import type { VirtualItem } from "@tanstack/react-virtual";

import type {
  SpreadsheetPreviewCellData,
  SpreadsheetPreviewBorderSide,
  SpreadsheetPreviewSheetData,
} from "./spreadsheet-preview-model";

export const SPREADSHEET_GRID_DIMENSIONS = {
  columnHeaderHeight: 28,
  defaultColumnWidth: 96,
  defaultRowHeight: 26,
  maxColumnWidth: 260,
  minColumnWidth: 48,
  rowHeaderWidth: 48,
} as const;

export interface SpreadsheetSizeTable {
  sizes: number[];
  starts: number[];
  total: number;
}

export interface RenderedSpreadsheetCell {
  cell?: SpreadsheetPreviewCellData;
  columnIndex: number;
  columnStart: number;
  height: number;
  rowIndex: number;
  rowStart: number;
  width: number;
}

export function createRowSizeTable(
  sheet: SpreadsheetPreviewSheetData,
): SpreadsheetSizeTable {
  return createSizeTable(sheet.row_count, (index) => {
    const height = sheet.rows[index]?.height
      ?? SPREADSHEET_GRID_DIMENSIONS.defaultRowHeight;
    return height <= 1
      ? 1
      : Math.max(height, SPREADSHEET_GRID_DIMENSIONS.defaultRowHeight);
  });
}

export function createColumnSizeTable(
  sheet: SpreadsheetPreviewSheetData,
): SpreadsheetSizeTable {
  return createSizeTable(sheet.column_count, (index) => {
    const width = sheet.columns[index]?.width
      ?? SPREADSHEET_GRID_DIMENSIONS.defaultColumnWidth;
    if (width <= 1) {
      return 1;
    }
    return Math.min(
      Math.max(width, SPREADSHEET_GRID_DIMENSIONS.minColumnWidth),
      SPREADSHEET_GRID_DIMENSIONS.maxColumnWidth,
    );
  });
}

function createSizeTable(
  count: number,
  getSize: (index: number) => number,
): SpreadsheetSizeTable {
  const sizes: number[] = [];
  const starts: number[] = [];
  let total = 0;
  for (let index = 0; index < count; index += 1) {
    starts[index] = total;
    sizes[index] = getSize(index);
    total += sizes[index];
  }
  return { sizes, starts, total };
}

export function createRenderedSpreadsheetCells(
  sheet: SpreadsheetPreviewSheetData,
  rowSizes: SpreadsheetSizeTable,
  columnSizes: SpreadsheetSizeTable,
  virtualRows: VirtualItem[],
  virtualColumns: VirtualItem[],
): RenderedSpreadsheetCell[] {
  const rowRange = getVirtualRange(virtualRows);
  const columnRange = getVirtualRange(virtualColumns);
  if (!rowRange || !columnRange) {
    return [];
  }

  const cells = new Map<string, RenderedSpreadsheetCell>();
  for (const row of virtualRows) {
    for (const column of virtualColumns) {
      const merge = findMergeForCell(sheet, row.index, column.index);
      const isCoveredCell = merge && (
        merge.start_row !== row.index || merge.start_col !== column.index
      );
      if (!isCoveredCell) {
        addRenderedCell(
          cells,
          sheet,
          rowSizes,
          columnSizes,
          row.index,
          column.index,
          merge,
        );
      }
    }
  }

  for (const merge of sheet.merges) {
    const isVisible = rangesOverlap(
      rowRange.start,
      rowRange.end,
      merge.start_row,
      merge.end_row,
    ) && rangesOverlap(
      columnRange.start,
      columnRange.end,
      merge.start_col,
      merge.end_col,
    );
    if (isVisible) {
      addRenderedCell(
        cells,
        sheet,
        rowSizes,
        columnSizes,
        merge.start_row,
        merge.start_col,
        merge,
      );
    }
  }
  return Array.from(cells.values());
}

function addRenderedCell(
  cells: Map<string, RenderedSpreadsheetCell>,
  sheet: SpreadsheetPreviewSheetData,
  rowSizes: SpreadsheetSizeTable,
  columnSizes: SpreadsheetSizeTable,
  rowIndex: number,
  columnIndex: number,
  merge = findMergeForCell(sheet, rowIndex, columnIndex),
): void {
  const key = `${rowIndex}:${columnIndex}`;
  if (cells.has(key)) {
    return;
  }
  const endRow = Math.min(
    merge?.end_row ?? rowIndex,
    sheet.row_count - 1,
  );
  const endColumn = Math.min(
    merge?.end_col ?? columnIndex,
    sheet.column_count - 1,
  );
  cells.set(key, {
    cell: sheet.rows[rowIndex]?.cells[columnIndex],
    columnIndex,
    columnStart: columnSizes.starts[columnIndex] ?? 0,
    height: getSizeRange(rowSizes, rowIndex, endRow),
    rowIndex,
    rowStart: rowSizes.starts[rowIndex] ?? 0,
    width: getSizeRange(columnSizes, columnIndex, endColumn),
  });
}

function findMergeForCell(
  sheet: SpreadsheetPreviewSheetData,
  rowIndex: number,
  columnIndex: number,
) {
  return sheet.merges.find((merge) => (
    rowIndex >= merge.start_row &&
    rowIndex <= merge.end_row &&
    columnIndex >= merge.start_col &&
    columnIndex <= merge.end_col
  ));
}

function getSizeRange(
  sizeTable: SpreadsheetSizeTable,
  startIndex: number,
  endIndex: number,
): number {
  const start = sizeTable.starts[startIndex] ?? 0;
  const nextStart = endIndex + 1 < sizeTable.starts.length
    ? sizeTable.starts[endIndex + 1]
    : sizeTable.total;
  return Math.max(1, nextStart - start);
}

function getVirtualRange(items: VirtualItem[]) {
  if (items.length === 0) {
    return null;
  }
  return {
    end: items[items.length - 1].index,
    start: items[0].index,
  };
}

function rangesOverlap(
  firstStart: number,
  firstEnd: number,
  secondStart: number,
  secondEnd: number,
): boolean {
  return firstStart <= secondEnd && secondStart <= firstEnd;
}

export function columnIndexToLabel(index: number): string {
  let value = index + 1;
  let label = "";
  while (value > 0) {
    value -= 1;
    label = String.fromCharCode(65 + (value % 26)) + label;
    value = Math.floor(value / 26);
  }
  return label;
}

export function createSpreadsheetCellStyle(
  sheet: SpreadsheetPreviewSheetData,
  cell?: SpreadsheetPreviewCellData,
): CSSProperties {
  const previewStyle = cell?.style !== undefined
    ? sheet.styles[cell.style]
    : undefined;
  if (!previewStyle) {
    return {};
  }

  const textDecoration = [
    previewStyle.underline ? "underline" : "",
    previewStyle.strike ? "line-through" : "",
  ].filter(Boolean).join(" ") || undefined;
  return {
    backgroundColor: previewStyle.bgcolor,
    borderTop: createBorderCss(previewStyle.border?.top),
    borderRight: createBorderCss(previewStyle.border?.right),
    borderBottom: createBorderCss(previewStyle.border?.bottom),
    borderLeft: createBorderCss(previewStyle.border?.left),
    color: previewStyle.color,
    fontFamily: previewStyle.font?.name,
    fontSize: previewStyle.font?.size
      ? Math.max(10, previewStyle.font.size)
      : undefined,
    fontStyle: previewStyle.font?.italic ? "italic" : undefined,
    fontWeight: previewStyle.font?.bold ? 700 : undefined,
    textAlign: previewStyle.align,
    textDecoration,
    verticalAlign: previewStyle.valign,
    whiteSpace: previewStyle.textwrap ? "pre-wrap" : "nowrap",
  };
}

function createBorderCss(
  border?: SpreadsheetPreviewBorderSide,
): string | undefined {
  if (!border) {
    return undefined;
  }
  const [kind, color] = border;
  const lineStyle = ["dashed", "dotted", "double"].includes(kind)
    ? kind
    : "solid";
  const width = kind === "medium" || kind === "thick" ? 2 : 1;
  return `${width}px ${lineStyle} ${color}`;
}
