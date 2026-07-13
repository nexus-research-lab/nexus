import { useEffect } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";

import {
  workbookToSpreadsheetPreviewData,
  type SpreadsheetPreviewWorkbookData,
} from "./spreadsheet-preview-model";

export type SpreadsheetPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; sheetCount: number }
  | { state: "error"; message: string };

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
        const buffer = await fetchOfficePreviewBuffer({
          agentId,
          fileLabel: "xlsx",
          path,
          signal: abortController.signal,
        });
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
