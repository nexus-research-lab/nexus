"use client";

import { type CSSProperties, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { type VirtualItem, useVirtualizer } from "@tanstack/react-virtual";
import { Eye, FileSpreadsheet, FileWarning, LoaderCircle } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  type SpreadsheetPreviewCellData,
  type SpreadsheetPreviewCellStyle,
  type SpreadsheetPreviewSheetData,
  type SpreadsheetPreviewWorkbookData,
  workbook_to_spreadsheet_preview_data,
} from "./spreadsheet-preview-model";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_XLSX_PREVIEW_BYTES = 15 * 1024 * 1024;
const HEADER_ROW_HEIGHT = 28;
const ROW_HEADER_WIDTH = 48;
const DEFAULT_COLUMN_WIDTH = 96;
const MIN_COLUMN_WIDTH = 48;
const MAX_COLUMN_WIDTH = 260;
const DEFAULT_ROW_HEIGHT = 26;

interface SizeTable {
  sizes: number[];
  starts: number[];
  total: number;
}

interface RenderedCell {
  cell?: SpreadsheetPreviewCellData;
  column_index: number;
  column_start: number;
  height: number;
  row_index: number;
  row_start: number;
  width: number;
}

type SpreadsheetPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; sheet_count: number }
  | { state: "error"; message: string };

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
  const preview_key = `${agent_id}\x1f${path}`;
  const [workbook_data, set_workbook_data] = useResettableState<SpreadsheetPreviewWorkbookData | null>(null, preview_key);
  const [active_sheet_index, set_active_sheet_index] = useResettableState(0, preview_key);
  const [status, set_status] = useResettableState<SpreadsheetPreviewStatus>({
    state: "loading",
    message: "加载表格预览中",
  }, preview_key);

  useEffect(() => {
    const abort_controller = new AbortController();
    let cancelled = false;

    async function load_preview() {
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
          throw new Error("文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        const buffer = await response.arrayBuffer();
        if (buffer.byteLength > MAX_XLSX_PREVIEW_BYTES) {
          throw new Error("文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }
        if (cancelled) {
          return;
        }

        set_status({ state: "loading", message: "解析 workbook 中" });
        const ExcelJS = await import("exceljs");
        if (cancelled) {
          return;
        }

        const workbook = new ExcelJS.Workbook();
        await workbook.xlsx.load(buffer);
        const workbook_preview = workbook_to_spreadsheet_preview_data(workbook);
        if (workbook_preview.sheets.length === 0) {
          throw new Error("未找到可预览的工作表");
        }
        if (cancelled) {
          return;
        }

        set_workbook_data(workbook_preview);
        set_status({
          state: "loaded",
          sheet_count: workbook_preview.sheets.length,
        });
      } catch (error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        set_workbook_data(null);
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
        {workbook_data ? (
          <SpreadsheetReadonlyWorkbook
            active_sheet_index={active_sheet_index}
            on_select_sheet={set_active_sheet_index}
            workbook={workbook_data}
          />
        ) : null}
        {status.state !== "loaded" ? (
          <SpreadsheetPreviewOverlay status={status} />
        ) : null}
      </div>
    </>
  );
}

function SpreadsheetReadonlyWorkbook({
  active_sheet_index,
  on_select_sheet,
  workbook,
}: {
  active_sheet_index: number;
  on_select_sheet: (index: number) => void;
  workbook: SpreadsheetPreviewWorkbookData;
}) {
  const active_sheet = workbook.sheets[Math.min(active_sheet_index, workbook.sheets.length - 1)];
  if (!active_sheet) {
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
                index === active_sheet_index
                  ? "bg-primary text-primary-foreground"
                  : "text-(--text-muted) hover:bg-(--button-ghost-hover-background) hover:text-(--text-strong)",
              )}
              key={`${sheet.name}-${index}`}
              onClick={() => on_select_sheet(index)}
              title={sheet.name}
              type="button"
            >
              {sheet.name}
            </button>
          ))}
        </div>
      ) : null}
      <SpreadsheetReadonlySheet
        key={`${active_sheet.name}-${active_sheet_index}`}
        sheet={active_sheet}
      />
    </div>
  );
}

function SpreadsheetReadonlySheet({ sheet }: { sheet: SpreadsheetPreviewSheetData }) {
  const scroll_ref = useRef<HTMLDivElement>(null);
  const [scroll_offset, set_scroll_offset] = useState({ left: 0, top: 0 });
  const row_sizes = useMemo(
    () => make_size_table(sheet.row_count, (index) => get_row_height(sheet, index)),
    [sheet],
  );
  const column_sizes = useMemo(
    () => make_size_table(sheet.column_count, (index) => get_column_width(sheet, index)),
    [sheet],
  );
  const row_virtualizer = useVirtualizer({
    count: sheet.row_count,
    estimateSize: (index) => row_sizes.sizes[index] ?? DEFAULT_ROW_HEIGHT,
    getScrollElement: () => scroll_ref.current,
    overscan: 8,
  });
  const column_virtualizer = useVirtualizer({
    count: sheet.column_count,
    estimateSize: (index) => column_sizes.sizes[index] ?? DEFAULT_COLUMN_WIDTH,
    getScrollElement: () => scroll_ref.current,
    horizontal: true,
    overscan: 3,
  });
  const virtual_rows = row_virtualizer.getVirtualItems();
  const virtual_columns = column_virtualizer.getVirtualItems();
  const rendered_cells = make_rendered_cells(sheet, row_sizes, column_sizes, virtual_rows, virtual_columns);
  const handle_scroll = useCallback(() => {
    const element = scroll_ref.current;
    if (!element) {
      return;
    }
    set_scroll_offset({
      left: element.scrollLeft,
      top: element.scrollTop,
    });
  }, []);

  return (
    <div
      className="grid min-h-0 flex-1 bg-[var(--surface-panel-subtle-background)] text-xs text-(--text-default)"
      style={{
        gridTemplateColumns: `${ROW_HEADER_WIDTH}px minmax(0, 1fr)`,
        gridTemplateRows: `${HEADER_ROW_HEIGHT}px minmax(0, 1fr)`,
      }}
    >
      <div className="z-30 border-r border-b border-(--divider-subtle-color) bg-(--surface-panel-background)" />
      <div className="relative overflow-hidden border-b border-(--divider-subtle-color) bg-(--surface-panel-background)">
        <div
          className="relative h-full"
          style={{
            transform: `translateX(${-scroll_offset.left}px)`,
            width: column_sizes.total,
          }}
        >
          {virtual_columns.map((column) => (
            <div
              className="absolute top-0 flex h-full items-center justify-center border-r border-(--divider-subtle-color) px-2 text-[10px] font-semibold text-(--text-muted)"
              key={column.key}
              style={{
                transform: `translateX(${column.start}px)`,
                width: column.size,
              }}
            >
              {column_index_to_label(column.index)}
            </div>
          ))}
        </div>
      </div>
      <div className="relative overflow-hidden border-r border-(--divider-subtle-color) bg-(--surface-panel-background)">
        <div
          className="relative w-full"
          style={{
            height: row_sizes.total,
            transform: `translateY(${-scroll_offset.top}px)`,
          }}
        >
          {virtual_rows.map((row) => (
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
        onScroll={handle_scroll}
        ref={scroll_ref}
      >
        <div
          className="relative"
          role="grid"
          style={{
            height: row_sizes.total,
            width: column_sizes.total,
          }}
        >
          {rendered_cells.map((cell) => (
            <div
              className="absolute overflow-hidden border-r border-b border-(--divider-subtle-color) px-2 py-1"
              key={`${cell.row_index}:${cell.column_index}`}
              role="gridcell"
              style={{
                ...make_cell_style(sheet, cell.cell),
                height: cell.height,
                transform: `translate(${cell.column_start}px, ${cell.row_start}px)`,
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

function SpreadsheetPreviewMeta({ status }: { status: SpreadsheetPreviewStatus }) {
  return (
    <>
      <span className="flex items-center gap-1">
        <FileSpreadsheet className="h-3 w-3" />
        xlsx 预览
      </span>
      {status.state === "loaded" ? (
        <span className="flex items-center gap-1 text-(--success)">
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

function make_cell_style(
  sheet: SpreadsheetPreviewSheetData,
  cell?: SpreadsheetPreviewCellData,
): CSSProperties {
  const preview_style = cell?.style !== undefined ? sheet.styles[cell.style] : undefined;
  if (!preview_style) {
    return {};
  }

  const style: CSSProperties = {
    backgroundColor: preview_style.bgcolor,
    color: preview_style.color,
    fontFamily: preview_style.font?.name,
    fontSize: preview_style.font?.size ? Math.max(10, preview_style.font.size) : undefined,
    fontStyle: preview_style.font?.italic ? "italic" : undefined,
    fontWeight: preview_style.font?.bold ? 700 : undefined,
    textAlign: preview_style.align,
    textDecoration: [
      preview_style.underline ? "underline" : "",
      preview_style.strike ? "line-through" : "",
    ].filter(Boolean).join(" ") || undefined,
    verticalAlign: preview_style.valign,
    whiteSpace: preview_style.textwrap ? "pre-wrap" : "nowrap",
  };

  apply_border_style(style, preview_style);
  return style;
}

function apply_border_style(style: CSSProperties, preview_style: SpreadsheetPreviewCellStyle) {
  if (preview_style.border?.top) {
    style.borderTop = make_border_css(preview_style.border.top);
  }
  if (preview_style.border?.right) {
    style.borderRight = make_border_css(preview_style.border.right);
  }
  if (preview_style.border?.bottom) {
    style.borderBottom = make_border_css(preview_style.border.bottom);
  }
  if (preview_style.border?.left) {
    style.borderLeft = make_border_css(preview_style.border.left);
  }
}

function make_border_css([kind, color]: [string, string]) {
  const line_style = kind === "dashed" || kind === "dotted" || kind === "double" ? kind : "solid";
  const width = kind === "medium" || kind === "thick" ? 2 : 1;
  return `${width}px ${line_style} ${color}`;
}

function get_column_width(sheet: SpreadsheetPreviewSheetData, column_index: number) {
  const width = sheet.columns[column_index]?.width ?? DEFAULT_COLUMN_WIDTH;
  if (width <= 1) {
    return 1;
  }
  return Math.min(Math.max(width, MIN_COLUMN_WIDTH), MAX_COLUMN_WIDTH);
}

function get_row_height(sheet: SpreadsheetPreviewSheetData, row_index: number) {
  const height = sheet.rows[row_index]?.height ?? DEFAULT_ROW_HEIGHT;
  return height <= 1 ? 1 : Math.max(height, DEFAULT_ROW_HEIGHT);
}

function make_size_table(count: number, get_size: (index: number) => number): SizeTable {
  const sizes: number[] = [];
  const starts: number[] = [];
  let total = 0;
  for (let index = 0; index < count; index += 1) {
    starts[index] = total;
    const size = get_size(index);
    sizes[index] = size;
    total += size;
  }
  return { sizes, starts, total };
}

function make_rendered_cells(
  sheet: SpreadsheetPreviewSheetData,
  row_sizes: SizeTable,
  column_sizes: SizeTable,
  virtual_rows: VirtualItem[],
  virtual_columns: VirtualItem[],
): RenderedCell[] {
  const cells = new Map<string, RenderedCell>();
  const row_range = virtual_range(virtual_rows);
  const column_range = virtual_range(virtual_columns);
  if (!row_range || !column_range) {
    return [];
  }

  for (const row of virtual_rows) {
    for (const column of virtual_columns) {
      const merge = find_merge_for_cell(sheet, row.index, column.index);
      if (merge && (merge.start_row !== row.index || merge.start_col !== column.index)) {
        continue;
      }
      add_rendered_cell(cells, sheet, row_sizes, column_sizes, row.index, column.index, merge);
    }
  }

  for (const merge of sheet.merges) {
    if (!ranges_overlap(row_range.start, row_range.end, merge.start_row, merge.end_row)) {
      continue;
    }
    if (!ranges_overlap(column_range.start, column_range.end, merge.start_col, merge.end_col)) {
      continue;
    }
    add_rendered_cell(cells, sheet, row_sizes, column_sizes, merge.start_row, merge.start_col, merge);
  }

  return Array.from(cells.values());
}

function add_rendered_cell(
  cells: Map<string, RenderedCell>,
  sheet: SpreadsheetPreviewSheetData,
  row_sizes: SizeTable,
  column_sizes: SizeTable,
  row_index: number,
  column_index: number,
  merge = find_merge_for_cell(sheet, row_index, column_index),
) {
  const key = `${row_index}:${column_index}`;
  if (cells.has(key)) {
    return;
  }
  const end_row = Math.min(merge?.end_row ?? row_index, sheet.row_count - 1);
  const end_col = Math.min(merge?.end_col ?? column_index, sheet.column_count - 1);
  cells.set(key, {
    cell: sheet.rows[row_index]?.cells[column_index],
    column_index,
    column_start: column_sizes.starts[column_index] ?? 0,
    height: size_range(row_sizes, row_index, end_row),
    row_index,
    row_start: row_sizes.starts[row_index] ?? 0,
    width: size_range(column_sizes, column_index, end_col),
  });
}

function find_merge_for_cell(
  sheet: SpreadsheetPreviewSheetData,
  row_index: number,
  column_index: number,
) {
  return sheet.merges.find((merge) => (
    row_index >= merge.start_row
    && row_index <= merge.end_row
    && column_index >= merge.start_col
    && column_index <= merge.end_col
  ));
}

function size_range(size_table: SizeTable, start_index: number, end_index: number) {
  const start = size_table.starts[start_index] ?? 0;
  const next_start = end_index + 1 < size_table.starts.length
    ? size_table.starts[end_index + 1]
    : size_table.total;
  return Math.max(1, next_start - start);
}

function virtual_range(items: VirtualItem[]) {
  if (items.length === 0) {
    return null;
  }
  return {
    end: items[items.length - 1].index,
    start: items[0].index,
  };
}

function ranges_overlap(a_start: number, a_end: number, b_start: number, b_end: number) {
  return a_start <= b_end && b_start <= a_end;
}

function column_index_to_label(index: number) {
  let value = index + 1;
  let label = "";
  while (value > 0) {
    value -= 1;
    label = String.fromCharCode(65 + (value % 26)) + label;
    value = Math.floor(value / 26);
  }
  return label;
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
