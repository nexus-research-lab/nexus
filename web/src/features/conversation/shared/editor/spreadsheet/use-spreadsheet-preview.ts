import { useCallback, useEffect, useState } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";

import {
  workbookToSpreadsheetPreviewData,
  type SpreadsheetPreviewWorkbookData,
} from "./spreadsheet-preview-model";

export type SpreadsheetPreviewStatus =
  | { state: "loading" }
  | { state: "loaded"; sheetCount: number }
  | { state: "error" };

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
  }, previewKey);
  const [retryRevision, setRetryRevision] = useState(0);
  const retryPreview = useCallback(() => {
    setRetryRevision((current) => current + 1);
  }, []);

  useEffect(() => {
    const abortController = new AbortController();
    let active = true;
    setStatus({ state: "loading" });
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
        setStatus({ state: "loading" });
        const nextWorkbook = await parseSpreadsheetBuffer(buffer);
        if (!active) {
          return;
        }
        setWorkbook(nextWorkbook);
        setStatus({
          state: "loaded",
          sheetCount: nextWorkbook.sheets.length,
        });
      } catch {
        if (!active || abortController.signal.aborted) {
          return;
        }
        setWorkbook(null);
        setStatus({
          state: "error",
        });
      }
    };
    void loadPreview();
    return () => {
      active = false;
      abortController.abort();
    };
  }, [agentId, path, retryRevision, setStatus, setWorkbook]);

  return {
    activeSheetIndex,
    setActiveSheetIndex,
    retryPreview,
    status,
    workbook,
  };
}
