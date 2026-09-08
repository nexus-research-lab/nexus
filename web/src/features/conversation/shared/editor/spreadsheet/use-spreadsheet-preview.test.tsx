// INPUT: Deferred workbook reads/decoding and real Office/controller scope transitions.
// OUTPUT: Only the current file/owner publishes data; retry and sheet selection retain their contracts.
// POS: Offline lifecycle regression; decoding/projection fixtures do not assert Excel rendering fidelity.
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import { useSpreadsheetPreview } from "./use-spreadsheet-preview";

const { decode } = vi.hoisted(() => ({ decode: vi.fn<(buffer: ArrayBuffer) => Promise<void>>() }));
vi.mock("../office-preview-resource", () => ({ fetchOfficePreviewBuffer: vi.fn() }));
vi.mock("exceljs", () => ({ Workbook: class {
  source = 0;
  xlsx = { load: async (buffer: ArrayBuffer) => {
    this.source = new Uint8Array(buffer)[0];
    await decode(buffer);
  } };
} }));
vi.mock("./spreadsheet-preview-model", () => ({
  workbookToSpreadsheetPreviewData: (workbook: { source: number }) => ({
    sheets: [0, 1].map((index) => ({
      column_count: 1, columns: {}, merges: [], name: `File ${workbook.source} sheet ${index}`,
      row_count: 1, rows: {}, styles: [],
    })),
  }),
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
const buffer = (source: number) => new Uint8Array([source]).buffer;
beforeEach(() => {
  vi.mocked(fetchOfficePreviewBuffer).mockReset();
  decode.mockReset().mockResolvedValue(undefined);
});
afterEach(cleanup);

it.each(["resolve", "reject"] as const)("ignores a stale parse %s after leaving and reopening the same file", async (outcome) => {
  const oldParse = deferred<void>();
  const nextRead = deferred<ArrayBuffer>();
  decode.mockReturnValueOnce(oldParse.promise);
  vi.mocked(fetchOfficePreviewBuffer).mockResolvedValueOnce(buffer(1))
    .mockReturnValueOnce(nextRead.promise).mockResolvedValueOnce(buffer(3));
  const view = renderHook(({ path }) => useSpreadsheetPreview("agent", path), { initialProps: { path: "a.xlsx" } });
  await waitFor(() => expect(decode).toHaveBeenCalledTimes(1));
  view.rerender({ path: "b.xlsx" });
  view.rerender({ path: "a.xlsx" });
  await waitFor(() => expect(view.result.current.workbook?.sheets[0].name).toBe("File 3 sheet 0"));
  await act(async () => {
    if (outcome === "resolve") oldParse.resolve(); else oldParse.reject(new Error("old parse"));
    nextRead.resolve(buffer(2));
  });
  expect(view.result.current.status.state).toBe("loaded");
  expect(view.result.current.workbook?.sheets[0].name).toBe("File 3 sheet 0");
  expect(decode).toHaveBeenCalledTimes(2);
  expect(vi.mocked(fetchOfficePreviewBuffer).mock.calls[0][0].signal.aborted).toBe(true);
  expect(vi.mocked(fetchOfficePreviewBuffer).mock.calls[1][0].signal.aborted).toBe(true);
});

it("rejects an unpublished owner transition and clears the previous owner's sheet selection", async () => {
  const oldParse = deferred<void>();
  decode.mockReturnValueOnce(oldParse.promise);
  vi.mocked(fetchOfficePreviewBuffer).mockResolvedValueOnce(buffer(1)).mockResolvedValueOnce(buffer(2));
  const view = renderHook(() => useSpreadsheetPreview("agent", "same.xlsx"));
  await waitFor(() => expect(decode).toHaveBeenCalledTimes(1));
  advanceAuthOwnerScopeGeneration();
  await act(async () => oldParse.resolve());
  expect(view.result.current.workbook).toBeNull();
  expect(view.result.current.status.state).toBe("loading");
  act(() => publishAuthOwnerScopeGeneration());
  await waitFor(() => expect(view.result.current.workbook?.sheets[0].name).toBe("File 2 sheet 0"));
  act(() => view.result.current.setActiveSheetIndex(1));
  expect(view.result.current.activeSheetIndex).toBe(1);
  const ownerRead = deferred<ArrayBuffer>();
  vi.mocked(fetchOfficePreviewBuffer).mockReturnValueOnce(ownerRead.promise);
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  expect(view.result.current.activeSheetIndex).toBe(0);
  expect(view.result.current.workbook).toBeNull();
  await act(async () => ownerRead.resolve(buffer(3)));
  await waitFor(() => expect(view.result.current.workbook?.sheets[0].name).toBe("File 3 sheet 0"));
});

it("retries a failed read explicitly and aborts a pending retry on unmount", async () => {
  vi.mocked(fetchOfficePreviewBuffer).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(buffer(2));
  const view = renderHook(() => useSpreadsheetPreview("agent", "retry.xlsx"));
  await waitFor(() => expect(view.result.current.status.state).toBe("error"));
  act(() => view.result.current.retryPreview());
  expect(view.result.current.status.state).toBe("loading");
  await waitFor(() => expect(view.result.current.workbook?.sheets[0].name).toBe("File 2 sheet 0"));
  const pending = deferred<ArrayBuffer>();
  vi.mocked(fetchOfficePreviewBuffer).mockReturnValueOnce(pending.promise);
  act(() => view.result.current.retryPreview());
  view.unmount();
  expect(vi.mocked(fetchOfficePreviewBuffer).mock.calls[2][0].signal.aborted).toBe(true);
  await act(async () => pending.resolve(buffer(3)));
  expect(decode).toHaveBeenCalledTimes(1);
});
