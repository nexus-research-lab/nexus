import type { Workbook, Worksheet } from "exceljs";

import {
  getSpreadsheetCellStyle,
  type SpreadsheetPreviewCellStyle,
} from "./spreadsheet-cell-style";
import { getSpreadsheetCellText } from "./spreadsheet-cell-value";

export type {
  SpreadsheetPreviewBorderSide,
} from "./spreadsheet-cell-style";

const MIN_SHEET_ROWS = 1;
const MIN_SHEET_COLS = 1;
const EXCEL_COLUMN_WIDTH_TO_PX = 6;
const EXCEL_ROW_HEIGHT_TO_PX = 4 / 3;

export interface SpreadsheetPreviewCellData {
  merge?: [number, number];
  style?: number;
  text: string;
}

interface SpreadsheetPreviewRowData {
  cells: Record<number, SpreadsheetPreviewCellData>;
  height?: number;
}

interface SpreadsheetPreviewColumnData {
  width?: number;
}

interface SpreadsheetPreviewRange {
  end_col: number;
  end_row: number;
  ref: string;
  start_col: number;
  start_row: number;
}

export interface SpreadsheetPreviewSheetData {
  column_count: number;
  columns: Record<number, SpreadsheetPreviewColumnData>;
  merges: SpreadsheetPreviewRange[];
  name: string;
  row_count: number;
  rows: Record<number, SpreadsheetPreviewRowData>;
  styles: SpreadsheetPreviewCellStyle[];
}

export interface SpreadsheetPreviewWorkbookData {
  sheets: SpreadsheetPreviewSheetData[];
}

export function workbookToSpreadsheetPreviewData(workbook: Workbook): SpreadsheetPreviewWorkbookData {
  return {
    sheets: workbook.worksheets
      .filter((worksheet) => worksheet.state !== "hidden" && worksheet.state !== "veryHidden")
      .map(worksheetToSpreadsheetPreviewSheet),
  };
}

function worksheetToSpreadsheetPreviewSheet(worksheet: Worksheet): SpreadsheetPreviewSheetData {
  const sheet: SpreadsheetPreviewSheetData = {
    column_count: MIN_SHEET_COLS,
    columns: {},
    merges: [],
    name: worksheet.name,
    row_count: MIN_SHEET_ROWS,
    rows: {},
    styles: [],
  };
  const styleIndexes = new Map<string, number>();
  let maxRowIndex = -1;
  let maxColIndex = -1;

  const worksheetColumns = Array.isArray(worksheet.columns) ? worksheet.columns : [];
  worksheetColumns.forEach((column, index) => {
    const width = column.hidden
      ? 0.1
      : column.width
        ? Math.round(column.width * EXCEL_COLUMN_WIDTH_TO_PX)
        : undefined;
    if (width !== undefined) {
      sheet.columns[index] = { width };
    }
  });

  worksheet.eachRow({ includeEmpty: false }, (row, rowNumber) => {
    const rowIndex = rowNumber - 1;
    const rowData = ensureRow(sheet.rows, rowIndex);
    maxRowIndex = Math.max(maxRowIndex, rowIndex);

    if (row.hidden) {
      rowData.height = 0.1;
    } else if (row.height) {
      rowData.height = Math.round(row.height * EXCEL_ROW_HEIGHT_TO_PX);
    }

    row.eachCell({ includeEmpty: false }, (cell, colNumber) => {
      if (cell.isMerged && cell.master.address !== cell.address) {
        return;
      }

      const colIndex = colNumber - 1;
      const text = getSpreadsheetCellText(cell);
      const style = getSpreadsheetCellStyle(cell);
      const styleIndex = style ? registerStyle(sheet.styles, styleIndexes, style) : undefined;
      rowData.cells[colIndex] = {
        text,
        ...(styleIndex !== undefined ? { style: styleIndex } : {}),
      };
      maxColIndex = Math.max(maxColIndex, colIndex);
    });
  });

  for (const mergeRange of worksheet.model.merges || []) {
    applyMergeRange(sheet, worksheet, mergeRange, styleIndexes);
    const parsed = parseSpreadsheetCellRange(mergeRange);
    if (parsed) {
      maxRowIndex = Math.max(maxRowIndex, parsed.end_row);
      maxColIndex = Math.max(maxColIndex, parsed.end_col);
    }
  }

  sheet.row_count = Math.max(maxRowIndex + 1, MIN_SHEET_ROWS);
  sheet.column_count = Math.max(maxColIndex + 1, MIN_SHEET_COLS);

  return sheet;
}

function ensureRow(
  rows: SpreadsheetPreviewSheetData["rows"],
  rowIndex: number,
): SpreadsheetPreviewRowData {
  rows[rowIndex] ??= { cells: {} };
  return rows[rowIndex];
}

function applyMergeRange(
  sheet: SpreadsheetPreviewSheetData,
  worksheet: Worksheet,
  mergeRange: string,
  styleIndexes: Map<string, number>,
) {
  const parsed = parseSpreadsheetCellRange(mergeRange);
  if (!parsed) {
    return;
  }
  const rowSpan = parsed.end_row - parsed.start_row;
  const colSpan = parsed.end_col - parsed.start_col;
  if (rowSpan <= 0 && colSpan <= 0) {
    return;
  }

  sheet.merges.push(parsed);
  const rowData = ensureRow(sheet.rows, parsed.start_row);
  const cell = worksheet.getCell(parsed.start_row + 1, parsed.start_col + 1);
  const existingCell = rowData.cells[parsed.start_col];
  const style = getSpreadsheetCellStyle(cell);
  const styleIndex = style ? registerStyle(sheet.styles, styleIndexes, style) : undefined;
  rowData.cells[parsed.start_col] = {
    ...(existingCell?.style !== undefined || styleIndex === undefined ? {} : { style: styleIndex }),
    ...existingCell,
    merge: [rowSpan, colSpan],
    text: existingCell?.text ?? getSpreadsheetCellText(cell),
  };
}

function registerStyle(
  styles: SpreadsheetPreviewCellStyle[],
  styleIndexes: Map<string, number>,
  style: SpreadsheetPreviewCellStyle,
): number {
  const key = JSON.stringify(style);
  const existing = styleIndexes.get(key);
  if (existing !== undefined) {
    return existing;
  }
  const nextIndex = styles.length;
  styles.push(style);
  styleIndexes.set(key, nextIndex);
  return nextIndex;
}

function parseSpreadsheetCellRange(range: string): SpreadsheetPreviewRange | null {
  const [start, end = start] = range.split(":");
  const startCell = parseCellAddress(start);
  const endCell = parseCellAddress(end);
  if (!startCell || !endCell) {
    return null;
  }
  return {
    end_col: Math.max(startCell.col, endCell.col),
    end_row: Math.max(startCell.row, endCell.row),
    ref: range,
    start_col: Math.min(startCell.col, endCell.col),
    start_row: Math.min(startCell.row, endCell.row),
  };
}

function parseCellAddress(address: string): { col: number; row: number } | null {
  const match = address.replaceAll("$", "").match(/^([A-Z]+)(\d+)$/i);
  if (!match) {
    return null;
  }
  return {
    col: columnLettersToIndex(match[1]),
    row: Number(match[2]) - 1,
  };
}

function columnLettersToIndex(letters: string): number {
  let index = 0;
  for (const char of letters.toUpperCase()) {
    index = index * 26 + char.charCodeAt(0) - 64;
  }
  return index - 1;
}
