import type { CellStyle } from "x-data-spreadsheet";

import type {
  SpreadsheetPreviewCellStyle,
  SpreadsheetPreviewSheetData,
  SpreadsheetPreviewWorkbookData,
} from "./spreadsheet-preview-model";

const MIN_SHEET_ROWS = 1;
const MIN_SHEET_COLS = 1;
const SPREADSHEET_ROW_HEADER_WIDTH = 60;
const SPREADSHEET_VIEW_PADDING = 24;

interface XSpreadsheetCellStyle extends Omit<CellStyle, "border" | "font"> {
  border?: SpreadsheetPreviewCellStyle["border"];
  font?: SpreadsheetPreviewCellStyle["font"];
  strike?: boolean;
  underline?: boolean;
}

interface XSpreadsheetCellData {
  merge?: [number, number];
  style?: number;
  text: string;
}

interface XSpreadsheetRowData {
  cells: Record<number, XSpreadsheetCellData>;
  height?: number;
}

type XSpreadsheetRows = Record<number, XSpreadsheetRowData> & { len?: number };
type XSpreadsheetCols = Record<number, { width?: number }> & { len?: number };

export interface XSpreadsheetSheetData {
  cols: XSpreadsheetCols;
  merges: string[];
  name: string;
  rows: XSpreadsheetRows;
  styles: XSpreadsheetCellStyle[];
}

export type XSpreadsheetData = XSpreadsheetSheetData[];

export function spreadsheet_preview_to_x_spreadsheet_data(
  workbook: SpreadsheetPreviewWorkbookData,
): XSpreadsheetData {
  return workbook.sheets.map((sheet) => ({
    cols: convert_columns(sheet),
    merges: sheet.merges.map((merge) => merge.ref),
    name: sheet.name,
    rows: convert_rows(sheet),
    styles: sheet.styles.map((style) => ({ ...style })),
  }));
}

export function estimate_spreadsheet_sheet_content_width(sheet: XSpreadsheetSheetData): number {
  const used_col_indexes = new Set<number>();

  Object.values(sheet.rows).forEach((row) => {
    if (!row || typeof row !== "object" || !("cells" in row)) {
      return;
    }
    Object.keys(row.cells).forEach((key) => {
      const index = Number(key);
      if (Number.isInteger(index) && index >= 0) {
        used_col_indexes.add(index);
      }
    });
  });

  sheet.merges.forEach((merge_range) => {
    const end_col = parse_merge_end_col(merge_range);
    if (end_col !== null) {
      used_col_indexes.add(end_col);
    }
  });

  const max_col_index = Math.max(...used_col_indexes, 0);
  let width = SPREADSHEET_ROW_HEADER_WIDTH + SPREADSHEET_VIEW_PADDING;
  for (let col_index = 0; col_index <= max_col_index; col_index += 1) {
    width += sheet.cols[col_index]?.width ?? 80;
  }
  return width;
}

function convert_columns(sheet: SpreadsheetPreviewSheetData): XSpreadsheetCols {
  const cols: XSpreadsheetCols = {};
  Object.entries(sheet.columns).forEach(([key, column]) => {
    const index = Number(key);
    if (Number.isInteger(index) && index >= 0 && column.width !== undefined) {
      cols[index] = { width: column.width };
    }
  });
  cols.len = Math.max(sheet.column_count, MIN_SHEET_COLS);
  return cols;
}

function convert_rows(sheet: SpreadsheetPreviewSheetData): XSpreadsheetRows {
  const rows: XSpreadsheetRows = {};
  Object.entries(sheet.rows).forEach(([key, row]) => {
    const index = Number(key);
    if (!Number.isInteger(index) || index < 0) {
      return;
    }
    rows[index] = {
      cells: { ...row.cells },
      ...(row.height !== undefined ? { height: row.height } : {}),
    };
  });
  rows.len = Math.max(sheet.row_count, MIN_SHEET_ROWS);
  return rows;
}

function parse_merge_end_col(range: string): number | null {
  const [, end = range] = range.split(":");
  const match = end.replaceAll("$", "").match(/^([A-Z]+)\d+$/i);
  if (!match) {
    return null;
  }

  let index = 0;
  for (const char of match[1].toUpperCase()) {
    index = index * 26 + char.charCodeAt(0) - 64;
  }
  return index - 1;
}
