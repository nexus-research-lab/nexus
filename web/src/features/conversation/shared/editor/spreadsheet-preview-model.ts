import type {
  Alignment,
  Border,
  Cell,
  Color,
  Fill,
  Font,
  Workbook,
  Worksheet,
} from "exceljs";

const MIN_SHEET_ROWS = 1;
const MIN_SHEET_COLS = 1;
const EXCEL_COLUMN_WIDTH_TO_PX = 6;
const EXCEL_ROW_HEIGHT_TO_PX = 4 / 3;

const THEME_COLORS = [
  "#ffffff",
  "#000000",
  "#bfbfbf",
  "#323232",
  "#4472c4",
  "#ed7d31",
  "#a5a5a5",
  "#ffc000",
  "#5b9bd5",
  "#71ad47",
];

const INDEXED_COLORS = [
  "#000000",
  "#ffffff",
  "#ff0000",
  "#00ff00",
  "#0000ff",
  "#ffff00",
  "#ff00ff",
  "#00ffff",
  "#000000",
  "#ffffff",
  "#ff0000",
  "#00ff00",
  "#0000ff",
  "#ffff00",
  "#ff00ff",
  "#00ffff",
  "#800000",
  "#008000",
  "#000080",
  "#808000",
  "#800080",
  "#008080",
  "#c0c0c0",
  "#808080",
  "#9999ff",
  "#993366",
  "#ffffcc",
  "#ccffff",
  "#660066",
  "#ff8080",
  "#0066cc",
  "#ccccff",
  "#000080",
  "#ff00ff",
  "#ffff00",
  "#00ffff",
  "#800080",
  "#800000",
  "#008080",
  "#0000ff",
  "#00ccff",
  "#ccffff",
  "#ccffcc",
  "#ffff99",
  "#99ccff",
  "#ff99cc",
  "#cc99ff",
  "#ffcc99",
  "#3366ff",
  "#33cccc",
  "#99cc00",
  "#ffcc00",
  "#ff9900",
  "#ff6600",
  "#666699",
  "#969696",
  "#003366",
  "#339966",
  "#003300",
  "#333300",
  "#993300",
  "#993366",
  "#333399",
  "#333333",
  "#000000",
];

type SpreadsheetPreviewBorderSide = [string, string];

export interface SpreadsheetPreviewCellStyle {
  align?: "left" | "center" | "right";
  bgcolor?: string;
  border?: Partial<Record<"top" | "right" | "bottom" | "left", SpreadsheetPreviewBorderSide>>;
  color?: string;
  font?: {
    bold?: boolean;
    italic?: boolean;
    name?: string;
    size?: number;
  };
  strike?: boolean;
  textwrap?: boolean;
  underline?: boolean;
  valign?: "top" | "middle" | "bottom";
}

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
      const text = getCellText(cell);
      const style = getCellStyle(cell);
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
  const style = getCellStyle(cell);
  const styleIndex = style ? registerStyle(sheet.styles, styleIndexes, style) : undefined;
  rowData.cells[parsed.start_col] = {
    ...(existingCell?.style !== undefined || styleIndex === undefined ? {} : { style: styleIndex }),
    ...existingCell,
    merge: [rowSpan, colSpan],
    text: existingCell?.text ?? getCellText(cell),
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

function getCellText(cell: Cell): string {
  const valueText = formatCellValue(cell.value);
  if (valueText !== "") {
    return valueText;
  }
  return cell.text || "";
}

function formatCellValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? "" : value.toLocaleString();
  }
  if (typeof value === "number" || typeof value === "boolean" || typeof value === "string") {
    return String(value);
  }
  if (typeof value !== "object") {
    return "";
  }

  const record = value as Record<string, unknown>;
  if ("result" in record) {
    return formatCellValue(record.result);
  }
  if ("richText" in record && Array.isArray(record.richText)) {
    return record.richText
      .map((part) => typeof part === "object" && part !== null && "text" in part ? String(part.text) : "")
      .join("");
  }
  if ("text" in record) {
    return String(record.text ?? "");
  }
  if ("error" in record) {
    return typeof record.error === "string" ? record.error : "";
  }
  return "";
}

function getCellStyle(cell: Cell): SpreadsheetPreviewCellStyle | undefined {
  const style: SpreadsheetPreviewCellStyle = {};
  const alignment = getAlignmentStyle(cell.alignment);
  const font = getFontStyle(cell.font);
  const fontColor = getExcelColor(cell.font?.color);
  const fillColor = getFillColor(cell.fill);
  const border = getBorderStyle(cell.border);

  if (alignment.align) {
    style.align = alignment.align;
  }
  if (alignment.valign) {
    style.valign = alignment.valign;
  }
  if (cell.alignment?.wrapText) {
    style.textwrap = true;
  }
  if (font) {
    style.font = font;
  }
  if (fontColor) {
    style.color = fontColor;
  }
  if (fillColor) {
    style.bgcolor = fillColor;
  }
  if (border) {
    style.border = border;
  }
  if (cell.font?.strike) {
    style.strike = true;
  }
  if (cell.font?.underline && cell.font.underline !== "none") {
    style.underline = true;
  }

  return Object.keys(style).length > 0 ? style : undefined;
}

function getAlignmentStyle(alignment?: Partial<Alignment>) {
  const result: Pick<SpreadsheetPreviewCellStyle, "align" | "valign"> = {};
  switch (alignment?.horizontal) {
    case "center":
    case "centerContinuous":
      result.align = "center";
      break;
    case "right":
      result.align = "right";
      break;
    case "left":
    case "fill":
    case "justify":
    case "distributed":
      result.align = "left";
      break;
  }
  switch (alignment?.vertical) {
    case "middle":
      result.valign = "middle";
      break;
    case "bottom":
      result.valign = "bottom";
      break;
    case "top":
    case "distributed":
    case "justify":
      result.valign = "top";
      break;
  }
  return result;
}

function getFontStyle(font?: Partial<Font>): SpreadsheetPreviewCellStyle["font"] | undefined {
  if (!font) {
    return undefined;
  }
  const result: NonNullable<SpreadsheetPreviewCellStyle["font"]> = {};
  if (font.bold) {
    result.bold = true;
  }
  if (font.italic) {
    result.italic = true;
  }
  if (font.name) {
    result.name = font.name;
  }
  if (font.size) {
    result.size = Math.round(font.size / EXCEL_ROW_HEIGHT_TO_PX);
  }
  return Object.keys(result).length > 0 ? result : undefined;
}

function getFillColor(fill?: Fill): string | undefined {
  if (!fill || fill.type !== "pattern") {
    return undefined;
  }
  return getExcelColor(fill.fgColor) ?? getExcelColor(fill.bgColor);
}

function getBorderStyle(border?: Cell["border"]): SpreadsheetPreviewCellStyle["border"] | undefined {
  if (!border) {
    return undefined;
  }
  const result: NonNullable<SpreadsheetPreviewCellStyle["border"]> = {};
  const top = getBorderSide(border.top);
  const right = getBorderSide(border.right);
  const bottom = getBorderSide(border.bottom);
  const left = getBorderSide(border.left);

  if (top) {
    result.top = top;
  }
  if (right) {
    result.right = right;
  }
  if (bottom) {
    result.bottom = bottom;
  }
  if (left) {
    result.left = left;
  }
  return Object.keys(result).length > 0 ? result : undefined;
}

function getBorderSide(border?: Partial<Border>): SpreadsheetPreviewBorderSide | undefined {
  if (!border?.style) {
    return undefined;
  }
  return [border.style, getExcelColor(border.color) ?? "#d1d5db"];
}

function getExcelColor(color?: Partial<Color> | null): string | undefined {
  const runtimeColor = color as (Partial<Color> & { indexed?: number }) | undefined | null;
  if (!runtimeColor) {
    return undefined;
  }
  if (runtimeColor.argb) {
    const hex = runtimeColor.argb.replace(/^#/, "");
    if (/^[a-f\d]{8}$/i.test(hex)) {
      return `#${hex.slice(2)}`;
    }
    if (/^[a-f\d]{6}$/i.test(hex)) {
      return `#${hex}`;
    }
  }
  if (typeof runtimeColor.theme === "number") {
    return THEME_COLORS[runtimeColor.theme];
  }
  if (typeof runtimeColor.indexed === "number") {
    return INDEXED_COLORS[runtimeColor.indexed];
  }
  return undefined;
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
