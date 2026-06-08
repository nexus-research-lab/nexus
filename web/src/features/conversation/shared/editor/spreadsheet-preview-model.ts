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

export type SpreadsheetPreviewBorderSide = [string, string];

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

export interface SpreadsheetPreviewRowData {
  cells: Record<number, SpreadsheetPreviewCellData>;
  height?: number;
}

export interface SpreadsheetPreviewColumnData {
  width?: number;
}

export interface SpreadsheetPreviewRange {
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

export function workbook_to_spreadsheet_preview_data(workbook: Workbook): SpreadsheetPreviewWorkbookData {
  return {
    sheets: workbook.worksheets
      .filter((worksheet) => worksheet.state !== "hidden" && worksheet.state !== "veryHidden")
      .map(worksheet_to_spreadsheet_preview_sheet),
  };
}

function worksheet_to_spreadsheet_preview_sheet(worksheet: Worksheet): SpreadsheetPreviewSheetData {
  const sheet: SpreadsheetPreviewSheetData = {
    column_count: MIN_SHEET_COLS,
    columns: {},
    merges: [],
    name: worksheet.name,
    row_count: MIN_SHEET_ROWS,
    rows: {},
    styles: [],
  };
  const style_indexes = new Map<string, number>();
  let max_row_index = -1;
  let max_col_index = -1;

  const worksheet_columns = Array.isArray(worksheet.columns) ? worksheet.columns : [];
  worksheet_columns.forEach((column, index) => {
    const width = column.hidden
      ? 0.1
      : column.width
        ? Math.round(column.width * EXCEL_COLUMN_WIDTH_TO_PX)
        : undefined;
    if (width !== undefined) {
      sheet.columns[index] = { width };
    }
  });

  worksheet.eachRow({ includeEmpty: false }, (row, row_number) => {
    const row_index = row_number - 1;
    const row_data = ensure_row(sheet.rows, row_index);
    max_row_index = Math.max(max_row_index, row_index);

    if (row.hidden) {
      row_data.height = 0.1;
    } else if (row.height) {
      row_data.height = Math.round(row.height * EXCEL_ROW_HEIGHT_TO_PX);
    }

    row.eachCell({ includeEmpty: false }, (cell, col_number) => {
      if (cell.isMerged && cell.master.address !== cell.address) {
        return;
      }

      const col_index = col_number - 1;
      const text = get_cell_text(cell);
      const style = get_cell_style(cell);
      const style_index = style ? register_style(sheet.styles, style_indexes, style) : undefined;
      row_data.cells[col_index] = {
        text,
        ...(style_index !== undefined ? { style: style_index } : {}),
      };
      max_col_index = Math.max(max_col_index, col_index);
    });
  });

  for (const merge_range of worksheet.model.merges || []) {
    apply_merge_range(sheet, worksheet, merge_range, style_indexes);
    const parsed = parse_spreadsheet_cell_range(merge_range);
    if (parsed) {
      max_row_index = Math.max(max_row_index, parsed.end_row);
      max_col_index = Math.max(max_col_index, parsed.end_col);
    }
  }

  sheet.row_count = Math.max(max_row_index + 1, MIN_SHEET_ROWS);
  sheet.column_count = Math.max(max_col_index + 1, MIN_SHEET_COLS);

  return sheet;
}

function ensure_row(
  rows: SpreadsheetPreviewSheetData["rows"],
  row_index: number,
): SpreadsheetPreviewRowData {
  rows[row_index] ??= { cells: {} };
  return rows[row_index];
}

function apply_merge_range(
  sheet: SpreadsheetPreviewSheetData,
  worksheet: Worksheet,
  merge_range: string,
  style_indexes: Map<string, number>,
) {
  const parsed = parse_spreadsheet_cell_range(merge_range);
  if (!parsed) {
    return;
  }
  const row_span = parsed.end_row - parsed.start_row;
  const col_span = parsed.end_col - parsed.start_col;
  if (row_span <= 0 && col_span <= 0) {
    return;
  }

  sheet.merges.push(parsed);
  const row_data = ensure_row(sheet.rows, parsed.start_row);
  const cell = worksheet.getCell(parsed.start_row + 1, parsed.start_col + 1);
  const existing_cell = row_data.cells[parsed.start_col];
  const style = get_cell_style(cell);
  const style_index = style ? register_style(sheet.styles, style_indexes, style) : undefined;
  row_data.cells[parsed.start_col] = {
    ...(existing_cell?.style !== undefined || style_index === undefined ? {} : { style: style_index }),
    ...existing_cell,
    merge: [row_span, col_span],
    text: existing_cell?.text ?? get_cell_text(cell),
  };
}

function register_style(
  styles: SpreadsheetPreviewCellStyle[],
  style_indexes: Map<string, number>,
  style: SpreadsheetPreviewCellStyle,
): number {
  const key = JSON.stringify(style);
  const existing = style_indexes.get(key);
  if (existing !== undefined) {
    return existing;
  }
  const next_index = styles.length;
  styles.push(style);
  style_indexes.set(key, next_index);
  return next_index;
}

function get_cell_text(cell: Cell): string {
  const value_text = format_cell_value(cell.value);
  if (value_text !== "") {
    return value_text;
  }
  return cell.text || "";
}

function format_cell_value(value: unknown): string {
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
    return format_cell_value(record.result);
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

function get_cell_style(cell: Cell): SpreadsheetPreviewCellStyle | undefined {
  const style: SpreadsheetPreviewCellStyle = {};
  const alignment = get_alignment_style(cell.alignment);
  const font = get_font_style(cell.font);
  const font_color = get_excel_color(cell.font?.color);
  const fill_color = get_fill_color(cell.fill);
  const border = get_border_style(cell.border);

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
  if (font_color) {
    style.color = font_color;
  }
  if (fill_color) {
    style.bgcolor = fill_color;
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

function get_alignment_style(alignment?: Partial<Alignment>) {
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

function get_font_style(font?: Partial<Font>): SpreadsheetPreviewCellStyle["font"] | undefined {
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

function get_fill_color(fill?: Fill): string | undefined {
  if (!fill || fill.type !== "pattern") {
    return undefined;
  }
  return get_excel_color(fill.fgColor) ?? get_excel_color(fill.bgColor);
}

function get_border_style(border?: Cell["border"]): SpreadsheetPreviewCellStyle["border"] | undefined {
  if (!border) {
    return undefined;
  }
  const result: NonNullable<SpreadsheetPreviewCellStyle["border"]> = {};
  const top = get_border_side(border.top);
  const right = get_border_side(border.right);
  const bottom = get_border_side(border.bottom);
  const left = get_border_side(border.left);

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

function get_border_side(border?: Partial<Border>): SpreadsheetPreviewBorderSide | undefined {
  if (!border?.style) {
    return undefined;
  }
  return [border.style, get_excel_color(border.color) ?? "#d1d5db"];
}

function get_excel_color(color?: Partial<Color> | null): string | undefined {
  const runtime_color = color as (Partial<Color> & { indexed?: number }) | undefined | null;
  if (!runtime_color) {
    return undefined;
  }
  if (runtime_color.argb) {
    const hex = runtime_color.argb.replace(/^#/, "");
    if (/^[a-f\d]{8}$/i.test(hex)) {
      return `#${hex.slice(2)}`;
    }
    if (/^[a-f\d]{6}$/i.test(hex)) {
      return `#${hex}`;
    }
  }
  if (typeof runtime_color.theme === "number") {
    return THEME_COLORS[runtime_color.theme];
  }
  if (typeof runtime_color.indexed === "number") {
    return INDEXED_COLORS[runtime_color.indexed];
  }
  return undefined;
}

export function parse_spreadsheet_cell_range(range: string): SpreadsheetPreviewRange | null {
  const [start, end = start] = range.split(":");
  const start_cell = parse_cell_address(start);
  const end_cell = parse_cell_address(end);
  if (!start_cell || !end_cell) {
    return null;
  }
  return {
    end_col: Math.max(start_cell.col, end_cell.col),
    end_row: Math.max(start_cell.row, end_cell.row),
    ref: range,
    start_col: Math.min(start_cell.col, end_cell.col),
    start_row: Math.min(start_cell.row, end_cell.row),
  };
}

function parse_cell_address(address: string): { col: number; row: number } | null {
  const match = address.replaceAll("$", "").match(/^([A-Z]+)(\d+)$/i);
  if (!match) {
    return null;
  }
  return {
    col: column_letters_to_index(match[1]),
    row: Number(match[2]) - 1,
  };
}

function column_letters_to_index(letters: string): number {
  let index = 0;
  for (const char of letters.toUpperCase()) {
    index = index * 26 + char.charCodeAt(0) - 64;
  }
  return index - 1;
}
