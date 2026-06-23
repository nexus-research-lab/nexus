"use client";

import { useEffect, useRef, useState } from "react";
import { Eye, FileSpreadsheet, FileWarning, LoaderCircle } from "lucide-react";
import "x-data-spreadsheet/dist/xspreadsheet.css";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import { workbook_to_spreadsheet_preview_data } from "./spreadsheet-preview-model";
import {
  mount_spreadsheet,
  resolve_spreadsheet_entrypoint,
} from "./spreadsheet-preview-runtime";
import { spreadsheet_preview_to_x_spreadsheet_data } from "./spreadsheet-x-data-adapter";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_XLSX_PREVIEW_BYTES = 15 * 1024 * 1024;

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
