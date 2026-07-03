"use client";

import { type CSSProperties, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { type VirtualItem, useVirtualizer } from "@tanstack/react-virtual";
import { Eye, FileSpreadsheet, FileWarning, LoaderCircle } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  type SpreadsheetPreviewCellData,
  type SpreadsheetPreviewCellStyle,
  type SpreadsheetPreviewSheetData,
  type SpreadsheetPreviewWorkbookData,
  workbookToSpreadsheetPreviewData,
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
  agentId: string;
  embedded?: boolean;
  fileName: string;
  isPreviewFocused?: boolean;
  onResizeStart: () => void;
  onTogglePreviewFocus?: () => void;
  path: string;
}

export function SpreadsheetFilePreview({
  agentId: agentId,
  embedded,
  fileName: fileName,
  isPreviewFocused: isPreviewFocused,
  onResizeStart: onResizeStart,
  onTogglePreviewFocus: onTogglePreviewFocus,
  path,
}: SpreadsheetFilePreviewProps) {
  const previewKey = `${agentId}\x1f${path}`;
  const [workbookData, setWorkbookData] = useResettableState<SpreadsheetPreviewWorkbookData | null>(null, previewKey);
  const [activeSheetIndex, setActiveSheetIndex] = useResettableState(0, previewKey);
  const [status, setStatus] = useResettableState<SpreadsheetPreviewStatus>({
    state: "loading",
    message: "加载表格预览中",
  }, previewKey);

  useEffect(() => {
    const abortController = new AbortController();
    let cancelled = false;

    async function loadPreview() {
      try {
        const previewUrl = getWorkspaceFilePreviewUrl(agentId, path);
        const response = await fetch(previewUrl, {
          credentials: "include",
          signal: abortController.signal,
        });
        if (!response.ok) {
          throw new Error(`读取文件失败：HTTP ${response.status}`);
        }

        const contentLength = Number(response.headers.get("content-length") || 0);
        if (contentLength > MAX_XLSX_PREVIEW_BYTES) {
          throw new Error("文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        const buffer = await response.arrayBuffer();
        if (buffer.byteLength > MAX_XLSX_PREVIEW_BYTES) {
          throw new Error("文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }
        if (cancelled) {
          return;
        }

        setStatus({ state: "loading", message: "解析 workbook 中" });
        const ExcelJS = await import("exceljs");
        if (cancelled) {
          return;
        }

        const workbook = new ExcelJS.Workbook();
        await workbook.xlsx.load(buffer);
        const workbookPreview = workbookToSpreadsheetPreviewData(workbook);
        if (workbookPreview.sheets.length === 0) {
          throw new Error("未找到可预览的工作表");
        }
        if (cancelled) {
          return;
        }

        setWorkbookData(workbookPreview);
        setStatus({
          state: "loaded",
          sheet_count: workbookPreview.sheets.length,
        });
      } catch (error) {
        if (cancelled || abortController.signal.aborted) {
          return;
        }
        setWorkbookData(null);
        setStatus({
          state: "error",
          message: error instanceof Error ? error.message : "xlsx 预览失败",
        });
      }
    }

    void loadPreview();

    return () => {
      cancelled = true;
      abortController.abort();
    };
  }, [agentId, path]);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          ariaLabel="调整编辑器宽度"
          className="flex"
          onMouseDown={onResizeStart}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        embedded={embedded}
        meta={<SpreadsheetPreviewMeta status={status} />}
        title={fileName}
      />

      <div className="relative min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {workbookData ? (
          <SpreadsheetReadonlyWorkbook
            activeSheetIndex={activeSheetIndex}
            onSelectSheet={setActiveSheetIndex}
            workbook={workbookData}
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
  activeSheetIndex: activeSheetIndex,
  onSelectSheet: onSelectSheet,
  workbook,
}: {
  activeSheetIndex: number;
  onSelectSheet: (index: number) => void;
  workbook: SpreadsheetPreviewWorkbookData;
}) {
  const activeSheet = workbook.sheets[Math.min(activeSheetIndex, workbook.sheets.length - 1)];
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
                index === activeSheetIndex
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
        key={`${activeSheet.name}-${activeSheetIndex}`}
        sheet={activeSheet}
      />
    </div>
  );
}

function SpreadsheetReadonlySheet({ sheet }: { sheet: SpreadsheetPreviewSheetData }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [scrollOffset, setScrollOffset] = useState({ left: 0, top: 0 });
  const rowSizes = useMemo(
    () => makeSizeTable(sheet.row_count, (index) => getRowHeight(sheet, index)),
    [sheet],
  );
  const columnSizes = useMemo(
    () => makeSizeTable(sheet.column_count, (index) => getColumnWidth(sheet, index)),
    [sheet],
  );
  const rowVirtualizer = useVirtualizer({
    count: sheet.row_count,
    estimateSize: (index) => rowSizes.sizes[index] ?? DEFAULT_ROW_HEIGHT,
    getScrollElement: () => scrollRef.current,
    overscan: 8,
  });
  const columnVirtualizer = useVirtualizer({
    count: sheet.column_count,
    estimateSize: (index) => columnSizes.sizes[index] ?? DEFAULT_COLUMN_WIDTH,
    getScrollElement: () => scrollRef.current,
    horizontal: true,
    overscan: 3,
  });
  const virtualRows = rowVirtualizer.getVirtualItems();
  const virtualColumns = columnVirtualizer.getVirtualItems();
  const renderedCells = makeRenderedCells(sheet, rowSizes, columnSizes, virtualRows, virtualColumns);
  const handleScroll = useCallback(() => {
    const element = scrollRef.current;
    if (!element) {
      return;
    }
    setScrollOffset({
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
          style={{
            height: rowSizes.total,
            width: columnSizes.total,
          }}
        >
          {renderedCells.map((cell) => (
            <div
              className="absolute overflow-hidden border-r border-b border-(--divider-subtle-color) px-2 py-1"
              key={`${cell.row_index}:${cell.column_index}`}
              role="gridcell"
              style={{
                ...makeCellStyle(sheet, cell.cell),
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

function makeCellStyle(
  sheet: SpreadsheetPreviewSheetData,
  cell?: SpreadsheetPreviewCellData,
): CSSProperties {
  const previewStyle = cell?.style !== undefined ? sheet.styles[cell.style] : undefined;
  if (!previewStyle) {
    return {};
  }

  const style: CSSProperties = {
    backgroundColor: previewStyle.bgcolor,
    color: previewStyle.color,
    fontFamily: previewStyle.font?.name,
    fontSize: previewStyle.font?.size ? Math.max(10, previewStyle.font.size) : undefined,
    fontStyle: previewStyle.font?.italic ? "italic" : undefined,
    fontWeight: previewStyle.font?.bold ? 700 : undefined,
    textAlign: previewStyle.align,
    textDecoration: [
      previewStyle.underline ? "underline" : "",
      previewStyle.strike ? "line-through" : "",
    ].filter(Boolean).join(" ") || undefined,
    verticalAlign: previewStyle.valign,
    whiteSpace: previewStyle.textwrap ? "pre-wrap" : "nowrap",
  };

  applyBorderStyle(style, previewStyle);
  return style;
}

function applyBorderStyle(style: CSSProperties, previewStyle: SpreadsheetPreviewCellStyle) {
  if (previewStyle.border?.top) {
    style.borderTop = makeBorderCss(previewStyle.border.top);
  }
  if (previewStyle.border?.right) {
    style.borderRight = makeBorderCss(previewStyle.border.right);
  }
  if (previewStyle.border?.bottom) {
    style.borderBottom = makeBorderCss(previewStyle.border.bottom);
  }
  if (previewStyle.border?.left) {
    style.borderLeft = makeBorderCss(previewStyle.border.left);
  }
}

function makeBorderCss([kind, color]: [string, string]) {
  const lineStyle = kind === "dashed" || kind === "dotted" || kind === "double" ? kind : "solid";
  const width = kind === "medium" || kind === "thick" ? 2 : 1;
  return `${width}px ${lineStyle} ${color}`;
}

function getColumnWidth(sheet: SpreadsheetPreviewSheetData, columnIndex: number) {
  const width = sheet.columns[columnIndex]?.width ?? DEFAULT_COLUMN_WIDTH;
  if (width <= 1) {
    return 1;
  }
  return Math.min(Math.max(width, MIN_COLUMN_WIDTH), MAX_COLUMN_WIDTH);
}

function getRowHeight(sheet: SpreadsheetPreviewSheetData, rowIndex: number) {
  const height = sheet.rows[rowIndex]?.height ?? DEFAULT_ROW_HEIGHT;
  return height <= 1 ? 1 : Math.max(height, DEFAULT_ROW_HEIGHT);
}

function makeSizeTable(count: number, getSize: (index: number) => number): SizeTable {
  const sizes: number[] = [];
  const starts: number[] = [];
  let total = 0;
  for (let index = 0; index < count; index += 1) {
    starts[index] = total;
    const size = getSize(index);
    sizes[index] = size;
    total += size;
  }
  return { sizes, starts, total };
}

function makeRenderedCells(
  sheet: SpreadsheetPreviewSheetData,
  rowSizes: SizeTable,
  columnSizes: SizeTable,
  virtualRows: VirtualItem[],
  virtualColumns: VirtualItem[],
): RenderedCell[] {
  const cells = new Map<string, RenderedCell>();
  const rowRange = virtualRange(virtualRows);
  const columnRange = virtualRange(virtualColumns);
  if (!rowRange || !columnRange) {
    return [];
  }

  for (const row of virtualRows) {
    for (const column of virtualColumns) {
      const merge = findMergeForCell(sheet, row.index, column.index);
      if (merge && (merge.start_row !== row.index || merge.start_col !== column.index)) {
        continue;
      }
      addRenderedCell(cells, sheet, rowSizes, columnSizes, row.index, column.index, merge);
    }
  }

  for (const merge of sheet.merges) {
    if (!rangesOverlap(rowRange.start, rowRange.end, merge.start_row, merge.end_row)) {
      continue;
    }
    if (!rangesOverlap(columnRange.start, columnRange.end, merge.start_col, merge.end_col)) {
      continue;
    }
    addRenderedCell(cells, sheet, rowSizes, columnSizes, merge.start_row, merge.start_col, merge);
  }

  return Array.from(cells.values());
}

function addRenderedCell(
  cells: Map<string, RenderedCell>,
  sheet: SpreadsheetPreviewSheetData,
  rowSizes: SizeTable,
  columnSizes: SizeTable,
  rowIndex: number,
  columnIndex: number,
  merge = findMergeForCell(sheet, rowIndex, columnIndex),
) {
  const key = `${rowIndex}:${columnIndex}`;
  if (cells.has(key)) {
    return;
  }
  const endRow = Math.min(merge?.end_row ?? rowIndex, sheet.row_count - 1);
  const endCol = Math.min(merge?.end_col ?? columnIndex, sheet.column_count - 1);
  cells.set(key, {
    cell: sheet.rows[rowIndex]?.cells[columnIndex],
    column_index: columnIndex,
    column_start: columnSizes.starts[columnIndex] ?? 0,
    height: sizeRange(rowSizes, rowIndex, endRow),
    row_index: rowIndex,
    row_start: rowSizes.starts[rowIndex] ?? 0,
    width: sizeRange(columnSizes, columnIndex, endCol),
  });
}

function findMergeForCell(
  sheet: SpreadsheetPreviewSheetData,
  rowIndex: number,
  columnIndex: number,
) {
  return sheet.merges.find((merge) => (
    rowIndex >= merge.start_row
    && rowIndex <= merge.end_row
    && columnIndex >= merge.start_col
    && columnIndex <= merge.end_col
  ));
}

function sizeRange(sizeTable: SizeTable, startIndex: number, endIndex: number) {
  const start = sizeTable.starts[startIndex] ?? 0;
  const nextStart = endIndex + 1 < sizeTable.starts.length
    ? sizeTable.starts[endIndex + 1]
    : sizeTable.total;
  return Math.max(1, nextStart - start);
}

function virtualRange(items: VirtualItem[]) {
  if (items.length === 0) {
    return null;
  }
  return {
    end: items[items.length - 1].index,
    start: items[0].index,
  };
}

function rangesOverlap(aStart: number, aEnd: number, bStart: number, bEnd: number) {
  return aStart <= bEnd && bStart <= aEnd;
}

function columnIndexToLabel(index: number) {
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
