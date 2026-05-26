"use client";

import { useEffect, useRef, useState } from "react";
import { Eye, FileSpreadsheet, FileWarning, LoaderCircle } from "lucide-react";
import type { Options as SpreadsheetOptions } from "x-data-spreadsheet";
import "x-data-spreadsheet/dist/xspreadsheet.css";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import { workbook_to_spreadsheet_preview_data } from "./spreadsheet-preview-model";
import {
  estimate_spreadsheet_sheet_content_width,
  spreadsheet_preview_to_x_spreadsheet_data,
  type XSpreadsheetData,
} from "./spreadsheet-x-data-adapter";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_XLSX_PREVIEW_BYTES = 15 * 1024 * 1024;
// 只读预览不补大面积空白网格，避免内容右侧/底部空白区域继续响应选区操作。
const MIN_SHEET_ROWS = 1;
const MIN_SHEET_COLS = 1;

type SpreadsheetPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; sheet_count: number }
  | { state: "error"; message: string };

interface SpreadsheetRuntime {
  loadData: (data: XSpreadsheetData) => SpreadsheetRuntime;
  reRender?: () => SpreadsheetRuntime;
  sheet?: {
    reload?: () => unknown;
  };
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

interface MountedSpreadsheet {
  cleanup: () => void;
  refresh_layout: () => void;
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
  const refresh_layout_ref = useRef<(() => void) | null>(null);
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
    refresh_layout_ref.current = null;
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
        const workbook_preview = workbook_to_spreadsheet_preview_data(workbook);
        if (workbook_preview.sheets.length === 0) {
          throw new Error("未找到可预览的工作表");
        }
        const spreadsheet_data = spreadsheet_preview_to_x_spreadsheet_data(workbook_preview);
        if (cancelled) {
          return;
        }

        set_status({ state: "loading", message: "渲染表格中" });
        const mounted_spreadsheet = mount_spreadsheet(
          container,
          resolve_spreadsheet_entrypoint(spreadsheet_module),
          spreadsheet_data,
        );
        cleanup_ref.current = mounted_spreadsheet.cleanup;
        refresh_layout_ref.current = mounted_spreadsheet.refresh_layout;

        if (!cancelled) {
          set_status({ state: "loaded", sheet_count: spreadsheet_data.length });
        }
      } catch (error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        cleanup_ref.current?.();
        cleanup_ref.current = null;
        refresh_layout_ref.current = null;
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
      refresh_layout_ref.current = null;
    };
  }, [agent_id, path]);

  useEffect(() => {
    let first_frame = 0;
    let second_frame = 0;
    let settled_timeout = 0;

    const refresh_layout = () => {
      refresh_layout_ref.current?.();
    };

    first_frame = window.requestAnimationFrame(() => {
      refresh_layout();
      second_frame = window.requestAnimationFrame(refresh_layout);
    });
    settled_timeout = window.setTimeout(refresh_layout, 360);

    return () => {
      window.cancelAnimationFrame(first_frame);
      window.cancelAnimationFrame(second_frame);
      window.clearTimeout(settled_timeout);
    };
  }, [is_preview_focused]);

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
            "h-full w-full overflow-hidden [&_.x-spreadsheet]:inline-block [&_.x-spreadsheet]:max-w-full [&_.x-spreadsheet]:align-top",
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
): MountedSpreadsheet {
  const event_scope = capture_event_listeners([window, document, document.body]);
  let spreadsheet: SpreadsheetRuntime | null = null;
  let active_sheet_index = 0;
  const sheet_view_widths = data.map(estimate_spreadsheet_sheet_content_width);
  const get_view_width = () => get_spreadsheet_view_width(container, sheet_view_widths, active_sheet_index);
  const options: SpreadsheetOptions = {
    mode: "read",
    showContextmenu: false,
    showToolbar: false,
    view: {
      height: () => Math.max(container.clientHeight, 300),
      width: get_view_width,
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

  const refresh_layout = () => {
    refresh_spreadsheet_layout(spreadsheet, container);
  };
  refresh_layout();

  const handle_sheet_tab_click = (event: MouseEvent) => {
    const tab = (event.target as HTMLElement | null)?.closest<HTMLLIElement>(".x-spreadsheet-menu > li");
    if (!tab) {
      return;
    }

    const menu = tab.parentElement;
    const sheet_tabs = Array.from(menu?.querySelectorAll<HTMLLIElement>(":scope > li") ?? []).slice(1);
    const next_index = sheet_tabs.indexOf(tab);
    if (next_index < 0 || next_index >= data.length) {
      return;
    }

    active_sheet_index = next_index;
    requestAnimationFrame(refresh_layout);
  };
  container.addEventListener("click", handle_sheet_tab_click);

  const resize_observer = new ResizeObserver(() => {
    refresh_layout();
  });
  resize_observer.observe(container);

  return {
    cleanup: () => {
      container.removeEventListener("click", handle_sheet_tab_click);
      resize_observer.disconnect();
      event_scope.cleanup();
      container.innerHTML = "";
      spreadsheet = null;
    },
    refresh_layout,
  };
}

function refresh_spreadsheet_layout(spreadsheet: SpreadsheetRuntime | null, container: HTMLElement) {
  spreadsheet?.sheet?.reload?.();
  spreadsheet?.reRender?.();
  sync_spreadsheet_root_width(container);
}

function get_spreadsheet_view_width(
  container: HTMLElement,
  sheet_view_widths: number[],
  active_sheet_index: number,
): number {
  const content_width = sheet_view_widths[Math.min(Math.max(active_sheet_index, 0), sheet_view_widths.length - 1)] ?? 320;
  return Math.max(Math.min(container.clientWidth, content_width), 320);
}

function sync_spreadsheet_root_width(container: HTMLElement) {
  const root = container.querySelector<HTMLElement>(".x-spreadsheet");
  const sheet = container.querySelector<HTMLElement>(".x-spreadsheet-sheet");
  if (!root || !sheet) {
    return;
  }

  const width = sheet.getBoundingClientRect().width || sheet.clientWidth;
  if (width <= 0) {
    return;
  }

  root.style.width = `${Math.round(width)}px`;
  root.style.maxWidth = "100%";
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
