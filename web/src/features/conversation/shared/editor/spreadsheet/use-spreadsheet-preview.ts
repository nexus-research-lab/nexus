// INPUT: Exact workbook scope and explicit retry.
// OUTPUT: Current workbook/count/loading facts; selection stays scoped to the file identity.
// POS: Spreadsheet read/parse lifecycle; stale completions cannot publish another file or owner data.
import { useEffect } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import { useOfficePreviewScope } from "../use-office-preview-scope";

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
  const { scopeKey, requestKey, isCurrent, retryPreview } = useOfficePreviewScope(agentId, path);
  const [workbook, setWorkbook] = useResettableState<
    SpreadsheetPreviewWorkbookData | null
  >(null, scopeKey);
  const [activeSheetIndex, setActiveSheetIndex] = useResettableState(
    0,
    scopeKey,
  );
  const [status, setStatus] = useResettableState<SpreadsheetPreviewStatus>({
    state: "loading",
  }, requestKey);
  useEffect(() => {
    if (!isCurrent()) return;
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
        if (!active || !isCurrent()) {
          return;
        }
        const nextWorkbook = await parseSpreadsheetBuffer(buffer);
        if (!active || !isCurrent()) {
          return;
        }
        setWorkbook(nextWorkbook);
        setStatus({
          state: "loaded",
          sheetCount: nextWorkbook.sheets.length,
        });
      } catch {
        if (!active || !isCurrent() || abortController.signal.aborted) {
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
  }, [agentId, isCurrent, path, setStatus, setWorkbook]);

  return {
    activeSheetIndex,
    setActiveSheetIndex,
    retryPreview,
    status,
    workbook,
  };
}
