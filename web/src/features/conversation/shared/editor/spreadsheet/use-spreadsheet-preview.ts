import { useEffect } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent-manage-api";

import {
  workbookToSpreadsheetPreviewData,
  type SpreadsheetPreviewWorkbookData,
} from "./spreadsheet-preview-model";

const MAX_XLSX_PREVIEW_BYTES = 15 * 1024 * 1024;

export type SpreadsheetPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; sheetCount: number }
  | { state: "error"; message: string };

function assertPreviewSize(byteLength: number): void {
  if (byteLength > MAX_XLSX_PREVIEW_BYTES) {
    throw new Error(
      "文件超过 15MB，当前无法内置预览，请使用上方按钮处理",
    );
  }
}

async function fetchSpreadsheetBuffer(
  agentId: string,
  path: string,
  signal: AbortSignal,
): Promise<ArrayBuffer> {
  const response = await fetch(getWorkspaceFilePreviewUrl(agentId, path), {
    credentials: "include",
    signal,
  });
  if (!response.ok) {
    throw new Error(`读取文件失败：HTTP ${response.status}`);
  }
  assertPreviewSize(Number(response.headers.get("content-length") || 0));
  const buffer = await response.arrayBuffer();
  assertPreviewSize(buffer.byteLength);
  return buffer;
}

async function parseSpreadsheetBuffer(
  buffer: ArrayBuffer,
): Promise<SpreadsheetPreviewWorkbookData> {
  const ExcelJS = await import("exceljs");
  const workbook = new ExcelJS.Workbook();
  await workbook.xlsx.load(buffer);
  const preview = workbookToSpreadsheetPreviewData(workbook);
  if (preview.sheets.length === 0) {
    throw new Error("未找到可预览的工作表");
  }
  return preview;
}

export function useSpreadsheetPreview(agentId: string, path: string) {
  const previewKey = `${agentId}\x1f${path}`;
  const [workbook, setWorkbook] = useResettableState<
    SpreadsheetPreviewWorkbookData | null
  >(null, previewKey);
  const [activeSheetIndex, setActiveSheetIndex] = useResettableState(
    0,
    previewKey,
  );
  const [status, setStatus] = useResettableState<SpreadsheetPreviewStatus>({
    state: "loading",
    message: "加载表格预览中",
  }, previewKey);

  useEffect(() => {
    const abortController = new AbortController();
    let active = true;
    const loadPreview = async (): Promise<void> => {
      try {
        const buffer = await fetchSpreadsheetBuffer(
          agentId,
          path,
          abortController.signal,
        );
        if (!active) {
          return;
        }
        setStatus({ state: "loading", message: "解析 workbook 中" });
        const nextWorkbook = await parseSpreadsheetBuffer(buffer);
        if (!active) {
          return;
        }
        setWorkbook(nextWorkbook);
        setStatus({
          state: "loaded",
          sheetCount: nextWorkbook.sheets.length,
        });
      } catch (error) {
        if (!active || abortController.signal.aborted) {
          return;
        }
        setWorkbook(null);
        setStatus({
          state: "error",
          message: error instanceof Error ? error.message : "xlsx 预览失败",
        });
      }
    };
    void loadPreview();
    return () => {
      active = false;
      abortController.abort();
    };
  }, [agentId, path, setStatus, setWorkbook]);

  return {
    activeSheetIndex,
    setActiveSheetIndex,
    status,
    workbook,
  };
}
