"use client";

import { useEffect, useRef, useState } from "react";
import { Eye, FileSpreadsheet, FileWarning, LoaderCircle } from "lucide-react";
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
import type { CellStyle, Options as SpreadsheetOptions } from "x-data-spreadsheet";
import "x-data-spreadsheet/dist/xspreadsheet.css";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_XLSX_PREVIEW_BYTES = 15 * 1024 * 1024;
const MIN_SHEET_ROWS = 100;
const MIN_SHEET_COLS = 20;
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

type SpreadsheetPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; sheet_count: number }
  | { state: "error"; message: string };

type XSpreadsheetBorderSide = [string, string];

interface XSpreadsheetCellStyle extends Omit<CellStyle, "border" | "font"> {
  border?: Partial<Record<"top" | "right" | "bottom" | "left", XSpreadsheetBorderSide>>;
  font?: {
    bold?: boolean;
    italic?: boolean;
    name?: string;
    size?: number;
  };
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

interface XSpreadsheetSheetData {
  cols: XSpreadsheetCols;
  merges: string[];
  name: string;
  rows: XSpreadsheetRows;
  styles: XSpreadsheetCellStyle[];
}

type XSpreadsheetData = XSpreadsheetSheetData[];

interface SpreadsheetRuntime {
  loadData: (data: XSpreadsheetData) => SpreadsheetRuntime;
  reload?: () => SpreadsheetRuntime;
}

type SpreadsheetEntrypoint =
  | ((container: HTMLElement, options?: SpreadsheetOptions) => SpreadsheetRuntime)
  | (new (container: HTMLElement, options?: SpreadsheetOptions) => SpreadsheetRuntime);

interface CapturedListener {
  listener: EventListenerOrEventListenerObject;
  options?: AddEventListenerOptions | boolean;
  target: EventTarget;
  type: string;
}

interface SpreadsheetFilePreviewProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
  path: string;
}

export function SpreadsheetFilePreview({
  agent_id,
  embedded,
  file_name,
  is_preview_focused,
  on_resize_start,
  on_toggle_preview_focus,
  path,
}: SpreadsheetFilePreviewProps) {
  const container_ref = useRef<HTMLDivElement>(null);
  const cleanup_ref = useRef<(() => void) | null>(null);
  const [status, set_status] = useState<SpreadsheetPreviewStatus>({
    state: "loading",
    message: "加载表格预览中",
  });

  useEffect(() => {
    const container = container_ref.current;
    const abort_controller = new AbortController();
    let cancelled = false;

    cleanup_ref.current?.();
    cleanup_ref.current = null;
    if (container) {
      container.innerHTML = "";
    }

    async function load_preview() {
      if (!container) {
        return;
      }

      set_status({ state: "loading", message: "读取 xlsx 文件中" });

      try {
        const preview_url = get_workspace_file_preview_url(agent_id, path);
        const response = await fetch(preview_url, {
          credentials: "include",
          signal: abort_controller.signal,
        });
        if (!response.ok) {
          throw new Error(`读取文件失败：HTTP ${response.status}`);
        }

        const content_length = Number(response.headers.get("content-length") || 0);
        if (content_length > MAX_XLSX_PREVIEW_BYTES) {
          throw new Error("文件超过 15MB，当前仅支持下载后查看");
        }

        const buffer = await response.arrayBuffer();
        if (buffer.byteLength > MAX_XLSX_PREVIEW_BYTES) {
          throw new Error("文件超过 15MB，当前仅支持下载后查看");
        }
        if (cancelled) {
          return;
        }

        set_status({ state: "loading", message: "解析 workbook 中" });
        const [ExcelJS, spreadsheet_module] = await Promise.all([
          import("exceljs"),
          import("x-data-spreadsheet/dist/xspreadsheet.js"),
        ]);
        if (cancelled) {
          return;
        }

        const workbook = new ExcelJS.Workbook();
        await workbook.xlsx.load(buffer);
        const spreadsheet_data = workbook_to_x_spreadsheet_data(workbook);
        if (spreadsheet_data.length === 0) {
          throw new Error("未找到可预览的工作表");
        }
        if (cancelled) {
          return;
        }

        set_status({ state: "loading", message: "渲染表格中" });
        const cleanup = mount_spreadsheet(
          container,
          resolve_spreadsheet_entrypoint(spreadsheet_module),
          spreadsheet_data,
        );
        cleanup_ref.current = cleanup;

        if (!cancelled) {
          set_status({ state: "loaded", sheet_count: spreadsheet_data.length });
        }
      } catch (error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        cleanup_ref.current?.();
        cleanup_ref.current = null;
        set_status({
          state: "error",
          message: error instanceof Error ? error.message : "xlsx 预览失败",
        });
      }
    }

    void load_preview();

    return () => {
      cancelled = true;
      abort_controller.abort();
      cleanup_ref.current?.();
      cleanup_ref.current = null;
    };
  }, [agent_id, path]);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={<SpreadsheetPreviewMeta status={status} />}
        title={file_name}
      />

      <div className="relative min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        <div
          ref={container_ref}
          className={cn(
            "h-full w-full overflow-hidden",
            status.state === "error" && "opacity-0",
          )}
        />
        {status.state !== "loaded" ? (
          <SpreadsheetPreviewOverlay status={status} />
        ) : null}
      </div>
    </>
  );
}

function SpreadsheetPreviewMeta({ status }: { status: SpreadsheetPreviewStatus }) {
  return (
    <>
      <span className="flex items-center gap-1">
        <FileSpreadsheet className="h-3 w-3" />
        xlsx 预览
      </span>
      {status.state === "loaded" ? (
        <span className="flex items-center gap-1 text-emerald-600">
          <Eye className="h-3 w-3" />
          已加载 {status.sheet_count} 个工作表
        </span>
      ) : status.state === "error" ? (
        <span className="flex min-w-0 items-center gap-1 text-destructive">
          <FileWarning className="h-3 w-3 shrink-0" />
          <span className="truncate">{status.message}</span>
        </span>
      ) : (
        <span className="flex min-w-0 items-center gap-1">
          <LoaderCircle className="h-3 w-3 shrink-0 animate-spin" />
          <span className="truncate">{status.message}</span>
        </span>
      )}
    </>
  );
}

function SpreadsheetPreviewOverlay({ status }: { status: Exclude<SpreadsheetPreviewStatus, { state: "loaded" }> }) {
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
      <div className="max-w-xs">
        <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
          {status.state === "error" ? (
            <FileWarning className="h-7 w-7 text-(--icon-muted)" />
          ) : (
            <LoaderCircle className="h-7 w-7 animate-spin text-primary" />
          )}
        </div>
        <p className="text-sm font-medium text-(--text-strong)">
          {status.state === "error" ? "xlsx 预览失败" : "正在准备表格预览"}
        </p>
        <p className="mt-2 text-xs leading-5 text-(--text-soft)">
          {status.message}
        </p>
      </div>
    </div>
  );
}

function mount_spreadsheet(
  container: HTMLElement,
  spreadsheet_entrypoint: SpreadsheetEntrypoint,
  data: XSpreadsheetData,
): () => void {
  const event_scope = capture_event_listeners([window, document, document.body]);
  let spreadsheet: SpreadsheetRuntime | null = null;
  const options: SpreadsheetOptions = {
    mode: "read",
    showContextmenu: false,
    showToolbar: false,
    view: {
      height: () => Math.max(container.clientHeight, 300),
      width: () => Math.max(container.clientWidth, 320),
    },
    row: {
      height: 24,
      len: MIN_SHEET_ROWS,
    },
    col: {
      indexWidth: 60,
      len: MIN_SHEET_COLS,
      minWidth: 60,
      width: 80,
    },
  };

  try {
    spreadsheet = create_spreadsheet_runtime(spreadsheet_entrypoint, container, options)
      .loadData(data);
  } finally {
    event_scope.restore();
  }

  const resize_observer = new ResizeObserver(() => {
    spreadsheet?.reload?.();
  });
  resize_observer.observe(container);

  return () => {
    resize_observer.disconnect();
    event_scope.cleanup();
    container.innerHTML = "";
    spreadsheet = null;
  };
}

function resolve_spreadsheet_entrypoint(module_value: unknown): SpreadsheetEntrypoint {
  const candidates = [
    read_record_property(module_value, "default"),
    read_record_property(read_record_property(module_value, "default"), "default"),
    module_value,
    typeof window !== "undefined" ? window.x_spreadsheet : undefined,
  ];

  for (const candidate of candidates) {
    if (typeof candidate === "function") {
      return candidate as SpreadsheetEntrypoint;
    }
  }

  throw new Error("x-data-spreadsheet 初始化入口不可用");
}

function create_spreadsheet_runtime(
  entrypoint: SpreadsheetEntrypoint,
  container: HTMLElement,
  options: SpreadsheetOptions,
): SpreadsheetRuntime {
  try {
    return new (entrypoint as new (
      container: HTMLElement,
      options?: SpreadsheetOptions,
    ) => SpreadsheetRuntime)(container, options);
  } catch (error) {
    const message = error instanceof Error ? error.message : "";
    if (!(error instanceof TypeError) || !message.toLowerCase().includes("constructor")) {
      throw error;
    }
    return (entrypoint as (
      container: HTMLElement,
      options?: SpreadsheetOptions,
    ) => SpreadsheetRuntime)(container, options);
  }
}

function read_record_property(value: unknown, key: string): unknown {
  if (!value || typeof value !== "object") {
    return undefined;
  }
  return (value as Record<string, unknown>)[key];
}

function capture_event_listeners(targets: EventTarget[]) {
  const captured: CapturedListener[] = [];
  const restores = targets.map((target) => {
    const original_add = target.addEventListener;
    target.addEventListener = function patched_add_event_listener(type, listener, options) {
      if (listener) {
        captured.push({
          listener,
          options,
          target,
          type: String(type),
        });
      }
      return original_add.call(this, type, listener, options);
    };
    return () => {
      target.addEventListener = original_add;
    };
  });

  return {
    cleanup: () => {
      for (const item of captured) {
        item.target.removeEventListener(item.type, item.listener, item.options);
      }
    },
    restore: () => {
      for (const restore of restores) {
        restore();
      }
    },
  };
}

function workbook_to_x_spreadsheet_data(workbook: Workbook): XSpreadsheetData {
  return workbook.worksheets
    .filter((worksheet) => worksheet.state !== "hidden" && worksheet.state !== "veryHidden")
    .map(worksheet_to_x_spreadsheet_sheet);
}

function worksheet_to_x_spreadsheet_sheet(worksheet: Worksheet): XSpreadsheetSheetData {
  const sheet: XSpreadsheetSheetData = {
    cols: {},
    merges: [],
    name: worksheet.name,
    rows: {},
    styles: [],
  };
  const style_indexes = new Map<string, number>();
  let max_row_index = Math.max(worksheet.rowCount - 1, 0);
  let max_col_index = Math.max(worksheet.columnCount - 1, 0);

  worksheet.columns.forEach((column, index) => {
    const width = column.hidden
      ? 0.1
      : column.width
        ? Math.round(column.width * EXCEL_COLUMN_WIDTH_TO_PX)
        : undefined;
    if (width !== undefined) {
      sheet.cols[index] = { width };
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
    const parsed = parse_cell_range(merge_range);
    if (parsed) {
      max_row_index = Math.max(max_row_index, parsed.end_row);
      max_col_index = Math.max(max_col_index, parsed.end_col);
    }
  }

  sheet.rows.len = Math.max(max_row_index + 1, MIN_SHEET_ROWS);
  sheet.cols.len = Math.max(max_col_index + 1, MIN_SHEET_COLS);

  return sheet;
}

function ensure_row(rows: XSpreadsheetRows, row_index: number): XSpreadsheetRowData {
  rows[row_index] ??= { cells: {} };
  return rows[row_index];
}

function apply_merge_range(
  sheet: XSpreadsheetSheetData,
  worksheet: Worksheet,
  merge_range: string,
  style_indexes: Map<string, number>,
) {
  const parsed = parse_cell_range(merge_range);
  if (!parsed) {
    return;
  }
  const row_span = parsed.end_row - parsed.start_row;
  const col_span = parsed.end_col - parsed.start_col;
  if (row_span <= 0 && col_span <= 0) {
    return;
  }

  sheet.merges.push(merge_range);
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
  styles: XSpreadsheetCellStyle[],
  style_indexes: Map<string, number>,
  style: XSpreadsheetCellStyle,
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

function get_cell_style(cell: Cell): XSpreadsheetCellStyle | undefined {
  const style: XSpreadsheetCellStyle = {};
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
  const result: Pick<XSpreadsheetCellStyle, "align" | "valign"> = {};
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

function get_font_style(font?: Partial<Font>): XSpreadsheetCellStyle["font"] | undefined {
  if (!font) {
    return undefined;
  }
  const result: NonNullable<XSpreadsheetCellStyle["font"]> = {};
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

function get_border_style(border?: Cell["border"]): XSpreadsheetCellStyle["border"] | undefined {
  if (!border) {
    return undefined;
  }
  const result: NonNullable<XSpreadsheetCellStyle["border"]> = {};
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

function get_border_side(border?: Partial<Border>): XSpreadsheetBorderSide | undefined {
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

function parse_cell_range(range: string) {
  const [start, end = start] = range.split(":");
  const start_cell = parse_cell_address(start);
  const end_cell = parse_cell_address(end);
  if (!start_cell || !end_cell) {
    return null;
  }
  return {
    end_col: Math.max(start_cell.col, end_cell.col),
    end_row: Math.max(start_cell.row, end_cell.row),
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
